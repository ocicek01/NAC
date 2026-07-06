<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Str;
use RuntimeException;

class SwitchStatsService
{
    protected array $goSwitchIdCache = [];
    protected int $nacCacheTtlSeconds = 10;

    public function __construct(
        protected ?NacApiClient $nacApiClient = null,
    ) {
    }

    public function listItem(NetworkSwitch $switch): array
    {
        $ports = $switch->ports;
        $endpoints = $ports->map->currentLocation->filter()->map->endpoint->filter()->unique('id');
        $effectivePortCount = $ports->count() > 0 ? $ports->count() : $switch->port_count;
        $goPortSummary = $this->goPortSummaryForSwitch($switch);

        return [
            'id' => $switch->id,
            'state' => $this->switchState($switch),
            'hostname' => $switch->hostname,
            'ip' => $switch->ip_address,
            'vendor' => $switch->vendor,
            'model' => $switch->model,
            'ports' => $effectivePortCount,
            'up' => $ports->where('status', 'up')->count(),
            'down' => $ports->where('status', 'down')->count(),
            'endpoint' => $endpoints->count(),
            'last_seen' => $this->relativeTime($switch->last_seen_at),
            'lastSeen' => $this->relativeTime($switch->last_seen_at),
            'managed' => $switch->managed,
            'nac_mode' => $switch->nac_mode,
            'location' => $switch->location,
            'uplink_ports' => (int) ($goPortSummary['uplink_ports'] ?? 0),
            'learned_macs' => (int) ($goPortSummary['total_learned_macs'] ?? 0),
            'top_mac_port_name' => (string) ($goPortSummary['top_mac_port_name'] ?? ''),
            'top_mac_count' => (int) ($goPortSummary['top_mac_count'] ?? 0),
        ];
    }

    public function detail(NetworkSwitch $switch): array
    {
        $ports = $switch->ports;
        $topologyLinks = [];
        $goPorts = [];
        $goDevices = [];
        $goPortSummary = [];

        if ($this->shouldLoadRemoteSwitchDetailEnrichment()) {
            $topologyLinks = $this->topologyLinksForSwitch($switch);
            $goPorts = $this->goPortsForSwitch($switch);
            $goDevices = $this->goDevicesForSwitch($switch);
            $goPortSummary = $this->goPortSummaryForSwitch($switch);
        }
        $effectivePortCount = $ports->count() > 0 ? $ports->count() : $switch->port_count;
        $portPayload = $ports
            ->map(fn ($port) => $this->portPayload($port, $topologyLinks, $goPorts, $goDevices))
            ->sortBy('display_order')
            ->values();
        $selectedPort = $portPayload->firstWhere('state', 'up') ?? $portPayload->first();
        $portStatusSegments = $this->portStatusSegments($ports);
        $usedPoe = (float) $ports->sum(fn ($port) => $port->poe_enabled ? (float) $port->poe_power : 0.0);
        $poeBudget = $this->poeBudget($switch);
        $supportsPoe = $poeBudget > 0;
        $panelProfile = $this->panelProfile($switch);
        $panelLayout = $this->buildPanelLayout($switch, $portPayload);

        return [
            'switch' => [
                'id' => $switch->id,
                'hostname' => $switch->hostname,
                'zone' => $switch->zone?->name,
                'zone_slug' => $switch->zone?->slug,
                'status' => $this->switchState($switch),
                'status_label' => $this->switchStatusLabel($switch),
                'status_class' => $this->switchStatusClass($switch),
                'nac_mode' => $switch->nac_mode,
                'vendor' => $switch->vendor,
                'model' => $switch->model,
                'ip_address' => $switch->ip_address,
                'serial' => '-',
                'firmware' => '-',
                'mac' => '-',
                'last_seen_at' => optional($switch->last_seen_at)->toDateTimeString(),
                'uptime' => $this->uptimeLabel($switch),
                'location' => $switch->location,
                'managed' => $switch->managed,
            ],
            'kpis' => [
                'total_ports' => $effectivePortCount,
                'up_ports' => $ports->where('status', 'up')->count(),
                'down_ports' => $ports->where('status', 'down')->count(),
                'disabled_ports' => $ports->where('status', 'disabled')->count(),
                'poe_ports' => $ports->where('poe_enabled', true)->count(),
                'endpoint_count' => $ports->map->currentLocation->filter()->count(),
                'poe_budget' => $poeBudget,
                'poe_used' => round($usedPoe, 1),
                'supports_poe' => $supportsPoe,
                'cpu_percent' => min(95, 18 + $ports->where('status', 'up')->count()),
                'memory_percent' => min(95, 24 + (int) round($ports->map->currentLocation->filter()->count() / 2)),
            ],
            'port_map' => $portPayload->all(),
            'side_panel' => [
                'traffic' => $this->emptyTrafficSeries(),
                'port_status_segments' => $portStatusSegments,
                'poe_segments' => [
                    ['label' => 'Kullanilan', 'value' => round($usedPoe), 'color' => '#2f6fec'],
                    ['label' => 'Kullanilabilir', 'value' => max(0, $poeBudget - round($usedPoe)), 'color' => '#cbd5e1'],
                ],
            ],
            'recent_events' => $this->eventEntries($switch),
            'view' => [
                'id' => $switch->id,
                'hostname' => $switch->hostname,
                'zone' => $switch->zone?->name,
                'zoneLabel' => $switch->zone?->name,
                'status' => $this->switchStatusLabel($switch),
                'statusClass' => $this->switchStatusClass($switch),
                'statusDetail' => $this->switchStatusDetail($switch),
                'nacMode' => $this->nacModeLabel($switch->nac_mode),
                'vendor' => $switch->vendor,
                'model' => $switch->model,
                'ip' => $switch->ip_address,
                'serial' => '-',
                'firmware' => '-',
                'mac' => '-',
                'lastSeen' => optional($switch->last_seen_at)->format('d.m.Y H:i:s'),
                'lastPolledAt' => optional($switch->last_polled_at)->format('d.m.Y H:i:s'),
                'pollingFailures' => (int) ($switch->consecutive_polling_failures ?? 0),
                'pollingError' => $switch->polling_error,
                'uptime' => $this->uptimeLabel($switch),
                'totalPorts' => $effectivePortCount,
                'poeBudget' => $poeBudget,
                'supportsPoe' => $supportsPoe,
                'summary' => array_values(array_filter([
                    ['label' => 'Toplam Port', 'value' => (string) $effectivePortCount, 'icon' => 'bi-hdd-network', 'tone' => 'dark'],
                    ['label' => 'UP Port', 'value' => (string) $ports->where('status', 'up')->count(), 'icon' => 'bi-arrow-up-circle', 'tone' => 'success'],
                    ['label' => 'DOWN Port', 'value' => (string) $ports->where('status', 'down')->count(), 'icon' => 'bi-arrow-down-circle', 'tone' => 'danger'],
                    $goPortSummary !== [] ? ['label' => 'Uplink Port', 'value' => (string) ($goPortSummary['uplink_ports'] ?? 0), 'icon' => 'bi-diagram-2', 'tone' => 'warning'] : null,
                    $goPortSummary !== [] ? ['label' => 'Ogrenilen MAC', 'value' => (string) ($goPortSummary['total_learned_macs'] ?? 0), 'icon' => 'bi-hdd-network', 'tone' => 'primary'] : null,
                    $supportsPoe ? ['label' => 'PoE Port', 'value' => (string) $ports->where('poe_enabled', true)->count(), 'icon' => 'bi-lightning-charge', 'tone' => 'secondary'] : null,
                    ['label' => 'Kullanilan Port', 'value' => (string) $ports->map->currentLocation->filter()->count(), 'icon' => 'bi-diagram-3', 'tone' => 'primary'],
                    $supportsPoe ? ['label' => 'Toplam Guc', 'value' => $poeBudget.' W', 'icon' => 'bi-battery-charging', 'tone' => 'dark'] : null,
                    $supportsPoe ? ['label' => 'Kullanilan Guc', 'value' => round($usedPoe).' W', 'sub' => '('.($poeBudget > 0 ? round(($usedPoe / $poeBudget) * 100) : 0).'%)', 'icon' => 'bi-plug', 'tone' => 'dark'] : null,
                    ['label' => 'CPU Kullanimi', 'value' => min(95, 18 + $ports->where('status', 'up')->count()).'%', 'progress' => min(95, 18 + $ports->where('status', 'up')->count()), 'icon' => 'bi-cpu', 'tone' => 'success'],
                    ['label' => 'Bellek Kullanimi', 'value' => min(95, 24 + (int) round($ports->map->currentLocation->filter()->count() / 2)).'%', 'progress' => min(95, 24 + (int) round($ports->map->currentLocation->filter()->count() / 2)), 'icon' => 'bi-memory', 'tone' => 'success'],
                ], fn ($item) => $item !== null)),
                'ports' => $portPayload->all(),
                'panelPortPairs' => $panelLayout['pairs'],
                'panelAuxPorts' => $panelLayout['auxiliary'],
                'panelColumnCount' => $panelLayout['column_count'],
                'panelProfile' => $panelProfile,
                'selectedPort' => $selectedPort['id'] ?? null,
                'events' => $this->eventEntries($switch),
                'traffic' => $this->emptyTrafficSeries(),
                'portStatusSegments' => $portStatusSegments,
                'poeSegments' => [
                    ['label' => 'Kullanilan', 'value' => round($usedPoe), 'color' => '#2f6fec'],
                    ['label' => 'Kullanilabilir', 'value' => max(0, $poeBudget - round($usedPoe)), 'color' => '#cbd5e1'],
                ],
            ],
        ];
    }

