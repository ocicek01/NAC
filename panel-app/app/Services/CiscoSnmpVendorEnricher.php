<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use Illuminate\Support\Collection;
use Illuminate\Support\Str;

class CiscoSnmpVendorEnricher implements SnmpVendorEnricher
{
    public function supports(NetworkSwitch $switch): bool
    {
        return Str::contains(strtolower($switch->vendor), 'cisco');
    }

    public function enrich(NetworkSwitch $switch, array $portsByIfIndex): array
    {
        $ports = collect($portsByIfIndex)
            ->reject(function (array $port) {
                $label = strtolower((string) ($port['port_name'] ?? ''));

                return in_array($label, ['gi0/0', 'gigabitethernet0/0'], true);
            })
            ->values();

        $model = strtolower((string) $switch->model);

        if (! str_contains($model, '9k')) {
            return $ports->all();
        }

        $primaryPattern = $this->primaryPatternForModel($model);
        $primary = $ports
            ->filter(fn (array $port) => preg_match($primaryPattern, strtolower((string) ($port['port_name'] ?? ''))) === 1)
            ->values();

        $auxiliary = $this->dedupeAuxiliaryPorts($ports, $primaryPattern);

        return $primary
            ->concat($auxiliary)
            ->sortBy('port_index')
            ->values()
            ->all();
    }

    protected function primaryPatternForModel(string $model): string
    {
        if (str_contains($model, 'lite')) {
            return '/^(gi|gigabitethernet)1\/0\/([1-9]|1[0-9]|2[0-4])$/';
        }

        return '/^(gi|gigabitethernet)1\/0\/([1-9]|[1-3][0-9]|4[0-8])$/';
    }

    protected function dedupeAuxiliaryPorts(Collection $ports, string $primaryPattern): Collection
    {
        $selected = [];

        foreach ($ports as $port) {
            $label = strtolower((string) ($port['port_name'] ?? ''));

            if (preg_match($primaryPattern, $label) === 1) {
                continue;
            }

            if (preg_match('/^(gi|gigabitethernet|te|tengigabitethernet)1\/1\/(\d+)$/', $label, $matches) !== 1) {
                continue;
            }

            $slotPort = (int) $matches[2];
            $prefix = $matches[1];
            $score = str_starts_with($prefix, 'te') ? 2 : 1;
            $existing = $selected[$slotPort] ?? null;

            if ($existing === null || $score > $existing['score']) {
                $selected[$slotPort] = [
                    'score' => $score,
                    'port' => $port,
                ];
            }
        }

        return collect($selected)
            ->sortKeys()
            ->map(fn (array $entry) => $entry['port'])
            ->values();
    }
}
