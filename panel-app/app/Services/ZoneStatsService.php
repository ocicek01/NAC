<?php

namespace App\Services;

use App\Models\Zone;
use Illuminate\Support\Collection;
use Illuminate\Support\Str;

class ZoneStatsService
{
    public function getZoneCollection(): Collection
    {
        return Zone::query()
            ->with([
                'switches.ports.currentLocation.endpoint',
                'switches.auditLogs',
            ])
            ->orderBy('name')
            ->get();
    }

    public function summaryCards(Collection $zones): array
    {
        $switches = $zones->flatMap->switches;
        $ports = $switches->flatMap->ports;
        $locations = $ports->map->currentLocation->filter();
        $endpoints = $locations->map->endpoint->filter()->unique('id');
        $totalPorts = $switches->sum(fn ($switch) => $switch->ports->count() > 0 ? $switch->ports->count() : $switch->port_count);

        return [
            ['label' => 'Toplam Switch', 'value' => $switches->count(), 'icon' => 'bi-diagram-3', 'tone' => 'dark'],
            ['label' => 'Aktif Switch', 'value' => $switches->where('status', 'online')->count(), 'icon' => 'bi-check-circle', 'tone' => 'success'],
            ['label' => 'Pasif Switch', 'value' => $switches->where('status', 'offline')->count(), 'icon' => 'bi-x-circle', 'tone' => 'danger'],
            ['label' => 'Toplam Port', 'value' => $totalPorts, 'icon' => 'bi-hdd-network', 'tone' => 'dark'],
            ['label' => 'UP Port', 'value' => $ports->where('status', 'up')->count(), 'icon' => 'bi-arrow-up-circle', 'tone' => 'success'],
            ['label' => 'DOWN Port', 'value' => $ports->where('status', 'down')->count(), 'icon' => 'bi-arrow-down-circle', 'tone' => 'danger'],
            ['label' => 'Toplam Endpoint', 'value' => $endpoints->count(), 'icon' => 'bi-pc-display', 'tone' => 'dark'],
        ];
    }

    public function zoneCard(Zone $zone): array
    {
        $switches = $zone->switches;
        $ports = $switches->flatMap->ports;
        $locations = $ports->map->currentLocation->filter();
        $endpoints = $locations->map->endpoint->filter()->unique('id');
        $guestCount = $endpoints->where('status', 'guest')->count();
        $onlineCount = $switches->where('status', 'online')->count();
        $offlineCount = $switches->where('status', 'offline')->count();
        $totalPorts = $switches->sum(fn ($switch) => $switch->ports->count() > 0 ? $switch->ports->count() : $switch->port_count);
        $upPorts = $ports->where('status', 'up')->count();
        $downPorts = $ports->where('status', 'down')->count();

        return [
            'id' => $zone->id,
            'name' => $zone->name,
            'slug' => $zone->slug,
            'label' => Str::upper(Str::ascii($zone->name)),
            'status' => $this->healthLabel($zone, $offlineCount),
            'statusClass' => $this->healthStatus($zone, $offlineCount),
            'switch_count' => $switches->count(),
            'online_switch_count' => $onlineCount,
            'offline_switch_count' => $offlineCount,
            'total_ports' => $totalPorts,
            'up_ports' => $upPorts,
            'down_ports' => $downPorts,
            'endpoint_count' => $endpoints->count(),
            'guest_count' => $guestCount,
            'health_status' => $this->healthStatus($zone, $offlineCount),
            'stats' => [
                ['label' => 'Switch', 'value' => $switches->count(), 'tone' => 'dark'],
                ['label' => 'Aktif', 'value' => $onlineCount, 'tone' => 'success'],
                ['label' => 'Pasif', 'value' => $offlineCount, 'tone' => 'danger'],
                ['label' => 'Toplam Port', 'value' => $totalPorts, 'tone' => 'dark'],
                ['label' => 'UP Port', 'value' => $upPorts, 'tone' => 'success'],
                ['label' => 'DOWN Port', 'value' => $downPorts, 'tone' => 'danger'],
                ['label' => 'Endpoint', 'value' => $endpoints->count(), 'tone' => 'primary'],
                ['label' => 'Guest', 'value' => $guestCount, 'tone' => 'secondary'],
            ],
            'switches' => $switches->map(fn ($switch) => app(SwitchStatsService::class)->listItem($switch))->values()->all(),
        ];
    }