    public function portPayload(SwitchPort $port, array $topologyLinks = [], array $goPorts = [], array $goDevices = []): array
    {
        $location = $port->currentLocation;
        $endpoint = $location?->endpoint;
        $goPort = $this->goPortForLocalPort($port, $goPorts);
        $goDevice = $this->goDeviceForLocalPort($port, $goDevices);
        $state = $this->portVisualState($port, $endpoint?->status, $goPort, $goDevice);
        $policy = $endpoint?->policy_name ?? $this->goDevicePolicyName($goDevice) ?? '-';
        $connected = $endpoint !== null || $goDevice !== null;
        $speedLabel = $port->speed ?: '0';
        $statusText = $this->portStatusLabel($state);
        $panelPosition = $this->panelPosition($port);
        $topology = $this->topologyForPort($port, $topologyLinks);
        if ($topology === null && $goPort !== null) {
            $topology = $this->topologyFromGoPort($goPort);
        }
        $macCount = (int) ($goPort['mac_count'] ?? $port->mac_count ?? 0);
        $isLinkedUplink = $this->shouldTreatPortAsUplink($topology, $goPort, $macCount, $connected);
        $portTypeLabel = $isLinkedUplink ? 'Uplink' : Str::headline($port->port_type);
        $uplinkSource = $this->uplinkSourceLabel($topology, $goPort, $isLinkedUplink, $macCount);

        $userLabel = $endpoint?->user_name ?? $this->goDeviceUserLabel($goDevice) ?? '-';
        $roleLabel = $endpoint?->role_name ?? $this->goDeviceRoleLabel($goDevice) ?? '-';
        $hostname = $endpoint?->hostname ?? ($goDevice['hostname'] ?? '-') ?: '-';
        $macAddress = $endpoint?->mac_address ?? ($goDevice['mac_address'] ?? '-') ?: '-';
        $ipAddress = $endpoint?->ip_address ?? ($goDevice['current_ip_address'] ?? '-') ?: '-';
        $deviceType = $endpoint?->device_type ?? ($goDevice['device_type'] ?? '-') ?: '-';
        $identitySource = $this->goDeviceIdentitySourceLabel($goDevice) ?? '-';
        $enforcementMethod = $this->goDeviceEnforcementMethodLabel($goDevice) ?? '-';

        $effectiveVlan = $this->effectivePortVlan($port, $location?->vlan_id, $goPort, $goDevice);
        $effectiveMacCount = $this->effectivePortMacCount($goPort, $goDevice, $macAddress ?? null);

        return [
            'id' => $port->id,
            'label' => $port->port_name,
            'state' => $state,
            'statusText' => $statusText,
            'user' => $userLabel,
            'mac' => $macAddress,
            'ip' => $ipAddress,
            'hostname' => $hostname,
            'policy' => $policy,
            'policyText' => strtoupper($policy),
            'vlan' => (string) $effectiveVlan,
            'vlanLabel' => (string) $effectiveVlan.($policy !== '-' ? ' ('.strtoupper($policy).')' : ''),
            'speed' => $speedLabel,
            'speedLabel' => $speedLabel === '0' ? '-' : $speedLabel.' ('.$port->duplex.')',
            'duplex' => $speedLabel === '0' ? '-' : $port->duplex,
            'poe' => $port->poe_enabled ? 'PoE+ ('.rtrim(rtrim((string) $port->poe_power, '0'), '.').' W)' : 'Kapali',
            'lastChange' => optional($port->last_change ?? $port->last_change_at)->format('H:i:s'),
            'lastSeen' => optional($port->last_seen)->format('d.m.Y H:i:s'),
            'if_index' => $port->if_index,
            'admin_status' => $port->admin_status ?: 'unknown',
            'oper_status' => $port->oper_status ?: 'unknown',
            'auth' => $this->authLabel($state),
            'portType' => $portTypeLabel,
            'port_type' => $portTypeLabel,
            'deviceType' => $deviceType,
            'role' => $roleLabel,
            'identitySource' => $identitySource,
            'enforcementMethod' => $enforcementMethod,
            'duration' => $connected ? $this->connectionDuration($location?->first_seen_at) : '-',
            'portNacMode' => $this->nacModeLabel($port->nac_mode),
            'port_nac_mode' => $this->nacModeLabel($port->nac_mode),
            'linkedSwitchName' => $topology['switch_name'] ?? '-',
            'linkedSwitchId' => $topology['switch_id'] ?? '',
            'linkedPortName' => $topology['port_name'] ?? '-',
            'linkedProtocol' => $topology['protocol'] ?? '-',
            'isTopologyLinked' => $isLinkedUplink,
            'isUplink' => $isLinkedUplink,
            'macCount' => $effectiveMacCount,
            'uplinkSource' => $uplinkSource,
            'panel_number' => $panelPosition['number'],
            'panel_label' => $panelPosition['label'],
            'display_order' => $panelPosition['order'],
            'panel_group' => $panelPosition['group'],
        ];
    }

