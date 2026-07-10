<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\SwitchPort;
use App\Services\AuditLogService;
use App\Services\NacApiClient;
use App\Services\PortNeighborDiscoveryService;
use App\Services\SwitchStatsService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Validator;
use Illuminate\Validation\Rule;

class SwitchPortController extends Controller
{
    public function __construct(
        protected SwitchStatsService $switchStatsService,
        protected AuditLogService $auditLogService,
        protected PortNeighborDiscoveryService $portNeighborDiscoveryService,
        protected NacApiClient $nacApiClient
    ) {
    }

    public function show(SwitchPort $port): JsonResponse
    {
        $port->loadMissing(['switch', 'currentLocation.endpoint']);

        return response()->json([
            'data' => $this->switchStatsService->portDetail($port),
        ]);
    }

    public function refreshLive(Request $request, SwitchPort $port): JsonResponse
    {
        $port->loadMissing('switch');
        if (! $port->switch) {
            return response()->json([
                'message' => 'Port switch kaydi bulunamadi.',
            ], 422);
        }

        $resolved = $this->nacApiClient->resolveSwitch($port->switch->hostname, $port->switch->ip_address);
        if (! is_array($resolved) || blank($resolved['id'] ?? null)) {
            return response()->json([
                'message' => 'Go switch kaydi bulunamadi.',
            ], 422);
        }

        $lookupIfIndex = (int) ($port->if_index ?: $port->port_index ?: 0);
        if ($lookupIfIndex <= 0 && preg_match('/(\d+)\s*$/', (string) $port->port_name, $matches) === 1) {
            $lookupIfIndex = (int) ($matches[1] ?? 0);
        }
        if ($lookupIfIndex <= 0) {
            return response()->json([
                'message' => 'Port lookup index bulunamadi.',
            ], 422);
        }

        $result = $this->nacApiClient->refreshSwitchPortSnapshot((string) $resolved['id'], $lookupIfIndex);

        $this->auditLogService->log('switch_port_live_refresh_requested', 'switch_port', $port->id, array_merge(
            $this->auditLogService->contextFromRequest($request),
            [
                'switch_id' => $port->switch_id,
                'switch_port_id' => $port->id,
                'new_value' => [
                    'go_switch_id' => $resolved['id'] ?? null,
                    'lookup_if_index' => $lookupIfIndex,
                    'status' => $result['status'] ?? 'queued',
                ],
            ]
        ));

        return response()->json([
            'message' => 'Canli port yenilemesi kuyruga alindi.',
            'data' => [
                'switch_id' => $port->switch_id,
                'switch_port_id' => $port->id,
                'lookup_if_index' => $lookupIfIndex,
                'status' => $result['status'] ?? 'queued',
            ],
        ], 202);
    }
    public function updateNacMode(Request $request, SwitchPort $port): JsonResponse
    {
        $validated = Validator::make($request->all(), [
            'nac_mode' => ['required', Rule::in(['inherit', 'disabled', 'monitor', 'enforcement'])],
        ])->validate();

        $old = $port->nac_mode;
        $port->update(['nac_mode' => $validated['nac_mode']]);

        $this->auditLogService->log('switch_port_nac_mode_updated', 'switch_port', $port->id, array_merge(
            $this->auditLogService->contextFromRequest($request),
            [
                'switch_id' => $port->switch_id,
                'switch_port_id' => $port->id,
                'old_value' => ['nac_mode' => $old],
                'new_value' => ['nac_mode' => $validated['nac_mode']],
            ]
        ));

        return response()->json([
            'message' => 'Port NAC modu guncellendi.',
            'data' => [
                'id' => $port->id,
                'nac_mode' => $port->nac_mode,
            ],
        ]);
    }

    public function lldp(SwitchPort $port): JsonResponse
    {
        $port->loadMissing('switch');

        return response()->json([
            'data' => $this->portNeighborDiscoveryService->discover($port),
        ]);
    }

    public function rediscover(Request $request, SwitchPort $port): JsonResponse
    {
        $port->loadMissing('switch');
        if (! $port->switch) {
            return response()->json([
                'message' => 'Port switch kaydi bulunamadi.',
            ], 422);
        }

        $resolved = $this->nacApiClient->resolveSwitch($port->switch->hostname, $port->switch->ip_address);
        if (! is_array($resolved) || blank($resolved['id'] ?? null)) {
            return response()->json([
                'message' => 'Go switch kaydi bulunamadi.',
            ], 422);
        }

        $job = $this->nacApiClient->createDiscoveryJob([
            'switch_id' => (string) $resolved['id'],
            'scope' => 'full',
            'requested_source' => 'panel-port',
            'requested_by' => (string) optional($request->user())->id,
        ]);

        $dispatched = $this->nacApiClient->dispatchDiscoveryJob((string) ($job['id'] ?? ''), 'panel-web');

        $this->auditLogService->log('switch_port_rediscovery_requested', 'switch_port', $port->id, array_merge(
            $this->auditLogService->contextFromRequest($request),
            [
                'switch_id' => $port->switch_id,
                'switch_port_id' => $port->id,
                'new_value' => [
                    'go_switch_id' => $resolved['id'] ?? null,
                    'job_id' => $dispatched['id'] ?? ($job['id'] ?? null),
                    'scope' => $dispatched['scope'] ?? ($job['scope'] ?? null),
                    'status' => $dispatched['status'] ?? ($job['status'] ?? null),
                ],
            ]
        ));

        return response()->json([
            'message' => 'Port taramasi baslatildi.',
            'data' => [
                'job' => $dispatched,
                'go_switch_id' => $resolved['id'] ?? null,
                'switch_id' => $port->switch_id,
                'selected_port_id' => $port->id,
            ],
        ], 202);
    }
}


