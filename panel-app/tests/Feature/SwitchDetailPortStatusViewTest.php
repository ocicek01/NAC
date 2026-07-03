<?php

namespace Tests\Feature;

use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Models\Zone;
use App\Services\NacApiClient;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class SwitchDetailPortStatusViewTest extends TestCase
{
    use RefreshDatabase;

    public function test_switch_detail_marks_admin_down_port_with_live_status_class(): void
    {
        $this->mock(NacApiClient::class, function ($mock): void {
            $mock->shouldReceive('resolveSwitch')->andReturn([]);
            $mock->shouldReceive('switches')->andReturn([]);
            $mock->shouldReceive('topologyLinks')->andReturn([]);
            $mock->shouldReceive('switchPorts')->andReturn([]);
            $mock->shouldReceive('switchPortSummary')->andReturn([]);
            $mock->shouldReceive('devicesBySwitch')->andReturn([]);
        });

        $zone = Zone::query()->create([
            'name' => 'Core Zone',
            'slug' => 'core-zone',
            'status' => 'normal',
        ]);

        $switch = NetworkSwitch::query()->create([
            'zone_id' => $zone->id,
            'hostname' => 'SW-CORE-01',
            'ip_address' => '10.0.0.1',
            'vendor' => 'Cisco',
            'model' => 'C9300-48P',
            'status' => 'online',
            'managed' => true,
            'nac_mode' => 'monitor',
            'port_count' => 48,
        ]);

        SwitchPort::query()->create([
            'switch_id' => $switch->id,
            'if_index' => 45,
            'port_index' => 45,
            'port_name' => 'Gi1/0/45',
            'status' => 'disabled',
            'admin_status' => 'down',
            'oper_status' => 'down',
            'speed' => '1 Gbps',
            'duplex' => 'Full',
            'last_seen' => now(),
            'last_change' => now(),
            'last_change_at' => now(),
        ]);

        $this->get('/switches/'.$zone->slug.'/'.strtolower($switch->hostname))
            ->assertOk()
            ->assertSee('state-admin_down', false)
            ->assertSee('Admin Down');
    }
}