    public function portDetail(SwitchPort $port): array
    {
        $goPorts = $port->switch ? $this->goPortsForSwitch($port->switch) : [];

        return $this->portPayload($port, [], $goPorts) + [
            'status' => $port->status,
            'nac_mode' => $port->nac_mode,
            'switch_id' => $port->switch_id,
            'switch_port_id' => $port->id,
        ];
    }

    protected function portStatusSegments(Collection $ports): array
    {
        $states = $ports->map(fn ($port) => $this->portVisualState($port, $port->currentLocation?->endpoint?->status));

        return [
            ['label' => 'Up', 'value' => $states->filter(fn ($state) => $state === 'up')->count(), 'color' => '#41b349'],
            ['label' => 'Down', 'value' => $states->filter(fn ($state) => $state === 'down')->count(), 'color' => '#94a3b8'],
            ['label' => 'Admin Down', 'value' => $states->filter(fn ($state) => $state === 'admin_down')->count(), 'color' => '#ef4444'],
            ['label' => 'Guest', 'value' => $states->filter(fn ($state) => $state === 'guest')->count(), 'color' => '#facc15'],
            ['label' => 'Quarantine', 'value' => $states->filter(fn ($state) => $state === 'quarantine')->count(), 'color' => '#8e59d1'],
        ];
    }

    protected function portVisualState(SwitchPort $port, ?string $endpointStatus, ?array $goPort = null, ?array $goDevice = null): string
    {
        $adminStatus = strtolower(trim((string) ($port->admin_status ?? '')));
        $operStatus = strtolower(trim((string) ($port->oper_status ?? '')));

        if ($adminStatus !== '' || $operStatus !== '') {
            if ($adminStatus === 'down') {
                return 'admin_down';
            }

            return match ($operStatus) {
                'up' => 'up',
                'down' => 'down',
                default => 'unknown',
            };
        }

        $goPolicy = strtolower(trim((string) ($goDevice['policy_action'] ?? '')));
        $goStatus = strtolower(trim((string) ($goDevice['status'] ?? '')));
        $goAppliedVlan = (int) ($goDevice['applied_enforcement_vlan'] ?? 0);

        if ($goPolicy === 'active' || $goStatus === 'allowed') {
            return 'up';
        }

        if ($goPolicy === 'guest' || in_array($goStatus, ['pending', 'expired'], true)) {
            return 'guest';
        }

        if ($goPolicy === 'blocked' || in_array($goStatus, ['blocked', 'retired'], true)) {
            return $goAppliedVlan > 0 ? 'quarantine' : 'blocked';
        }

        if (is_array($goPort)) {
            $operStatus = strtolower(trim((string) ($goPort['oper_status'] ?? '')));
            $adminStatus = strtolower(trim((string) ($goPort['admin_status'] ?? '')));
            if ($adminStatus === 'up' && $operStatus === 'up') {
                return 'up';
            }
            if ($adminStatus === 'down' || $operStatus === 'down') {
                return 'down';
            }
        }

        if ($port->status === 'disabled') {
            return 'disabled';
        }

        if ($port->status === 'down') {
            return $endpointStatus ? 'down' : 'empty';
        }

        if ($endpointStatus === 'guest') {
            return 'guest';
        }

        if ($endpointStatus === 'quarantine') {
            return 'quarantine';
        }

        if ($endpointStatus === 'unauthorized') {
            return 'blocked';
        }

        if ($port->nac_mode === 'monitor') {
            return 'monitor';
        }

        return 'up';
    }

    protected function switchState(NetworkSwitch $switch): string
    {
        if (! $switch->managed || $switch->status === 'unmanaged') {
            return 'unmanaged';
        }

        return $switch->status;
    }

    protected function switchStatusClass(NetworkSwitch $switch): string
    {
        return match ($this->switchState($switch)) {
            'offline' => 'danger',
            'warning' => 'warning',
            default => 'success',
        };
    }

