@php
    $navItems = [
        ['label' => 'Dashboard', 'icon' => 'bi-house-door'],
        ['label' => 'Switches', 'icon' => 'bi-diagram-3', 'active' => true],
        ['label' => 'Endpoints', 'icon' => 'bi-display'],
        ['label' => 'Policies', 'icon' => 'bi-journal-check'],
        ['label' => 'RADIUS Events', 'icon' => 'bi-broadcast-pin'],
        ['label' => 'Guests', 'icon' => 'bi-people'],
        ['label' => 'Investigation', 'icon' => 'bi-search'],
        ['label' => 'Reports', 'icon' => 'bi-bar-chart'],
        ['label' => 'Settings', 'icon' => 'bi-gear'],
        ['label' => 'System', 'icon' => 'bi-gear-wide-connected'],
    ];

    $legendItems = [
        ['label' => 'Authenticated', 'color' => '#41b349'],
        ['label' => 'Guest', 'color' => '#facc15'],
        ['label' => 'Quarantine', 'color' => '#8e59d1'],
        ['label' => 'Blocked', 'color' => '#ef4444'],
        ['label' => 'Down', 'color' => '#94a3b8'],
        ['label' => 'Monitor Only', 'color' => '#2f6fec'],
        ['label' => 'Uplink', 'color' => '#18b7c9'],
    ];

    $statusLabels = [
        'up' => 'Online',
        'monitor' => 'Monitor Only',
        'blocked' => 'Blocked',
        'down' => 'Down',
        'quarantine' => 'Quarantine',
        'guest' => 'Guest',
        'empty' => 'Bos',
        'disabled' => 'Disabled',
    ];

    $stateDots = [
        'up' => '#41b349',
        'monitor' => '#2f6fec',
        'blocked' => '#ef4444',
        'down' => '#94a3b8',
        'quarantine' => '#8e59d1',
        'guest' => '#facc15',
        'empty' => '#b8c3d3',
        'disabled' => '#677489',
    ];

    $nacModeDescriptions = [
        'Disabled' => 'NAC aktif degil. Sadece cihaz kesfi yapilir.',
        'Monitor Only' => 'NAC kararlari uretilir, kullanicilara mudahale edilmez.',
        'Enforcement' => 'NAC politikalari aktif olarak uygulanir.',
    ];

    $portPayload = collect($switchData['ports'])->values();
    $panelProfile = $switchData['panelProfile'] ?? [
        'primary_limit' => 24,
        'main_columns' => 12,
        'aux_columns' => 2,
        'aux_title' => 'SFP / Uplink',
    ];
    $panelPrimaryLimit = $panelProfile['primary_limit'] ?? 24;
    $panelMainPorts = $portPayload
        ->filter(fn ($port) => is_numeric($port['panel_number'] ?? null) && ($panelPrimaryLimit === null || (int) $port['panel_number'] <= $panelPrimaryLimit))
        ->sortBy('panel_number')
        ->values();
    $panelAuxPorts = $portPayload
        ->reject(fn ($port) => is_numeric($port['panel_number'] ?? null) && ($panelPrimaryLimit === null || (int) $port['panel_number'] <= $panelPrimaryLimit))
        ->sortBy('panel_number')
        ->values();
    $panelPortPairs = collect();
    foreach (range(1, max(1, $panelMainPorts->count()), 2) as $panelNumber) {
        $panelPortPairs->push([
            'top' => $panelMainPorts->firstWhere('panel_number', $panelNumber),
            'bottom' => $panelMainPorts->firstWhere('panel_number', $panelNumber + 1),
        ]);
    }
    $panelColumnCount = $panelProfile['main_columns'] ?? max(8, $panelPortPairs->count());
    $selectedPort = $portPayload->firstWhere('id', $switchData['selectedPort']) ?? $portPayload->first() ?? [
        'label' => '-',
        'port_type' => '-',
        'statusText' => '-',
        'vlanLabel' => '-',
        'speedLabel' => '-',
        'duplex' => '-',
        'poe' => '-',
        'port_nac_mode' => '-',
        'user' => '-',
        'mac' => '-',
        'ip' => '-',
        'hostname' => '-',
        'deviceType' => '-',
        'policyText' => '-',
        'role' => '-',
        'identitySource' => '-',
        'enforcementMethod' => '-',
        'duration' => '-',
        'state' => 'down',
        'macCount' => 0,
        'uplinkSource' => '-',
    ];
    $pendingValue = static fn ($value, string $hint) => filled($value) && $value !== '-' ? $value : $hint;
    $selectedUser = $pendingValue($selectedPort['user'] ?? '-', 'RADIUS verisi bekleniyor');
    $selectedDeviceType = $pendingValue($selectedPort['deviceType'] ?? '-', 'Profiling verisi bekleniyor');
    $selectedPolicy = $pendingValue($selectedPort['policyText'] ?? '-', 'NAC policy verisi bekleniyor');
    $selectedRole = $pendingValue($selectedPort['role'] ?? '-', 'NAC role verisi bekleniyor');
    $selectedPortInfo = [
        ['label' => 'Port', 'key' => 'label', 'value' => $selectedPort['label']],
        ['label' => 'Port Tipi', 'key' => 'portType', 'value' => $selectedPort['port_type'] ?? '-'],
        ['label' => 'Status', 'key' => 'statusText', 'value' => $statusLabels[$selectedPort['state']] ?? ucfirst($selectedPort['state'])],
        ['label' => 'VLAN', 'key' => 'vlanLabel', 'value' => $selectedPort['vlanLabel'] ?? '-'],
        ['label' => 'Speed', 'key' => 'speedLabel', 'value' => $selectedPort['speedLabel'] ?? '-'],
        ['label' => 'Duplex', 'key' => 'duplex', 'value' => $selectedPort['duplex'] ?? '-'],
        ['label' => 'PoE Durumu', 'key' => 'poe', 'value' => $selectedPort['poe'] ?? '-'],
        ['label' => 'NAC Mode', 'key' => 'portNacMode', 'value' => $selectedPort['port_nac_mode'] ?? '-'],
        ['label' => 'MAC Count', 'key' => 'macCount', 'value' => (string) ($selectedPort['macCount'] ?? 0)],
        ['label' => 'Uplink Kaynagi', 'key' => 'uplinkSource', 'value' => $selectedPort['uplinkSource'] ?? '-'],
        ['label' => 'Komsu Switch', 'key' => 'linkedSwitchName', 'value' => $selectedPort['linkedSwitchName'] ?? '-'],
        ['label' => 'Komsu Port', 'key' => 'linkedPortName', 'value' => $selectedPort['linkedPortName'] ?? '-'],
        ['label' => 'Kesisim Protokolu', 'key' => 'linkedProtocol', 'value' => $selectedPort['linkedProtocol'] ?? '-'],
    ];
    $endpointInfo = [
        ['label' => 'Kullanici', 'key' => 'user', 'value' => $selectedUser],
        ['label' => 'MAC Address', 'key' => 'mac', 'value' => $selectedPort['mac']],
        ['label' => 'IP Address', 'key' => 'ip', 'value' => $selectedPort['ip']],
        ['label' => 'Hostname', 'key' => 'hostname', 'value' => $selectedPort['hostname']],
        ['label' => 'Cihaz Turu', 'key' => 'deviceType', 'value' => $selectedDeviceType],
        ['label' => 'Policy', 'key' => 'policyText', 'value' => $selectedPolicy],
        ['label' => 'Role', 'key' => 'role', 'value' => $selectedRole],
        ['label' => 'Identity Source', 'key' => 'identitySource', 'value' => $selectedPort['identitySource'] ?? '-'],
        ['label' => 'Enforcement', 'key' => 'enforcementMethod', 'value' => $selectedPort['enforcementMethod'] ?? '-'],
        ['label' => 'Baglanti Suresi', 'key' => 'duration', 'value' => $selectedPort['duration'] ?? '-'],
    ];

    $defaultAllowVlan = (int) config('services.nac.default_allow_vlan', 106);
    $guestVlan = (int) config('services.nac.guest_vlan', 300);
    $quarantineVlan = (int) config('services.nac.quarantine_vlan', 333);
    $availableVlans = $portPayload
        ->map(fn ($port) => (int) ($port['vlan'] ?? 0))
        ->filter(fn ($vlan) => $vlan > 0)
        ->merge([$defaultAllowVlan, $guestVlan, $quarantineVlan, 1])
        ->unique()
        ->sort()
        ->values();

    $portStatusSegments = [
        ['label' => 'Up', 'value' => collect($switchData['ports'])->where('state', 'up')->count(), 'color' => '#41b349'],
        ['label' => 'Down', 'value' => collect($switchData['ports'])->where('state', 'down')->count(), 'color' => '#94a3b8'],
        ['label' => 'Disabled', 'value' => collect($switchData['ports'])->where('state', 'disabled')->count(), 'color' => '#677489'],
        ['label' => 'Guest', 'value' => collect($switchData['ports'])->where('state', 'guest')->count(), 'color' => '#facc15'],
        ['label' => 'Quarantine', 'value' => collect($switchData['ports'])->where('state', 'quarantine')->count(), 'color' => '#8e59d1'],
    ];

    $portStatusTotal = array_sum(array_column($portStatusSegments, 'value'));
    $poeTotal = array_sum(array_column($switchData['poeSegments'], 'value'));

    $statusStops = [];
    $offset = 0;
    foreach ($portStatusSegments as $segment) {
        $size = $portStatusTotal > 0 ? round(($segment['value'] / $portStatusTotal) * 100, 2) : 0;
        $statusStops[] = "{$segment['color']} {$offset}% " . ($offset + $size) . '%';
        $offset += $size;
    }
    $statusChart = 'conic-gradient(' . implode(', ', $statusStops) . ')';

    $poeStops = [];
    $offset = 0;
    foreach ($switchData['poeSegments'] as $segment) {
        $size = $poeTotal > 0 ? round(($segment['value'] / $poeTotal) * 100, 2) : 0;
        $poeStops[] = "{$segment['color']} {$offset}% " . ($offset + $size) . '%';
        $offset += $size;
    }
    $poeChart = 'conic-gradient(' . implode(', ', $poeStops) . ')';
