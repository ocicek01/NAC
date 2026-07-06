<?php

namespace App\Services;

use InvalidArgumentException;

class SnmpTrapPacketDecoder
{
    private const SNMP_VERSION_MAP = [
        0 => '1',
        1 => '2c',
    ];

    private const TRAP_OID_MAP = [
        '1.3.6.1.6.3.1.1.5.1' => 'coldStart',
        '1.3.6.1.6.3.1.1.5.2' => 'warmStart',
        '1.3.6.1.6.3.1.1.5.3' => 'linkDown',
        '1.3.6.1.6.3.1.1.5.4' => 'linkUp',
        '1.3.6.1.6.3.1.1.5.5' => 'authenticationFailure',
        '1.3.6.1.6.3.1.1.5.6' => 'egpNeighborLoss',
    ];

    private const OID_SYS_UPTIME = '1.3.6.1.2.1.1.3.0';
    private const OID_SNMP_TRAP = '1.3.6.1.6.3.1.1.4.1.0';
    private const OID_IF_INDEX = '1.3.6.1.2.1.2.2.1.1';
    private const OID_IF_DESCR = '1.3.6.1.2.1.2.2.1.2';
    private const OID_IF_SPEED = '1.3.6.1.2.1.2.2.1.5';
    private const OID_IF_ADMIN_STATUS = '1.3.6.1.2.1.2.2.1.7';
    private const OID_IF_OPER_STATUS = '1.3.6.1.2.1.2.2.1.8';
    private const OID_IF_NAME = '1.3.6.1.2.1.31.1.1.1.1';
    private const OID_IF_HIGH_SPEED = '1.3.6.1.2.1.31.1.1.1.15';

    public function decode(string $packet): array
    {
        $offset = 0;
        $message = $this->readElement($packet, $offset);
        if ($message['tag'] !== 0x30) {
            throw new InvalidArgumentException('Gecersiz SNMP paketi: beklenen sequence bulunamadi.');
        }

        $body = $message['value'];
        $innerOffset = 0;

        $version = $this->readInteger($body, $innerOffset);
        if (! array_key_exists($version, self::SNMP_VERSION_MAP)) {
            throw new InvalidArgumentException('Desteklenmeyen SNMP surumu. Yalnizca v1/v2c destekleniyor.');
        }

        $community = $this->readString($body, $innerOffset);
        $pdu = $this->readElement($body, $innerOffset);

        return match ($pdu['tag']) {
            0xA4 => $this->decodeV1Trap($pdu['value'], self::SNMP_VERSION_MAP[$version], $community),
            0xA7 => $this->decodeV2Trap($pdu['value'], self::SNMP_VERSION_MAP[$version], $community),
            default => throw new InvalidArgumentException('Desteklenmeyen SNMP PDU tipi.'),
        };
    }

    protected function decodeV2Trap(string $pduBody, string $version, string $community): array
    {
        $offset = 0;
        $requestId = $this->readInteger($pduBody, $offset);
        $this->readInteger($pduBody, $offset);
        $this->readInteger($pduBody, $offset);
        $varbinds = $this->readVarbindList($pduBody, $offset);

        return $this->buildDecodedTrap([
            'snmp_version' => $version,
            'community' => $community,
            'request_id' => $requestId,
            'agent_address' => null,
            'uptime' => $this->findVarbindValue($varbinds, self::OID_SYS_UPTIME),
            'trap_oid' => $this->findVarbindValue($varbinds, self::OID_SNMP_TRAP),
            'varbinds' => $varbinds,
        ]);
    }

    protected function decodeV1Trap(string $pduBody, string $version, string $community): array
    {
        $offset = 0;
        $enterpriseOid = $this->readOidValue($pduBody, $offset);
        $agentAddress = $this->readIpAddress($pduBody, $offset);
        $genericTrap = $this->readInteger($pduBody, $offset);
        $specificTrap = $this->readInteger($pduBody, $offset);
        $uptime = $this->readAnyValue($pduBody, $offset)['value'];
        $varbinds = $this->readVarbindList($pduBody, $offset);
        $trapOid = $this->mapV1TrapOid($enterpriseOid, $genericTrap, $specificTrap);

        return $this->buildDecodedTrap([
            'snmp_version' => $version,
            'community' => $community,
            'request_id' => null,
            'agent_address' => $agentAddress,
            'uptime' => $uptime,
            'trap_oid' => $trapOid,
            'varbinds' => $varbinds,
        ]);
    }

    protected function buildDecodedTrap(array $decoded): array
    {
        $varbinds = $decoded['varbinds'];
        $interface = $this->extractInterfaceData($varbinds);
        $trapOid = (string) ($decoded['trap_oid'] ?? '');

        return [
            'snmp_version' => $decoded['snmp_version'],
            'community' => $decoded['community'],
            'request_id' => $decoded['request_id'],
            'agent_address' => $decoded['agent_address'],
            'uptime' => $decoded['uptime'],
            'trap_oid' => $trapOid !== '' ? $trapOid : null,
            'trap_type' => $this->mapTrapType($trapOid),
            'if_index' => $interface['if_index'],
            'if_name' => $interface['if_name'],
            'if_descr' => $interface['if_descr'],
            'admin_status' => $interface['admin_status'],
            'oper_status' => $interface['oper_status'],
            'speed' => $interface['speed'],
            'varbinds' => $varbinds,
        ];
    }