    protected function switchStatusLabel(NetworkSwitch $switch): string
    {
        return match ($this->switchState($switch)) {
            'offline' => 'Offline',
            'warning' => 'Uyari',
            'unmanaged' => 'Yonetilmiyor',
            default => 'Online',
        };
    }

    protected function switchStatusDetail(NetworkSwitch $switch): string
    {
        return match ($this->switchState($switch)) {
            'offline' => 'SNMP polling basarisiz. Son durum korunuyor.',
            'warning' => 'SNMP polling gecici olarak hata veriyor.',
            'unmanaged' => 'Bu switch yonetilmeyen modda.',
            default => 'SNMP polling aktif ve switch erisilebilir.',
        };
    }

    protected function shouldLoadRemoteSwitchDetailEnrichment(): bool
    {
        return (bool) config('services.nac.switch_detail_remote_enrichment', false);
    }

    protected function portStatusLabel(string $state): string
    {
        return match ($state) {
            'monitor' => 'Monitor Only',
            'blocked' => 'Blocked',
            'down' => 'Down',
            'admin_down' => 'Admin Down',
            'unknown' => 'Unknown',
            'quarantine' => 'Quarantine',
            'guest' => 'Guest',
            'empty' => 'Bos',
            'disabled' => 'Disabled',
            default => 'Online',
        };
    }

    protected function authLabel(string $state): string
    {
        return match ($state) {
            'monitor' => 'Monitor: Riskli Davranis',
            'blocked' => 'Policy Reject',
            'down' => 'Port Down',
            'admin_down' => 'Admin Down',
            'unknown' => 'Status Bilinmiyor',
            'guest' => 'Guest Portal',
            'quarantine' => 'Reddedildi',
            'disabled' => 'Disabled',
            'empty' => 'Bos',
            default => 'Basarili',
        };
    }

    protected function nacModeLabel(string $mode): string
    {
        return match ($mode) {
            'disabled' => 'Disabled',
            'monitor' => 'Monitor Only',
            'enforcement' => 'Enforcement',
            default => 'Inherit',
        };
    }

    protected function uptimeLabel(NetworkSwitch $switch): string
    {
        if (! $switch->created_at) {
            return '-';
        }

        $hours = (int) floor(abs($switch->created_at->diffInHours(now())));
        $days = intdiv($hours, 24);
        $remainingHours = $hours % 24;

        return $days.' gun '.$remainingHours.' saat 0 dk';
    }

    protected function relativeTime($dateTime): string
    {
        if (! $dateTime) {
            return '-';
        }

        $seconds = (int) floor(abs($dateTime->diffInSeconds(now())));

        if ($seconds < 60) {
            return $seconds.' sn once';
        }

        $minutes = intdiv($seconds, 60);

        if ($minutes < 60) {
            return $minutes.' dk once';
        }

        $hours = intdiv($minutes, 60);
        if ($hours < 24) {
            return $hours.' sa once';
        }

        return intdiv($hours, 24).' gun once';
    }

    protected function connectionDuration($firstSeenAt): string
    {
        if (! $firstSeenAt) {
            return '-';
        }

        $minutes = (int) floor(abs($firstSeenAt->diffInMinutes(now())));
        $hours = intdiv($minutes, 60);
        $days = intdiv($hours, 24);
        $remainingHours = $hours % 24;
        $remainingMinutes = $minutes % 60;

        if ($days > 0) {
            return $days.' gun '.$remainingHours.' saat '.$remainingMinutes.' dk';
        }

        if ($hours > 0) {
            return $hours.' saat '.$remainingMinutes.' dk';
        }

        return $remainingMinutes.' dk';
    }

    protected function emptyTrafficSeries(): array
    {
        return [
            ['label' => 'Gelen Trafik', 'value' => 'Veri yok', 'color' => '#2f6fec', 'points' => '8,28 28,28 46,28 64,28 82,28 100,28 118,28 136,28 154,28 172,28 190,28'],
            ['label' => 'Giden Trafik', 'value' => 'Veri yok', 'color' => '#41b349', 'points' => '8,28 28,28 46,28 64,28 82,28 100,28 118,28 136,28 154,28 172,28 190,28'],
            ['label' => 'Toplam Trafik', 'value' => 'Veri yok', 'color' => '#8e59d1', 'points' => '8,28 28,28 46,28 64,28 82,28 100,28 118,28 136,28 154,28 172,28 190,28'],
        ];
    }

    protected function eventEntries(NetworkSwitch $switch): array
    {
        return $switch->auditLogs()
            ->latest('created_at')
            ->limit(5)
            ->get()
            ->map(function ($log) {
                return [
                    'icon' => 'bi-clock-history',
                    'tone' => 'primary',
                    'title' => Str::headline(str_replace('_', ' ', $log->action)),
                    'sub' => $log->ip_address ?: '-',
                    'time' => optional($log->created_at)->format('H:i:s'),
                ];
            })
            ->values()
            ->all();
    }

    protected function topologyLinksForSwitch(NetworkSwitch $switch): array
    {
        $cacheKey = sprintf('nac:switch:%s:topology-links', $switch->id);

        return Cache::remember($cacheKey, now()->addSeconds($this->nacCacheTtlSeconds), function () use ($switch) {
            $client = $this->nacApi();
            if (! $client) {
                return [];
            }

            try {
                $links = $client->topologyLinks();
            } catch (RuntimeException) {
                return [];
            }

            $indexed = [];
            foreach ($links as $link) {
                if (! is_array($link)) {
                    continue;
                }

                $sourceSwitchId = (string) ($link['source_switch_id'] ?? '');
                $targetSwitchId = (string) ($link['target_switch_id'] ?? '');

                if ($sourceSwitchId === (string) $switch->id) {
                    $localPort = (string) ($link['source_port_name'] ?? '');
                    $indexed = $this->storeTopologyLink($indexed, $localPort, [
                        'switch_id' => $targetSwitchId,
                        'switch_name' => (string) ($link['target_switch_name'] ?? ''),
                        'port_name' => (string) ($link['target_port_name'] ?? ''),
                        'protocol' => (string) ($link['discovery_method'] ?? ''),
                    ]);
                }

                if ($targetSwitchId === (string) $switch->id) {
                    $localPort = (string) ($link['target_port_name'] ?? '');
                    $indexed = $this->storeTopologyLink($indexed, $localPort, [
                        'switch_id' => $sourceSwitchId,
                        'switch_name' => (string) ($link['source_switch_name'] ?? ''),
                        'port_name' => (string) ($link['source_port_name'] ?? ''),
                        'protocol' => (string) ($link['discovery_method'] ?? ''),
                    ]);
                }
            }

            return $indexed;
        });
    }

