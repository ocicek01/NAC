<?php

namespace App\Services;

use App\Models\Endpoint;
use App\Models\EndpointLocation;
use App\Models\NetworkSwitch;
use Illuminate\Support\Arr;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Str;
use Illuminate\Validation\ValidationException;
use Symfony\Component\Process\Process;

class SnmpPortDiscoveryService
{
    private const OID_IF_NAME = '1.3.6.1.2.1.31.1.1.1.1';
    private const OID_IF_DESCR = '1.3.6.1.2.1.2.2.1.2';
    private const OID_IF_TYPE = '1.3.6.1.2.1.2.2.1.3';
    private const OID_IF_ALIAS = '1.3.6.1.2.1.31.1.1.1.18';
    private const OID_IF_ADMIN_STATUS = '1.3.6.1.2.1.2.2.1.7';
    private const OID_IF_OPER_STATUS = '1.3.6.1.2.1.2.2.1.8';
    private const OID_IF_HIGH_SPEED = '1.3.6.1.2.1.31.1.1.1.15';
    private const OID_IF_SPEED = '1.3.6.1.2.1.2.2.1.5';
    private const OID_BRIDGE_PORT_TO_IFINDEX = '1.3.6.1.2.1.17.1.4.1.2';
    private const OID_FDB_PORT = '1.3.6.1.2.1.17.4.3.1.2';
    private const OID_IP_NET_TO_MEDIA_PHYS_ADDRESS = '1.3.6.1.2.1.4.22.1.2';

    public function __construct(
        protected HuaweiSnmpVendorEnricher $huaweiEnricher = new HuaweiSnmpVendorEnricher(),
        protected CiscoSnmpVendorEnricher $ciscoEnricher = new CiscoSnmpVendorEnricher(),
        protected HpSnmpVendorEnricher $hpEnricher = new HpSnmpVendorEnricher(),
        protected NullSnmpVendorEnricher $nullEnricher = new NullSnmpVendorEnricher(),
    ) {
    }

    public function discover(NetworkSwitch $switch): array
    {
        if (! $switch->snmp_community) {
            throw ValidationException::withMessages([
                'switch' => "SNMP community tanimli degil: {$switch->hostname}",
            ]);
        }

        $ifName = $this->runWalk($switch, self::OID_IF_NAME, false);
        $ifDescr = $this->runWalk($switch, self::OID_IF_DESCR);
        $ifType = $this->runWalk($switch, self::OID_IF_TYPE, false);
        $ifAlias = $this->runWalk($switch, self::OID_IF_ALIAS, false);
        $ifAdminStatus = $this->runWalk($switch, self::OID_IF_ADMIN_STATUS);
        $ifOperStatus = $this->runWalk($switch, self::OID_IF_OPER_STATUS);
        $ifHighSpeed = $this->runWalk($switch, self::OID_IF_HIGH_SPEED, false);
        $ifSpeed = $this->runWalk($switch, self::OID_IF_SPEED, false);

        $portsByIfIndex = collect($ifDescr)
            ->reduce(function (array $carry, string $descr, int $ifIndex) use ($ifName, $ifType, $ifAlias, $ifAdminStatus, $ifOperStatus, $ifHighSpeed, $ifSpeed) {
                $name = $ifName[$ifIndex] ?? $descr;
                $type = $ifType[$ifIndex] ?? null;

                if (! $this->isPhysicalPort($name, $descr, $type)) {
                    return $carry;
                }

                $portIndex = $this->extractPortIndex($name, $descr, $ifIndex);
                $admin = $ifAdminStatus[$ifIndex] ?? null;
                $oper = $ifOperStatus[$ifIndex] ?? null;
                $adminLabel = $this->mapAdminStatus($admin);
                $operLabel = $this->mapOperStatus($oper);

                $carry[$ifIndex] = [
                    'if_index' => $ifIndex,
                    'port_index' => $portIndex,
                    'port_name' => $name,
                    'port_description' => $this->normalizeString($ifAlias[$ifIndex] ?? null),
                    'status' => $this->mapStatus($adminLabel, $operLabel),
                    'admin_status' => $adminLabel,
                    'oper_status' => $operLabel,
                    'port_type' => $this->inferPortType($name, $descr),
                    'nac_mode' => 'inherit',
                    'vlan_id' => null,
                    'native_vlan' => null,
                    'allowed_vlans' => null,
                    'mac_count' => 0,
                    'speed' => $this->formatSpeed($ifHighSpeed[$ifIndex] ?? null, $ifSpeed[$ifIndex] ?? null),
                    'duplex' => 'Full',
                    'poe_enabled' => false,
                    'poe_power' => 0,
                    'last_change_at' => now(),
                    'last_discovered_at' => now(),
                ];

                return $carry;
            }, []);

        $enriched = $this->resolveVendorEnricher($switch)->enrich($switch, $portsByIfIndex);
        $ports = collect($this->hydrateLiveInventory($switch, $enriched))
            ->sortBy('port_index')
            ->values();
        $existingEndpointTimeline = $this->existingEndpointTimeline($switch);

        DB::transaction(function () use ($switch, $ports, $existingEndpointTimeline) {
            $switch->ports()->delete();
            $createdPorts = collect();

            foreach ($ports as $port) {
                $createdPorts->push($switch->ports()->create(Arr::except($port, ['mac_addresses'])));
            }

            $this->syncEndpointLocations($switch, $createdPorts, $ports, $existingEndpointTimeline);

            $switch->update([
                'port_count' => $ports->count(),
                'last_seen_at' => now(),
            ]);
        });

        return [
            'switch_id' => $switch->id,
            'hostname' => $switch->hostname,
            'ip_address' => $switch->ip_address,
            'discovered_ports' => $ports->count(),
        ];
    }

