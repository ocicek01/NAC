<?php

namespace Tests\Unit;

use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Models\Zone;
use App\Services\PortStatusUpdater;
use App\Services\SnmpPortStatusPollingService;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class SnmpPortStatusPollingServiceTest extends TestCase
{
    use RefreshDatabase;

    public function test_poll_switch_updates_ports_and_marks_switch_online(): void
    {
        $switch = $this->makeSwitch();
        $service = new FakeSnmpPortStatusPollingService($this->app->make(PortStatusUpdater::class), [
            [
                'if_index' => 10,
                'if_name' => 'Gi1/0/10',
                'if_descr' => 'Port 10',
                'admin_status' => 'up',
                'oper_status' => 'up',
                'speed' => '1 Gbps',
            ],
        ]);

        $result = $service->pollSwitch($switch->fresh());

        $this->assertTrue($result['ok']);
        $this->assertSame(1, $result['ports']);
        $this->assertSame('online', $result['switch_status']);
        $this->assertSame(0, $result['consecutive_failures']);
        $this->assertDatabaseHas('switch_ports', [
            'switch_id' => $switch->id,
            'if_index' => 10,
            'oper_status' => 'up',
        ]);
        $this->assertDatabaseHas('switches', [
            'id' => $switch->id,
            'status' => 'online',
            'consecutive_polling_failures' => 0,
            'polling_error' => null,
        ]);
    }

    public function test_poll_switch_marks_switch_warning_then_offline_after_failures(): void
    {
        config(['services.nac.polling_failure_threshold' => 3]);

        $switch = $this->makeSwitch();
        $service = new FakeSnmpPortStatusPollingService($this->app->make(PortStatusUpdater::class), [], true);

        $first = $service->pollSwitch($switch->fresh());
        $second = $service->pollSwitch($switch->fresh());
        $third = $service->pollSwitch($switch->fresh());

        $this->assertFalse($first['ok']);
        $this->assertSame('warning', $first['switch_status']);
        $this->assertSame(1, $first['consecutive_failures']);

        $this->assertFalse($second['ok']);
        $this->assertSame('warning', $second['switch_status']);
        $this->assertSame(2, $second['consecutive_failures']);

        $this->assertFalse($third['ok']);
        $this->assertSame('offline', $third['switch_status']);
        $this->assertSame(3, $third['consecutive_failures']);

        $this->assertDatabaseHas('switches', [
            'id' => $switch->id,
            'status' => 'offline',
            'consecutive_polling_failures' => 3,
            'polling_error' => 'SNMP timeout',
        ]);
    }

    public function test_successful_poll_resets_failure_state_and_clears_error(): void
    {
        config(['services.nac.polling_failure_threshold' => 3]);

        $switch = $this->makeSwitch();
        $switch->forceFill([
            'status' => 'warning',
            'consecutive_polling_failures' => 2,
            'polling_error' => 'SNMP timeout',
        ])->save();

        $service = new FakeSnmpPortStatusPollingService($this->app->make(PortStatusUpdater::class), [
            [
                'if_index' => 22,
                'if_name' => 'Gi1/0/22',
                'if_descr' => 'Port 22',
                'admin_status' => 'up',
                'oper_status' => 'down',
                'speed' => '1 Gbps',
            ],
        ]);

        $result = $service->pollSwitch($switch->fresh());

        $this->assertTrue($result['ok']);
        $this->assertSame('online', $result['switch_status']);
        $this->assertSame(0, $result['consecutive_failures']);
        $this->assertDatabaseHas('switches', [
            'id' => $switch->id,
            'status' => 'online',
            'consecutive_polling_failures' => 0,
            'polling_error' => null,
        ]);
    }

    public function test_failed_poll_does_not_force_existing_ports_down(): void
    {
        config(['services.nac.polling_failure_threshold' => 3]);

        $switch = $this->makeSwitch();
        SwitchPort::query()->create([
            'switch_id' => $switch->id,
            'if_index' => 32,
            'port_index' => 32,
            'port_name' => '32',
            'status' => 'up',
            'admin_status' => 'up',
            'oper_status' => 'up',
            'status_source' => 'snmp_trap',
        ]);

        $service = new FakeSnmpPortStatusPollingService($this->app->make(PortStatusUpdater::class), [], true);
        $service->pollSwitch($switch->fresh());

        $port = SwitchPort::query()->where('switch_id', $switch->id)->where('if_index', 32)->firstOrFail();
        $this->assertSame('up', $port->admin_status);
        $this->assertSame('up', $port->oper_status);
        $this->assertSame('snmp_trap', $port->status_source);
    }

    private function makeSwitch(): NetworkSwitch
    {
        $zone = Zone::query()->create([
            'name' => 'Core',
            'slug' => 'core',
            'status' => 'normal',
        ]);

        return NetworkSwitch::query()->create([
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
    }
}

class FakeSnmpPortStatusPollingService extends SnmpPortStatusPollingService
{
    public function __construct(PortStatusUpdater $updater, private array $ports, private bool $shouldFail = false)
    {
        parent::__construct($updater);
    }

    public function collectPortStatuses(NetworkSwitch $switch): array
    {
        if ($this->shouldFail) {
            throw new \RuntimeException('SNMP timeout');
        }

        return $this->ports;
    }
}