    protected function goPortsForSwitch(NetworkSwitch $switch): array
    {
        $cacheKey = sprintf('nac:switch:%s:go-ports', $switch->id);

        return Cache::remember($cacheKey, now()->addSeconds($this->nacCacheTtlSeconds), function () use ($switch) {
            $client = $this->nacApi();
            if (! $client) {
                return [];
            }

            try {
                $goSwitchId = $this->resolveGoSwitchId($switch);
                if ($goSwitchId === null) {
                    return [];
                }

                $ports = $client->switchPorts($goSwitchId);
            } catch (RuntimeException) {
                return [];
            }

            $indexed = [];
            foreach ($ports as $port) {
                if (! is_array($port)) {
                    continue;
                }

                foreach ($this->goPortCandidates($port['interface_name'] ?? '', $port['port_index'] ?? null, $port['if_index'] ?? null) as $candidate) {
                    if ($candidate === '') {
                        continue;
                    }
                    $indexed[$candidate] = $port;
                }
            }

            return $indexed;
        });
    }

    protected function goPortSummaryForSwitch(NetworkSwitch $switch): array
    {
        $cacheKey = sprintf('nac:switch:%s:go-port-summary', $switch->id);

        return Cache::remember($cacheKey, now()->addSeconds($this->nacCacheTtlSeconds), function () use ($switch) {
            $client = $this->nacApi();
            if (! $client) {
                return [];
            }

            try {
                $goSwitchId = $this->resolveGoSwitchId($switch);
                if ($goSwitchId === null) {
                    return [];
                }

                $summary = $client->switchPortSummary($goSwitchId);
            } catch (RuntimeException) {
                return [];
            }

            return is_array($summary) ? $summary : [];
        });
    }

    protected function goDevicesForSwitch(NetworkSwitch $switch): array
    {
        $cacheKey = sprintf('nac:switch:%s:go-devices', $switch->id);

        return Cache::remember($cacheKey, now()->addSeconds($this->nacCacheTtlSeconds), function () use ($switch) {
            $client = $this->nacApi();
            if (! $client) {
                return [];
            }

            try {
                $goSwitchId = $this->resolveGoSwitchId($switch);
                if ($goSwitchId === null) {
                    return [];
                }

                $devices = $client->devicesBySwitch($goSwitchId);
            } catch (RuntimeException) {
                return [];
            }

            $indexed = [];
            foreach ($devices as $device) {
                if (! is_array($device)) {
                    continue;
                }

                foreach ($this->goDeviceCandidates($device) as $candidate) {
                    if ($candidate === '') {
                        continue;
                    }
                    if (! isset($indexed[$candidate])) {
                        $indexed[$candidate] = [];
                    }
                    $indexed[$candidate][] = $device;
                }
            }

            return $indexed;
        });
    }

    protected function goPortForLocalPort(SwitchPort $port, array $goPorts): ?array
    {
        foreach ($this->goPortCandidates($port->port_name, $port->port_index, $port->if_index) as $candidate) {
            if (isset($goPorts[$candidate]) && is_array($goPorts[$candidate])) {
                return $goPorts[$candidate];
            }
        }

        return null;
    }

    protected function goDeviceForLocalPort(SwitchPort $port, array $goDevices): ?array
    {
        foreach ($this->goPortCandidates((string) $port->port_name, $port->port_index, $port->if_index) as $candidate) {
            if (! isset($goDevices[$candidate]) || ! is_array($goDevices[$candidate])) {
                continue;
            }

            return collect($goDevices[$candidate])
                ->sortByDesc(function (array $device) {
                    $identityScore = trim((string) ($device['identity_full_name'] ?? '')) !== ''
                        || trim((string) ($device['identity_username'] ?? '')) !== ''
                        || trim((string) ($device['identity_type'] ?? '')) !== '' ? 100 : 0;
                    $status = strtolower(trim((string) ($device['status'] ?? '')));
                    $policy = strtolower(trim((string) ($device['policy_action'] ?? '')));

                    $stateScore = match (true) {
                        $status === 'allowed' || $policy === 'active' => 50,
                        $status === 'blocked' || $policy === 'blocked' => 10,
                        $status === 'guest' || $policy === 'guest' => 5,
                        default => 0,
                    };

                    $lastSeen = strtotime((string) ($device['last_seen_at'] ?? '1970-01-01T00:00:00Z')) ?: 0;

                    return ($identityScore * 1000000000000) + ($stateScore * 10000000000) + $lastSeen;
                })
                ->first();
        }

        return null;
    }

    protected function goDeviceUserLabel(?array $device): ?string
    {
        if (! is_array($device)) {
            return null;
        }

        $fullName = trim((string) ($device['identity_full_name'] ?? ''));
        if ($fullName !== '') {
            return $fullName;
        }

        $username = trim((string) ($device['identity_username'] ?? ''));
        if ($username !== '') {
            return $username;
        }

        return null;
    }

    protected function goDeviceRoleLabel(?array $device): ?string
    {
        if (! is_array($device)) {
            return null;
        }

        return match (strtolower(trim((string) ($device['identity_type'] ?? '')))) {
            'personel' => 'Personel',
            'ogrenci' => 'Ogrenci',
            'misafir' => 'Misafir',
            default => null,
        };
    }

    protected function goDevicePolicyName(?array $device): ?string
    {
        if (! is_array($device)) {
            return null;
        }

        $policyAction = trim((string) ($device['policy_action'] ?? ''));
        if ($policyAction === '') {
            return null;
        }

        return $policyAction;
    }

