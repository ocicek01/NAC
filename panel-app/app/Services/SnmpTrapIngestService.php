<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use Illuminate\Support\Arr;
use Illuminate\Support\Carbon;
use Illuminate\Validation\ValidationException;

class SnmpTrapIngestService
{
    public function __construct(
        protected PortStatusUpdater $portStatusUpdater,
        protected AuditLogService $auditLogService,
    ) {
    }

    public function ingest(array $payload, ?string $requestIp = null): array
    {
        if (! (bool) config('services.nac.trap_ingest_enabled', true)) {
            throw ValidationException::withMessages([
                'trap' => 'SNMP trap ingest devre disi.',
            ]);
        }

        $switch = $this->resolveSwitch($payload, $requestIp);
        $ifIndex = (int) ($payload['if_index'] ?? 0);
        if ($ifIndex <= 0) {
            throw ValidationException::withMessages([
                'if_index' => 'Gecerli if_index gerekli.',
            ]);
        }

        $existingPort = SwitchPort::query()
            ->where('switch_id', $switch->id)
            ->where('if_index', $ifIndex)
            ->first();

        $normalizedAdminStatus = $this->normalizeAdminStatus($payload['admin_status'] ?? null);
        $normalizedOperStatus = $this->normalizeOperStatus(
            $payload['oper_status'] ?? null,
            $payload['trap_type'] ?? ($payload['trap_oid'] ?? null)
        );

        if ($normalizedAdminStatus === 'unknown' && $existingPort?->admin_status) {
            $normalizedAdminStatus = (string) $existingPort->admin_status;
        }

        if ($normalizedOperStatus === 'unknown' && $existingPort?->oper_status) {
            $normalizedOperStatus = (string) $existingPort->oper_status;
        }

        $seenAt = $this->resolveSeenAt($payload['occurred_at'] ?? null);
        $rawStatus = [
            'trap_oid' => $payload['trap_oid'] ?? null,
            'trap_type' => $payload['trap_type'] ?? null,
            'source_ip' => $payload['source_ip'] ?? $requestIp,
            'switch_ip' => $payload['switch_ip'] ?? null,
            'switch_hostname' => $payload['switch_hostname'] ?? null,
            'if_index' => $ifIndex,
            'if_name' => $payload['if_name'] ?? null,
            'if_descr' => $payload['if_descr'] ?? null,
            'admin_status' => $normalizedAdminStatus,
            'oper_status' => $normalizedOperStatus,
            'speed' => $payload['speed'] ?? null,
            'occurred_at' => $seenAt->toIso8601String(),
            'varbinds' => Arr::wrap($payload['varbinds'] ?? []),
        ];

        $port = $this->portStatusUpdater->updatePortStatus(
            $switch,
            $ifIndex,
            $payload['if_name'] ?? $existingPort?->if_name,
            $payload['if_descr'] ?? $existingPort?->if_descr,
            $normalizedAdminStatus,
            $normalizedOperStatus,
            $payload['speed'] ?? $existingPort?->speed,
            'snmp_trap',
            $rawStatus,
            $seenAt,
        );

        $this->auditLogService->log('snmp_trap_received', 'switch_port', $port->id, [
            'switch_id' => $switch->id,
            'switch_port_id' => $port->id,
            'ip_address' => $requestIp,
            'new_value' => [
                'source' => 'snmp_trap',
                'trap_oid' => $payload['trap_oid'] ?? null,
                'trap_type' => $payload['trap_type'] ?? null,
                'if_index' => $ifIndex,
                'admin_status' => $normalizedAdminStatus,
                'oper_status' => $normalizedOperStatus,
            ],
            'created_at' => $seenAt,
        ]);

        return [
            'switch_id' => $switch->id,
            'switch_hostname' => $switch->hostname,
            'switch_port_id' => $port->id,
            'if_index' => $ifIndex,
            'admin_status' => $normalizedAdminStatus,
            'oper_status' => $normalizedOperStatus,
            'last_seen' => optional($port->last_seen)->toIso8601String(),
            'source' => 'snmp_trap',
        ];
    }

    protected function resolveSwitch(array $payload, ?string $requestIp): NetworkSwitch
    {
        $query = NetworkSwitch::query();
        $switchIp = trim((string) ($payload['switch_ip'] ?? ''));
        $sourceIp = trim((string) ($payload['source_ip'] ?? $requestIp ?? ''));
        $hostname = strtolower(trim((string) ($payload['switch_hostname'] ?? '')));

        if ($switchIp !== '') {
            $switch = (clone $query)->where('ip_address', $switchIp)->first();
            if ($switch) {
                return $switch;
            }
        }

        if ($sourceIp !== '') {
            $switch = (clone $query)->where('ip_address', $sourceIp)->first();
            if ($switch) {
                return $switch;
            }
        }

        if ($hostname !== '') {
            $switch = (clone $query)->whereRaw('lower(hostname) = ?', [$hostname])->first();
            if ($switch) {
                return $switch;
            }
        }

        throw ValidationException::withMessages([
            'switch' => 'Trap kaynagi ile eslesen switch bulunamadi.',
        ]);
    }

    protected function normalizeAdminStatus(mixed $value): string
    {
        $normalized = strtolower(trim((string) $value));

        return match ($normalized) {
            '1', 'up', 'enabled', 'enable' => 'up',
            '2', 'down', 'disabled', 'disable', 'admin_down' => 'down',
            default => 'unknown',
        };
    }

    protected function normalizeOperStatus(mixed $value, mixed $trapIndicator = null): string
    {
        $normalized = strtolower(trim((string) $value));
        if (in_array($normalized, ['1', 'up', 'linkup', 'portup'], true)) {
            return 'up';
        }

        if (in_array($normalized, ['2', 'down', 'linkdown', 'portdown'], true)) {
            return 'down';
        }

        $indicator = strtolower(trim((string) $trapIndicator));
        if (str_contains($indicator, 'linkup') || str_contains($indicator, 'portup')) {
            return 'up';
        }

        if (str_contains($indicator, 'linkdown') || str_contains($indicator, 'portdown')) {
            return 'down';
        }

        return 'unknown';
    }

    protected function resolveSeenAt(mixed $value): Carbon
    {
        if (! is_string($value) || trim($value) === '') {
            return now();
        }

        try {
            return Carbon::parse($value);
        } catch (\Throwable) {
            return now();
        }
    }
}
