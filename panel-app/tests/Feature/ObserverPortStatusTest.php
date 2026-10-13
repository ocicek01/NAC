<?php

namespace Tests\Feature;

use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Models\Zone;
use App\Services\NacApiClient;
use App\Services\ObserverPortStatus;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ObserverPortStatusTest extends TestCase
{
    use RefreshDatabase;

    private function inventory(): array
    {
        $zone = Zone::create(['name' => 'Observer', 'slug' => 'observer', 'status' => 'normal']);
        $switch = NetworkSwitch::create([
            'zone_id' => $zone->id, 'hostname' => 'SW-01', 'ip_address' => '10.0.0.1',
            'vendor' => 'Cisco', 'model' => '2960', 'status' => 'online',
            'managed' => true, 'nac_mode' => 'monitor', 'port_count' => 24,
        ]);
        $port = SwitchPort::create([
            'switch_id' => $switch->id, 'if_index' => 10101, 'port_index' => 1,
            'port_name' => 'Gi1/0/1', 'status' => 'up', 'admin_status' => 'up',
            'oper_status' => 'up', 'vlan_id' => 106,
        ]);
        return [$switch, $port];
    }

    public function test_snapshot_uses_go_database_and_preserves_empty_mac_set(): void
    {
        [$switch, $port] = $this->inventory();
        config()->set('services.nac.port_observer_enabled', true);
        $this->mock(NacApiClient::class, function ($mock): void {
            $mock->shouldReceive('resolveSwitch')->once()->with(null, '10.0.0.1')->andReturn(['id' => 'go-id']);
            $mock->shouldReceive('switchPorts')->once()->with('go-id')->andReturn([
                ['if_index' => 10101, 'interface_name' => 'Gi1/0/1', 'admin_status' => 'down',
                 'oper_status' => 'down', 'vlan_id' => 300, 'mac_addresses' => [], 'last_changed_at' => '2026-10-13T08:00:00Z'],
            ]);
            $mock->shouldNotReceive('switchLive');
            $mock->shouldNotReceive('refreshSwitchPortSnapshot');
        });
        $this->getJson('/api/switches/'.$switch->id.'/ports/status')
            ->assertOk()->assertJsonPath('data.0.port_id', $port->id)
            ->assertJsonPath('data.0.admin_status', 'down')
            ->assertJsonPath('data.0.vlan_id', 300)
            ->assertJsonPath('data.0.mac_count', 0)
            ->assertJsonPath('data.0.mac_addresses', []);
    }

    public function test_feed_maps_ids_and_replays_cursor_without_guessing_port_numbers(): void
    {
        [$switch, $port] = $this->inventory();
        $api = $this->mock(NacApiClient::class, function ($mock): void {
            $mock->shouldReceive('observerPortEvents')->once()->with('go-id', '42')->andReturn([
                'cursor' => '44', 'events' => [
                    ['if_index' => 10101, 'interface_name' => 'Gi1/0/1', 'metadata' => ['current' => ['admin_status' => 'up', 'oper_status' => 'up', 'vlan_id' => 107, 'mac_addresses' => ['AA:BB:CC:DD:EE:FF']]]],
                    ['if_index' => 1, 'interface_name' => 'other/1', 'metadata' => ['current' => []]],
                ],
            ]);
        });
        $feed = (new ObserverPortStatus($api))->feed($switch, 'go-id', '42');
        $this->assertSame('44', $feed['cursor']);
        $this->assertCount(1, $feed['ports']);
        $this->assertSame($port->id, $feed['ports'][0]['port_id']);
        $this->assertSame(107, $feed['ports'][0]['vlan_id']);
    }

    public function test_port_mapping_does_not_map_a_reused_index_by_name(): void
    {
        [$switch] = $this->inventory();
        $service = new ObserverPortStatus($this->mock(NacApiClient::class));
        $this->assertNull($service->map($switch, ['if_index' => 20201, 'interface_name' => 'Gi1/0/1']));
    }
}