    protected function goDeviceIdentitySourceLabel(?array $device): ?string
    {
        if (! is_array($device)) {
            return null;
        }

        return match (strtolower(trim((string) ($device['identity_source'] ?? '')))) {
            'ldap' => 'LDAP',
            'guest_registry' => 'Guest Registry',
            'panel_guest' => 'Panel Guest',
            'staff_service' => 'Staff Service',
            'student_service' => 'Student Service',
            default => null,
        };
    }

    protected function goDeviceEnforcementMethodLabel(?array $device): ?string
    {
        if (! is_array($device)) {
            return null;
        }

        return match (strtolower(trim((string) ($device['last_enforcement_method'] ?? '')))) {
            'snmp-write' => 'SNMP Write',
            'ssh' => 'SSH',
            'radius-coa' => 'RADIUS CoA',
            'radius-vlan' => 'RADIUS VLAN',
            default => null,
        };
    }

    protected function effectivePortVlan(SwitchPort $port, ?int $locationVlan, ?array $goPort, ?array $goDevice): int
    {
        $appliedVlan = (int) ($goDevice['applied_enforcement_vlan'] ?? 0);
        if ($appliedVlan > 0) {
            return $appliedVlan;
        }

        $goPortVlan = (int) ($goPort['vlan_id'] ?? 0);
        if ($goPortVlan > 0) {
            return $goPortVlan;
        }

        if (($locationVlan ?? 0) > 0) {
            return (int) $locationVlan;
        }

        return (int) ($port->vlan_id ?? 1);
    }

    protected function effectivePortMacCount(?array $goPort, ?array $goDevice, ?string $macAddress): int
    {
        $goPortMacCount = (int) ($goPort['mac_count'] ?? 0);
        if ($goPortMacCount > 0) {
            return $goPortMacCount;
        }

        return filled($macAddress) && $macAddress !== '-' ? 1 : 0;
    }

    protected function goDeviceCandidates(array $device): array
    {
        return $this->goPortCandidates(
            (string) ($device['current_interface_name'] ?? ''),
            null,
            $device['current_if_index'] ?? null
        );
    }

    protected function goPortCandidates(string $name, mixed $portIndex = null, mixed $ifIndex = null): array
    {
        $candidates = $this->topologyPortCandidates($name);

        if ($portIndex !== null && $portIndex !== '') {
            $candidates[] = 'port-index:'.(string) $portIndex;
        }
        if ($ifIndex !== null && $ifIndex !== '') {
            $candidates[] = 'if-index:'.(string) $ifIndex;
        }

        return array_values(array_unique(array_filter($candidates, fn ($value) => $value !== '')));
    }

    protected function uplinkSourceLabel(?array $topology, ?array $goPort, bool $isLinkedUplink, int $macCount): string
    {
        if (! $isLinkedUplink) {
            return '-';
        }

        if ($topology !== null && filled($topology['protocol'] ?? null)) {
            return strtoupper((string) $topology['protocol']);
        }

        if (($goPort['is_trunk'] ?? false) === true) {
            return 'SNMP Trunk';
        }

        if ($isLinkedUplink && $macCount >= 8) {
            return 'MAC Heuristigi';
        }

        if ($isLinkedUplink) {
            return 'Discovery';
        }

        return '-';
    }

    protected function topologyFromGoPort(?array $goPort): ?array
    {
        if (! is_array($goPort)) {
            return null;
        }

        $payload = [
            'switch_id' => $this->normalizeLinkedValue((string) ($goPort['neighbor_switch_id'] ?? '')),
            'switch_name' => $this->normalizeLinkedValue((string) ($goPort['neighbor_switch_name'] ?? '')),
            'port_name' => $this->normalizeLinkedValue((string) ($goPort['neighbor_port_name'] ?? '')),
            'protocol' => $this->normalizeLinkedValue((string) ($goPort['neighbor_protocol'] ?? '')),
        ];

        if (
            ($payload['switch_id'] === '' && $payload['switch_name'] === '')
            || $payload['port_name'] === ''
        ) {
            return null;
        }

        return $payload;
    }

    protected function shouldTreatPortAsUplink(?array $topology, ?array $goPort, int $macCount, bool $connected): bool
    {
        if ($this->hasResolvedSwitchTopology($topology)) {
            return true;
        }

        if (! is_array($goPort)) {
            return false;
        }

        if (($goPort['is_trunk'] ?? false) === true) {
            return true;
        }

        if (($goPort['is_uplink'] ?? false) !== true) {
            return false;
        }

        return $macCount >= 8;
    }

    protected function hasResolvedSwitchTopology(?array $topology): bool
    {
        if (! is_array($topology)) {
            return false;
        }

        return filled($topology['switch_id'] ?? null);
    }

    protected function normalizeLinkedValue(string $value): string
    {
        $normalized = strtolower(trim($value));

        return match ($normalized) {
            '', '-', '--', '0', '0#', 'n/a', 'na', 'none', 'null', 'unknown' => '',
            default => trim($value),
        };
    }

    protected function sanitizeTopologyPayload(?array $payload): ?array
    {
        if (! is_array($payload)) {
            return null;
        }

        $sanitized = [
            'switch_id' => $this->normalizeLinkedValue((string) ($payload['switch_id'] ?? '')),
            'switch_name' => $this->normalizeLinkedValue((string) ($payload['switch_name'] ?? '')),
            'port_name' => $this->normalizeLinkedValue((string) ($payload['port_name'] ?? '')),
            'protocol' => $this->normalizeLinkedValue((string) ($payload['protocol'] ?? '')),
        ];

        if (
            ($sanitized['switch_id'] === '' && $sanitized['switch_name'] === '')
            || $sanitized['port_name'] === ''
        ) {
            return null;
        }

        return $sanitized;
    }