    protected function extractInterfaceData(array $varbinds): array
    {
        $ifIndex = null;
        $ifName = null;
        $ifDescr = null;
        $adminStatus = null;
        $operStatus = null;
        $highSpeed = null;
        $speed = null;

        foreach ($varbinds as $varbind) {
            $oid = (string) ($varbind['oid'] ?? '');
            $value = $varbind['value'] ?? null;
            $suffix = $this->oidSuffix($oid, [
                self::OID_IF_INDEX,
                self::OID_IF_NAME,
                self::OID_IF_DESCR,
                self::OID_IF_SPEED,
                self::OID_IF_HIGH_SPEED,
                self::OID_IF_ADMIN_STATUS,
                self::OID_IF_OPER_STATUS,
            ]);

            if ($suffix !== null && $ifIndex === null) {
                $ifIndex = $suffix;
            }

            if ($this->matchesOidPrefix($oid, self::OID_IF_INDEX)) {
                $ifIndex = (int) $value;
            }

            if ($this->matchesOidPrefix($oid, self::OID_IF_NAME)) {
                $ifName = $this->normalizeString($value);
            }

            if ($this->matchesOidPrefix($oid, self::OID_IF_DESCR)) {
                $ifDescr = $this->normalizeString($value);
            }

            if ($this->matchesOidPrefix($oid, self::OID_IF_ADMIN_STATUS)) {
                $adminStatus = $value;
            }

            if ($this->matchesOidPrefix($oid, self::OID_IF_OPER_STATUS)) {
                $operStatus = $value;
            }

            if ($this->matchesOidPrefix($oid, self::OID_IF_HIGH_SPEED)) {
                $highSpeed = $value;
            }

            if ($this->matchesOidPrefix($oid, self::OID_IF_SPEED)) {
                $speed = $value;
            }
        }

        return [
            'if_index' => $ifIndex,
            'if_name' => $ifName,
            'if_descr' => $ifDescr,
            'admin_status' => $adminStatus,
            'oper_status' => $operStatus,
            'speed' => $this->formatSpeed($highSpeed, $speed),
        ];
    }

    protected function mapTrapType(string $trapOid): ?string
    {
        return self::TRAP_OID_MAP[$trapOid] ?? ($trapOid !== '' ? $trapOid : null);
    }

    protected function mapV1TrapOid(string $enterpriseOid, int $genericTrap, int $specificTrap): string
    {
        return match ($genericTrap) {
            0 => '1.3.6.1.6.3.1.1.5.1',
            1 => '1.3.6.1.6.3.1.1.5.2',
            2 => '1.3.6.1.6.3.1.1.5.3',
            3 => '1.3.6.1.6.3.1.1.5.4',
            4 => '1.3.6.1.6.3.1.1.5.5',
            5 => '1.3.6.1.6.3.1.1.5.6',
            default => $enterpriseOid.'.0.'.$specificTrap,
        };
    }

    protected function readVarbindList(string $data, int &$offset): array
    {
        $list = $this->readElement($data, $offset);
        if ($list['tag'] !== 0x30) {
            throw new InvalidArgumentException('Gecersiz varbind listesi.');
        }

        $varbindOffset = 0;
        $varbinds = [];

        while ($varbindOffset < strlen($list['value'])) {
            $varbind = $this->readElement($list['value'], $varbindOffset);
            if ($varbind['tag'] !== 0x30) {
                throw new InvalidArgumentException('Gecersiz varbind kaydi.');
            }

            $itemOffset = 0;
            $oid = $this->readOidValue($varbind['value'], $itemOffset);
            $value = $this->readAnyValue($varbind['value'], $itemOffset);

            $varbinds[] = [
                'oid' => $oid,
                'type' => $value['type'],
                'value' => $value['value'],
            ];
        }

        return $varbinds;
    }

    protected function readInteger(string $data, int &$offset): int
    {
        $element = $this->readElement($data, $offset);
        if ($element['tag'] !== 0x02) {
            throw new InvalidArgumentException('Beklenen integer alan bulunamadi.');
        }

        return $this->parseInteger($element['value']);
    }

    protected function readString(string $data, int &$offset): string
    {
        $element = $this->readElement($data, $offset);
        if ($element['tag'] !== 0x04) {
            throw new InvalidArgumentException('Beklenen string alan bulunamadi.');
        }

        return $element['value'];
    }

    protected function readOidValue(string $data, int &$offset): string
    {
        $element = $this->readElement($data, $offset);
        if ($element['tag'] !== 0x06) {
            throw new InvalidArgumentException('Beklenen OID alani bulunamadi.');
        }

        return $this->parseOid($element['value']);
    }

    protected function readIpAddress(string $data, int &$offset): string
    {
        $element = $this->readElement($data, $offset);
        if ($element['tag'] !== 0x40) {
            throw new InvalidArgumentException('Beklenen IP address alani bulunamadi.');
        }

        return implode('.', array_map('ord', str_split($element['value'])));
    }

