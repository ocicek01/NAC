<?php

namespace App\Services;

use App\Models\SwitchPort;
use Symfony\Component\Process\Process;

class PortNeighborDiscoveryService
{
    private const OID_LLDP_LOC_PORT_MAP = '1.0.8802.1.1.2.1.3.7.1.3';
    private const OID_LLDP_REM_SYS_NAME = '1.0.8802.1.1.2.1.4.1.1.9';
    private const OID_LLDP_REM_PORT_ID = '1.0.8802.1.1.2.1.4.1.1.7';
    private const OID_LLDP_REM_PORT_DESC = '1.0.8802.1.1.2.1.4.1.1.8';
    private const OID_LLDP_REM_SYS_DESC = '1.0.8802.1.1.2.1.4.1.1.10';

    public function discover(SwitchPort $port): array
    {
        $switch = $port->switch;

        if (! $switch || ! $switch->snmp_community) {
            return [
                'protocol' => 'lldp',
                'found' => false,
                'message' => 'SNMP bilgisi bulunamadi.',
            ];
        }

        $localMap = $this->runWalk($port, self::OID_LLDP_LOC_PORT_MAP);
        $localPortIndex = $this->resolveLocalLldpIndex($port, $localMap);

        if ($localPortIndex === null) {
            return [
                'protocol' => 'lldp',
                'found' => false,
                'message' => 'Bu port icin LLDP local index bulunamadi.',
            ];
        }

        $sysNames = $this->runWalk($port, self::OID_LLDP_REM_SYS_NAME);
        $portIds = $this->runWalk($port, self::OID_LLDP_REM_PORT_ID);
        $portDescs = $this->runWalk($port, self::OID_LLDP_REM_PORT_DESC);
        $sysDescs = $this->runWalk($port, self::OID_LLDP_REM_SYS_DESC);

        $neighbors = [];

        foreach ($sysNames as $suffix => $sysName) {
            $parsed = $this->parseLldpSuffix($suffix);
            if ($parsed === null || $parsed['local_port'] !== $localPortIndex) {
                continue;
            }

            $neighborKey = $parsed['suffix'];
            $neighbors[] = [
                'system_name' => $sysName,
                'port_id' => $portIds[$neighborKey] ?? '-',
                'port_description' => $portDescs[$neighborKey] ?? '-',
                'system_description' => $sysDescs[$neighborKey] ?? '-',
            ];
        }

        if ($neighbors === []) {
            return [
                'protocol' => 'lldp',
                'found' => false,
                'message' => 'Bu portta LLDP komsusu bulunamadi.',
            ];
        }

        return [
            'protocol' => 'lldp',
            'found' => true,
            'port' => $port->port_name,
            'neighbors' => $neighbors,
        ];
    }

    protected function resolveLocalLldpIndex(SwitchPort $port, array $localMap): ?int
    {
        foreach ($localMap as $suffix => $value) {
            $localIndex = $this->lastNumericSegment($suffix);
            $mappedValue = (int) preg_replace('/\D+/', '', (string) $value);

            if ($mappedValue > 0 && $mappedValue === (int) $port->if_index) {
                return $localIndex;
            }

            if ($localIndex === (int) $port->port_index) {
                return $localIndex;
            }
        }

        if (preg_match('/(\d+)\s*$/', (string) $port->port_name, $matches)) {
            $numericName = (int) $matches[1];

            foreach ($localMap as $suffix => $value) {
                $localIndex = $this->lastNumericSegment($suffix);
                if ($localIndex === $numericName) {
                    return $localIndex;
                }
            }
        }

        return null;
    }

    protected function parseLldpSuffix(string $suffix): ?array
    {
        $parts = array_values(array_filter(explode('.', $suffix), fn ($part) => $part !== ''));
        if (count($parts) < 3) {
            return null;
        }

        return [
            'suffix' => $suffix,
            'time_mark' => (int) $parts[count($parts) - 3],
            'local_port' => (int) $parts[count($parts) - 2],
            'remote_index' => (int) $parts[count($parts) - 1],
        ];
    }

    protected function lastNumericSegment(string $suffix): int
    {
        $parts = array_values(array_filter(explode('.', $suffix), fn ($part) => $part !== ''));

        return (int) end($parts);
    }

    protected function runWalk(SwitchPort $port, string $oid): array
    {
        $switch = $port->switch;
        $command = [
            'snmpwalk',
            '-v'.($switch->snmp_version ?: '2c'),
            '-c',
            $switch->snmp_community,
            '-t',
            (string) max(1, (int) ceil(($switch->snmp_timeout_ms ?: 2000) / 1000)),
            '-r',
            (string) ($switch->snmp_retries ?? 1),
            $switch->ip_address,
            $oid,
        ];

        $process = new Process($command);
        $process->setTimeout(20);
        $process->run();

        if (! $process->isSuccessful()) {
            return [];
        }

        $result = [];
        foreach (preg_split('/\r\n|\r|\n/', trim($process->getOutput())) as $line) {
            if ($line === '' || ! preg_match('/^.+?\.([0-9\.]+)\s=\s[^:]+:\s(.+)$/', $line, $matches)) {
                continue;
            }

            $result[$matches[1]] = trim($matches[2], "\" \t");
        }

        return $result;
    }
}