    protected function resolveGoSwitchId(NetworkSwitch $switch): ?string
    {
        $cacheKey = (string) $switch->id;
        if (array_key_exists($cacheKey, $this->goSwitchIdCache)) {
            return $this->goSwitchIdCache[$cacheKey];
        }

        $client = $this->nacApi();
        if (! $client) {
            return $this->goSwitchIdCache[$cacheKey] = null;
        }

        $hostname = strtolower(trim((string) $switch->hostname));
        $ipAddress = trim((string) $switch->ip_address);
        $hostnameIPs = $this->hostnameIPCandidates((string) $switch->hostname);

        try {
            $resolved = $client->resolveSwitch($switch->hostname, $switch->ip_address);
            if (is_array($resolved) && filled($resolved['id'] ?? null)) {
                return $this->goSwitchIdCache[$cacheKey] = (string) $resolved['id'];
            }

            $switches = $client->switches();
        } catch (RuntimeException) {
            return $this->goSwitchIdCache[$cacheKey] = null;
        }

        foreach ($switches as $candidate) {
            if (! is_array($candidate)) {
                continue;
            }

            $candidateName = strtolower(trim((string) ($candidate['name'] ?? '')));
            $candidateIP = trim((string) ($candidate['management_ip'] ?? ''));

            if (
                ($hostname !== '' && $candidateName === $hostname)
                || ($ipAddress !== '' && $candidateIP === $ipAddress)
                || in_array($candidateIP, $hostnameIPs, true)
            ) {
                return $this->goSwitchIdCache[$cacheKey] = (string) ($candidate['id'] ?? '');
            }
        }

        return $this->goSwitchIdCache[$cacheKey] = null;
    }

    protected function nacApi(): ?NacApiClient
    {
        if ($this->nacApiClient instanceof NacApiClient) {
            return $this->nacApiClient;
        }

        if (! function_exists('app')) {
            return null;
        }

        try {
            $resolved = app(NacApiClient::class);
        } catch (\Throwable) {
            return null;
        }

        if ($resolved instanceof NacApiClient) {
            $this->nacApiClient = $resolved;
        }

        return $this->nacApiClient;
    }

    protected function hostnameIPCandidates(string $hostname): array
    {
        $hostname = trim($hostname);
        if ($hostname === '') {
            return [];
        }

        $candidates = [];

        if (preg_match('/(\d{1,3})[-_](\d{1,3})[-_](\d{1,3})[-_](\d{1,3})/', $hostname, $matches) === 1) {
            $ip = implode('.', array_slice($matches, 1, 4));
            if (filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4)) {
                $candidates[] = $ip;
            }
        }