    public function zoneDetail(Zone $zone): array
    {
        $card = $this->zoneCard($zone);
        $switches = $zone->switches;
        $ports = $switches->flatMap->ports;
        $endpoints = $ports->map->currentLocation->filter()->map->endpoint->filter()->unique('id');

        $endpointDistribution = [
            ['label' => 'Employee', 'value' => $endpoints->where('role_name', 'Employee')->count(), 'color' => '#2f6fec'],
            ['label' => 'Student', 'value' => $endpoints->where('role_name', 'Student')->count(), 'color' => '#41b349'],
            ['label' => 'Guest', 'value' => $endpoints->where('status', 'guest')->count(), 'color' => '#ff9f1a'],
            ['label' => 'IoT', 'value' => $endpoints->where('role_name', 'IoT')->count(), 'color' => '#8e59d1'],
            ['label' => 'Other', 'value' => $endpoints->whereNotIn('role_name', ['Employee', 'Student', 'IoT'])->where('status', '!=', 'guest')->count(), 'color' => '#7c8798'],
        ];

        $portDistribution = [
            ['label' => 'UP', 'value' => $ports->where('status', 'up')->count(), 'color' => '#41b349'],
            ['label' => 'DOWN', 'value' => $ports->where('status', 'down')->count(), 'color' => '#ef4444'],
        ];

        $topSwitches = $switches->map(function ($switch) {
            $used = $switch->ports->where('status', 'up')->count();
            $total = max(1, $switch->ports->count() > 0 ? $switch->ports->count() : $switch->port_count);

            return [
                'hostname' => $switch->hostname,
                'used' => $used,
                'total' => $total,
                'ratio' => (int) round(($used / $total) * 100),
            ];
        })->sortByDesc('ratio')->take(5)->values()->all();

        return [
            'zone' => $card,
            'kpis' => [
                'switch_count' => $card['switch_count'],
                'online_switch_count' => $card['online_switch_count'],
                'offline_switch_count' => $card['offline_switch_count'],
                'total_ports' => $card['total_ports'],
                'up_ports' => $card['up_ports'],
                'down_ports' => $card['down_ports'],
                'endpoint_count' => $card['endpoint_count'],
                'guest_count' => $card['guest_count'],
            ],
            'switches' => $card['switches'],
            'endpoint_distribution' => $endpointDistribution,
            'port_distribution' => $portDistribution,
            'recent_alerts' => $this->recentAlerts($zone),
            'view' => [
                'summary' => [
                    ['label' => 'Switch Sayisi', 'value' => $card['switch_count'], 'icon' => 'bi-hdd-network', 'tone' => 'dark'],
                    ['label' => 'Aktif Switch', 'value' => $card['online_switch_count'], 'icon' => 'bi-check-circle', 'tone' => 'success'],
                    ['label' => 'Pasif Switch', 'value' => $card['offline_switch_count'], 'icon' => 'bi-x-circle', 'tone' => 'danger'],
                    ['label' => 'Toplam Port', 'value' => $card['total_ports'], 'icon' => 'bi-ethernet', 'tone' => 'dark'],
                    ['label' => 'UP Port', 'value' => $card['up_ports'], 'icon' => 'bi-arrow-up-circle', 'tone' => 'success'],
                    ['label' => 'DOWN Port', 'value' => $card['down_ports'], 'icon' => 'bi-arrow-down-circle', 'tone' => 'danger'],
                    ['label' => 'Toplam Endpoint', 'value' => $card['endpoint_count'], 'icon' => 'bi-pc-display', 'tone' => 'dark'],
                    ['label' => 'Guest Endpoint', 'value' => $card['guest_count'], 'icon' => 'bi-person', 'tone' => 'secondary'],
                ],
                'overview' => [
                    ['label' => 'Toplam Bandwidth Kullanimi', 'value' => 'Veri yok', 'percent' => 0, 'tone' => 'success'],
                    ['label' => 'Ortalama Port Kullanimi', 'value' => $card['total_ports'] > 0 ? round(($card['up_ports'] / $card['total_ports']) * 100).'%' : '0%', 'percent' => $card['total_ports'] > 0 ? (int) round(($card['up_ports'] / $card['total_ports']) * 100) : 0, 'tone' => 'warning'],
                    ['label' => 'Ortalama CPU Kullanimi', 'value' => 'Veri yok', 'percent' => 0, 'tone' => 'success'],
                    ['label' => 'Ortalama Bellek Kullanimi', 'value' => 'Veri yok', 'percent' => 0, 'tone' => 'success'],
                ],
                'security' => [
                    ['label' => 'Quarantine Endpoint', 'value' => $endpoints->where('status', 'quarantine')->count(), 'tone' => 'secondary'],
                    ['label' => 'Unauthorized Endpoint', 'value' => $endpoints->where('status', 'unauthorized')->count(), 'tone' => 'warning'],
                    ['label' => 'Son 24 Saatteki Auth Basarisi', 'value' => '-', 'tone' => 'success'],
                    ['label' => 'Son 24 Saatteki Auth Hatasi', 'value' => '-', 'tone' => 'danger'],
                ],
                'endpointSegments' => $endpointDistribution,
                'portSegments' => $portDistribution,
                'alarms' => $this->recentAlerts($zone),
                'topSwitches' => $topSwitches,
            ],
        ];
    }

    protected function healthStatus(Zone $zone, int $offlineCount): string
    {
        if ($offlineCount > 0 || $zone->status === 'warning') {
            return 'warning';
        }

        if ($zone->status === 'critical') {
            return 'danger';
        }

        return 'success';
    }

    protected function healthLabel(Zone $zone, int $offlineCount): string
    {
        return match ($this->healthStatus($zone, $offlineCount)) {
            'warning' => 'Uyari',
            'danger' => 'Kritik',
            default => 'Normal',
        };
    }

    protected function recentAlerts(Zone $zone): array
    {
        return $zone->switches
            ->flatMap->auditLogs
            ->sortByDesc('created_at')
            ->take(6)
            ->map(function ($log) {
                return [
                    'message' => Str::headline(str_replace('_', ' ', $log->action)),
                    'time' => optional($log->created_at)->format('H:i:s'),
                    'tone' => 'warning',
                ];
            })
            ->values()
            ->all();
    }
}
