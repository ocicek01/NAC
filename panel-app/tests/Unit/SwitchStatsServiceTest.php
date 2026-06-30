<?php

namespace Tests\Unit;

use App\Models\Endpoint;
use App\Models\EndpointLocation;
use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Services\SwitchStatsService;
use PHPUnit\Framework\TestCase;

class SwitchStatsServiceTest extends TestCase
{
    public function test_port_payload_marks_linked_port_as_uplink_and_exposes_neighbor_fields(): void
    {
        $service = new SwitchStatsServiceTestDouble();
        $port = $this->makePort('Gi1/0/43', 43, 'access');
        $port->setRelation('switch', new NetworkSwitch([
            'model' => 'C9300-48P',
        ]));
        $topologyLinks = $service->buildTopologyIndex('GigabitEthernet1/0/43', [
            'switch_id' => 'target-switch',
            'switch_name' => 'sw-10-6-8-3',
            'port_name' => 'GigabitEthernet1/1/1',
            'protocol' => 'cdp',
        ]);

        $payload = $service->portPayload($port, $topologyLinks);

        $this->assertSame('Uplink', $payload['portType']);
        $this->assertSame('Uplink', $payload['port_type']);
        $this->assertTrue($payload['isTopologyLinked']);
        $this->assertSame('sw-10-6-8-3', $payload['linkedSwitchName']);
        $this->assertSame('GigabitEthernet1/1/1', $payload['linkedPortName']);
        $this->assertSame('cdp', $payload['linkedProtocol']);
        $this->assertSame('CDP', $payload['uplinkSource']);
    }

    public function test_detail_keeps_numeric_port_topology_neighbor_in_selected_port_view(): void
    {
        $service = new SwitchStatsServiceTestDouble([
            '47' => [
                'switch_id' => 'neighbor-switch',
                'switch_name' => 'sw-10-6-8-12',
                'port_name' => '44',
                'protocol' => 'lldp',
            ],
        ]);

        $switch = new NetworkSwitch([
            'id' => 'hp-switch',
            'hostname' => 'HP-2530-48G',
            'vendor' => 'HP',
            'model' => 'J9775A 2530-48G',
            'status' => 'up',
            'managed' => true,
            'nac_mode' => 'enforce',
            'port_count' => 48,
            'ip_address' => '10.6.8.12',
        ]);
        $switch->setRelation('zone', null);

        $port = $this->makePort('47', 47, 'access');
        $switch->setRelation('ports', collect([$port]));
        $port->setRelation('switch', $switch);

        $detail = $service->detail($switch);
        $selectedPort = $detail['view']['ports'][0];

        $this->assertSame('Uplink', $selectedPort['port_type']);
        $this->assertSame('sw-10-6-8-12', $selectedPort['linkedSwitchName']);
        $this->assertSame('44', $selectedPort['linkedPortName']);
        $this->assertSame('lldp', $selectedPort['linkedProtocol']);
        $this->assertSame($selectedPort['id'], $detail['view']['selectedPort']);
    }

    public function test_port_payload_does_not_mark_unresolved_cdp_neighbor_as_uplink(): void
    {
        $service = new SwitchStatsServiceTestDouble();
        $port = $this->makePort('27', 27, 'access');
        $port->setRelation('switch', new NetworkSwitch([
            'model' => 'J9775A 2530-48G',
        ]));
        $topologyLinks = $service->buildTopologyIndex('27', [
            'switch_id' => '',
            'switch_name' => 'some-client-device',
            'port_name' => 'eth0',
            'protocol' => 'cdp',
        ]);

        $payload = $service->portPayload($port, $topologyLinks);

        $this->assertSame('Access', $payload['portType']);
        $this->assertFalse($payload['isUplink']);
        $this->assertSame('-', $payload['uplinkSource']);
        $this->assertSame('some-client-device', $payload['linkedSwitchName']);
        $this->assertSame('eth0', $payload['linkedPortName']);
        $this->assertSame('cdp', $payload['linkedProtocol']);
    }