@endphp
<!DOCTYPE html>
<html lang="tr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NAC Panel | Switch Detail</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.3/font/bootstrap-icons.css" rel="stylesheet">
    <style>
        :root {
            --sidebar-bg: #252525;
            --sidebar-active: #3a3a3a;
            --sidebar-border: rgba(255, 255, 255, 0.08);
            --sidebar-text: #f5f7fc;
            --page-bg: #f4f7fb;
            --surface-soft: #f8fbfe;
            --line: #d7e1ee;
            --heading: #13233d;
            --body: #5d6b80;
            --success: #41b349;
            --warning: #ff9f1a;
            --danger: #ef4444;
            --primary: #2f6fec;
            --secondary: #8e59d1;
        }
        * { box-sizing: border-box; }
        body {
            margin: 0;
            font-family: "Segoe UI", Tahoma, Geneva, Verdana, sans-serif;
            background: radial-gradient(circle at top left, rgba(80,127,174,.12), transparent 16%), linear-gradient(180deg,#f8fafc 0%,#f4f7fb 100%);
            color: var(--heading);
        }
        .app-shell { min-height: 100vh; display: flex; }
        .app-sidebar {
            width: 228px; min-width: 228px; background: linear-gradient(180deg, rgba(255,255,255,.02), transparent 18%), var(--sidebar-bg);
            color: var(--sidebar-text); padding: 18px 12px; display: flex; flex-direction: column; overflow-y: auto; position: sticky; top: 0; height: 100vh;
        }
        .sidebar-brand { padding: 6px 6px 18px; }
        .brand-row { display:flex; align-items:center; gap:12px; margin-bottom:18px; }
        .brand-icon { width:42px; height:42px; border-radius:12px; border:1px solid rgba(255,255,255,.12); display:inline-flex; align-items:center; justify-content:center; font-size:1.05rem; }
        .brand-title { font-size:.92rem; font-weight:800; letter-spacing:.08em; }
        .sidebar-nav-label { color: rgba(255,255,255,.48); letter-spacing:.18em; font-size:.68rem; font-weight:700; padding: 0 14px 8px; }
        .sidebar-nav { display:grid; gap:4px; }
        .sidebar-link { display:flex; align-items:center; justify-content:space-between; gap:12px; color:var(--sidebar-text); text-decoration:none; padding:12px 14px; border-radius:16px; transition:.18s ease; }
        .sidebar-link:hover { background: rgba(255,255,255,.04); color:#fff; }
        .sidebar-link.is-active { background:#1f465d; box-shadow: inset 0 0 0 1px rgba(93,198,255,.18); }
        .sidebar-link .label-wrap { display:flex; align-items:center; gap:10px; }
        .sidebar-link i { width:18px; text-align:center; }
        .sidebar-link .dot { width:9px; height:9px; border-radius:999px; background: rgba(255,255,255,.18); }
        .sidebar-link.is-active .dot { background:#67d1ff; box-shadow:0 0 0 5px rgba(103,209,255,.14); }
        .legend-box { margin-top:auto; border:1px solid var(--sidebar-border); border-radius:18px; padding:12px 14px; background: rgba(255,255,255,.03); }
        .legend-item { display:flex; align-items:center; gap:10px; color: rgba(255,255,255,.84); font-size:.88rem; }
        .legend-item + .legend-item { margin-top:10px; }
        .legend-dot { width:11px; height:11px; border-radius:999px; }
        main { flex:1; padding:10px 12px 14px; min-width:0; }
        .topbar, .card-shell {
            border:1px solid var(--line); border-radius:14px; background:#fff; box-shadow:0 8px 18px rgba(19,35,61,.04);
        }
        .topbar { min-height:56px; display:flex; align-items:center; justify-content:space-between; gap:16px; padding:8px 12px; margin-bottom:10px; }
        .toolbar-left, .toolbar-right { display:flex; align-items:center; gap:12px; }
        .menu-trigger, .icon-btn { width:40px; height:40px; border-radius:12px; border:1px solid var(--line); background:#fff; display:inline-flex; align-items:center; justify-content:center; color:var(--heading); text-decoration:none; position:relative; overflow:visible; }
        .search-wrap { width:min(100%, 280px); display:flex; align-items:center; gap:10px; border:1px solid var(--line); border-radius:12px; min-height:40px; padding:0 12px; background:#fff; }
        .search-wrap input { border:0; outline:0; background:transparent; width:100%; color:var(--heading); }
        .notify-badge { position:absolute; top:-5px; right:-4px; min-width:18px; height:18px; border-radius:999px; background:#295dd8; color:#fff; font-size:.68rem; display:inline-flex; align-items:center; justify-content:center; font-weight:700; border:2px solid #fff; }
        .profile-chip, .secondary-btn { min-height:40px; border-radius:12px; border:1px solid var(--line); background:#fff; padding:0 12px; display:inline-flex; align-items:center; gap:10px; text-decoration:none; color:var(--heading); font-weight:600; }
        .breadcrumb-line { color: var(--body); font-size:.92rem; }
        .title-row { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; margin: 8px 0 10px; }
        .switch-heading { display:flex; align-items:flex-start; gap:14px; }
        .status-indicator { width:14px; height:14px; border-radius:999px; background:var(--success); margin-top:8px; }
        .status-badge {
            min-width: 90px;
            min-height: 32px;
            padding: 0 14px;
            border-radius: 10px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            font-weight: 700;
            font-size: .92rem;
            border: 1px solid transparent;
            margin-top: 4px;
        }
        .status-badge.success { background: rgba(65,179,73,.08); color: var(--success); border-color: rgba(65,179,73,.22); }
        .status-badge.warning { background: rgba(255,159,26,.08); color: var(--warning); border-color: rgba(255,159,26,.22); }
        .status-badge.danger { background: rgba(239,68,68,.08); color: var(--danger); border-color: rgba(239,68,68,.22); }
        .switch-heading h1 { margin:0; font-size:1.6rem; font-weight:800; }
        .switch-sub { margin-top:4px; color: var(--body); font-size:.96rem; }
        .switch-sub span + span::before { content:"|"; margin:0 10px; color:#9aa5b5; }
        .nac-card {
            border:1px solid var(--line);
            border-radius:14px;
            background:#fff;
            padding:12px 14px;
            box-shadow:0 8px 18px rgba(19,35,61,.04);
            margin-bottom:12px;
        }
        .nac-head { display:flex; align-items:flex-start; justify-content:space-between; gap:14px; margin-bottom:12px; }
        .nac-title h2 { margin:0; font-size:1rem; font-weight:800; }
        .nac-title p { margin:4px 0 0; color:var(--body); font-size:.88rem; }
        .mode-grid { display:grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap:12px; }
        .mode-option { border:1px solid var(--line); border-radius:12px; padding:12px; background:#fbfdff; }
        .mode-option.is-active { border-color:#9bb6d8; box-shadow: inset 0 0 0 1px rgba(47,111,236,.14); background:#f8fbff; }
        .mode-name { font-size:.92rem; font-weight:700; margin-bottom:4px; }
        .mode-desc { color:var(--body); font-size:.82rem; line-height:1.45; }
        .action-row { display:flex; gap:10px; flex-wrap:wrap; margin-top:12px; }
        .action-btn {
            min-height:38px; border-radius:10px; border:1px solid var(--line); background:#fff; padding:0 12px;
            display:inline-flex; align-items:center; gap:8px; color:var(--heading); text-decoration:none; font-weight:600; font-size:.9rem;
        }
        .action-btn.primary { background:#1f465d; border-color:#1f465d; color:#fff; }
        .action-btn.warning { border-color:rgba(255,159,26,.22); background:rgba(255,159,26,.08); color:#a05d08; }
        .action-btn.danger { border-color:rgba(239,68,68,.22); background:rgba(239,68,68,.08); color:#b53838; }
        .port-action-strip { display:flex; gap:10px; flex-wrap:wrap; margin-top:14px; }
        .port-action-strip form { margin:0; }
        .port-action-strip button {
            min-height:38px; border-radius:10px; border:1px solid var(--line); background:#fff; padding:0 12px;
            display:inline-flex; align-items:center; gap:8px; color:var(--heading); text-decoration:none; font-weight:700; font-size:.9rem;
        }
        .port-action-strip .allow-btn { background:#e9f8ec; border-color:rgba(65,179,73,.24); color:#1f7f31; }
        .port-action-strip .guest-btn { background:#fff7df; border-color:rgba(250,204,21,.34); color:#a06b00; }
        .port-action-strip .block-btn { background:#fff0f0; border-color:rgba(239,68,68,.24); color:#b53838; }
        .port-action-strip button:disabled { opacity:.45; cursor:not-allowed; }
        .summary-row { display:grid; grid-template-columns: repeat(9, minmax(0, 1fr)); border:1px solid var(--line); border-radius:14px; background:#fff; padding:6px 8px; box-shadow:0 8px 18px rgba(19,35,61,.04); margin-bottom:12px; }
        .summary-card { display:flex; align-items:center; gap:12px; padding:10px 12px; min-height:76px; position:relative; }
        .summary-card + .summary-card::before { content:""; position:absolute; left:0; top:18px; bottom:18px; width:1px; background:var(--line); }
        .summary-icon { width:38px; height:38px; border-radius:10px; display:inline-flex; align-items:center; justify-content:center; background:var(--surface-soft); font-size:1.1rem; }
        .summary-title { display:block; font-size:.8rem; font-weight:600; margin-bottom:2px; }
        .summary-value { display:block; font-size:1.08rem; font-weight:800; }
        .summary-sub { color:#7a8798; font-size:.8rem; }
        .tone-success .summary-icon, .text-success { color:var(--success)!important; }
        .tone-danger .summary-icon, .text-danger { color:var(--danger)!important; }
        .tone-dark .summary-icon, .text-dark { color:var(--heading)!important; }
        .tone-secondary .summary-icon, .text-secondary { color:var(--secondary)!important; }
        .tone-primary .summary-icon, .text-primary { color:var(--primary)!important; }
        .tabs-row { display:flex; gap:22px; margin: 8px 0 12px; padding: 0 8px; }
        .tab-link { color:#495b75; text-decoration:none; font-weight:500; padding: 8px 0; border-bottom:2px solid transparent; }
        .tab-link.is-active { color:var(--primary); border-bottom-color:var(--primary); }
        .detail-grid { display:flex; gap:12px; align-items:flex-start; }
        .main-column { flex:1 1 auto; min-width:0; display:grid; gap:12px; }
        .side-column { flex:0 0 330px; min-width:330px; display:grid; gap:12px; }
        .card-head { display:flex; align-items:center; justify-content:space-between; gap:12px; padding:12px 14px; border-bottom:1px solid var(--line); }
        .card-head h2, .card-head h3 { margin:0; font-size:1rem; font-weight:800; }
        .switch-toolbar { display:flex; align-items:center; gap:10px; }
        .switch-select { min-height:36px; border:1px solid var(--line); border-radius:10px; padding:0 12px; background:#fff; color:var(--heading); }
        .toggle { display:flex; align-items:center; gap:10px; font-size:.9rem; color:#55647a; }
        .toggle-pill { width:38px; height:22px; border-radius:999px; background:#2f6fec; position:relative; }
        .toggle-pill::after { content:""; position:absolute; top:3px; right:3px; width:16px; height:16px; background:#fff; border-radius:50%; }
        .icon-square { width:36px; height:36px; border-radius:10px; border:1px solid var(--line); display:inline-flex; align-items:center; justify-content:center; color:var(--heading); }
        .switch-map-wrap { padding:14px; }
        .front-panel { background: linear-gradient(180deg,#2d2d2d 0%,#212121 100%); border-radius:14px; padding:18px 16px 16px; color:#fff; }
        .panel-header { display:flex; align-items:flex-start; justify-content:space-between; gap:14px; margin-bottom:12px; }
        .vendor-block .vendor { font-size:1.6rem; font-weight:700; color:#f59e0b; line-height:1; }
        .vendor-block .model { margin-top:10px; font-size:.95rem; color:#d8dde7; }
        .ports-area { display:grid; grid-template-columns: 1fr 120px; gap:16px; align-items:center; }
        .ports-grid { display:grid; gap:8px; }
        .port-pair { display:grid; gap:4px; }
        .port-num { font-size:.72rem; text-align:center; color:#d8dde7; min-height:16px; }
        .port-tile { height:28px; border-radius:4px; border:2px solid rgba(255,255,255,.18); display:flex; align-items:center; justify-content:center; position:relative; cursor:pointer; }
        .port-tile::before { content:""; width:12px; height:10px; border:2px solid rgba(0,0,0,.28); border-top-width:4px; background: rgba(255,255,255,.65); }
        .port-tile.is-placeholder { opacity:0; cursor:default; pointer-events:none; }
        .port-tile.is-placeholder::before { display:none; }
        .port-tile.state-up { background:#41b349; }
        .port-tile.state-monitor { background:#2f6fec; }
        .port-tile.state-blocked { background:#ef4444; }
        .port-tile.state-down { background:#94a3b8; }
        .port-tile.state-quarantine { background:#8e59d1; }
        .port-tile.state-guest { background:#facc15; }
        .port-tile.state-empty { background:#b8c3d3; }
        .port-tile.state-disabled { background:#677489; }
        .port-tile.is-uplink-role {
            background:linear-gradient(180deg, #58e3f0 0%, #19bfd3 100%);
            color:#062c33;
            border-color:#8af3ff;
            box-shadow:0 0 0 2px rgba(24,183,201,.7), 0 8px 18px rgba(24,183,201,.28), inset 0 0 0 1px rgba(255,255,255,.38);
        }
        .port-tile.is-uplink-role::after { content:""; position:absolute; inset:2px; border-radius:2px; border:1px solid rgba(6,31,36,.35); pointer-events:none; }
        .port-tile.is-selected { outline:2px solid #fff; box-shadow:0 0 0 2px #2f6fec; }
        .uplink-block { display:grid; gap:8px; }
        .uplink-title { font-size:.72rem; letter-spacing:.08em; text-transform:uppercase; color:#d8dde7; text-align:center; }
        .uplink-grid { display:grid; gap:10px; }
        .uplink-port { height:34px; border-radius:6px; border:2px solid rgba(255,255,255,.18); background:#111; display:flex; align-items:center; justify-content:center; color:#fff; font-size:.8rem; }
        .uplink-port.state-up { background:#41b349; }
        .uplink-port.state-monitor { background:#2f6fec; }
        .uplink-port.state-blocked { background:#ef4444; }
        .uplink-port.state-down { background:#94a3b8; }
        .uplink-port.state-quarantine { background:#8e59d1; }
        .uplink-port.state-guest { background:#facc15; }
        .uplink-port.state-empty { background:#b8c3d3; color:#13233d; }
        .uplink-port.state-disabled { background:#677489; }
        .uplink-port.is-uplink-role {
            background:linear-gradient(180deg, #58e3f0 0%, #19bfd3 100%);
            color:#062c33;
            border-color:#8af3ff;
            box-shadow:0 0 0 2px rgba(24,183,201,.7), 0 8px 18px rgba(24,183,201,.28), inset 0 0 0 1px rgba(255,255,255,.32);
        }
        .uplink-port.is-selected { outline:2px solid #fff; box-shadow:0 0 0 2px #2f6fec; }
        .panel-legend { display:flex; flex-wrap:wrap; gap:18px; padding:12px 0 0; align-items:center; font-size:.88rem; color:#55647a; }
        .panel-legend .dot { width:12px; height:12px; border-radius:999px; display:inline-block; margin-right:8px; }
        .panel-update { margin-left:auto; }
        .lower-grid { display:grid; grid-template-columns: 1.15fr 1fr; gap:12px; }
        .info-dual { display:grid; grid-template-columns: 1fr 1fr; }
        .info-col { padding:14px 16px; }
        .info-col + .info-col { border-left:1px solid var(--line); }
        .badge-port { display:inline-flex; align-items:center; justify-content:center; min-width:78px; min-height:32px; border-radius:8px; border:1px solid rgba(65,179,73,.22); background:rgba(65,179,73,.08); color:var(--success); font-weight:700; margin-right:10px; }
        .mini-state { display:inline-flex; align-items:center; gap:8px; color:#495b75; font-weight:600; }
        .mini-state .dot { width:11px; height:11px; border-radius:999px; background:var(--success); }
        .info-list { margin-top:12px; display:grid; gap:9px; }
        .info-item { display:grid; grid-template-columns: 120px 1fr; gap:10px; font-size:.9rem; }
        .endpoint-btn { margin-top:16px; min-height:36px; border-radius:10px; border:1px solid var(--line); background:#fff; padding:0 14px; display:inline-flex; align-items:center; gap:8px; color:var(--heading); text-decoration:none; font-weight:600; }
        .event-list, .traffic-list { display:grid; gap:10px; padding:14px 16px; }
        .event-row { display:grid; grid-template-columns: 28px 1fr auto; gap:12px; align-items:flex-start; }
        .event-icon { width:28px; height:28px; border-radius:50%; display:inline-flex; align-items:center; justify-content:center; background:#eef4ff; }
        .traffic-row { display:grid; grid-template-columns: 1fr auto 120px; gap:12px; align-items:center; font-size:.9rem; }
        .mini-line svg { width:120px; height:26px; }
        .switch-info-list { display:grid; gap:10px; padding:14px 16px; font-size:.9rem; }
        .switch-info-row { display:grid; grid-template-columns: 120px 1fr; gap:10px; }
        .donut-layout { display:grid; grid-template-columns: 130px 1fr; gap:12px; align-items:center; padding:14px 16px; }
        .donut-chart { width:120px; height:120px; border-radius:50%; display:grid; place-items:center; position:relative; margin:0 auto; }
        .donut-chart::before { content:""; width:72px; height:72px; border-radius:50%; background:#fff; box-shadow: inset 0 0 0 1px var(--line); }
        .donut-center { position:absolute; text-align:center; }
        .donut-center strong { display:block; font-size:1.8rem; line-height:1; }
        .donut-center span { font-size:.8rem; color:var(--body); }
        .legend-list { display:grid; gap:8px; }
        .legend-row { display:grid; grid-template-columns: 12px 1fr auto auto; gap:10px; align-items:center; font-size:.88rem; }
        .legend-row .dot { width:12px; height:12px; border-radius:999px; }
        .modal-copy { color: var(--body); line-height: 1.55; }
        .modal-warning {
            display: grid;
            grid-template-columns: 40px 1fr;
            gap: 12px;
            align-items: start;
            padding: 12px 14px;
            border: 1px solid rgba(255, 159, 26, .24);
            border-radius: 12px;
            background: rgba(255, 159, 26, .08);
        }
        .modal-warning-icon {
            width: 40px;
            height: 40px;
            border-radius: 12px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            background: rgba(255, 159, 26, .16);
            color: #a05d08;
            font-size: 1.1rem;
        }
        .modal-warning-title { font-weight: 800; color: var(--heading); margin-bottom: 4px; }
        .modal-warning-copy { color: #6b7280; font-size: .92rem; line-height: 1.55; }
        .task-progress { display: grid; gap: 14px; }
        .task-progress-head { display: flex; align-items: center; gap: 12px; }
        .task-progress-copy strong { display: block; font-size: 1rem; color: var(--heading); margin-bottom: 2px; }
        .task-progress-copy span { color: var(--body); font-size: .92rem; }
        .task-progress .progress { height: 10px; border-radius: 999px; background: #e7eef8; overflow: hidden; }
        .task-progress .progress-bar { background: linear-gradient(90deg, #1f465d 0%, #2f6fec 100%); }
        .task-progress-note { color: #6b7280; font-size: .86rem; }
        .tooltip {
            --bs-tooltip-bg: #1d2c42;
            --bs-tooltip-max-width: 260px;
        }
        .port-menu {
            position: fixed;
            z-index: 1080;
            width: 240px;
            border: 1px solid var(--line);
            border-radius: 12px;
            background: #fff;
            box-shadow: 0 18px 40px rgba(19,35,61,.18);
            overflow: hidden;
            display: none;
        }
        .port-menu.show { display:block; }
        .port-menu-head { padding:10px 12px; font-weight:800; border-bottom:1px solid var(--line); }
        .port-menu-item {
            width:100%; border:0; background:#fff; text-align:left; padding:8px 12px; font-size:.88rem; color:var(--heading);
            display:flex; align-items:center; justify-content:space-between; gap:10px;
        }
        .port-menu-item:hover { background:#f8fbff; }
        .port-menu-item:disabled { color:#9aa5b5; background:#fff; }
        .port-menu-item.has-submenu::after { content:"\F285"; font-family:"bootstrap-icons"; font-size:.82rem; color:#7c8798; }
        .port-menu-item.is-open { background:#f8fbff; }
        .port-submenu {
            position: fixed;
            z-index: 1081;
            width: 250px;
            border: 1px solid var(--line);
            border-radius: 12px;
            background: #fff;
            box-shadow: 0 18px 40px rgba(19,35,61,.18);
            overflow: hidden;
            display: none;
        }
        .port-submenu.show { display:block; }
        .port-submenu-head { padding:10px 12px; font-weight:700; border-bottom:1px solid var(--line); background:#fbfdff; }
        .modal-copy { color:var(--body); font-size:.92rem; line-height:1.55; }
        @media (max-width: 1499.98px) { .summary-row { grid-template-columns: repeat(5, minmax(0,1fr)); } }
        @media (max-width: 1199.98px) { .detail-grid, .lower-grid, .info-dual { display:block; } .side-column { min-width:0; } .side-column, .main-column { display:grid; gap:12px; } .mode-grid { grid-template-columns:1fr; } }
        @media (max-width: 991.98px) { .app-shell { display:block; } .app-sidebar { width:100%; min-width:100%; height:auto; position:static; } .topbar, .title-row { flex-direction:column; align-items:stretch; } .toolbar-left, .toolbar-right { width:100%; justify-content:space-between; flex-wrap:wrap; } .search-wrap { width:100%; } .ports-grid { grid-template-columns: repeat(12, 1fr); } .ports-area { grid-template-columns:1fr; } }
        @media (max-width: 767.98px) { .summary-row { grid-template-columns: repeat(2, minmax(0,1fr)); } .donut-layout, .traffic-row, .switch-info-row, .info-item, .event-row { grid-template-columns:1fr; } }
    </style>
</head>
<body>
    <div class="app-shell">
        <aside class="app-sidebar">
            <div class="sidebar-brand">
                <div class="brand-row">
                    <span class="brand-icon"><i class="bi bi-shield-check"></i></span>
                    <div class="brand-title">NAC PANEL</div>
                </div>
            </div>
            <div class="sidebar-nav-label">NAVIGATION</div>
            <nav class="sidebar-nav">
                @foreach ($navItems as $item)
                    <a href="#" class="sidebar-link {{ !empty($item['active']) ? 'is-active' : '' }}">
                        <span class="label-wrap"><i class="bi {{ $item['icon'] }}"></i><span>{{ $item['label'] }}</span></span>
                        <span class="dot"></span>
                    </a>
                @endforeach
            </nav>
            <div class="legend-box">
                <h6 class="mb-3">Port Durumlari</h6>
                @foreach ($legendItems as $legend)
                    <div class="legend-item"><span class="legend-dot" style="background: {{ $legend['color'] }};"></span><span>{{ $legend['label'] }}</span></div>
                @endforeach
            </div>
        </aside>

        <main>
            <div class="topbar">
                <div class="toolbar-left">
                    <a href="#" class="menu-trigger"><i class="bi bi-list fs-4"></i></a>
                    <div class="breadcrumb-line">Switches &nbsp;&gt;&nbsp; Zone Detail &nbsp;&gt;&nbsp; Switch Detail</div>
                </div>
                <div class="toolbar-right">
                    <label class="search-wrap"><input type="search" placeholder="Ara..."><i class="bi bi-search"></i></label>
                    <a href="#" class="icon-btn"><i class="bi bi-bell"></i><span class="notify-badge">3</span></a>
                    <a href="#" class="icon-btn"><i class="bi bi-question-circle"></i></a>
                    <a href="#" class="profile-chip"><i class="bi bi-person"></i><span>admin</span><i class="bi bi-chevron-down small"></i></a>
                </div>
            </div>

            <div class="title-row">
                <div class="switch-heading">
                    <span class="status-indicator"></span>
                    <div>
                        <div class="d-flex align-items-center gap-3 flex-wrap">
                            <h1>{{ $switchData['hostname'] }}</h1>
                            <span class="status-badge {{ $switchData['statusClass'] }}">{{ $switchData['status'] }}</span>
                        </div>
                        <div class="switch-sub">
                            <span>{{ $switchData['vendor'] }} {{ $switchData['model'] }}</span>
                            <span>{{ $switchData['ip'] }}</span>
                            <span>{{ $switchData['zoneLabel'] }}</span>
                        </div>
                    </div>
                </div>
                <div class="d-flex gap-2 flex-wrap">
                    <a href="{{ url()->current() }}" class="secondary-btn"><i class="bi bi-arrow-clockwise"></i><span>Yenile</span></a>
                    <a href="#" class="secondary-btn"><i class="bi bi-download"></i><span>Rapor Al</span></a>
                    <a href="#" class="secondary-btn"><i class="bi bi-gear"></i><span>Ayarlar</span></a>
                </div>
            </div>

            <section class="nac-card">
                <div class="nac-head">
                    <div class="nac-title">
                        <h2>NAC Protection</h2>
                        <p>Switch seviyesinde NAC modu secilir. Portlar varsayilan olarak switch modunu devralir.</p>
                    </div>
                </div>
                <div class="mode-grid">
                    @foreach (['Disabled', 'Monitor Only', 'Enforcement'] as $mode)
                        <div class="mode-option {{ $switchData['nacMode'] === $mode ? 'is-active' : '' }}">
                            <div class="mode-name">{{ $mode }}</div>
                            <div class="mode-desc">{{ $nacModeDescriptions[$mode] }}</div>
                        </div>
                    @endforeach
                </div>
                <div class="action-row">
                    <a href="{{ url()->current() }}" class="action-btn"><i class="bi bi-arrow-clockwise"></i><span>Refresh</span></a>
                    <button type="button" class="action-btn" data-switch-action="rediscover-ports"><i class="bi bi-router"></i><span>Discover Ports</span></button>
                    <button type="button" class="action-btn warning" data-switch-action="monitor"><i class="bi bi-eye"></i><span>Monitor Mode</span></button>
                    <button type="button" class="action-btn primary" data-switch-action="enforcement"><i class="bi bi-shield-check"></i><span>Enable Enforcement</span></button>
                    <button type="button" class="action-btn danger" data-switch-action="disable"><i class="bi bi-slash-circle"></i><span>Disable NAC</span></button>
                </div>
            </section>

            <section class="summary-row">
                @foreach ($switchData['summary'] as $card)
                    <div class="summary-card tone-{{ $card['tone'] }}">
                        <span class="summary-icon"><i class="bi {{ $card['icon'] }}"></i></span>
                        <div>
                            <span class="summary-title">{{ $card['label'] }}</span>
                            <span class="summary-value">{{ $card['value'] }} @if(!empty($card['sub']))<span class="summary-sub">{{ $card['sub'] }}</span>@endif</span>
                            @if(!empty($card['progress']))
                                <div style="margin-top:6px;height:6px;background:#e9eef5;border-radius:999px;overflow:hidden;"><span style="display:block;height:100%;width:{{ $card['progress'] }}%;background:#41b349;border-radius:inherit;"></span></div>
                            @endif
                        </div>
                    </div>
                @endforeach
            </section>

            <div class="tabs-row">
                <a href="#" class="tab-link is-active">Port Haritasi</a>
                <a href="#" class="tab-link">Port Listesi</a>
                <a href="#" class="tab-link">Endpoint Listesi</a>
                <a href="#" class="tab-link">Olaylar</a>
                <a href="#" class="tab-link">Konfigurasyon</a>
                @if (!empty($switchData['supportsPoe']))
                    <a href="#" class="tab-link">PoE</a>
                @endif
                <a href="#" class="tab-link">Performans</a>
            </div>

            <section class="detail-grid">
                <div class="main-column">
                    <div class="card-shell">
                        <div class="card-head">
                            <h2>FRONT PANEL GORUNUMU</h2>
                            <div class="switch-toolbar">
                                <select class="switch-select"><option>Gorunum</option><option selected>Front</option></select>
                                <div class="toggle"><span>Port Numaralarini Goster</span><span class="toggle-pill"></span></div>
                                <span class="icon-square"><i class="bi bi-arrows-fullscreen"></i></span>
                            </div>
                        </div>
                        <div class="switch-map-wrap">
                            <div class="front-panel">
                                <div class="panel-header">
                                    <div class="vendor-block">
                                        <div class="vendor">{{ strtolower($switchData['vendor']) }}</div>
                                        <div class="model">{{ $switchData['model'] }}</div>
                                    </div>
                                </div>
                                <div class="ports-area">
                                    <div>
                                        <div class="ports-grid" style="grid-template-columns: repeat({{ $panelColumnCount }}, minmax(0, 1fr));">
                                            @foreach ($panelPortPairs as $pair)
                                                <div class="port-pair">
                                                    @foreach (['top', 'bottom'] as $slot)
                                                        @php($port = $pair[$slot] ?? null)
                                                        <div class="port-num">{{ $port['panel_label'] ?? '' }}</div>
                                                        <div
                                                            class="port-tile {{ $port ? 'state-'.$port['state'].' '.(($port['isUplink'] ?? false) ? 'is-uplink-role ' : '').($port['id'] === $switchData['selectedPort'] ? 'is-selected' : '') : 'is-placeholder' }}"
                                                            @if($port)
                                                                data-port-id="{{ $port['id'] }}"
                                                                data-bs-toggle="tooltip"
                                                                data-bs-html="true"
                                                                title="<div class='text-start'><div class='fw-bold mb-1'>{{ $port['label'] }}</div><div><strong>Status:</strong> {{ $statusLabels[$port['state']] ?? ucfirst($port['state']) }}</div><div><strong>User:</strong> {{ filled($port['user']) && $port['user'] !== '-' ? $port['user'] : 'RADIUS verisi bekleniyor' }}</div><div><strong>MAC:</strong> {{ $port['mac'] }}</div><div><strong>IP:</strong> {{ $port['ip'] }}</div><div><strong>Hostname:</strong> {{ $port['hostname'] }}</div><div><strong>Policy:</strong> {{ filled($port['policy']) && $port['policy'] !== '-' ? $port['policy'] : 'NAC policy verisi bekleniyor' }}</div><div><strong>Role:</strong> {{ filled($port['role'] ?? null) && ($port['role'] ?? '-') !== '-' ? $port['role'] : 'NAC role verisi bekleniyor' }}</div><div><strong>VLAN:</strong> {{ $port['vlan'] }}</div><div><strong>Device Type:</strong> {{ filled($port['deviceType'] ?? null) && ($port['deviceType'] ?? '-') !== '-' ? $port['deviceType'] : 'Profiling verisi bekleniyor' }}</div><div><strong>Last Auth Result:</strong> {{ $port['auth'] }}</div></div>"
                                                            @endif
                                                        ></div>
                                                    @endforeach
                                                </div>
                                            @endforeach
                                        </div>
                                    </div>
                                    @if ($panelAuxPorts->isNotEmpty())
                                        <div class="uplink-block">
                                            <div class="uplink-title">{{ $panelProfile['aux_title'] ?? 'SFP / Uplink' }}</div>
                                            <div class="uplink-grid" style="grid-template-columns: repeat({{ $panelProfile['aux_columns'] ?? 2 }}, minmax(0, 1fr));">
                                                @foreach ($panelAuxPorts as $port)
                                                    <div
                                                        class="uplink-port state-{{ $port['state'] }} {{ ($port['isUplink'] ?? false) ? 'is-uplink-role ' : '' }}{{ $port['id'] === $switchData['selectedPort'] ? 'is-selected' : '' }}"
                                                        data-port-id="{{ $port['id'] }}"
                                                        data-bs-toggle="tooltip"
                                                        data-bs-html="true"
                                                        title="<div class='text-start'><div class='fw-bold mb-1'>{{ $port['label'] }}</div><div><strong>Status:</strong> {{ $statusLabels[$port['state']] ?? ucfirst($port['state']) }}</div><div><strong>User:</strong> {{ filled($port['user']) && $port['user'] !== '-' ? $port['user'] : 'RADIUS verisi bekleniyor' }}</div><div><strong>MAC:</strong> {{ $port['mac'] }}</div><div><strong>IP:</strong> {{ $port['ip'] }}</div><div><strong>Hostname:</strong> {{ $port['hostname'] }}</div><div><strong>Policy:</strong> {{ filled($port['policy']) && $port['policy'] !== '-' ? $port['policy'] : 'NAC policy verisi bekleniyor' }}</div><div><strong>Role:</strong> {{ filled($port['role'] ?? null) && ($port['role'] ?? '-') !== '-' ? $port['role'] : 'NAC role verisi bekleniyor' }}</div><div><strong>VLAN:</strong> {{ $port['vlan'] }}</div><div><strong>Device Type:</strong> {{ filled($port['deviceType'] ?? null) && ($port['deviceType'] ?? '-') !== '-' ? $port['deviceType'] : 'Profiling verisi bekleniyor' }}</div><div><strong>Last Auth Result:</strong> {{ $port['auth'] }}</div></div>"
                                                    >{{ $port['panel_label'] }}</div>
                                                @endforeach
                                            </div>
                                        </div>
                                    @endif
                                </div>
                            </div>
                            <div class="panel-legend">
                                @foreach ($legendItems as $legend)
                                    <span><span class="dot" style="background: {{ $legend['color'] }};"></span>{{ $legend['label'] }}</span>
                                @endforeach
                                <span class="panel-update">Son guncelleme: 10:24:35 <i class="bi bi-arrow-clockwise ms-2"></i></span>
                            </div>
                        </div>
                    </div>

                    <div class="lower-grid">
                        <div class="card-shell">
                            <div class="card-head"><h3>SECILI PORT BILGILERI</h3></div>
                            <div class="info-dual">
                                <div class="info-col">
                                    <div>
                                        <span class="badge-port" id="selected-port-label">{{ $selectedPort['label'] }}</span>
                                        <span class="mini-state"><span class="dot" id="selected-port-dot" style="background: {{ $stateDots[$selectedPort['state']] ?? '#41b349' }};"></span><span id="selected-port-state">{{ $statusLabels[$selectedPort['state']] ?? ucfirst($selectedPort['state']) }}</span></span>
                                    </div>
                                    <div class="info-list">
                                        @foreach ($selectedPortInfo as $item)
                                            <div class="info-item"><span>{{ $item['label'] }}</span><strong data-port-field="{{ $item['key'] }}">{{ $item['value'] }}</strong></div>
                                        @endforeach
                                    </div>
                                </div>
                                <div class="info-col">
                                    <h3 style="margin:0;font-size:1rem;font-weight:800;">BAGLI ENDPOINT</h3>
                                    <div class="info-list">
                                        @foreach ($endpointInfo as $item)
                                            <div class="info-item"><span>{{ $item['label'] }}</span><strong data-endpoint-field="{{ $item['key'] }}">{{ $item['value'] }}</strong></div>
                                        @endforeach
                                    </div>
                                    <div class="port-action-strip">
                                        <form id="port-allow-form" method="POST" action="{{ route('devices.approve', ['mac' => rawurlencode((string) ($selectedPort['mac'] ?? '00:00:00:00:00:00'))]) }}">
                                            @csrf
                                            <input type="hidden" name="target_vlan" id="port-allow-vlan" value="{{ $defaultAllowVlan }}">
                                            <input type="hidden" name="identity_type" id="port-identity-type" value="{{ strtolower((string) ($selectedPort['role'] ?? '')) }}">
                                            <button type="submit" class="allow-btn" id="port-allow-button"><i class="bi bi-check-circle"></i> Allow</button>
                                        </form>
                                        <form id="port-guest-form" method="POST" action="{{ route('devices.guest', ['mac' => rawurlencode((string) ($selectedPort['mac'] ?? '00:00:00:00:00:00'))]) }}">
                                            @csrf
                                            <input type="hidden" name="target_vlan" id="port-guest-vlan" value="{{ $guestVlan }}">
                                            <button type="submit" class="guest-btn" id="port-guest-button"><i class="bi bi-person"></i> Guest</button>
                                        </form>
                                        <form id="port-block-form" method="POST" action="{{ route('devices.block', ['mac' => rawurlencode((string) ($selectedPort['mac'] ?? '00:00:00:00:00:00'))]) }}">
                                            @csrf
                                            <button type="submit" class="block-btn" id="port-block-button"><i class="bi bi-shield-slash"></i> Block</button>
                                        </form>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="card-shell">
                            <div class="card-head"><h3>SON OLAYLAR</h3></div>
                            <div class="event-list">
                                @foreach ($switchData['events'] as $event)
                                    <div class="event-row">
                                        <span class="event-icon text-{{ $event['tone'] }}"><i class="bi {{ $event['icon'] }}"></i></span>
                                        <div><div>{{ $event['title'] }}</div><div class="text-muted">{{ $event['sub'] }}</div></div>
                                        <div class="text-muted">{{ $event['time'] }}</div>
                                    </div>
                                @endforeach
                            </div>
                        </div>
                    </div>
                </div>

                <div class="side-column">
                    <div class="card-shell">
                        <div class="card-head"><h3>SWITCH BILGILERI</h3></div>
                            <div class="switch-info-list">
                            <div class="switch-info-row"><span>Hostname</span><strong>{{ $switchData['hostname'] }}</strong></div>
                            <div class="switch-info-row"><span>IP Adresi</span><strong>{{ $switchData['ip'] }}</strong></div>
                            <div class="switch-info-row"><span>Vendor</span><strong>{{ $switchData['vendor'] }}</strong></div>
                            <div class="switch-info-row"><span>Model</span><strong>{{ $switchData['model'] }}</strong></div>
                            <div class="switch-info-row"><span>Serial Number</span><strong>{{ $switchData['serial'] }}</strong></div>
                            <div class="switch-info-row"><span>Firmware</span><strong>{{ $switchData['firmware'] }}</strong></div>
                            <div class="switch-info-row"><span>Calisma Suresi</span><strong>{{ $switchData['uptime'] }}</strong></div>
                        </div>
                    </div>

                    <div class="card-shell">
                        <div class="card-head"><h3>TRAFIK OZETI (SON 24 SAAT)</h3></div>
                        <div class="traffic-list">
                            @foreach ($switchData['traffic'] as $item)
                                <div class="traffic-row">
                                    <span>{{ $item['label'] }}</span>
                                    <strong>{{ $item['value'] }}</strong>
                                    <span class="mini-line">
                                        <svg viewBox="0 0 200 44" fill="none" xmlns="http://www.w3.org/2000/svg">
                                            <polyline points="{{ $item['points'] }}" stroke="{{ $item['color'] }}" stroke-width="3" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
                                        </svg>
                                    </span>
                                </div>
                            @endforeach
                        </div>
                    </div>

                    <div class="card-shell">
                        <div class="card-head"><h3>PORT DURUM DAGILIMI</h3></div>
                        <div class="donut-layout">
                            <div class="donut-chart" style="background: {{ $statusChart }};">
                                <div class="donut-center"><strong>{{ $switchData['totalPorts'] ?? $portStatusTotal }}</strong><span>Toplam Port</span></div>
                            </div>
                            <div class="legend-list">
                                @foreach ($portStatusSegments as $segment)
                                    <div class="legend-row"><span class="dot" style="background: {{ $segment['color'] }};"></span><span>{{ $segment['label'] }}</span><strong>{{ $segment['value'] }}</strong><span>({{ $portStatusTotal ? number_format(($segment['value'] / $portStatusTotal) * 100, 1) : 0 }}%)</span></div>
                                @endforeach
                            </div>
                        </div>
                    </div>

                    @if (!empty($switchData['supportsPoe']))
                        <div class="card-shell">
                            <div class="card-head"><h3>POE KULLANIMI</h3></div>
                            <div class="donut-layout">
                                <div class="donut-chart" style="background: {{ $poeChart }};">
                                    <div class="donut-center"><strong>{{ $switchData['poeBudget'] ?? 0 }} W</strong><span>Toplam Guc</span></div>
                                </div>
                                <div class="legend-list">
                                    @foreach ($switchData['poeSegments'] as $segment)
                                        <div class="legend-row"><span class="dot" style="background: {{ $segment['color'] }};"></span><span>{{ $segment['label'] }}</span><strong>{{ $segment['value'] }}W</strong><span>({{ $poeTotal ? number_format(($segment['value'] / $poeTotal) * 100, 0) : 0 }}%)</span></div>
                                    @endforeach
                                </div>
                            </div>
                        </div>
                    @endif
                </div>
            </section>
        </main>
    </div>

    <div id="portContextMenu" class="port-menu">
        <div class="port-menu-head" id="portMenuTitle">Gi1/0/12</div>
        <button type="button" class="port-menu-item has-submenu" data-submenu="info">Bilgi</button>
        <button type="button" class="port-menu-item has-submenu" data-submenu="nac">NAC</button>
        <button type="button" class="port-menu-item has-submenu" data-submenu="vlan">VLAN</button>
        <button type="button" class="port-menu-item has-submenu" data-submenu="radius">RADIUS</button>
        <button type="button" class="port-menu-item has-submenu" data-submenu="port">Port</button>
        <button type="button" class="port-menu-item has-submenu" data-submenu="other">Diger</button>
    </div>

    <div id="portSubmenu-info" class="port-submenu">
        <div class="port-submenu-head">Bilgi</div>
        <button type="button" class="port-menu-item" data-menu-action="LLDP Bilgisi">LLDP Bilgisi</button>
        <button type="button" class="port-menu-item" data-menu-action="View Port Detail">View Port Detail</button>
        <button type="button" class="port-menu-item" data-menu-action="View Endpoint Detail">View Endpoint Detail</button>
        <button type="button" class="port-menu-item" data-menu-action="Open Investigation">Open Investigation</button>
        <button type="button" class="port-menu-item" data-menu-action="View Port History">View Port History</button>
    </div>
    <div id="portSubmenu-nac" class="port-submenu">
        <div class="port-submenu-head">NAC</div>
        <button type="button" class="port-menu-item" data-menu-action="Enable NAC Protection">Enable NAC Protection</button>
        <button type="button" class="port-menu-item" data-menu-action="Set Monitor Only">Set Monitor Only</button>
        <button type="button" class="port-menu-item" data-menu-action="Disable NAC Protection">Disable NAC Protection</button>
    </div>
    <div id="portSubmenu-vlan" class="port-submenu">
        <div class="port-submenu-head">VLAN</div>
        <button type="button" class="port-menu-item" data-menu-action="Move To VLAN...">Move To VLAN...</button>
        <button type="button" class="port-menu-item" data-menu-action="Move To Guest VLAN">Move To Guest VLAN</button>
        <button type="button" class="port-menu-item" data-menu-action="Move To Quarantine VLAN">Move To Quarantine VLAN</button>
        <button type="button" class="port-menu-item" data-menu-action="Move To Reject VLAN">Move To Reject VLAN</button>
    </div>
    <div id="portSubmenu-radius" class="port-submenu">
        <div class="port-submenu-head">RADIUS</div>
        <button type="button" class="port-menu-item" data-menu-action="Force Reauthentication">Force Reauthentication</button>
        <button type="button" class="port-menu-item" data-menu-action="CoA Disconnect">CoA Disconnect</button>
    </div>
    <div id="portSubmenu-port" class="port-submenu">
        <div class="port-submenu-head">Port</div>
        <button type="button" class="port-menu-item" data-menu-action="Rediscover Port">Portu Tekrar Tara</button>
        <button type="button" class="port-menu-item" data-menu-action="Bounce Port">Bounce Port</button>
        <button type="button" class="port-menu-item" data-menu-action="Enable Port">Enable Port</button>
        <button type="button" class="port-menu-item" data-menu-action="Disable Port">Disable Port</button>
    </div>
    <div id="portSubmenu-other" class="port-submenu">
        <div class="port-submenu-head">Diger</div>
        <button type="button" class="port-menu-item" data-menu-action="Add Note">Add Note</button>
        <button type="button" class="port-menu-item" data-menu-action="Copy MAC Address">Copy MAC Address</button>
        <button type="button" class="port-menu-item" data-menu-action="Copy IP Address">Copy IP Address</button>
    </div>

    <div class="modal fade" id="actionConfirmModal" tabindex="-1" aria-hidden="true">
        <div class="modal-dialog modal-dialog-centered">
            <div class="modal-content" style="border-radius: 14px; border-color: var(--line);">
                <div class="modal-header">
                    <h5 class="modal-title" id="confirmModalTitle">Onay Gerekiyor</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Kapat"></button>
                </div>
                <div class="modal-body">
                    <div class="modal-copy" id="confirmModalBody">Bu islem onay gerektirir.</div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="action-btn" data-bs-dismiss="modal">Vazgec</button>
                    <button type="button" class="action-btn primary" id="confirmModalButton">Devam Et</button>
                </div>
            </div>
        </div>
    </div>

    <div class="modal fade" id="lldpInfoModal" tabindex="-1" aria-hidden="true">
        <div class="modal-dialog modal-dialog-centered">
            <div class="modal-content" style="border-radius: 14px; border-color: var(--line);">
                <div class="modal-header">
                    <h5 class="modal-title" id="lldpModalTitle">LLDP Bilgisi</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Kapat"></button>
                </div>
                <div class="modal-body" id="lldpModalBody">
                    Veri yukleniyor...
                </div>
                <div class="modal-footer">
                    <button type="button" class="action-btn" data-bs-dismiss="modal">Kapat</button>
                    <button type="button" class="action-btn warning d-none" id="discoveryBackgroundButton">Arka Planda Devam Et</button>
                </div>
            </div>
        </div>
    </div>

    <div class="modal fade" id="vlanSelectModal" tabindex="-1" aria-hidden="true">
        <div class="modal-dialog modal-dialog-centered">
            <div class="modal-content" style="border-radius: 14px; border-color: var(--line);">
                <div class="modal-header">
                    <h5 class="modal-title">VLAN Sec</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Kapat"></button>
                </div>
                <div class="modal-body">
                    <div class="modal-copy mb-3">Secili port icin hedef VLAN secin.</div>
                    <select id="vlanSelectInput" class="form-select">
                        @foreach ($availableVlans as $vlanOption)
                            <option value="{{ $vlanOption }}">{{ $vlanOption }}</option>
                        @endforeach
                    </select>
                </div>
                <div class="modal-footer">
                    <button type="button" class="action-btn" data-bs-dismiss="modal">Vazgec</button>
                    <button type="button" class="action-btn primary" id="vlanSelectConfirmButton">Uygula</button>
                </div>
            </div>
        </div>
    </div>

    <div class="toast-container position-fixed bottom-0 end-0 p-3" style="z-index: 1095;">
        <div id="discoveryToast" class="toast border-0 shadow" role="status" aria-live="polite" aria-atomic="true">
            <div class="toast-header">
                <i class="bi bi-diagram-3 me-2 text-primary"></i>
                <strong class="me-auto" id="discoveryToastTitle">Discovery</strong>
                <small id="discoveryToastMeta">simdi</small>
                <button type="button" class="btn-close ms-2 mb-1" data-bs-dismiss="toast" aria-label="Kapat"></button>
            </div>
            <div class="toast-body" id="discoveryToastBody">
                Discovery arka planda calisiyor.
            </div>
            <div class="px-3 pb-3 d-flex gap-2">
                <button type="button" class="action-btn d-none" id="discoveryToastDetailsButton">Detaylari Gor</button>
                <button type="button" class="action-btn primary d-none" id="discoveryToastRefreshButton">Simdi Yenile</button>
            </div>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script>
    <script>
        const portData = @json($portPayload);
        const stateColors = @json($stateDots);
        const portMap = Object.fromEntries(portData.map(function (item) {
            return [String(item.id), item];
        }));
        const confirmModalEl = document.getElementById('actionConfirmModal');
        const confirmModal = new bootstrap.Modal(confirmModalEl);
        const confirmModalTitle = document.getElementById('confirmModalTitle');
        const confirmModalBody = document.getElementById('confirmModalBody');
        const lldpModalEl = document.getElementById('lldpInfoModal');
        const lldpModal = new bootstrap.Modal(lldpModalEl);
        const lldpModalTitle = document.getElementById('lldpModalTitle');
        const lldpModalBody = document.getElementById('lldpModalBody');
        const vlanSelectModalEl = document.getElementById('vlanSelectModal');
        const vlanSelectModal = new bootstrap.Modal(vlanSelectModalEl);
        const vlanSelectInput = document.getElementById('vlanSelectInput');
        const vlanSelectConfirmButton = document.getElementById('vlanSelectConfirmButton');
        const discoveryBackgroundButton = document.getElementById('discoveryBackgroundButton');
        const discoveryToastEl = document.getElementById('discoveryToast');
        const discoveryToast = new bootstrap.Toast(discoveryToastEl, { delay: 2600 });
        const discoveryToastTitle = document.getElementById('discoveryToastTitle');
        const discoveryToastBody = document.getElementById('discoveryToastBody');
        const discoveryToastMeta = document.getElementById('discoveryToastMeta');
        const discoveryToastDetailsButton = document.getElementById('discoveryToastDetailsButton');
        const discoveryToastRefreshButton = document.getElementById('discoveryToastRefreshButton');
        const contextMenu = document.getElementById('portContextMenu');
        const menuTitle = document.getElementById('portMenuTitle');
        const submenuButtons = Array.from(document.querySelectorAll('[data-submenu]'));
        const submenus = Array.from(document.querySelectorAll('.port-submenu'));
        const switchId = @json($switchData['id'] ?? null);
        let selectedContextPort = null;
        let activeSubmenu = null;
        let currentSelectedPortId = @json($switchData['selectedPort']);
        let activeDiscoveryJob = null;
        let pendingVlanAction = null;
        const endpointPendingLabels = {
            user: 'RADIUS verisi bekleniyor',
            ip: 'IP verisi yok',
            deviceType: 'Profiling verisi bekleniyor',
            policyText: 'NAC policy verisi bekleniyor',
            role: 'NAC role verisi bekleniyor'
        };
        const defaultAllowVlan = @json($defaultAllowVlan);
        const guestVlan = @json($guestVlan);
        const quarantineVlan = @json($quarantineVlan);
        const availableVlans = @json($availableVlans);
        const csrfToken = @json(csrf_token());
        const switchPortActionUrlTemplate = @json(url('/switch-ports/__PORT__/actions'));
        const portAllowForm = document.getElementById('port-allow-form');
        const portGuestForm = document.getElementById('port-guest-form');
        const portBlockForm = document.getElementById('port-block-form');
        const portAllowVlanInput = document.getElementById('port-allow-vlan');
        const portGuestVlanInput = document.getElementById('port-guest-vlan');
        const portIdentityTypeInput = document.getElementById('port-identity-type');
        const portActionButtons = [
            document.getElementById('port-allow-button'),
            document.getElementById('port-guest-button'),
            document.getElementById('port-block-button')
        ].filter(Boolean);

        const confirmMessages = {
            enforcement: 'Bu switch uzerindeki endpointler NAC politikalarina tabi olacaktir. Devam etmek istiyor musunuz?',
            'Enable Enforcement': 'Bu switch uzerindeki endpointler NAC politikalarina tabi olacaktir. Devam etmek istiyor musunuz?',
            'Rediscover All Ports': 'Bu switch uzerindeki tum portlar icin SNMP envanteri yeniden toplanacak. Devam etmek istiyor musunuz?',
            'Rediscover Port': 'Secili port icin SNMP envanteri yeniden toplanacak. Devam etmek istiyor musunuz?',
            'Disable Port': 'Secili port devre disi birakilacak. Devam etmek istiyor musunuz?',
            'Bounce Port': 'Secili port kisa sureli kesilip tekrar aktif edilecek. Devam etmek istiyor musunuz?',
            'Move To VLAN...': 'Secili port baska bir VLAN\'a tasinacak. Devam etmek istiyor musunuz?',
            'Move To Guest VLAN': 'Secili port Guest VLAN\'a tasinacak. Devam etmek istiyor musunuz?',
            'Move To Quarantine VLAN': 'Secili port Quarantine VLAN\'a tasinacak. Devam etmek istiyor musunuz?',
            'Move To Reject VLAN': 'Secili port Reject VLAN\'a tasinacak. Devam etmek istiyor musunuz?',
            'CoA Disconnect': 'Secili endpoint icin CoA Disconnect gonderilecek. Devam etmek istiyor musunuz?'
        };

        document.querySelectorAll('[data-bs-toggle="tooltip"]').forEach(function (element) {
            new bootstrap.Tooltip(element);
        });

        function showConfirm(actionLabel) {
            confirmModalTitle.textContent = actionLabel;
            const message = confirmMessages[actionLabel] || 'Bu islem onay gerektirir. Devam etmek istiyor musunuz?';
            confirmModalBody.innerHTML = '' +
                '<div class="modal-warning">' +
                    '<div class="modal-warning-icon"><i class="bi bi-exclamation-triangle"></i></div>' +
                    '<div>' +
                        '<div class="modal-warning-title">Islem onayi gerekli</div>' +
                        '<div class="modal-warning-copy">' + message + '</div>' +
                    '</div>' +
                '</div>';
            confirmModalEl.dataset.action = actionLabel;
            confirmModal.show();
        }

        function renderTaskProgress(title, description, percent) {
            const safePercent = Math.max(8, Math.min(100, Number(percent) || 0));
            lldpModalBody.innerHTML = '' +
                '<div class="task-progress">' +
                    '<div class="task-progress-head">' +
                        '<div class="spinner-border text-primary" role="status" aria-hidden="true"></div>' +
                        '<div class="task-progress-copy">' +
                            '<strong>' + title + '</strong>' +
                            '<span>' + description + '</span>' +
                        '</div>' +
                    '</div>' +
                    '<div class="progress">' +
                        '<div class="progress-bar progress-bar-striped progress-bar-animated" style="width:' + safePercent + '%"></div>' +
                    '</div>' +
                    '<div class="task-progress-note">SNMP sorgulari calisiyor. Islem switch buyuklugune gore biraz zaman alabilir.</div>' +
                '</div>';
        }

        function renderTaskError(message) {
            lldpModalBody.innerHTML = '' +
                '<div class="task-progress">' +
                    '<div class="modal-warning">' +
                        '<div class="modal-warning-icon"><i class="bi bi-exclamation-octagon"></i></div>' +
                        '<div>' +
                            '<div class="modal-warning-title">Discovery tamamlanamadi</div>' +
                            '<div class="modal-warning-copy">' + message + '</div>' +
                        '</div>' +
                    '</div>' +
                '</div>';
        }

        function showDiscoveryToast(title, message) {
            discoveryToastTitle.textContent = title;
            discoveryToastBody.textContent = message;
            discoveryToastMeta.textContent = 'simdi';
            discoveryToast.show();
        }

        async function executePortAction(action, extraPayload) {
            if (!selectedContextPort) {
                return;
            }

            const response = await fetch(switchPortActionUrlTemplate.replace('__PORT__', encodeURIComponent(String(selectedContextPort.id))), {
                method: 'POST',
                headers: {
                    'Accept': 'application/json',
                    'Content-Type': 'application/json',
                    'X-Requested-With': 'XMLHttpRequest',
                    'X-CSRF-TOKEN': csrfToken
                },
                body: JSON.stringify(Object.assign({ action: action }, extraPayload || {}))
            });

            const rawText = await response.text();
            let payload = {};
            if (rawText) {
                try {
                    payload = JSON.parse(rawText);
                } catch (error) {
                    payload = { message: rawText.trim() };
                }
            }

            if (!response.ok) {
                const validationMessage = payload.errors?.action?.[0];
                const genericMessage = payload.message;
                throw new Error(validationMessage || genericMessage || 'Port aksiyonu uygulanamadi.');
            }

            return payload;
        }

        async function handlePortVlanAction(actionLabel) {
            if (!selectedContextPort) {
                return;
            }

            if (actionLabel === 'Move To VLAN...') {
                const currentVlan = Number.parseInt(String(selectedContextPort.vlan || '1').trim(), 10);
                const fallbackVlan = availableVlans.includes(currentVlan) ? currentVlan : (availableVlans[0] || 1);
                pendingVlanAction = { action: 'move_vlan', portId: selectedContextPort.id };
                vlanSelectInput.value = String(fallbackVlan);
                vlanSelectModal.show();
                return;
            }

            let action = '';
            let payload = {};

            if (false) {
                const entered = window.prompt('Tasınacak VLAN ID degerini girin (1-4094):', String(selectedContextPort.vlan || '1'));
                if (entered === null) {
                    return;
                }

                const vlanId = Number.parseInt(String(entered).trim(), 10);
                if (!Number.isInteger(vlanId) || vlanId < 1 || vlanId > 4094) {
                    window.alert('Gecerli bir VLAN ID girin.');
                    return;
                }

                action = 'move_vlan';
                payload = { vlan_id: vlanId };
            } else if (actionLabel === 'Move To Guest VLAN') {
                action = 'move_guest_vlan';
                payload = { vlan_id: guestVlan };
            } else if (actionLabel === 'Move To Quarantine VLAN' || actionLabel === 'Move To Reject VLAN') {
                action = 'move_quarantine_vlan';
                payload = { vlan_id: quarantineVlan };
            } else {
                return;
            }

            try {
                const result = await executePortAction(action, payload);
                const vlanId = result.execution?.vlan_id || payload.vlan_id || '-';
                applyPortVlanChange(selectedContextPort.id, vlanId);
                showDiscoveryToast('VLAN degisti', selectedContextPort.label + ' icin VLAN ' + vlanId + ' uygulandi.');
                window.setTimeout(function () {
                    refreshPortDetail(selectedContextPort.id).catch(function () {});
                }, 400);
            } catch (error) {
                window.alert(error.message || 'Port VLAN degisikligi uygulanamadi.');
            }
        }

        async function confirmVlanSelection() {
            if (!pendingVlanAction || !selectedContextPort || !vlanSelectInput) {
                return;
            }

            const vlanId = Number.parseInt(String(vlanSelectInput.value || '').trim(), 10);
            if (!Number.isInteger(vlanId) || vlanId < 1 || vlanId > 4094) {
                window.alert('Gecerli bir VLAN secin.');
                return;
            }

            try {
                const result = await executePortAction(pendingVlanAction.action, { vlan_id: vlanId });
                const appliedVlan = result.execution?.vlan_id || vlanId;
                vlanSelectModal.hide();
                pendingVlanAction = null;
                applyPortVlanChange(selectedContextPort.id, appliedVlan);
                showDiscoveryToast('VLAN degisti', selectedContextPort.label + ' icin VLAN ' + appliedVlan + ' uygulandi.');
                window.setTimeout(function () {
                    refreshPortDetail(selectedContextPort.id).catch(function () {});
                }, 400);
            } catch (error) {
                window.alert(error.message || 'Port VLAN degisikligi uygulanamadi.');
            }
        }

        function setDiscoveryToastActions(showDetails, showRefresh) {
            discoveryToastDetailsButton.classList.toggle('d-none', !showDetails);
            discoveryToastRefreshButton.classList.toggle('d-none', !showRefresh);
        }

        function reloadAfterDiscovery(portId) {
            const url = new URL(window.location.href);
            if (portId) {
                url.searchParams.set('selected_port', String(portId));
            }
            window.location.assign(url.toString());
        }

        function openDiscoveryJobDetails() {
            if (!activeDiscoveryJob || !activeDiscoveryJob.lastJob) {
                return;
            }

            const details = activeDiscoveryJob.lastJob;
            lldpModalTitle.textContent = activeDiscoveryJob.label + ' - Discovery Durumu';
            lldpModal.show();
            setDiscoveryBackgroundEnabled(Boolean(activeDiscoveryJob.running));
            renderTaskProgress(
                discoveryStepLabel(details.current_step),
                discoveryStepDescription(details, activeDiscoveryJob.label),
                Number(details.progress_percent || 0)
            );
        }

        function setDiscoveryBackgroundEnabled(enabled) {
            discoveryBackgroundButton.classList.toggle('d-none', !enabled);
        }

        function discoveryStepLabel(step) {
            const labels = {
                claimed: 'Kuyruktan alindi',
                'resolving-switch': 'Switch hazirlaniyor',
                'walking-interfaces': 'Port envanteri okunuyor',
                'persisting-ports': 'Port verileri kaydediliyor',
                'syncing-topology': 'Topoloji esleniyor',
                completed: 'Discovery tamamlandi',
                failed: 'Discovery basarisiz'
            };

            return labels[step] || 'Discovery calisiyor';
        }

        function discoveryStepDescription(job, fallbackLabel) {
            const step = String(job.current_step || '').trim();
            const summary = job.summary || {};

            if (step === 'completed') {
                const discoveredPorts = Number(summary.discovered_ports || 0);
                const uplinkPorts = Number(summary.uplink_port_count || 0);
                return fallbackLabel + ' icin discovery tamamlandi. ' + discoveredPorts + ' port bulundu, ' + uplinkPorts + ' uplink adayi isaretlendi.';
            }

            if (step === 'syncing-topology') {
                return 'Port verileri kaydedildi. Topoloji iliskileri guncelleniyor.';
            }

            if (step === 'persisting-ports') {
                return 'SNMP sorgulari bitti. Sonuclar veritabanina yaziliyor.';
            }

            if (step === 'walking-interfaces') {
                return fallbackLabel + ' icin SNMP interface ve FDB verileri toplaniyor.';
            }

            if (step === 'resolving-switch' || step === 'claimed') {
                return 'Discovery isi calistirildi, switch baglantisi hazirlaniyor.';
            }

            return 'Discovery isi arka planda ilerliyor.';
        }

        function sleep(ms) {
            return new Promise(function (resolve) {
                window.setTimeout(resolve, ms);
            });
        }

        async function pollDiscoveryJob(jobId, options) {
            const startedAt = Date.now();
            const timeoutMs = options.timeoutMs || 180000;
            const reloadPortId = options.reloadPortId || currentSelectedPortId;
            const fallbackLabel = options.label || 'Switch';
            activeDiscoveryJob = {
                id: jobId,
                label: fallbackLabel,
                reloadPortId: reloadPortId,
                background: false,
                running: true,
                lastJob: {}
            };
            setDiscoveryBackgroundEnabled(true);
            setDiscoveryToastActions(true, false);

            while (Date.now() - startedAt < timeoutMs) {
                const response = await fetch('/api/discovery-jobs/' + jobId, {
                    headers: {
                        'Accept': 'application/json',
                        'X-Requested-With': 'XMLHttpRequest'
                    }
                });

                if (!response.ok) {
                    throw new Error('Discovery durum bilgisi alinamadi.');
                }

                const payload = await response.json();
                const job = payload.data || {};
                const status = String(job.status || '').toLowerCase();
                activeDiscoveryJob.lastJob = job;
                const title = discoveryStepLabel(job.current_step);
                const description = discoveryStepDescription(job, fallbackLabel);
                const percent = Number(job.progress_percent || 0);

                if (!activeDiscoveryJob?.background) {
                    renderTaskProgress(title, description, percent);
                }

                if (status === 'completed') {
                    setDiscoveryBackgroundEnabled(false);
                    activeDiscoveryJob.running = false;
                    setDiscoveryToastActions(true, true);
                    if (activeDiscoveryJob?.background) {
                        showDiscoveryToast('Discovery tamamlandi', fallbackLabel + ' icin discovery tamamlandi. Sayfa yenileniyor.');
                    } else {
                        renderTaskProgress('Discovery tamamlandi', description, 100);
                    }
                    window.setTimeout(function () {
                        reloadAfterDiscovery(reloadPortId);
                    }, 700);
                    return;
                }

                if (status === 'failed') {
                    setDiscoveryBackgroundEnabled(false);
                    activeDiscoveryJob.running = false;
                    setDiscoveryToastActions(true, false);
                    if (activeDiscoveryJob?.background) {
                        showDiscoveryToast('Discovery basarisiz', job.error_message || 'Discovery basarisiz oldu.');
                        return;
                    }
                    throw new Error(job.error_message || 'Discovery basarisiz oldu.');
                }

                await sleep(1500);
            }

            setDiscoveryBackgroundEnabled(false);
            activeDiscoveryJob = null;
            throw new Error('Discovery zaman asimina ugradi.');
        }

        function mergePortData(portId, patch) {
            const key = String(portId);
            if (!portMap[key]) {
                return;
            }
            portMap[key] = Object.assign({}, portMap[key], patch || {});
        }

        function applyPortVlanChange(portId, vlanId) {
            const normalizedVlan = String(vlanId);
            mergePortData(portId, {
                vlan: normalizedVlan,
                vlanLabel: normalizedVlan,
            });
            if (String(currentSelectedPortId) === String(portId)) {
                updateSelectedPort(portId);
            }
            if (selectedContextPort && String(selectedContextPort.id) === String(portId)) {
                selectedContextPort = portMap[String(portId)];
            }
        }

        async function refreshPortDetail(portId) {
            const response = await fetch('/api/switch-ports/' + encodeURIComponent(String(portId)), {
                headers: {
                    'Accept': 'application/json',
                    'X-Requested-With': 'XMLHttpRequest'
                }
            });

            if (!response.ok) {
                throw new Error('Port detay bilgisi alinamadi.');
            }

            const payload = await response.json();
            const data = payload.data || {};
            mergePortData(portId, data);
            if (String(currentSelectedPortId) === String(portId)) {
                updateSelectedPort(portId);
            }
            if (selectedContextPort && String(selectedContextPort.id) === String(portId)) {
                selectedContextPort = portMap[String(portId)];
            }
        }

        function syncSelectedPortQuery(portId) {
            const url = new URL(window.location.href);

            if (portId && portMap[String(portId)]) {
                url.searchParams.set('selected_port', String(portId));
            } else {
                url.searchParams.delete('selected_port');
            }

            window.history.replaceState({}, '', url.toString());
        }

        function applySelectedPortFromQuery() {
            const selectedPortId = new URL(window.location.href).searchParams.get('selected_port');

            if (!selectedPortId || !portMap[selectedPortId]) {
                syncSelectedPortQuery(currentSelectedPortId);
                return;
            }

            updateSelectedPort(selectedPortId);
        }

        async function rediscoverAllPorts() {
            if (!switchId) {
                return;
            }

            confirmModal.hide();
            lldpModalTitle.textContent = 'Tum Portlar - Port Taramasi';
            lldpModal.show();
            renderTaskProgress('Discovery baslatiliyor', 'Switch uzerindeki tum portlar icin asenkron discovery kuyruga aliniyor.', 12);

            try {
                const response = await fetch('/api/switches/' + switchId + '/rediscover-ports', {
                    method: 'POST',
                    headers: {
                        'Accept': 'application/json',
                        'X-Requested-With': 'XMLHttpRequest'
                    }
                });

                if (!response.ok) {
                    throw new Error('Tum port discovery isi baslatilamadi.');
                }

                const payload = await response.json();
                const job = payload.data?.job || {};
                if (!job.id) {
                    throw new Error('Discovery job kimligi alinmadi.');
                }

                await pollDiscoveryJob(job.id, {
                    label: 'Tum portlar',
                    reloadPortId: currentSelectedPortId
                });
            } catch (error) {
                renderTaskError(error.message || 'Tum port discovery tamamlanamadi.');
            }
        }

        async function rediscoverSelectedPort() {
            if (!selectedContextPort) {
                return;
            }

            confirmModal.hide();
            lldpModalTitle.textContent = selectedContextPort.label + ' - Port Taramasi';
            lldpModal.show();
            renderTaskProgress('Discovery baslatiliyor', selectedContextPort.label + ' icin ilgili switchte discovery isi kuyruga aliniyor.', 12);

            try {
                const response = await fetch('/api/switch-ports/' + selectedContextPort.id + '/rediscover', {
                    method: 'POST',
                    headers: {
                        'Accept': 'application/json',
                        'X-Requested-With': 'XMLHttpRequest'
                    }
                });

                if (!response.ok) {
                    throw new Error('Port discovery isi baslatilamadi.');
                }

                const payload = await response.json();
                const job = payload.data?.job || {};
                if (!job.id) {
                    throw new Error('Discovery job kimligi alinmadi.');
                }

                await pollDiscoveryJob(job.id, {
                    label: selectedContextPort.label,
                    reloadPortId: payload.data?.selected_port_id || selectedContextPort.id
                });
            } catch (error) {
                renderTaskError(error.message || 'Port discovery tamamlanamadi.');
            }
        }

        async function showLldpInfo() {
            if (!selectedContextPort) {
                return;
            }

            setDiscoveryBackgroundEnabled(false);
            lldpModalTitle.textContent = selectedContextPort.label + ' - LLDP Bilgisi';
            lldpModalBody.textContent = 'Veri yukleniyor...';
            lldpModal.show();

            try {
                const response = await fetch('/api/switch-ports/' + selectedContextPort.id + '/lldp');
                const payload = await response.json();
                const data = payload.data || {};

                if (!data.found) {
                    lldpModalBody.textContent = data.message || 'LLDP komsusu bulunamadi.';
                    return;
                }

                lldpModalBody.innerHTML = data.neighbors.map(function (neighbor) {
                    return "<div style='display:grid;gap:8px;margin-bottom:14px;'>" +
                        "<div><strong>Sistem Adi:</strong> " + (neighbor.system_name || '-') + "</div>" +
                        "<div><strong>Port ID:</strong> " + (neighbor.port_id || '-') + "</div>" +
                        "<div><strong>Port Aciklamasi:</strong> " + (neighbor.port_description || '-') + "</div>" +
                        "<div><strong>Sistem Aciklamasi:</strong> " + (neighbor.system_description || '-') + "</div>" +
                    "</div>";
                }).join('');
            } catch (error) {
                lldpModalBody.textContent = 'LLDP bilgisi alinamadi.';
            }
        }

        function hideContextMenu() {
            contextMenu.classList.remove('show');
            activeSubmenu = null;
            submenus.forEach(function (menu) { menu.classList.remove('show'); });
            submenuButtons.forEach(function (button) { button.classList.remove('is-open'); });
        }

        function deviceActionUrl(type, mac) {
            return '/devices/' + encodeURIComponent(mac) + '/' + type;
        }

        function resolveAllowVlan(data) {
            const role = String(data.role || '').toLowerCase();
            if (role === 'ogrenci' || role === 'misafir') {
                return guestVlan;
            }
            return defaultAllowVlan;
        }

        function syncPortActionForms(data) {
            const mac = String(data.mac || '').trim();
            const hasMac = mac !== '' && mac !== '-';

            if (portAllowForm) {
                portAllowForm.action = hasMac ? deviceActionUrl('approve', mac) : '#';
            }
            if (portGuestForm) {
                portGuestForm.action = hasMac ? deviceActionUrl('guest', mac) : '#';
            }
            if (portBlockForm) {
                portBlockForm.action = hasMac ? deviceActionUrl('block', mac) : '#';
            }
            if (portAllowVlanInput) {
                portAllowVlanInput.value = String(resolveAllowVlan(data));
            }
            if (portGuestVlanInput) {
                portGuestVlanInput.value = String(guestVlan);
            }
            if (portIdentityTypeInput) {
                portIdentityTypeInput.value = String(data.role || '').toLowerCase();
            }
            portActionButtons.forEach(function (button) {
                button.disabled = !hasMac;
            });
        }

        function openSubmenu(name, trigger) {
            const submenu = document.getElementById('portSubmenu-' + name);
            if (!submenu) {
                return;
            }

            submenus.forEach(function (menu) { menu.classList.remove('show'); });
            submenuButtons.forEach(function (button) { button.classList.toggle('is-open', button === trigger); });

            const rect = trigger.getBoundingClientRect();
            const width = 250;
            const left = rect.right + width + 12 > window.innerWidth ? rect.left - width - 8 : rect.right + 4;
            const top = Math.min(rect.top, window.innerHeight - submenu.offsetHeight - 12);
            submenu.style.left = Math.max(8, left) + 'px';
            submenu.style.top = Math.max(8, top) + 'px';
            submenu.classList.add('show');
            activeSubmenu = submenu;
        }

        function updateSelectedPort(portId) {
            const data = portMap[String(portId)];
            if (!data) {
                return;
            }

            currentSelectedPortId = String(portId);

            document.getElementById('selected-port-label').textContent = data.label;
            document.getElementById('selected-port-state').textContent = data.statusText;
            document.getElementById('selected-port-dot').style.background = stateColors[data.state] || '#41b349';
            document.querySelectorAll('[data-port-field]').forEach(function (node) {
                const key = node.getAttribute('data-port-field');
                node.textContent = data[key] ?? '-';
            });

            document.querySelectorAll('[data-endpoint-field]').forEach(function (node) {
                const key = node.getAttribute('data-endpoint-field');
                const value = data[key] ?? '-';
                if (endpointPendingLabels[key] && (!value || value === '-')) {
                    node.textContent = endpointPendingLabels[key];
                    return;
                }
                node.textContent = value;
            });

            document.querySelectorAll('.port-tile, .uplink-port').forEach(function (tile) {
                tile.classList.toggle('is-selected', tile.dataset.portId === String(portId));
            });

            syncPortActionForms(data);
            syncSelectedPortQuery(portId);
        }

        document.querySelectorAll('.port-tile, .uplink-port').forEach(function (tile) {
            tile.addEventListener('click', function () {
                updateSelectedPort(tile.dataset.portId);
                hideContextMenu();
            });

            tile.addEventListener('contextmenu', function (event) {
                event.preventDefault();
                updateSelectedPort(tile.dataset.portId);
                selectedContextPort = portMap[tile.dataset.portId];
                menuTitle.textContent = selectedContextPort.label;

                const criticalPort = ['Trunk', 'Uplink'].includes(selectedContextPort.portType);
                document.querySelectorAll('[data-menu-action]').forEach(function (button) {
                    const action = button.dataset.menuAction;
                    const disableForCritical = criticalPort && ['Disable Port', 'Move To VLAN...', 'Move To Guest VLAN', 'Move To Quarantine VLAN', 'Move To Reject VLAN'].includes(action);
                    button.disabled = disableForCritical;
                });

                contextMenu.style.left = Math.min(event.clientX, window.innerWidth - 252) + 'px';
                contextMenu.style.top = Math.min(event.clientY, window.innerHeight - 280) + 'px';
                contextMenu.classList.add('show');
            });
        });

        document.querySelectorAll('[data-switch-action]').forEach(function (button) {
            button.addEventListener('click', function () {
                const action = button.dataset.switchAction;
                if (action === 'rediscover-ports') {
                    showConfirm('Rediscover All Ports');
                    return;
                }
                if (action === 'enforcement') {
                    showConfirm('Enable Enforcement');
                    return;
                }
                if (action === 'disable') {
                    showConfirm('Disable NAC Protection');
                }
            });
        });

        document.getElementById('confirmModalButton').addEventListener('click', async function () {
            const action = confirmModalEl.dataset.action;

            if (action === 'Rediscover All Ports') {
                await rediscoverAllPorts();
                return;
            }

            if (action === 'Rediscover Port') {
                await rediscoverSelectedPort();
                return;
            }

            confirmModal.hide();
        });

        discoveryBackgroundButton.addEventListener('click', function () {
            if (!activeDiscoveryJob) {
                return;
            }

            activeDiscoveryJob.background = true;
            lldpModal.hide();
            showDiscoveryToast('Discovery arka planda', activeDiscoveryJob.label + ' icin discovery arka planda devam ediyor.');
        });

        lldpModalEl.addEventListener('hidden.bs.modal', function () {
            if (activeDiscoveryJob) {
                activeDiscoveryJob.background = true;
                setDiscoveryBackgroundEnabled(false);
            }
        });

        discoveryToastDetailsButton.addEventListener('click', function () {
            openDiscoveryJobDetails();
        });

        discoveryToastRefreshButton.addEventListener('click', function () {
            if (!activeDiscoveryJob) {
                return;
            }

            reloadAfterDiscovery(activeDiscoveryJob.reloadPortId || currentSelectedPortId);
        });

        if (vlanSelectConfirmButton) {
            vlanSelectConfirmButton.addEventListener('click', async function () {
                await confirmVlanSelection();
            });
        }

        if (vlanSelectModalEl) {
            vlanSelectModalEl.addEventListener('hidden.bs.modal', function () {
                pendingVlanAction = null;
            });
        }

        submenuButtons.forEach(function (button) {
            button.addEventListener('mouseenter', function () {
                openSubmenu(button.dataset.submenu, button);
            });
            button.addEventListener('click', function (event) {
                event.stopPropagation();
                openSubmenu(button.dataset.submenu, button);
            });
        });

        document.querySelectorAll('.port-submenu [data-menu-action]').forEach(function (button) {
            button.addEventListener('click', async function () {
                const action = button.dataset.menuAction;
                if (action === 'LLDP Bilgisi') {
                    hideContextMenu();
                    await showLldpInfo();
                    return;
                }

                if (['Copy MAC Address', 'Copy IP Address'].includes(action) && selectedContextPort) {
                    const text = action === 'Copy MAC Address' ? selectedContextPort.mac : selectedContextPort.ip;
                    if (navigator.clipboard && text && text !== '-') {
                        await navigator.clipboard.writeText(text);
                    }
                    hideContextMenu();
                    return;
                }

                if (['Move To VLAN...', 'Move To Guest VLAN', 'Move To Quarantine VLAN', 'Move To Reject VLAN'].includes(action)) {
                    hideContextMenu();
                    await handlePortVlanAction(action);
                    return;
                }

                hideContextMenu();
                if (confirmMessages[action]) {
                    showConfirm(action);
                }
            });
        });

        document.addEventListener('click', function (event) {
            if (!contextMenu.contains(event.target) && !submenus.some(function (menu) { return menu.contains(event.target); })) {
                hideContextMenu();
            }
        });

        applySelectedPortFromQuery();
        if (portMap[String(currentSelectedPortId)]) {
            syncPortActionForms(portMap[String(currentSelectedPortId)]);
        }
        window.addEventListener('scroll', hideContextMenu, true);
        window.addEventListener('resize', hideContextMenu);
    </script>
</body>
</html>


