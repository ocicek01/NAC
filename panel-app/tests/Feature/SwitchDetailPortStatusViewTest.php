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


    public function test_switch_detail_shows_polling_health_context_for_warning_switch(): void
    {
        config()->set('services.nac.switch_detail_remote_enrichment', false);

        $this->mock(NacApiClient::class, function ($mock): void {
            $mock->shouldNotReceive('resolveSwitch');
            $mock->shouldNotReceive('switches');
            $mock->shouldNotReceive('topologyLinks');
            $mock->shouldNotReceive('switchPorts');
            $mock->shouldNotReceive('switchPortSummary');
            $mock->shouldNotReceive('devicesBySwitch');
        });

        $zone = Zone::query()->create([
            'name' => 'Kutuphane',
            'slug' => 'kutuphane',
            'status' => 'normal',
        ]);

        $switch = NetworkSwitch::query()->create([
            'zone_id' => $zone->id,
            'hostname' => 'SW-WARN-01',
            'ip_address' => '10.6.8.50',
            'vendor' => 'HP',
            'model' => '2530-48G',
            'status' => 'warning',
            'managed' => true,
            'nac_mode' => 'monitor',
            'port_count' => 48,
            'consecutive_polling_failures' => 2,
            'polling_error' => 'SNMP walk basarisiz',
            'last_polled_at' => now(),
        ]);

        $this->get('/switches/'.$zone->slug.'/'.strtolower($switch->hostname))
            ->assertOk()
            ->assertSee('SNMP polling gecici olarak hata veriyor.')
            ->assertSee('Ardisik Hata:')
            ->assertSee('SNMP walk basarisiz');
    }
    public function test_switch_detail_marks_admin_down_port_with_live_status_class(): void
    {
        config()->set('services.nac.switch_detail_remote_enrichment', false);

        $this->mock(NacApiClient::class, function ($mock): void {
            $mock->shouldNotReceive('resolveSwitch');
            $mock->shouldNotReceive('switches');
            $mock->shouldNotReceive('topologyLinks');
            $mock->shouldNotReceive('switchPorts');
            $mock->shouldNotReceive('switchPortSummary');
            $mock->shouldNotReceive('devicesBySwitch');
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
