<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use App\Models\Zone;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Validator;
use Illuminate\Support\Str;
use Illuminate\Validation\ValidationException;

class SwitchInventoryImportService
{
    public function importFromJsonFile(string $path): array
    {
        if (! is_file($path)) {
            throw ValidationException::withMessages([
                'path' => 'JSON dosyasi bulunamadi: '.$path,
            ]);
        }

        $decoded = json_decode((string) file_get_contents($path), true);

        if (! is_array($decoded)) {
            throw ValidationException::withMessages([
                'path' => 'JSON dosyasi okunamadi ya da gecersiz.',
            ]);
        }

        return $this->import($decoded);
    }

    public function import(array $payload): array
    {
        $validated = Validator::make($payload, [
            'defaults' => ['nullable', 'array'],
            'defaults.status' => ['nullable', 'string'],
            'defaults.snmp_version' => ['nullable', 'string', 'max:20'],
            'defaults.snmp_community' => ['nullable', 'string', 'max:255'],
            'defaults.snmp_port' => ['nullable', 'integer', 'between:1,65535'],
            'defaults.snmp_timeout_ms' => ['nullable', 'integer', 'min:1'],
            'defaults.snmp_retries' => ['nullable', 'integer', 'min:0', 'max:10'],
            'switches' => ['required', 'array', 'min:1'],
            'switches.*.name' => ['required', 'string', 'max:255'],
            'switches.*.management_ip' => ['required', 'ip'],
            'switches.*.vendor' => ['nullable', 'string', 'max:255'],
            'switches.*.model' => ['nullable', 'string', 'max:255'],
        ])->validate();

        $defaults = $validated['defaults'] ?? [];
        $rows = $validated['switches'];

        return DB::transaction(function () use ($rows, $defaults) {
            $imported = [];

            foreach ($rows as $row) {
                $zoneName = $this->resolveZoneName($row['management_ip']);
                $zone = Zone::firstOrCreate(
                    ['slug' => Str::slug($zoneName)],
                    ['name' => $zoneName, 'status' => 'normal']
                );

                $switch = NetworkSwitch::updateOrCreate(
                    ['ip_address' => $row['management_ip']],
                    [
                        'zone_id' => $zone->id,
                        'hostname' => $row['name'],
                        'vendor' => $row['vendor'] ?? '-',
                        'model' => $row['model'] ?? '-',
                        'location' => $zoneName,
                        'status' => $this->mapStatus($defaults['status'] ?? null),
                        'managed' => true,
                        'nac_mode' => 'disabled',
                        'port_count' => $this->inferPortCount($row['model'] ?? ''),
                        'snmp_version' => $defaults['snmp_version'] ?? null,
                        'snmp_community' => $defaults['snmp_community'] ?? null,
                        'snmp_port' => $defaults['snmp_port'] ?? 161,
                        'snmp_timeout_ms' => $defaults['snmp_timeout_ms'] ?? 2000,
                        'snmp_retries' => $defaults['snmp_retries'] ?? 1,
                        'last_seen_at' => null,
                    ]
                );

                $imported[] = [
                    'id' => $switch->id,
                    'hostname' => $switch->hostname,
                    'ip_address' => $switch->ip_address,
                    'zone' => $zone->name,
                    'port_count' => $switch->port_count,
                ];
            }

            return [
                'imported_count' => count($imported),
                'data' => $imported,
            ];
        });
    }

    protected function resolveZoneName(string $ipAddress): string
    {
        $parts = explode('.', $ipAddress);
        $lastOctet = (int) end($parts);

        return match (true) {
            $lastOctet >= 2 && $lastOctet <= 6 => 'Merkezi Derslikler',
            $lastOctet >= 10 && $lastOctet <= 19 => 'Kutuphane',
            default => throw ValidationException::withMessages([
                'management_ip' => "IP adresi icin zone eslemesi bulunamadi: {$ipAddress}",
            ]),
        };
    }

    protected function mapStatus(?string $status): string
    {
        return match (strtolower((string) $status)) {
            'active', 'online' => 'online',
            'offline', 'inactive' => 'offline',
            'warning' => 'warning',
            'unmanaged' => 'unmanaged',
            default => 'online',
        };
    }

    protected function inferPortCount(string $model): int
    {
        $upper = strtoupper($model);

        return match (true) {
            str_contains($upper, '-48'),
            str_contains($upper, '48G'),
            str_contains($upper, '9K') => 48,
            str_contains($upper, '-28'),
            str_contains($upper, '28G') => 28,
            str_contains($upper, '-24'),
            str_contains($upper, '24G') => 24,
            default => 24,
        };
    }
}
