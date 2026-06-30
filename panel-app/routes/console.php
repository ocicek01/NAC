<?php

use Illuminate\Foundation\Inspiring;
use Illuminate\Support\Facades\Artisan;
use App\Services\SwitchInventoryImportService;
use App\Services\SnmpPortDiscoveryService;
use App\Models\NetworkSwitch;

Artisan::command('inspire', function () {
    $this->comment(Inspiring::quote());
})->purpose('Display an inspiring quote');

Artisan::command('nac:import-switches {path}', function (string $path, SwitchInventoryImportService $importService) {
    $result = $importService->importFromJsonFile($path);

    $this->info("Aktarilan switch sayisi: {$result['imported_count']}");

    foreach ($result['data'] as $row) {
        $this->line(" - {$row['hostname']} ({$row['ip_address']}) => {$row['zone']} [{$row['port_count']} port]");
    }
})->purpose('Import switch inventory from a JSON file');

Artisan::command('nac:discover-ports {switchId?} {--zone=} {--hostname=}', function (
    SnmpPortDiscoveryService $discoveryService
) {
    $switchId = $this->argument('switchId');
    $query = NetworkSwitch::query()->orderBy('hostname');

    if ($switchId !== null) {
        $query->whereKey($switchId);
    }

    if ($zone = $this->option('zone')) {
        $query->whereHas('zone', fn ($zoneQuery) => $zoneQuery->where('slug', $zone)->orWhere('name', $zone));
    }

    if ($hostname = $this->option('hostname')) {
        $query->where('hostname', $hostname);
    }

    $switches = $query->get();

    if ($switches->isEmpty()) {
        $this->warn('Discovery icin switch bulunamadi.');
        return 1;
    }

    foreach ($switches as $switch) {
        $result = $discoveryService->discover($switch);
        $this->info("{$result['hostname']} ({$result['ip_address']}) => {$result['discovered_ports']} port kesfedildi");
    }

    return 0;
})->purpose('Discover switch ports over SNMP and store them in switch_ports');