        if (preg_match('/(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/', $hostname, $matches) === 1) {
            $ip = $matches[1];
            if (filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4)) {
                $candidates[] = $ip;
            }
        }

        return array_values(array_unique($candidates));
    }

    protected function storeTopologyLink(array $indexed, string $localPortName, array $payload): array
    {
        $payload = $this->sanitizeTopologyPayload($payload);
        if ($payload === null) {
            return $indexed;
        }

        foreach ($this->topologyPortCandidates($localPortName) as $candidate) {
            if ($candidate === '') {
                continue;
            }

            $indexed[$candidate] = $payload;
        }

        return $indexed;
    }

    protected function topologyForPort(SwitchPort $port, array $topologyLinks): ?array
    {
        foreach ($this->topologyPortCandidates((string) $port->port_name) as $candidate) {
            if (isset($topologyLinks[$candidate])) {
                return $this->sanitizeTopologyPayload($topologyLinks[$candidate]);
            }
        }

        foreach ($this->topologyPortCandidates((string) $port->port_index) as $candidate) {
            if (isset($topologyLinks[$candidate])) {
                return $this->sanitizeTopologyPayload($topologyLinks[$candidate]);
            }
        }

        return null;
    }

    protected function topologyPortCandidates(string $value): array
    {
        $normalized = $this->normalizeTopologyPortName($value);
        if ($normalized === '') {
            return [];
        }

        $candidates = [$normalized];
        $fullMap = [
            'gi' => 'gigabitethernet',
            'gigabitethernet' => 'gi',
            'te' => 'tengigabitethernet',
            'tengigabitethernet' => 'te',
            'fa' => 'fastethernet',
            'fastethernet' => 'fa',
            'eth' => 'ethernet',
            'ethernet' => 'eth',
        ];

        foreach ($fullMap as $from => $to) {
            if (str_starts_with($normalized, $from)) {
                $candidates[] = $to.substr($normalized, strlen($from));
            }
        }

        return array_values(array_unique($candidates));
    }

    protected function normalizeTopologyPortName(string $value): string
    {
        $value = strtolower(trim($value));
        $value = preg_replace('/\s+/', '', $value) ?? '';

        if (str_starts_with($value, 'port')) {
            $value = ltrim(substr($value, 4));
        }

        return $value;
    }

    protected function buildPanelLayout(NetworkSwitch $switch, Collection $ports): array
    {
        $ports = $this->normalizePanelPorts($switch, $ports);
        $primaryLimit = $this->primaryPanelLimit($switch->model);

        $primary = $ports
            ->filter(function (array $port) use ($primaryLimit) {
                if ($primaryLimit === null) {
                    return $port['panel_group'] === 'primary';
                }

                return is_numeric($port['panel_number']) && (int) $port['panel_number'] <= $primaryLimit;
            })
            ->sortBy('display_order')
            ->values();

        $auxiliary = $ports
            ->reject(function (array $port) use ($primaryLimit) {
                if ($primaryLimit === null) {
                    return $port['panel_group'] !== 'primary';
                }

                return is_numeric($port['panel_number']) && (int) $port['panel_number'] <= $primaryLimit;
            })
            ->sortBy('display_order')
            ->values();

        $pairs = $primary
            ->chunk(2)
            ->map(fn (Collection $chunk) => [
                'top' => $chunk->get(0),
                'bottom' => $chunk->get(1),
            ])
            ->values();

        return [
            'pairs' => $pairs->all(),
            'auxiliary' => $auxiliary->all(),
            'column_count' => max(8, $pairs->count()),
        ];
    }

    protected function normalizePanelPorts(NetworkSwitch $switch, Collection $ports): Collection
    {
        $vendor = strtolower((string) $switch->vendor);
        $model = strtolower((string) $switch->model);

        if (! str_contains($vendor, 'cisco')) {
            return $ports->values();
        }

        $filtered = $ports
            ->reject(function (array $port) {
                $label = strtolower((string) ($port['label'] ?? ''));

                return in_array($label, ['gi0/0', 'gigabitethernet0/0'], true);
            })
            ->values();

        if (! str_contains($model, '9k')) {
            return $filtered;
        }

        $primary = $filtered
            ->filter(function (array $port) {
                $label = strtolower((string) ($port['label'] ?? ''));

                return preg_match('/^(gi|gigabitethernet)\d+\/0\/\d+$/', $label) === 1;
            })
            ->values();

        $auxMap = [];

        foreach ($filtered as $port) {
            $label = strtolower((string) ($port['label'] ?? ''));

            if (preg_match('/^(gi|gigabitethernet|te|tengigabitethernet)\d+\/1\/(\d+)$/', $label, $matches) !== 1) {
                continue;
            }

            $slotPort = (int) $matches[2];
            $prefix = $matches[1];
            $key = 'uplink-'.$slotPort;
            $score = str_starts_with($prefix, 'te') ? 2 : 1;

            if (! isset($auxMap[$key]) || $score > $auxMap[$key]['score']) {
                $port['panel_number'] = 48 + $slotPort;
                $port['panel_label'] = (string) (48 + $slotPort);
                $port['display_order'] = 48 + $slotPort;
                $port['panel_group'] = 'auxiliary';
                $auxMap[$key] = [
                    'score' => $score,
                    'port' => $port,
                ];
            }
        }

        return $primary
            ->concat(collect($auxMap)->sortKeys()->map(fn (array $entry) => $entry['port'])->values())
            ->values();
    }

    protected function panelPosition(SwitchPort $port): array
    {
        $name = (string) $port->port_name;
        $primaryLimit = $this->primaryPanelLimit($port->switch?->model);

        if (preg_match('/(?:gi|gigabitethernet|fa|fastethernet|te|tengigabitethernet|eth|ethernet|ge|xge)\s*(\d+)\/(\d+)\/(\d+)$/i', $name, $matches)) {
            $slot = (int) $matches[2];
            $number = (int) $matches[3];
            $panelNumber = $slot === 0 ? $number : ($slot * 48) + $number;
            $group = $slot === 0 ? 'primary' : 'auxiliary';

            if ($primaryLimit !== null && $panelNumber > $primaryLimit) {
                $group = 'auxiliary';
            }

            return [
                'number' => $panelNumber,
                'label' => (string) $panelNumber,
                'order' => $panelNumber,
                'group' => $group,
            ];
        }

        if (preg_match('/(?:gi|gigabitethernet|fa|fastethernet|te|tengigabitethernet|eth|ethernet|ge|xge)\s*(\d+)\/(\d+)$/i', $name, $matches)) {
            $number = (int) $matches[2];

            return [
                'number' => $number,
                'label' => (string) $number,
                'order' => $number,
                'group' => $primaryLimit !== null && $number > $primaryLimit ? 'auxiliary' : 'primary',
            ];
        }

        if (preg_match('/(\d+)\s*$/', $name, $matches)) {
            $number = (int) $matches[1];

            return [
                'number' => $number,
                'label' => (string) $number,
                'order' => $number,
                'group' => $primaryLimit !== null && $number > $primaryLimit ? 'auxiliary' : 'primary',
            ];
        }

        return [
            'number' => $port->port_index,
            'label' => $name,
            'order' => $port->port_index,
            'group' => in_array($port->port_type, ['uplink', 'trunk'], true) ? 'auxiliary' : 'primary',
        ];
    }

    protected function primaryPanelLimit(?string $model): ?int
    {
        $model = strtoupper((string) $model);

        return match (true) {
            str_contains($model, '-48'), str_contains($model, '48G'), str_contains($model, '2530-48') => 48,
            str_contains($model, '-24'), str_contains($model, '24G'), str_contains($model, 'V1910-24') => 24,
            str_contains($model, '-28'), str_contains($model, '28G') => 24,
            default => null,
        };
    }

    protected function poeBudget(NetworkSwitch $switch): int
    {
        $model = strtoupper((string) $switch->model);

        if (! $this->supportsPoe($model)) {
            return 0;
        }

        return max(370, (int) ceil($switch->port_count * 7.8));
    }

    protected function supportsPoe(string $model): bool
    {
        return str_contains($model, 'PWR')
            || str_contains($model, 'POE')
            || str_contains($model, 'POE+');
    }

    protected function panelProfile(NetworkSwitch $switch): array
    {
        $vendor = strtolower((string) $switch->vendor);
        $model = strtolower((string) $switch->model);

        if (str_contains($vendor, 'cisco') && str_contains($model, '9k lite')) {
            return [
                'primary_limit' => 24,
                'main_columns' => 12,
                'aux_columns' => 2,
                'aux_title' => 'SFP / Uplink',
            ];
        }

        if (str_contains($vendor, 'cisco') && str_contains($model, '9k')) {
            return [
                'primary_limit' => 48,
                'main_columns' => 24,
                'aux_columns' => 4,
                'aux_title' => 'SFP / Uplink',
            ];
        }

        if ((str_contains($vendor, 'hp') || str_contains($vendor, 'hpe')) && str_contains($model, '48')) {
            return [
                'primary_limit' => 48,
                'main_columns' => 24,
                'aux_columns' => 2,
                'aux_title' => 'SFP / Uplink',
            ];
        }

        if ((str_contains($vendor, 'hp') || str_contains($vendor, 'hpe')) && str_contains($model, '24')) {
            return [
                'primary_limit' => 24,
                'main_columns' => 12,
                'aux_columns' => 2,
                'aux_title' => 'SFP / Uplink',
            ];
        }

        if (str_contains($vendor, 'huawei') && str_contains($model, '28')) {
            return [
                'primary_limit' => 24,
                'main_columns' => 12,
                'aux_columns' => 2,
                'aux_title' => 'Uplink',
            ];
        }

        $primaryLimit = $this->primaryPanelLimit($switch->model) ?? 24;

        return [
            'primary_limit' => $primaryLimit,
            'main_columns' => max(8, (int) ceil($primaryLimit / 2)),
            'aux_columns' => 2,
            'aux_title' => 'SFP / Uplink',
        ];
    }
}
