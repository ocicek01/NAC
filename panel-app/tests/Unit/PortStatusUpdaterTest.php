<?php

namespace Tests\Unit;

use App\Models\NacAuditLog;
use App\Models\NetworkSwitch;
use App\Models\Zone;
use App\Services\PortStatusUpdater;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Cache;
use Tests\TestCase;

class PortStatusUpdaterTest extends TestCase
{
    use RefreshDatabase;

    public function test_it_updates_port_status_and_emits_event_on_change(): void
    {
        config(['cache.default' => 'array']);
        Cache::flush();

        $zone = Zone::query()->create([
            'name' => 'Core',
            'slug' => 'core',
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
            'snmp_version' => '2c',
            'snmp_community' => 'public',
            'snmp_port' => 161,
            'snmp_timeout_ms' => 2000,
            'snmp_retries' => 1,
        ]);

        $service = $this->app->make(PortStatusUpdater::class);

        $service->updatePortStatus($switch, 45, 'Gi1/0/45', 'Port 45', 'up', 'down', '1 Gbps', 'snmp_poll');
        $updated = $service->updatePortStatus($switch, 45, 'Gi1/0/45', 'Port 45', 'up', 'up', '1 Gbps', 'snmp_poll');

        $this->assertSame('up', $updated->admin_status);
        $this->assertSame('up', $updated->oper_status);
        $this->assertSame('snmp_poll', $updated->status_source);
        $this->assertNotNull($updated->last_seen);
        $this->assertNotNull($updated->last_change);
        $this->assertSame(1, NacAuditLog::query()->where('action', 'switch_port_status_changed')->count());

        $events = $service->eventsAfter(null);
        $this->assertIsArray($events);
        $this->assertCount(1, $events);
        $this->assertSame('port_status_changed', $events[0]['type']);
        $this->assertSame('down', $events[0]['old_oper_status']);
        $this->assertSame('up', $events[0]['new_oper_status']);
    }
}
