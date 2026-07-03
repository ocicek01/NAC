<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use Illuminate\Support\Arr;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Str;

class PortStatusUpdater
{
    public function __construct(
        protected AuditLogService $auditLogService,
    ) {
    }

    public function updatePortStatus(
        NetworkSwitch $switch,
        int $ifIndex,
        ?string $ifName,
        ?string $ifDescr,
        string $adminStatus,
        string $operStatus,
        ?string $speed,
        string $source,
        ?array $rawStatus = null,
        ?Carbon $seenAt = null,
    ): SwitchPort {
        $seenAt ??= now();

        $port = SwitchPort::query()->firstOrNew([
            'switch_id' => $switch->id,
            'if_index' => $ifIndex,
        ]);

        $previousAdmin = (string) ($port->admin_status ?? 'unknown');
        $previousOper = (string) ($port->oper_status ?? 'unknown');
        $statusChanged = $port->exists
            && ($previousAdmin !== $adminStatus || $previousOper !== $operStatus);

        $lastChange = $statusChanged || ! $port->exists
            ? $seenAt
            : ($port->last_change ?? $port->last_change_at ?? $seenAt);

        $portName = $ifName ?: $port->port_name ?: (string) $ifIndex;
        $portDescription = $ifDescr ?: $port->port_description;
        $portIndex = $port->port_index ?: $this->portIndexFromName($portName, $ifIndex);

        $port->fill([
            'switch_id' => $switch->id,
            'if_index' => $ifIndex,
            'if_name' => $ifName,
            'if_descr' => $ifDescr,
            'port_index' => $portIndex,
            'port_name' => $portName,
            'port_description' => $portDescription,
            'status' => $this->mapLegacyStatus($adminStatus, $operStatus),
            'admin_status' => $adminStatus,
            'oper_status' => $operStatus,
            'speed' => $speed ?: ($port->speed ?: '0'),
            'status_source' => $source,
            'raw_status' => $rawStatus,
            'last_seen' => $seenAt,
            'last_change' => $lastChange,
            'last_change_at' => $lastChange,
            'last_discovered_at' => $port->last_discovered_at ?: $seenAt,
        ]);
        $port->save();

        if ($statusChanged) {
            $event = [
                'id' => (string) Str::uuid(),
                'type' => 'port_status_changed',
                'switch_id' => $switch->id,
                'switch_name' => $switch->hostname,
                'port_id' => $port->id,
                'port_no' => $port->port_index,
                'if_index' => $ifIndex,
                'if_name' => $port->if_name ?: $port->port_name,
                'old_oper_status' => $previousOper,
                'new_oper_status' => $operStatus,
                'old_admin_status' => $previousAdmin,
                'admin_status' => $adminStatus,
                'changed_at' => $lastChange->toIso8601String(),
                'last_seen' => $seenAt->toIso8601String(),
                'source' => $source,
            ];

            $this->auditLogService->log('switch_port_status_changed', 'switch_port', $port->id, [
                'switch_id' => $switch->id,
                'switch_port_id' => $port->id,
                'old_value' => [
                    'admin_status' => $previousAdmin,
                    'oper_status' => $previousOper,
                ],
                'new_value' => [
                    'admin_status' => $adminStatus,
                    'oper_status' => $operStatus,
                    'source' => $source,
                    'message' => sprintf('Port %s changed from %s to %s on switch %s', $port->port_name, $previousOper, $operStatus, $switch->hostname),
                ],
            ]);

            Log::info(sprintf('Port %s changed from %s to %s on switch %s', $port->port_name, $previousOper, $operStatus, $switch->hostname), [
                'switch_id' => $switch->id,
                'switch_port_id' => $port->id,
                'if_index' => $ifIndex,
                'source' => $source,
            ]);

            $this->pushEvent($event);
        }

        return $port;
    }

    protected function mapLegacyStatus(string $adminStatus, string $operStatus): string
    {
        if ($adminStatus === 'down') {
            return 'disabled';
        }

        if ($operStatus === 'up') {
            return 'up';
        }

        if ($operStatus === 'down') {
            return 'down';
        }

        return 'down';
    }

    protected function portIndexFromName(string $portName, int $fallback): int
    {
        if (preg_match('/(\d+)\s*$/', $portName, $matches) === 1) {
            return (int) $matches[1];
        }

        return $fallback;
    }

    protected function pushEvent(array $event): void
    {
        $cacheKey = (string) config('services.switch_port_status.event_cache_key', 'switch_port_status_events');
        $limit = max(1, (int) config('services.switch_port_status.event_cache_limit', 200));
        $ttl = max(60, (int) config('services.switch_port_status.event_cache_ttl_seconds', 300));
        $events = Cache::get($cacheKey, []);

        if (! is_array($events)) {
            $events = [];
        }

        $events[] = $event;
        if (count($events) > $limit) {
            $events = array_slice($events, -1 * $limit);
        }

        Cache::put($cacheKey, $events, now()->addSeconds($ttl));
    }

    public function eventsAfter(?string $eventId): array
    {
        $cacheKey = (string) config('services.switch_port_status.event_cache_key', 'switch_port_status_events');
        $events = Cache::get($cacheKey, []);

        if (! is_array($events) || $events === []) {
            return [];
        }

        if (blank($eventId)) {
            return $events;
        }

        $offset = null;
        foreach ($events as $index => $event) {
            if (($event['id'] ?? null) === $eventId) {
                $offset = $index;
            }
        }

        return $offset === null ? $events : array_slice($events, $offset + 1);
    }
}
