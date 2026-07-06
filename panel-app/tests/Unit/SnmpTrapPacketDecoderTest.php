<?php

namespace Tests\Unit;

use App\Services\SnmpTrapPacketDecoder;
use Tests\TestCase;

class SnmpTrapPacketDecoderTest extends TestCase
{
    public function test_it_decodes_a_v2c_link_down_trap(): void
    {
        $decoder = $this->app->make(SnmpTrapPacketDecoder::class);
        $packet = $this->buildV2TrapPacket('public', [
            ['oid' => '1.3.6.1.2.1.1.3.0', 'tag' => 0x43, 'value' => 123456],
            ['oid' => '1.3.6.1.6.3.1.1.4.1.0', 'tag' => 0x06, 'value' => '1.3.6.1.6.3.1.1.5.3'],
            ['oid' => '1.3.6.1.2.1.2.2.1.1.45', 'tag' => 0x02, 'value' => 45],
            ['oid' => '1.3.6.1.2.1.31.1.1.1.1.45', 'tag' => 0x04, 'value' => '45'],
            ['oid' => '1.3.6.1.2.1.2.2.1.2.45', 'tag' => 0x04, 'value' => 'Port 45'],
            ['oid' => '1.3.6.1.2.1.2.2.1.7.45', 'tag' => 0x02, 'value' => 1],
            ['oid' => '1.3.6.1.2.1.2.2.1.8.45', 'tag' => 0x02, 'value' => 2],
            ['oid' => '1.3.6.1.2.1.31.1.1.1.15.45', 'tag' => 0x42, 'value' => 1000],
        ]);

        $decoded = $decoder->decode($packet);

        $this->assertSame('2c', $decoded['snmp_version']);
        $this->assertSame('public', $decoded['community']);
        $this->assertSame('1.3.6.1.6.3.1.1.5.3', $decoded['trap_oid']);
        $this->assertSame('linkDown', $decoded['trap_type']);
        $this->assertSame(45, $decoded['if_index']);
        $this->assertSame('45', $decoded['if_name']);
        $this->assertSame('Port 45', $decoded['if_descr']);
        $this->assertSame(1, $decoded['admin_status']);
        $this->assertSame(2, $decoded['oper_status']);
        $this->assertSame('1 Gbps', $decoded['speed']);
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
            $this->encodeInteger(123)
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
