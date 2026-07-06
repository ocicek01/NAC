<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use Illuminate\Support\Arr;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Str;
use RuntimeException;
use Symfony\Component\Process\Process;

class SnmpPortStatusPollingService
{
    private const OID_IF_NAME = '1.3.6.1.2.1.31.1.1.1.1';
    private const OID_IF_DESCR = '1.3.6.1.2.1.2.2.1.2';
    private const OID_IF_TYPE = '1.3.6.1.2.1.2.2.1.3';
    private const OID_IF_ADMIN_STATUS = '1.3.6.1.2.1.2.2.1.7';
    private const OID_IF_OPER_STATUS = '1.3.6.1.2.1.2.2.1.8';
    private const OID_IF_HIGH_SPEED = '1.3.6.1.2.1.31.1.1.1.15';
    private const OID_IF_SPEED = '1.3.6.1.2.1.2.2.1.5';

    public function __construct(
        protected PortStatusUpdater $portStatusUpdater,
    ) {
    }

    public function pollAll(?Collection $switches = null): array
    {
        $switches ??= NetworkSwitch::query()
            ->where('managed', true)
            ->whereNotNull('snmp_community')
            ->orderBy('hostname')
            ->get();

        $results = [];

        foreach ($switches as $switch) {
            $results[] = $this->pollSwitch($switch);
        }

        return $results;
    }

    public function pollSwitch(NetworkSwitch $switch): array
    {
        try {
            $ports = $this->collectPortStatuses($switch);
            $seenAt = now();

            foreach ($ports as $port) {
                $this->portStatusUpdater->updatePortStatus(
                    $switch,
                    (int) $port['if_index'],
                    $port['if_name'],
                    $port['if_descr'],
                    $port['admin_status'],
                    $port['oper_status'],
                    $port['speed'],
                    'snmp_poll',
                    Arr::only($port, ['if_index', 'if_name', 'if_descr', 'admin_status', 'oper_status', 'speed']),
                    $seenAt,
                );
            }

            $switch->forceFill([
                'status' => 'online',
                'polling_error' => null,
                'consecutive_polling_failures' => 0,
                'last_polled_at' => $seenAt,
                'last_seen_at' => $seenAt,
                'port_count' => max((int) $switch->port_count, count($ports)),
            ])->save();

            return [
                'switch_id' => $switch->id,
                'hostname' => $switch->hostname,
                'ok' => true,
                'ports' => count($ports),
                'switch_status' => 'online',
                'consecutive_failures' => 0,
            ];
        } catch (\Throwable $exception) {
            $failureState = $this->markPollingFailure($switch, $exception);

            return [
                'switch_id' => $switch->id,
                'hostname' => $switch->hostname,
                'ok' => false,
                'ports' => 0,
                'error' => $exception->getMessage(),
                'switch_status' => $failureState['status'],
                'consecutive_failures' => $failureState['consecutive_failures'],
            ];
        }
    }

    public function collectPortStatuses(NetworkSwitch $switch): array
    {
        if (! $switch->snmp_community) {
            throw new RuntimeException(sprintf('SNMP community tanimli degil: %s', $switch->hostname));
        }

        $ifName = $this->runWalk($switch, self::OID_IF_NAME, false);
        $ifDescr = $this->runWalk($switch, self::OID_IF_DESCR);
        $ifType = $this->runWalk($switch, self::OID_IF_TYPE, false);
        $ifAdminStatus = $this->runWalk($switch, self::OID_IF_ADMIN_STATUS);
        $ifOperStatus = $this->runWalk($switch, self::OID_IF_OPER_STATUS);
        $ifHighSpeed = $this->runWalk($switch, self::OID_IF_HIGH_SPEED, false);
        $ifSpeed = $this->runWalk($switch, self::OID_IF_SPEED, false);

        $ports = [];

        foreach ($ifDescr as $ifIndex => $descr) {
            $name = $ifName[$ifIndex] ?? $descr;
            $type = $ifType[$ifIndex] ?? null;

            if (! $this->isPhysicalPort($name, $descr, $type)) {
                continue;
            }

            $ports[] = [
                'if_index' => (int) $ifIndex,
                'if_name' => $this->normalizeString($name),
                'if_descr' => $this->normalizeString($descr),
                'admin_status' => $this->mapAdminStatus($ifAdminStatus[$ifIndex] ?? null),
                'oper_status' => $this->mapOperStatus($ifOperStatus[$ifIndex] ?? null),
                'speed' => $this->formatSpeed($ifHighSpeed[$ifIndex] ?? null, $ifSpeed[$ifIndex] ?? null),
            ];
        }

        if ($ports === []) {
            throw new RuntimeException(sprintf('SNMP polling bos dondu: %s (%s)', $switch->hostname, $switch->ip_address));
        }

        return $ports;
    }

    protected function runWalk(NetworkSwitch $switch, string $oid, bool $failOnEmpty = true): array
    {
        $process = $this->runSnmpProcess($switch, $oid);

        if (! $process->isSuccessful()) {
            throw new RuntimeException(sprintf('SNMP walk basarisiz: %s (%s)', $switch->hostname, $switch->ip_address));
        }

        $parsed = $this->parseWalkOutput($process->getOutput());

        if ($failOnEmpty && $parsed === []) {
            throw new RuntimeException(sprintf('SNMP walk bos dondu: %s (%s)', $switch->hostname, $switch->ip_address));
        }

        return $parsed;
    }

