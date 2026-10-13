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

    public function test_port_payload_falls_back_to_go_device_when_endpoint_fields_are_blank(): void
    {
        $goPorts = [
            'port-index:7' => [
                'port_index' => 7,
                'interface_name' => 'Gi1/0/7',
                'mac_addresses' => ['aa:bb:cc:dd:ee:ff'],
            ],
        ];

        $goDevices = [
            'port-index:7' => [[
                'id' => 'device-7',
                'current_if_index' => 7,
                'hostname' => 'lab-pc-07',
                'current_ip_address' => '10.10.7.15',
                'mac_address' => 'aa:bb:cc:dd:ee:ff',
                'device_type' => 'workstation',
                'identity_username' => 'lab.user',
                'identity_type' => 'personel',
                'policy_action' => 'active',
                'identity_source' => 'ldap',
                'last_enforcement_method' => 'radius-coa',
                'last_seen_at' => '2026-07-07T10:00:00Z',
            ]],
        ];

        $service = new SwitchStatsServiceTestDouble([], $goPorts, [], $goDevices);

        $port = $this->makePort('Gi1/0/7', 7, 'access');
        $port->setRelation('switch', new NetworkSwitch(['model' => 'C9300-48P']));
        $endpoint = $port->getRelation('currentLocation')->getRelation('endpoint');
        $endpoint->forceFill([
            'mac_address' => '',
            'ip_address' => '',
            'hostname' => '',
            'user_name' => '',
            'device_type' => '',
            'policy_name' => '',
            'role_name' => '',
        ]);

        $payload = $service->portPayload($port, [], $goPorts, $goDevices);

        $this->assertSame('lab.user', $payload['user']);
        $this->assertSame('aa:bb:cc:dd:ee:ff', $payload['mac']);
        $this->assertSame('10.10.7.15', $payload['ip']);
        $this->assertSame('lab-pc-07', $payload['hostname']);
        $this->assertSame('workstation', $payload['deviceType']);
        $this->assertSame('ACTIVE', $payload['policyText']);
        $this->assertSame('Personel', $payload['role']);
        $this->assertSame('LDAP', $payload['identitySource']);
        $this->assertSame('RADIUS CoA', $payload['enforcementMethod']);
    }

    public function test_port_payload_does_not_mix_stale_endpoint_with_observed_mac(): void
    {
        $goPorts = [
            'port-index:5' => [
                'port_index' => 5,
                'interface_name' => '5',
                'mac_addresses' => ['30:9C:23:9B:97:AA'],
            ],
        ];
        $goDevices = [
            'port-index:5' => [[
                'id' => 'observed-device-5',
                'current_if_index' => 5,
                'mac_address' => '30:9C:23:9B:97:AA',
                'current_ip_address' => '10.6.9.93/32',
                'hostname' => 'omer_cicek',
                'last_seen_at' => '2026-10-13T12:23:14Z',
            ]],
        ];

        $service = new SwitchStatsServiceTestDouble([], $goPorts, [], $goDevices);
        $port = $this->makePort('5', 5, 'access');
        $port->setRelation('switch', new NetworkSwitch(['model' => 'HP J9775A 2530-48G']));
        $staleEndpoint = $port->getRelation('currentLocation')->getRelation('endpoint');
        $staleEndpoint->forceFill([
            'mac_address' => '00:08:D1:04:EB:FB',
            'ip_address' => '10.6.9.205',
            'hostname' => 'stale-device',
            'user_name' => 'stale.user',
        ]);

        $payload = $service->portPayload($port, [], $goPorts, $goDevices);

        $this->assertSame('30:9C:23:9B:97:AA', $payload['mac']);
        $this->assertSame('10.6.9.93', $payload['ip']);
        $this->assertSame('omer_cicek', $payload['hostname']);
        $this->assertSame('-', $payload['user']);
    }

    public function test_infrastructure_port_detection_uses_flags_and_mac_density(): void
    {
        $service = new SwitchStatsServiceTestDouble();

        $this->assertTrue($service->isInfrastructure(['is_uplink' => true, 'mac_count' => 1]));
        $this->assertTrue($service->isInfrastructure(['is_trunk' => true, 'mac_count' => 1]));
        $this->assertTrue($service->isInfrastructure(['mac_count' => 8]));
        $this->assertFalse($service->isInfrastructure(['mac_count' => 1]));
    }

    public function test_uplink_payload_does_not_present_one_mac_as_connected_endpoint(): void
    {
        $goPorts = [
            'port-index:47' => [
                'port_index' => 47,
                'is_uplink' => true,
                'mac_count' => 2,
                'mac_addresses' => ['AA:BB:CC:DD:EE:01', 'AA:BB:CC:DD:EE:02'],
            ],
        ];
        $service = new SwitchStatsServiceTestDouble([], $goPorts);
        $port = $this->makePort('47', 47, 'access');
        $port->setRelation('switch', new NetworkSwitch(['model' => 'HP J9775A 2530-48G']));

        $payload = $service->portPayload($port, [], $goPorts);

        $this->assertTrue($payload['isUplink']);
        $this->assertSame(2, $payload['macCount']);
        $this->assertSame('-', $payload['mac']);
        $this->assertSame('-', $payload['ip']);
        $this->assertSame('-', $payload['user']);
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
    public function __construct(private array $topologyLinks = [], private array $goPorts = [], private array $goSummary = [], private array $goDevices = [])
    {
        parent::__construct();
    }

    public function buildTopologyIndex(string $localPortName, array $payload): array
    {
        return $this->storeTopologyLink([], $localPortName, $payload);
    }

    public function isInfrastructure(?array $goPort): bool
    {
        return $this->isInfrastructurePort($goPort);
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

    protected function goDevicesForSwitch(NetworkSwitch $switch): array
    {
        return $this->goDevices;
    }

    protected function eventEntries(NetworkSwitch $switch): array
    {
        return [];
    }
}


