<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Services\AuditLogService;
use App\Services\NacApiClient;
use App\Services\SwitchStatsService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Validator;
use Illuminate\Validation\Rule;

class SwitchController extends Controller
{
    public function __construct(
        protected SwitchStatsService $switchStatsService,
        protected AuditLogService $auditLogService,
        protected NacApiClient $nacApiClient
    ) {
    }

    public function index(): JsonResponse
    {
        $switches = NetworkSwitch::query()
            ->with(['zone', 'ports.currentLocation.endpoint'])
            ->orderBy('hostname')
            ->get()
            ->map(fn (NetworkSwitch $switch) => $this->switchStatsService->listItem($switch))
            ->values();

        return response()->json(['data' => $switches]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = Validator::make($request->all(), [
            'zone_id' => ['required', 'exists:zones,id'],
            'hostname' => ['required', 'string', 'max:255', 'unique:switches,hostname'],
            'ip_address' => ['required', 'ip', 'unique:switches,ip_address'],
            'vendor' => ['required', 'string', 'max:255'],
            'model' => ['required', 'string', 'max:255'],
            'location' => ['nullable', 'string', 'max:255'],
            'status' => ['required', Rule::in(['online', 'offline', 'warning', 'unmanaged'])],
            'managed' => ['required', 'boolean'],
            'nac_mode' => ['required', Rule::in(['disabled', 'monitor', 'enforcement'])],
            'port_count' => ['required', 'integer', 'between:1,128'],
        ])->validate();

        $switch = DB::transaction(function () use ($validated, $request) {
            $switch = NetworkSwitch::create(array_merge($validated, [
                'last_seen_at' => now(),
            ]));

            for ($index = 1; $index <= $switch->port_count; $index++) {
                SwitchPort::create([
                    'switch_id' => $switch->id,
                    'port_index' => $index,
                    'port_name' => 'Gi1/0/'.$index,
                    'status' => $index <= 2 ? 'up' : 'down',
                    'port_type' => $index === 1 ? 'trunk' : ($index === 2 ? 'uplink' : 'access'),
                    'nac_mode' => 'inherit',
                    'vlan_id' => 1,
                    'speed' => $index <= 2 ? '1 Gbps' : '0',
                    'duplex' => 'Full',
                    'poe_enabled' => false,
                    'poe_power' => 0,
                    'last_change_at' => now(),
                    'last_change' => now(),
                    'last_seen' => now(),
                ]);
            }

            $this->auditLogService->log('switch_created', 'switch', $switch->id, array_merge(
                $this->auditLogService->contextFromRequest($request),
                [
                    'switch_id' => $switch->id,
                    'new_value' => $validated,
                ]
            ));

            return $switch;
        });

        $switch->load(['zone', 'ports.currentLocation.endpoint']);

        return response()->json([
            'message' => 'Switch kaydi basariyla olusturuldu.',
            'data' => $this->switchStatsService->detail($switch),
        ], 201);
    }

    public function show(NetworkSwitch $switch): JsonResponse
    {
        $switch->loadMissing(['zone', 'ports.currentLocation.endpoint']);

        return response()->json($this->switchStatsService->detail($switch));
    }

    public function update(Request $request, NetworkSwitch $switch): JsonResponse
    {
        $validated = Validator::make($request->all(), [
            'zone_id' => ['sometimes', 'exists:zones,id'],
            'hostname' => ['sometimes', 'string', 'max:255', Rule::unique('switches', 'hostname')->ignore($switch->id)],
            'ip_address' => ['sometimes', 'ip', Rule::unique('switches', 'ip_address')->ignore($switch->id)],
            'vendor' => ['sometimes', 'string', 'max:255'],
            'model' => ['sometimes', 'string', 'max:255'],
            'location' => ['nullable', 'string', 'max:255'],
            'status' => ['sometimes', Rule::in(['online', 'offline', 'warning', 'unmanaged'])],
            'managed' => ['sometimes', 'boolean'],
            'nac_mode' => ['sometimes', Rule::in(['disabled', 'monitor', 'enforcement'])],
            'port_count' => ['sometimes', 'integer', 'between:1,128'],
        ])->validate();

        $old = $switch->only(array_keys($validated));
        $switch->fill($validated)->save();

        $this->auditLogService->log('switch_updated', 'switch', $switch->id, array_merge(
            $this->auditLogService->contextFromRequest($request),
            [
                'switch_id' => $switch->id,
                'old_value' => $old,
                'new_value' => $validated,
            ]
        ));

        $switch->loadMissing(['zone', 'ports.currentLocation.endpoint']);

        return response()->json([
            'message' => 'Switch guncellendi.',
            'data' => $this->switchStatsService->detail($switch),
        ]);
    }

    public function updateNacMode(Request $request, NetworkSwitch $switch): JsonResponse
    {
        $validated = Validator::make($request->all(), [
            'nac_mode' => ['required', Rule::in(['disabled', 'monitor', 'enforcement'])],
        ])->validate();

        $old = $switch->nac_mode;
        $switch->update(['nac_mode' => $validated['nac_mode']]);

        $this->auditLogService->log('switch_nac_mode_updated', 'switch', $switch->id, array_merge(
            $this->auditLogService->contextFromRequest($request),
            [
                'switch_id' => $switch->id,
                'old_value' => ['nac_mode' => $old],
                'new_value' => ['nac_mode' => $validated['nac_mode']],
            ]
        ));

        return response()->json([
            'message' => 'Switch NAC modu guncellendi.',
            'data' => [
                'id' => $switch->id,
                'nac_mode' => $switch->nac_mode,
            ],
        ]);
    }

    public function ports(NetworkSwitch $switch): JsonResponse
    {
        $switch->loadMissing('ports.currentLocation.endpoint');

        return response()->json([
            'data' => $switch->ports->map(fn (SwitchPort $port) => $this->switchStatsService->portDetail($port))->values(),
        ]);
    }

    public function portsStatus(NetworkSwitch $switch): JsonResponse
    {
        $ports = $switch->ports()
            ->orderBy('port_index')
            ->orderBy('if_index')
            ->get()
            ->map(function (SwitchPort $port) {
                return [
                    'port_id' => $port->id,
                    'port_no' => $port->port_index,
                    'if_index' => $port->if_index,
                    'if_name' => $port->if_name ?: $port->port_name,
                    'if_descr' => $port->if_descr ?: $port->port_description,
                    'admin_status' => $port->admin_status ?: 'unknown',
                    'oper_status' => $port->oper_status ?: 'unknown',
                    'speed' => $port->speed,
                    'status_source' => $port->status_source ?: 'snmp_poll',
                    'last_seen' => optional($port->last_seen)->toIso8601String(),
                    'last_change' => optional($port->last_change ?? $port->last_change_at)->toIso8601String(),
                ];
            })
            ->values();

        $lastUpdate = $ports
            ->pluck('last_seen')
            ->filter()
            ->map(fn (string $value) => Carbon::parse($value))
            ->max();

        return response()->json([
            'data' => $ports,
            'meta' => [
                'switch_id' => $switch->id,
                'last_update' => $lastUpdate?->toIso8601String(),
            ],
        ]);
    }

    public function rediscoverPorts(Request $request, NetworkSwitch $switch): JsonResponse
    {
        $resolved = $this->nacApiClient->resolveSwitch($switch->hostname, $switch->ip_address);
        if (! is_array($resolved) || blank($resolved['id'] ?? null)) {
            return response()->json([
                'message' => 'Go switch kaydi bulunamadi.',
            ], 422);
        }

        $job = $this->nacApiClient->createDiscoveryJob([
            'switch_id' => (string) $resolved['id'],
            'scope' => 'full',
            'requested_source' => 'panel',
            'requested_by' => (string) optional($request->user())->id,
        ]);

        $dispatched = $this->nacApiClient->dispatchDiscoveryJob((string) ($job['id'] ?? ''), 'panel-web');

        $this->auditLogService->log('switch_ports_rediscovery_requested', 'switch', $switch->id, array_merge(
            $this->auditLogService->contextFromRequest($request),
            [
                'switch_id' => $switch->id,
                'new_value' => [
                    'go_switch_id' => $resolved['id'] ?? null,
                    'job_id' => $dispatched['id'] ?? ($job['id'] ?? null),
                    'scope' => $dispatched['scope'] ?? ($job['scope'] ?? null),
                    'status' => $dispatched['status'] ?? ($job['status'] ?? null),
                ],
            ]
        ));

        return response()->json([
            'message' => 'Tum portlar icin tarama baslatildi.',
            'data' => [
                'job' => $dispatched,
                'go_switch_id' => $resolved['id'] ?? null,
                'switch_id' => $switch->id,
            ],
        ], 202);
    }
}