    protected function runSnmpProcess(NetworkSwitch $switch, string $oid): Process
    {
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
        $process->setTimeout(30);
        $process->run();

        return $process;
    }

    protected function parseWalkOutput(string $output): array
    {
        $result = [];

        foreach (preg_split('/\r\n|\r|\n/', trim($output)) as $line) {
            if ($line === '') {
                continue;
            }

            if (! preg_match('/\.([0-9]+)\s=\s[^:]+:\s(.+)$/', $line, $matches)) {
                continue;
            }

            $result[(int) $matches[1]] = trim($matches[2], "\" \t");
        }

        return $result;
    }

    protected function isPhysicalPort(string $name, string $descr, ?string $ifType = null): bool
    {
        $normalized = strtolower(trim($name));
        $descrNormalized = strtolower(trim($descr));
        $combined = $normalized.' '.$descrNormalized;
        $type = (int) preg_replace('/\D+/', '', (string) $ifType);

        if (Str::contains($combined, [
            'vlanif',
            'vlan-interface',
            'vlan-',
            'vlan ',
            'loopback',
            'null',
            'console',
            'inloopback',
            'route-port',
            'stackport',
            'port-channel',
            'bluetooth',
        ])) {
            return false;
        }

        if ($type !== 0 && ! in_array($type, [6, 117], true)) {
            return false;
        }

        return preg_match('/^(gigabitethernet|fastethernet|tengigabitethernet|ten-gigabitethernet|xgigabitethernet|ethernet)([0-9\/\.\-]+)$/i', $normalized) === 1
            || preg_match('/^(gi|fa|te|tw|fo|hu|eth)([0-9\/\.\-]+)$/i', $normalized) === 1
            || preg_match('/^(ge|te|xge)([0-9\/\.\-]+)$/i', $normalized) === 1
            || preg_match('/^\d+$/', $normalized) === 1
            || preg_match('/^[a-z]\d+$/i', $normalized) === 1
            || preg_match('/^(gigabitethernet|fastethernet|tengigabitethernet|ten-gigabitethernet|xgigabitethernet|ethernet)([0-9\/\.\-]+)$/i', $descrNormalized) === 1
            || preg_match('/^(gi|fa|te|tw|fo|hu|eth)([0-9\/\.\-]+)$/i', $descrNormalized) === 1
            || preg_match('/^(ge|te|xge)([0-9\/\.\-]+)$/i', $descrNormalized) === 1
            || preg_match('/^\d+$/', $descrNormalized) === 1
            || preg_match('/^[a-z]\d+$/i', $descrNormalized) === 1;
    }

    protected function mapAdminStatus(?string $value): string
    {
        return match ((int) preg_replace('/\D+/', '', (string) $value)) {
            1 => 'up',
            2 => 'down',
            default => 'unknown',
        };
    }

    protected function mapOperStatus(?string $value): string
    {
        return match ((int) preg_replace('/\D+/', '', (string) $value)) {
            1 => 'up',
            2 => 'down',
            default => 'unknown',
        };
    }

    protected function formatSpeed(?string $highSpeed, ?string $speed): string
    {
        $high = (int) preg_replace('/\D+/', '', (string) $highSpeed);
        if ($high > 0) {
            return $high >= 1000
                ? rtrim(rtrim(number_format($high / 1000, 1, '.', ''), '0'), '.').' Gbps'
                : $high.' Mbps';
        }

        $raw = (int) preg_replace('/\D+/', '', (string) $speed);
        if ($raw <= 0) {
            return '0';
        }

        $mbps = (int) round($raw / 1000000);

        return $mbps >= 1000
            ? rtrim(rtrim(number_format($mbps / 1000, 1, '.', ''), '0'), '.').' Gbps'
            : $mbps.' Mbps';
    }

    protected function normalizeString(?string $value): ?string
    {
        $trimmed = trim((string) $value, "\" \t");

        return $trimmed === '' ? null : $trimmed;
    }

    protected function markPollingFailure(NetworkSwitch $switch, \Throwable $exception): array
    {
        $failures = (int) $switch->consecutive_polling_failures + 1;
        $status = $this->failureStatusFor($failures);

        $switch->forceFill([
            'status' => $status,
            'last_polled_at' => now(),
            'consecutive_polling_failures' => $failures,
            'polling_error' => $exception->getMessage(),
        ])->save();

        Log::warning(sprintf('Switch port polling failed for %s (%s)', $switch->hostname, $switch->ip_address), [
            'switch_id' => $switch->id,
            'failures' => $failures,
            'status' => $status,
            'error' => $exception->getMessage(),
        ]);

        return [
            'status' => $status,
            'consecutive_failures' => $failures,
        ];
    }

    protected function failureStatusFor(int $failures): string
    {
        $threshold = max(1, (int) config('services.nac.polling_failure_threshold', 3));

        return $failures >= $threshold ? 'offline' : 'warning';
    }
}