    public function test_port_payload_uses_go_switch_port_inventory_for_uplink_and_mac_count(): void
    {
        $service = new SwitchStatsServiceTestDouble([], [
            'port-index:24' => [
                'port_index' => 24,
                'interface_name' => 'GigabitEthernet1/0/24',
                'is_uplink' => true,
                'mac_count' => 171,
                'neighbor_protocol' => '',
                'neighbor_switch_id' => '',
                'neighbor_switch_name' => '',
                'neighbor_port_name' => '',
            ],
        ]);

        $port = $this->makePort('24', 24, 'access');
        $port->setRelation('switch', new NetworkSwitch([
            'model' => 'HP V1910-24G',
        ]));
        $payload = $service->portPayload($port, [], [
            'port-index:24' => [
                'port_index' => 24,
                'interface_name' => 'GigabitEthernet1/0/24',
                'is_uplink' => true,
                'mac_count' => 171,
            ],
        ]);

        $this->assertSame('Uplink', $payload['port_type']);
        $this->assertTrue($payload['isUplink']);
        $this->assertSame(171, $payload['macCount']);
        $this->assertSame('MAC Heuristigi', $payload['uplinkSource']);
    }

    public function test_detail_resolves_go_switch_id_by_hostname_when_local_id_differs(): void
    {
        $switch = new NetworkSwitch([
            'id' => 4021,
            'hostname' => 'sw-10-6-8-18',
            'vendor' => 'HP',
            'model' => 'V1910-24G',
            'status' => 'up',
            'managed' => true,
            'nac_mode' => 'enforce',
            'port_count' => 28,
            'ip_address' => '10.6.8.18',
        ]);
        $switch->setRelation('zone', null);

        $port = $this->makePort('24', 24, 'access');
        $port->setRelation('switch', $switch);
        $switch->setRelation('ports', collect([$port]));

        $service = new SwitchStatsServiceTestDouble([], [
            'port-index:24' => [
                'port_index' => 24,
                'interface_name' => 'GigabitEthernet1/0/24',
                'is_uplink' => true,
                'mac_count' => 171,
            ],
        ], [
            'uplink_ports' => 1,
            'total_learned_macs' => 172,
        ]);

        $detail = $service->detail($switch);
        $selectedPort = $detail['view']['ports'][0];

        $this->assertSame(171, $selectedPort['macCount']);
        $this->assertSame('MAC Heuristigi', $selectedPort['uplinkSource']);
    }

    private function makePort(string $name, int $index, string $type): SwitchPort
    {
        $endpoint = new Endpoint([
            'mac_address' => '00:11:22:33:44:55',
            'ip_address' => '10.0.0.10',
            'hostname' => 'printer-1',
            'user_name' => 'test.user',
            'device_type' => 'printer',
            'policy_name' => 'corp',
            'role_name' => 'employee',
            'status' => 'authorized',
        ]);

        $location = new EndpointLocation();
        $location->setRawAttributes([
            'vlan_id' => 106,
            'first_seen_at' => now()->subHour(),
            'last_seen_at' => now(),
        ], true);
        $location->setRelation('endpoint', $endpoint);

        $port = new SwitchPort([
            'id' => (string) $index,
            'switch_id' => 'switch-1',
            'if_index' => $index,
            'port_index' => $index,
            'port_name' => $name,
            'status' => 'up',
            'port_type' => $type,
            'nac_mode' => 'enforce',
            'vlan_id' => 106,
            'speed' => '1000 Mbps',
            'duplex' => 'Full',
            'poe_enabled' => false,
        ]);
        $port->setRelation('currentLocation', $location);

        return $port;
    }
}

class SwitchStatsServiceTestDouble extends SwitchStatsService
{
    public function __construct(private array $topologyLinks = [], private array $goPorts = [], private array $goSummary = [])
    {
        parent::__construct();
    }

    public function buildTopologyIndex(string $localPortName, array $payload): array
    {
        return $this->storeTopologyLink([], $localPortName, $payload);
    }

    protected function topologyLinksForSwitch(NetworkSwitch $switch): array
    {
        return $this->topologyLinks;
    }

    protected function goPortsForSwitch(NetworkSwitch $switch): array
    {
        return $this->goPorts;
    }

    protected function goPortSummaryForSwitch(NetworkSwitch $switch): array
    {
        return $this->goSummary;
    }

    protected function eventEntries(NetworkSwitch $switch): array
    {
        return [];
    }
}


