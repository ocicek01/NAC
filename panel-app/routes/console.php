<?php

use App\Models\NetworkSwitch;
use App\Services\SnmpPortDiscoveryService;
use App\Services\SnmpPortStatusPollingService;
use App\Services\SnmpTrapListenerService;
use App\Services\SwitchInventoryImportService;
use Illuminate\Foundation\Inspiring;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\Schedule;

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
    $query = NetworkSwitch::query()
        ->where('managed', true)
        ->whereNotNull('snmp_community')
        ->orderBy('hostname');

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

Artisan::command('nac:poll-port-status {switchId?} {--hostname=}', function (SnmpPortStatusPollingService $pollingService) {
    $switchId = $this->argument('switchId');
    $query = NetworkSwitch::query()
        ->where('managed', true)
        ->whereNotNull('snmp_community')
        ->orderBy('hostname');

    if ($switchId !== null) {
        $query->whereKey($switchId);
    }

    if ($hostname = $this->option('hostname')) {
        $query->where('hostname', $hostname);
    }

    $switches = $query->get();

    if ($switches->isEmpty()) {
        $this->warn('Polling icin switch bulunamadi.');
        return 1;
    }

    foreach ($pollingService->pollAll($switches) as $result) {
        if ($result['ok']) {
            $this->info("{$result['hostname']} => {$result['ports']} port durumu guncellendi");
            continue;
        }

        $this->warn("{$result['hostname']} => {$result['error']}");
    }

    return 0;
})->purpose('Poll live switch port status over SNMP and update switch_ports');

Artisan::command('nac:listen-snmp-traps {--host=} {--port=} {--max-packets=}', function (SnmpTrapListenerService $listenerService) {
    $host = $this->option('host') ?: null;
    $port = $this->option('port') !== null ? (int) $this->option('port') : null;
    $maxPackets = $this->option('max-packets') !== null ? (int) $this->option('max-packets') : null;

    $displayHost = $host ?: (string) config('services.nac.trap_listener_host', '0.0.0.0');
    $displayPort = $port ?: (int) config('services.nac.trap_listener_port', 9162);
    $this->info("SNMP trap listener basliyor: {$displayHost}:{$displayPort}");

    $listenerService->listen($host, $port, $maxPackets, function (array $result) {
        if ($result['ok']) {
            $ingest = $result['result'];
            $this->info(sprintf(
                '%s => %s ifIndex %d %s/%s',
                $result['remote_ip'] ?? '-',
                $ingest['switch_hostname'] ?? ('switch#'.$ingest['switch_id']),
                (int) ($ingest['if_index'] ?? 0),
                $ingest['admin_status'] ?? 'unknown',
                $ingest['oper_status'] ?? 'unknown'
            ));

            return;
        }

        $this->warn(sprintf('%s => %s', $result['remote_ip'] ?? '-', $result['error'] ?? 'Trap islenemedi.'));
    });

    return 0;
})->purpose('Listen for SNMP traps over UDP and update switch_ports');

Schedule::command('nac:poll-port-status')
    ->everyThirtySeconds()
    ->withoutOverlapping();

if ((bool) config('services.nac.discovery_schedule_enabled', true)) {
    $discoveryFrequencyMinutes = max(1, (int) config('services.nac.discovery_schedule_minutes', 10));

    Schedule::command('nac:discover-ports')
        ->cron('*/'.$discoveryFrequencyMinutes.' * * * *')
        ->withoutOverlapping();
}
