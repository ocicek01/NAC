<?php

namespace Tests\Unit;

use App\Models\NacAuditLog;
use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Models\Zone;
use App\Services\SnmpTrapListenerService;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class SnmpTrapListenerServiceTest extends TestCase
{
    use RefreshDatabase;

    public function test_it_ingests_a_v2c_udp_trap_packet(): void
    {
        config()->set('services.nac.trap_ingest_enabled', true);
        config()->set('services.nac.trap_validate_community', true);

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

        $packet = $this->buildV2TrapPacket('public', [
            ['oid' => '1.3.6.1.2.1.1.3.0', 'tag' => 0x43, 'value' => 123456],
            ['oid' => '1.3.6.1.6.3.1.1.4.1.0', 'tag' => 0x06, 'value' => '1.3.6.1.6.3.1.1.5.4'],
            ['oid' => '1.3.6.1.2.1.2.2.1.1.45', 'tag' => 0x02, 'value' => 45],
            ['oid' => '1.3.6.1.2.1.31.1.1.1.1.45', 'tag' => 0x04, 'value' => '45'],
            ['oid' => '1.3.6.1.2.1.2.2.1.2.45', 'tag' => 0x04, 'value' => 'Port 45'],
            ['oid' => '1.3.6.1.2.1.2.2.1.7.45', 'tag' => 0x02, 'value' => 1],
            ['oid' => '1.3.6.1.2.1.2.2.1.8.45', 'tag' => 0x02, 'value' => 1],
            ['oid' => '1.3.6.1.2.1.31.1.1.1.15.45', 'tag' => 0x42, 'value' => 1000],
        ]);

        $service = $this->app->make(SnmpTrapListenerService::class);
        $result = $service->ingestPacket($packet, $switch->ip_address, 162);

        $this->assertTrue($result['ok']);
        $this->assertSame($switch->id, $result['result']['switch_id']);
        $this->assertSame(45, $result['result']['if_index']);
        $this->assertSame('up', $result['result']['oper_status']);
        $this->assertSame('snmp_trap', $result['result']['source']);

        $port->refresh();
        $this->assertSame('up', $port->oper_status);
        $this->assertSame('snmp_trap', $port->status_source);
        $this->assertDatabaseHas('nac_audit_logs', [
            'action' => 'snmp_trap_received',
            'switch_port_id' => $port->id,
        ]);
        $this->assertGreaterThan(0, NacAuditLog::query()->count());
    }

    private function makeSwitch(): NetworkSwitch
    {
        $zone = Zone::query()->create([
            'name' => 'Trap Listener Zone',
            'slug' => 'trap-listener-zone',
            'status' => 'normal',
        ]);

        return NetworkSwitch::query()->create([
            'zone_id' => $zone->id,
            'hostname' => 'SW-LISTENER-01',
            'ip_address' => '10.0.0.10',
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

    private function buildV2TrapPacket(string $community, array $varbinds): string
    {
        $varbindPayload = '';
        foreach ($varbinds as $varbind) {
            $varbindPayload .= $this->encodeSequence(
                $this->encodeOid($varbind['oid']).$this->encodeTypedValue($varbind['tag'], $varbind['value'])
            );
        }

        $pdu = $this->encodeTagged(0xA7,
            $this->encodeInteger(321)
            .$this->encodeInteger(0)
            .$this->encodeInteger(0)
            .$this->encodeSequence($varbindPayload)
        );

        return $this->encodeSequence(
            $this->encodeInteger(1)
            .$this->encodeString($community)
            .$pdu
        );
    }

    private function encodeTypedValue(int $tag, mixed $value): string
    {
        return match ($tag) {
            0x02, 0x41, 0x42, 0x43, 0x46, 0x47 => $this->encodeTagged($tag, $this->integerBytes((int) $value)),
            0x04 => $this->encodeString((string) $value),
            0x06 => $this->encodeOid((string) $value),
            default => throw new \InvalidArgumentException('Desteklenmeyen test tagi.'),
        };
    }

    private function encodeInteger(int $value): string
    {
        return $this->encodeTagged(0x02, $this->integerBytes($value));
    }

    private function encodeString(string $value): string
    {
        return $this->encodeTagged(0x04, $value);
    }

    private function encodeOid(string $oid): string
    {
        $parts = array_map('intval', explode('.', $oid));
        $first = (40 * $parts[0]) + $parts[1];
        $encoded = chr($first);

        foreach (array_slice($parts, 2) as $part) {
            $encoded .= $this->encodeBase128($part);
        }

        return $this->encodeTagged(0x06, $encoded);
    }

    private function encodeSequence(string $payload): string
    {
        return $this->encodeTagged(0x30, $payload);
    }

    private function encodeTagged(int $tag, string $payload): string
    {
        return chr($tag).$this->encodeLength(strlen($payload)).$payload;
    }

    private function encodeLength(int $length): string
    {
        if ($length < 128) {
            return chr($length);
        }

        $bytes = ltrim(pack('N', $length), "\x00");

        return chr(0x80 | strlen($bytes)).$bytes;
    }

    private function integerBytes(int $value): string
    {
        if ($value === 0) {
            return "\x00";
        }

        $bytes = '';
        $current = $value;
        while ($current > 0) {
            $bytes = chr($current & 0xFF).$bytes;
            $current >>= 8;
        }

        if ((ord($bytes[0]) & 0x80) !== 0) {
            $bytes = "\x00".$bytes;
        }

        return $bytes;
    }

    private function encodeBase128(int $value): string
    {
        $chunks = [chr($value & 0x7F)];
        $value >>= 7;

        while ($value > 0) {
            array_unshift($chunks, chr(($value & 0x7F) | 0x80));
            $value >>= 7;
        }

        return implode('', $chunks);
    }
}
