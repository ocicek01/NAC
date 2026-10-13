<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use RuntimeException;

// The panel and Go inventories have different primary keys. Only match a real
// ifIndex (or an exact name for an unindexed legacy port), never a guessed suffix.
class ObserverPortStatus
{
    public function __construct(protected NacApiClient $api) {}

    public function resolve(NetworkSwitch $switch): string
    {
        $remote = $this->api->resolveSwitch(null, $switch->ip_address);
        if (empty($remote['id'])) {
            throw new RuntimeException('Observer switch not found');
        }
        return (string) $remote['id'];
    }

    public function snapshot(NetworkSwitch $switch): array
    {
        $remoteId = $this->resolve($switch);
        return array_values(array_filter(array_map(
            fn (array $port) => $this->map($switch, $port),
            $this->api->switchPorts($remoteId)
        )));
    }

    public function feed(NetworkSwitch $switch, string $remoteId, ?string $cursor): array
    {
        $feed = $this->api->observerPortEvents($remoteId, $cursor);
        $items = [];
        foreach ($feed['events'] ?? [] as $event) {
            $current = $event['metadata']['current'] ?? [];
            $mapped = $this->map($switch, array_merge($current, [
                'if_index' => $event['if_index'],
                'interface_name' => $event['interface_name'] ?? '',
                'last_changed_at' => $event['observed_at'] ?? null,
            ]));
            if ($mapped !== null) {
                $items[] = $mapped;
            }
        }
        return ['cursor' => (string) $feed['cursor'], 'ports' => $items];
    }

    public function map(NetworkSwitch $switch, array $remote): ?array
    {
        if (array_key_exists('is_physical', $remote) && ! (bool) $remote['is_physical']) {
            return null;
        }
        $switch->loadMissing('ports');
        $index = (int) ($remote['if_index'] ?? 0);
        if ($index <= 0) {
            return null;
        }
        $port = $switch->ports->first(fn (SwitchPort $p) => (int) $p->if_index === $index);
        if (! $port) {
            $name = (string) ($remote['interface_name'] ?? '');
            $port = $switch->ports->first(fn (SwitchPort $p) => ! $p->if_index && $name !== '' && $p->port_name === $name);
        }
        if (! $port) {
            return null;
        }
        $macs = array_values((array) ($remote['mac_addresses'] ?? []));
        return [
            'switch_id' => $switch->id,
            'port_id' => $port->id,
            'port_no' => $port->port_index,
            'if_index' => $index,
            'if_name' => $remote['interface_name'] ?? $port->port_name,
            'admin_status' => $remote['admin_status'] ?? 'unknown',
            'oper_status' => $remote['oper_status'] ?? 'unknown',
            'vlan_id' => (int) ($remote['vlan_id'] ?? 0),
            'mac_addresses' => $macs,
            'mac_count' => count($macs),
            'last_change' => $remote['last_changed_at'] ?? null,
            'last_seen' => $remote['last_discovered_at'] ?? $remote['last_changed_at'] ?? null,
            'status_source' => 'port-observer',
        ];
    }
}
