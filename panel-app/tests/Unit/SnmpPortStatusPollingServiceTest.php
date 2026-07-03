<?php

namespace Tests\Unit;

use App\Models\NetworkSwitch;
use App\Models\Zone;
use App\Services\AuditLogService;
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
        $this->assertDatabaseHas('switch_ports', [
            'switch_id' => $switch->id,
            'if_index' => 10,
            'oper_status' => 'up',
        ]);
        $this->assertDatabaseHas('switches', [
            'id' => $switch->id,
            'status' => 'online',
            'consecutive_polling_failures' => 0,
        ]);
    }

    public function test_poll_switch_marks_switch_warning_then_offline_after_failures(): void
    {
        $switch = $this->makeSwitch();
        $service = new FakeSnmpPortStatusPollingService($this->app->make(PortStatusUpdater::class), [], true);

        $first = $service->pollSwitch($switch->fresh());
        $second = $service->pollSwitch($switch->fresh());
        $third = $service->pollSwitch($switch->fresh());

        $this->assertFalse($first['ok']);
        $this->assertFalse($second['ok']);
        $this->assertFalse($third['ok']);
        $this->assertDatabaseHas('switches', [
            'id' => $switch->id,
            'status' => 'offline',
            'consecutive_polling_failures' => 3,
        ]);
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