    protected function readAnyValue(string $data, int &$offset): array
    {
        $element = $this->readElement($data, $offset);

        return [
            'type' => $this->typeName($element['tag']),
            'value' => match ($element['tag']) {
                0x02, 0x41, 0x42, 0x43, 0x46, 0x47 => $this->parseInteger($element['value']),
                0x04 => $this->normalizeString($element['value']),
                0x05 => null,
                0x06 => $this->parseOid($element['value']),
                0x40 => implode('.', array_map('ord', str_split($element['value']))),
                default => bin2hex($element['value']),
            },
        ];
    }

    protected function readElement(string $data, int &$offset): array
    {
        if ($offset >= strlen($data)) {
            throw new InvalidArgumentException('Paket beklenenden once sonlandi.');
        }

        $tag = ord($data[$offset]);
        $offset++;
        $length = $this->readLength($data, $offset);

        if ($offset + $length > strlen($data)) {
            throw new InvalidArgumentException('Paket uzunlugu gecersiz.');
        }

        $value = substr($data, $offset, $length);
        $offset += $length;

        return [
            'tag' => $tag,
            'length' => $length,
            'value' => $value,
        ];
    }

    protected function readLength(string $data, int &$offset): int
    {
        if ($offset >= strlen($data)) {
            throw new InvalidArgumentException('Paket uzunlugu okunamadi.');
        }

        $first = ord($data[$offset]);
        $offset++;

        if (($first & 0x80) === 0) {
            return $first;
        }

        $bytesToRead = $first & 0x7F;
        if ($bytesToRead === 0 || $bytesToRead > 4) {
            throw new InvalidArgumentException('Desteklenmeyen BER uzunluk alani.');
        }

        $length = 0;
        for ($index = 0; $index < $bytesToRead; $index++) {
            if ($offset >= strlen($data)) {
                throw new InvalidArgumentException('BER uzunluk alani eksik.');
            }

            $length = ($length << 8) | ord($data[$offset]);
            $offset++;
        }

        return $length;
    }

    protected function parseInteger(string $bytes): int
    {
        if ($bytes === '') {
            return 0;
        }

        $value = 0;
        foreach (str_split($bytes) as $byte) {
            $value = ($value << 8) | ord($byte);
        }

        $first = ord($bytes[0]);
        if (($first & 0x80) !== 0) {
            $value -= 1 << (strlen($bytes) * 8);
        }

        return $value;
    }

    protected function parseOid(string $bytes): string
    {
        if ($bytes === '') {
            return '';
        }

        $parts = [];
        $first = ord($bytes[0]);
        $parts[] = (int) floor($first / 40);
        $parts[] = $first % 40;

        $value = 0;
        for ($index = 1; $index < strlen($bytes); $index++) {
            $byte = ord($bytes[$index]);
            $value = ($value << 7) | ($byte & 0x7F);

            if (($byte & 0x80) === 0) {
                $parts[] = $value;
                $value = 0;
            }
        }

        return implode('.', $parts);
    }

    protected function findVarbindValue(array $varbinds, string $oid): mixed
    {
        foreach ($varbinds as $varbind) {
            if (($varbind['oid'] ?? null) === $oid) {
                return $varbind['value'] ?? null;
            }
        }

        return null;
    }

    protected function matchesOidPrefix(string $oid, string $prefix): bool
    {
        return $oid === $prefix || str_starts_with($oid, $prefix.'.');
    }

    protected function oidSuffix(string $oid, array $prefixes): ?int
    {
        foreach ($prefixes as $prefix) {
            if (! str_starts_with($oid, $prefix.'.')) {
                continue;
            }

            $suffix = substr($oid, strlen($prefix) + 1);
            if ($suffix !== '' && ctype_digit($suffix)) {
                return (int) $suffix;
            }
        }

        return null;
    }

    protected function normalizeString(mixed $value): ?string
    {
        $string = trim((string) $value, "\" \t\r\n");

        return $string === '' ? null : $string;
    }

    protected function formatSpeed(mixed $highSpeed, mixed $speed): ?string
    {
        $high = (int) $highSpeed;
        if ($high > 0) {
            return $high >= 1000
                ? rtrim(rtrim(number_format($high / 1000, 1, '.', ''), '0'), '.').' Gbps'
                : $high.' Mbps';
        }

        $raw = (int) $speed;
        if ($raw <= 0) {
            return null;
        }

        $mbps = (int) round($raw / 1000000);

        return $mbps >= 1000
            ? rtrim(rtrim(number_format($mbps / 1000, 1, '.', ''), '0'), '.').' Gbps'
            : $mbps.' Mbps';
    }

    protected function typeName(int $tag): string
    {
        return match ($tag) {
            0x02 => 'integer',
            0x04 => 'string',
            0x05 => 'null',
            0x06 => 'oid',
            0x40 => 'ipaddress',
            0x41 => 'counter32',
            0x42 => 'gauge32',
            0x43 => 'timeticks',
            0x46 => 'counter64',
            0x47 => 'unsigned32',
            default => 'tag_'.$tag,
        };
    }
}
