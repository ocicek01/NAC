<?php

namespace Tests\Feature;

use App\Models\NacAuditLog;
use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Models\Zone;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class SnmpTrapIngestTest extends TestCase
{
    use RefreshDatabase;

    public function test_it_updates_port_status_from_snmp_trap(): void
    {
        config()->set('services.nac.trap_ingest_enabled', true);
        config()->set('services.nac.trap_ingest_token', 'secret-trap-token');

        $switch = $this->makeSwitch();
        $port = SwitchPort::query()->create([
            'switch_id' => $switch->id,
            'if_index' => 45,
            'port_index' => 45,
            'port_name' => '45',
            'status' => 'down',
            'admin_status' => 'up',
            'oper_status' => 'down',
            'speed' => '1 Gbps',
        ]);

        $this->postJson('/api/traps/snmp', [
            'switch_ip' => $switch->ip_address,
            'if_index' => 45,
            'if_name' => '45',
            'if_descr' => 'Port 45',
            'oper_status' => 'up',
            'trap_type' => 'linkUp',
            'occurred_at' => '2026-07-06T10:30:00+03:00',
        ], ['X-TRAP-TOKEN' => 'secret-trap-token'])
            ->assertStatus(202)
            ->assertJsonPath('data.switch_id', $switch->id)
            ->assertJsonPath('data.if_index', 45)
            ->assertJsonPath('data.oper_status', 'up')
            ->assertJsonPath('data.source', 'snmp_trap');

        $port->refresh();
        $this->assertSame('up', $port->oper_status);
        $this->assertSame('snmp_trap', $port->status_source);
        $this->assertSame('linkUp', $port->raw_status['trap_type']);
        $this->assertDatabaseHas('nac_audit_logs', [
            'action' => 'snmp_trap_received',
            'switch_port_id' => $port->id,
        ]);
    }

    public function test_it_rejects_invalid_trap_token(): void
    {
        config()->set('services.nac.trap_ingest_enabled', true);
        config()->set('services.nac.trap_ingest_token', 'secret-trap-token');

        $switch = $this->makeSwitch();

        $this->postJson('/api/traps/snmp', [
            'switch_ip' => $switch->ip_address,
            'if_index' => 45,
        ], ['X-TRAP-TOKEN' => 'wrong-token'])
            ->assertStatus(403)
            ->assertJsonPath('message', 'Gecersiz trap token.');

        $this->assertSame(0, NacAuditLog::query()->count());
    }

    private function makeSwitch(): NetworkSwitch
    {
        $zone = Zone::query()->create([
            'name' => 'Trap Zone',
            'slug' => 'trap-zone',
            'status' => 'normal',
        ]);

        return NetworkSwitch::query()->create([
            'zone_id' => $zone->id,
            'hostname' => 'SW-TRAP-01',
            'ip_address' => '10.0.0.9',
            'vendor' => 'HP',
            'model' => 'J9775A 2530-48G',
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