    public function discoverMany(Collection $switches): array
    {
        return $switches->map(fn (NetworkSwitch $switch) => $this->discover($switch))->all();
    }

    public function rediscoverPort(\App\Models\SwitchPort $port): \App\Models\SwitchPort
    {
        $port->loadMissing('switch');

        $switch = $port->switch;
        $originalIfIndex = $port->if_index;
        $originalPortName = $port->port_name;

        $this->discover($switch);

        return $switch->fresh()
            ->ports()
            ->where('if_index', $originalIfIndex)
            ->orWhere('port_name', $originalPortName)
            ->orderByRaw('case when if_index = ? then 0 else 1 end', [$originalIfIndex])
            ->firstOrFail();
    }

    protected function runWalk(NetworkSwitch $switch, string $oid, bool $failOnEmpty = true): array
    {
        $process = $this->runSnmpProcess($switch, $oid);

        if (! $process->isSuccessful()) {
            throw ValidationException::withMessages([
                'switch' => "SNMP walk basarisiz: {$switch->hostname} ({$switch->ip_address})",
            ]);
        }

        $parsed = $this->parseWalkOutput($process->getOutput());

        if ($failOnEmpty && empty($parsed)) {
            throw ValidationException::withMessages([
                'switch' => "SNMP walk bos dondu: {$switch->hostname} ({$switch->ip_address})",
            ]);
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

            $index = (int) $matches[1];
            $value = trim($matches[2], "\" \t");
            $result[$index] = $value;
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
            'switch loopback interface',
            'stackport',
            'stacksub',
            'port-channel',
            'bluetooth',
            'default_vlan',
            'default vlan',
            'unrouted vlan',
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

    protected function extractPortIndex(string $name, string $descr, int $fallback): int
    {
        $nameSegments = $this->extractNumericSegments($name);
        if ($nameSegments !== []) {
            return $this->buildPortIndex($name, $nameSegments, $fallback);
        }

        $descrSegments = $this->extractNumericSegments($descr);
        if ($descrSegments !== []) {
            return $this->buildPortIndex($descr, $descrSegments, $fallback);
        }

        return $fallback;
    }

    protected function extractNumericSegments(string $value): array
    {
        if (! preg_match_all('/\d+/', $value, $matches)) {
            return [];
        }

        return array_map('intval', $matches[0]);
    }

    protected function buildPortIndex(string $source, array $segments, int $fallback): int
    {
        $prefix = $this->portPrefixNamespace($source);

        if (count($segments) === 1) {
            return $prefix > 0
                ? ($prefix * 100000) + $segments[0]
                : $segments[0];
        }

        $portIndex = $prefix > 0 ? $prefix : 0;

        foreach ($segments as $segment) {
            if ($segment > 99) {
                return $fallback;
            }

            $portIndex = ($portIndex * 100) + $segment;
        }

        return $portIndex > 0 ? $portIndex : $fallback;
    }

    protected function portPrefixNamespace(string $value): int
    {
        $normalized = strtolower(trim($value));

        return match (true) {
            preg_match('/^(fa|fastethernet)/i', $normalized) === 1 => 1,
            preg_match('/^(gi|gigabitethernet|ge)/i', $normalized) === 1 => 2,
            preg_match('/^(te|tengigabitethernet|ten-gigabitethernet|xge)/i', $normalized) === 1 => 3,
            preg_match('/^(fo)/i', $normalized) === 1 => 4,
            preg_match('/^(tw)/i', $normalized) === 1 => 5,
            preg_match('/^(hu)/i', $normalized) === 1 => 6,
            preg_match('/^(eth|ethernet)/i', $normalized) === 1 => 7,
            default => 0,
        };
    }

    protected function mapStatus(string $adminStatus, string $operStatus): string
    {
        if ($adminStatus === 'down') {
            return 'disabled';
        }

        if ($operStatus === 'up') {
            return 'up';
        }

        return 'down';
    }

    protected function mapAdminStatus(?string $value): string
    {
        return match ((int) preg_replace('/\D+/', '', (string) $value)) {
            1 => 'up',
            2 => 'down',
            3 => 'testing',
            default => 'unknown',
        };
    }

    protected function mapOperStatus(?string $value): string
    {
        return match ((int) preg_replace('/\D+/', '', (string) $value)) {
            1 => 'up',
            2 => 'down',
            3 => 'testing',
            5 => 'dormant',
            6 => 'not_present',
            7 => 'lower_layer_down',
            default => 'unknown',
        };
    }

    protected function inferPortType(string $name, string $descr): string
    {
        $normalized = strtolower($name.' '.$descr);

        if (Str::contains($normalized, ['tengig', 'xgig', 'uplink'])) {
            return 'uplink';
        }

        return 'access';
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

    protected function resolveVendorEnricher(NetworkSwitch $switch): SnmpVendorEnricher
    {
        foreach ([
            $this->huaweiEnricher,
            $this->ciscoEnricher,
            $this->hpEnricher,
            $this->nullEnricher,
        ] as $enricher) {
            if ($enricher->supports($switch)) {
                return $enricher;
            }
        }

        return $this->nullEnricher;
    }

    protected function hydrateLiveInventory(NetworkSwitch $switch, array $portsByIfIndex): array
    {
        $vlanMap = $this->discoverVlanMap($switch, $portsByIfIndex);
        $macMap = $this->discoverMacMap($switch);
        $arpMap = $this->discoverArpMap($switch);

        foreach ($portsByIfIndex as $ifIndex => &$port) {
            $port['vlan_id'] = $vlanMap[$ifIndex] ?? $port['vlan_id'];
            $port['mac_addresses'] = array_values(array_unique($macMap[$ifIndex] ?? []));
            $port['mac_count'] = count($port['mac_addresses']);
            $port['ip_addresses'] = collect($port['mac_addresses'])
                ->mapWithKeys(fn (string $mac) => [$mac => $arpMap[$mac] ?? null])
                ->filter()
                ->all();
        }

        unset($port);

        return $portsByIfIndex;
    }

    protected function discoverVlanMap(NetworkSwitch $switch, array $portsByIfIndex): array
    {
        $vendor = strtolower((string) $switch->vendor);
        $map = [];

        foreach ($portsByIfIndex as $ifIndex => $port) {
            $oid = Str::contains($vendor, 'cisco')
                ? '1.3.6.1.4.1.9.9.68.1.2.2.1.2.'.$ifIndex
                : '1.3.6.1.2.1.17.7.1.4.5.1.1.'.$ifIndex;

            $value = $this->runGetValue($switch, $oid);
            $vlan = (int) preg_replace('/\D+/', '', (string) $value);

            if ($vlan > 0) {
                $map[$ifIndex] = $vlan;
            }
        }

        return $map;
    }

    protected function discoverMacMap(NetworkSwitch $switch): array
    {
        $bridgePortMap = $this->runWalk($switch, self::OID_BRIDGE_PORT_TO_IFINDEX, false);
        if ($bridgePortMap === []) {
            return [];
        }

        $process = $this->runSnmpProcess($switch, self::OID_FDB_PORT);
        if (! $process->isSuccessful()) {
            return [];
        }

        $macMap = [];
        foreach (preg_split('/\r\n|\r|\n/', trim($process->getOutput())) as $line) {
            if ($line === '') {
                continue;
            }

            if (! preg_match('/(?:iso|\.?1)\.3\.6\.1\.2\.1\.17\.4\.3\.1\.2\.([0-9\.]+)\s=\s[^:]+:\s(.+)$/i', $line, $matches)) {
                continue;
            }

            $mac = $this->formatMacFromSuffix($matches[1]);
            $bridgePort = (int) preg_replace('/\D+/', '', $matches[2]);
            $ifIndex = isset($bridgePortMap[$bridgePort]) ? (int) preg_replace('/\D+/', '', (string) $bridgePortMap[$bridgePort]) : 0;

            if ($ifIndex > 0 && $mac !== null) {
                $macMap[$ifIndex][] = $mac;
            }
        }

        return $macMap;
    }

    protected function discoverArpMap(NetworkSwitch $switch): array
    {
        $process = $this->runSnmpProcess($switch, self::OID_IP_NET_TO_MEDIA_PHYS_ADDRESS);
        if (! $process->isSuccessful()) {
            return [];
        }

        $map = [];

        foreach (preg_split('/\r\n|\r|\n/', trim($process->getOutput())) as $line) {
            if ($line === '') {
                continue;
            }

            if (! preg_match('/(?:iso|\.?1)\.3\.6\.1\.2\.1\.4\.22\.1\.2\.(\d+)\.((?:\d+\.){3}\d+)\s=\s[^:]+:\s(.+)$/i', $line, $matches)) {
                continue;
            }

            $ipAddress = trim($matches[2]);
            $macAddress = $this->parseMacValue($matches[3]);

            if ($macAddress === null || ! filter_var($ipAddress, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4)) {
                continue;
            }

            $map[$macAddress] = $ipAddress;
        }

        return $map;
    }

    protected function runGetValue(NetworkSwitch $switch, string $oid): ?string
    {
        $command = [
            'snmpget',
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
        $process->setTimeout(15);
        $process->run();

        if (! $process->isSuccessful()) {
            return null;
        }

        $line = trim($process->getOutput());
        if ($line === '' || ! preg_match('/=\s[^:]+:\s(.+)$/', $line, $matches)) {
            return null;
        }

        return trim($matches[1], "\" \t");
    }

    protected function formatMacFromSuffix(string $suffix): ?string
    {
        $parts = array_filter(array_map('trim', explode('.', $suffix)), fn ($part) => $part !== '');
        if (count($parts) !== 6) {
            return null;
        }

        return collect($parts)
            ->map(fn ($part) => str_pad(dechex((int) $part), 2, '0', STR_PAD_LEFT))
            ->implode(':');
    }

    protected function parseMacValue(string $value): ?string
    {
        $normalized = trim($value, "\" \t");

        if (preg_match_all('/[0-9a-f]{2}/i', $normalized, $matches) !== false && count($matches[0]) === 6) {
            return strtolower(implode(':', array_map('strtolower', $matches[0])));
        }

        return null;
    }

    protected function existingEndpointTimeline(NetworkSwitch $switch): array
    {
        return $switch->ports()
            ->with(['endpointLocations.endpoint'])
            ->get()
            ->flatMap(function ($port) {
                return $port->endpointLocations->mapWithKeys(function ($location) use ($port) {
                    $macAddress = strtolower((string) $location->endpoint?->mac_address);

                    if ($macAddress === '') {
                        return [];
                    }

                    return [$port->if_index.'|'.$macAddress => $location->first_seen_at];
                });
            })
            ->all();
    }

    protected function syncEndpointLocations(NetworkSwitch $switch, Collection $createdPorts, Collection $ports, array $existingEndpointTimeline): void
    {
        EndpointLocation::query()->where('switch_id', $switch->id)->delete();

        $portsByIfIndex = $createdPorts->keyBy('if_index');

        foreach ($ports as $portData) {
            $switchPort = $portsByIfIndex->get($portData['if_index'] ?? null);
            if (! $switchPort) {
                continue;
            }

            foreach ($portData['mac_addresses'] ?? [] as $mac) {
                $mac = strtolower((string) $mac);
                $ipAddress = $portData['ip_addresses'][$mac] ?? null;
                $endpoint = Endpoint::query()->firstOrNew(['mac_address' => $mac]);
                $endpoint->fill([
                    'ip_address' => $ipAddress ?: $endpoint->ip_address,
                    'hostname' => $this->resolveHostname($ipAddress) ?: $endpoint->hostname,
                    'vlan_id' => $portData['vlan_id'] ?? $endpoint->vlan_id,
                    'status' => $portData['status'] === 'up' ? 'authenticated' : 'inactive',
                    'last_seen_at' => now(),
                ]);
                $endpoint->save();

                $timelineKey = ($portData['if_index'] ?? 0).'|'.$mac;
                $firstSeenAt = $existingEndpointTimeline[$timelineKey] ?? now();

                EndpointLocation::query()->create([
                    'endpoint_id' => $endpoint->id,
                    'switch_id' => $switch->id,
                    'switch_port_id' => $switchPort->id,
                    'vlan_id' => $portData['vlan_id'] ?? null,
                    'first_seen_at' => $firstSeenAt,
                    'last_seen_at' => now(),
                ]);
            }
        }
    }

    protected function resolveHostname(?string $ipAddress): ?string
    {
        if (! $ipAddress || ! filter_var($ipAddress, FILTER_VALIDATE_IP)) {
            return null;
        }

        $resolved = @gethostbyaddr($ipAddress);
        if ($this->isResolvableHostname($resolved, $ipAddress)) {
            return $resolved;
        }

        foreach ([
            $this->resolveHostnameViaNslookup($ipAddress),
            $this->resolveHostnameViaDig($ipAddress),
        ] as $candidate) {
            if ($this->isResolvableHostname($candidate, $ipAddress)) {
                return $candidate;
            }
        }

        return null;
    }

    protected function isResolvableHostname(mixed $value, string $ipAddress): bool
    {
        return is_string($value)
            && $value !== ''
            && $value !== $ipAddress
            && filter_var($value, FILTER_VALIDATE_IP) === false;
    }

    protected function resolveHostnameViaNslookup(string $ipAddress): ?string
    {
        $process = new Process(['nslookup', $ipAddress]);
        $process->setTimeout(8);
        $process->run();

        if (! $process->isSuccessful()) {
            return null;
        }

        foreach (preg_split('/\r\n|\r|\n/', trim($process->getOutput())) as $line) {
            if (preg_match('/name\s*=\s*(.+)$/i', trim($line), $matches) === 1) {
                return rtrim(trim($matches[1]), '.');
            }
        }

        return null;
    }

    protected function resolveHostnameViaDig(string $ipAddress): ?string
    {
        $process = new Process(['dig', '+short', '-x', $ipAddress]);
        $process->setTimeout(8);
        $process->run();

        if (! $process->isSuccessful()) {
            return null;
        }

        $lines = array_values(array_filter(array_map('trim', preg_split('/\r\n|\r|\n/', trim($process->getOutput())))));

        if ($lines === []) {
            return null;
        }

        return rtrim($lines[0], '.');
    }
}
