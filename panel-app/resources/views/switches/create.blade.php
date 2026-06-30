@php
    $errors = $errors ?? new \Illuminate\Support\ViewErrorBag();
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
@endphp
<!DOCTYPE html>
<html lang="tr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NAC Panel | Switch Ekle</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.3/font/bootstrap-icons.css" rel="stylesheet">
    <style>
        :root {
            --sidebar-bg: #252525;
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
            width: 228px;
            min-width: 228px;
            background: linear-gradient(180deg, rgba(255,255,255,.02), transparent 18%), var(--sidebar-bg);
            color: var(--sidebar-text);
            padding: 18px 12px;
            display: flex;
            flex-direction: column;
            overflow-y: auto;
            position: sticky;
            top: 0;
            height: 100vh;
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
        .legend-box p { color: rgba(255,255,255,.82); font-size: .88rem; margin: 0; line-height: 1.55; }
        main { flex:1; padding:10px 12px 14px; min-width:0; }
        .topbar, .card-shell {
            border:1px solid var(--line);
            border-radius:14px;
            background:#fff;
            box-shadow:0 8px 18px rgba(19,35,61,.04);
        }
        .topbar { min-height:56px; display:flex; align-items:center; justify-content:space-between; gap:16px; padding:8px 12px; margin-bottom:10px; }
        .toolbar-left, .toolbar-right { display:flex; align-items:center; gap:12px; }
        .menu-trigger, .icon-btn { width:40px; height:40px; border-radius:12px; border:1px solid var(--line); background:#fff; display:inline-flex; align-items:center; justify-content:center; color:var(--heading); text-decoration:none; position:relative; overflow:visible; }
        .search-wrap { width:min(100%, 430px); display:flex; align-items:center; gap:10px; border:1px solid var(--line); border-radius:12px; min-height:40px; padding:0 12px; background:#fff; }
        .search-wrap input { border:0; outline:0; background:transparent; width:100%; color:var(--heading); }
        .notify-badge { position:absolute; top:-5px; right:-4px; min-width:18px; height:18px; border-radius:999px; background:#295dd8; color:#fff; font-size:.68rem; display:inline-flex; align-items:center; justify-content:center; font-weight:700; border:2px solid #fff; }
        .profile-chip, .secondary-btn, .primary-btn, .ghost-btn {
            min-height:40px;
            border-radius:12px;
            border:1px solid var(--line);
            background:#fff;
            padding:0 14px;
            display:inline-flex;
            align-items:center;
            gap:10px;
            text-decoration:none;
            color:var(--heading);
            font-weight:600;
        }
        .primary-btn { background:#1f465d; border-color:#1f465d; color:#fff; }
        .ghost-btn { background:#fff; }
        .page-head {
            display:flex;
            align-items:flex-start;
            justify-content:space-between;
            gap:16px;
            margin-bottom:12px;
        }
        .breadcrumb-line { color: var(--body); font-size:.92rem; margin-bottom:8px; }
        .page-head h1 { margin:0; font-size:1.6rem; font-weight:800; }
        .page-sub { color: var(--body); font-size:.94rem; margin-top:4px; }
        .flash-box { border-radius:12px; padding:12px 14px; margin-bottom:12px; border:1px solid transparent; }
        .flash-box.success { background:rgba(65,179,73,.08); border-color:rgba(65,179,73,.22); color:#1e7f2a; }
        .flash-box.error { background:rgba(239,68,68,.08); border-color:rgba(239,68,68,.22); color:#a12f2f; }
        .form-layout { display:grid; grid-template-columns: 1.05fr .95fr; gap:12px; }
        .form-stack { display:grid; gap:12px; }
        .card-head { display:flex; align-items:center; justify-content:space-between; gap:12px; padding:12px 14px; border-bottom:1px solid var(--line); }
        .card-head h2, .card-head h3 { margin:0; font-size:1rem; font-weight:800; }
        .card-body { padding:14px; }
        .form-grid { display:grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap:12px 14px; }
        .form-grid.single { grid-template-columns: 1fr; }
        .field-full { grid-column: 1 / -1; }
        .section-note { color:var(--body); font-size:.84rem; }
        .form-label { font-size:.88rem; font-weight:600; color:#425169; margin-bottom:6px; }
        .form-control, .form-select { min-height:42px; border-color:var(--line); border-radius:10px; font-size:.92rem; }
        .form-control:focus, .form-select:focus { border-color:#9bb6d8; box-shadow:0 0 0 .2rem rgba(47,111,236,.12); }
        textarea.form-control { min-height:100px; }
        .switch-inline { display:flex; gap:12px; align-items:center; flex-wrap:wrap; min-height:42px; }
        .switch-inline .form-check { margin:0; }
        .actions-row { display:flex; justify-content:flex-end; gap:10px; margin-top:12px; }
        .invalid-feedback { display:block; font-size:.82rem; }
        .hidden-section { display:none; }
        @media (max-width: 1199.98px) {
            .form-layout { grid-template-columns: 1fr; }
        }
        @media (max-width: 991.98px) {
            .app-shell { display:block; }
            .app-sidebar { width:100%; min-width:100%; height:auto; position:static; }
            .topbar, .page-head { flex-direction:column; align-items:stretch; }
            .toolbar-left, .toolbar-right { width:100%; justify-content:space-between; flex-wrap:wrap; }
            .search-wrap { width:100%; }
        }
        @media (max-width: 767.98px) {
            .form-grid { grid-template-columns: 1fr; }
            .actions-row { flex-direction:column-reverse; }
            .actions-row .primary-btn, .actions-row .secondary-btn, .actions-row .ghost-btn { justify-content:center; }
        }
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
                <p>Bu ekran yeni switch tanimlarini eklemek icin hazirlandi. Sidebar, navbar ve kart dili mevcut NAC Panel akisi ile ayni tutuldu.</p>
            </div>
        </aside>

        <main>
            <div class="topbar">
                <div class="toolbar-left">
                    <a href="#" class="menu-trigger" aria-label="Menu"><i class="bi bi-list fs-4"></i></a>
                    <div class="breadcrumb-line mb-0">Switches &nbsp;&gt;&nbsp; Switch Ekle</div>
                </div>
                <div class="toolbar-right">
                    <label class="search-wrap"><input type="search" placeholder="Ara..."><i class="bi bi-search"></i></label>
                    <a href="#" class="icon-btn" aria-label="Bildirim"><i class="bi bi-bell"></i><span class="notify-badge">3</span></a>
                    <a href="#" class="icon-btn" aria-label="Yardim"><i class="bi bi-question-circle"></i></a>
                    <a href="#" class="profile-chip"><i class="bi bi-person"></i><span>admin</span><i class="bi bi-chevron-down small"></i></a>
                </div>
            </div>

            <div class="page-head">
                <div>
                    <div class="breadcrumb-line">Switches &nbsp;&gt;&nbsp; Create</div>
                    <h1>SWITCH EKLE</h1>
                    <div class="page-sub">Yeni switch tanimini NAC paneline ekleyin. Kaydet ve Test Et butonu su an sadece form aksiyonu olarak hazirdir.</div>
                </div>
                <div class="d-flex gap-2 flex-wrap">
                    <a href="{{ route('switches.index') }}" class="ghost-btn"><i class="bi bi-arrow-left"></i><span>Listeye Don</span></a>
                </div>
            </div>

            @if (session('success'))
                <div class="flash-box success">{{ session('success') }}</div>
            @endif

            @if (isset($errors) && $errors->any())
                <div class="flash-box error">Form kaydedilemedi. Zorunlu alanlari ve dogrulama hatalarini kontrol edin.</div>
            @endif

            <form method="POST" action="{{ route('switches.store') }}">
                @csrf
                <div class="form-layout">
                    <div class="form-stack">
                        <div class="card-shell">
                            <div class="card-head">
                                <h2>TEMEL BILGILER</h2>
                                <span class="section-note">Hostname, model ve zone bilgileri</span>
                            </div>
                            <div class="card-body">
                                <div class="form-grid">
                                    <div>
                                        <label class="form-label" for="hostname">Hostname</label>
                                        <input id="hostname" name="hostname" type="text" class="form-control @error('hostname') is-invalid @enderror" value="{{ old('hostname') }}">
                                        @error('hostname')<div class="invalid-feedback">{{ $message }}</div>@enderror
                                    </div>
                                    <div>
                                        <label class="form-label" for="ip_address">IP Address</label>
                                        <input id="ip_address" name="ip_address" type="text" class="form-control @error('ip_address') is-invalid @enderror" value="{{ old('ip_address') }}">
                                        @error('ip_address')<div class="invalid-feedback">{{ $message }}</div>@enderror
                                    </div>
                                    <div>
                                        <label class="form-label" for="vendor">Vendor</label>
                                        <input id="vendor" name="vendor" type="text" class="form-control @error('vendor') is-invalid @enderror" value="{{ old('vendor') }}">
                                        @error('vendor')<div class="invalid-feedback">{{ $message }}</div>@enderror
                                    </div>
                                    <div>
                                        <label class="form-label" for="model">Model</label>
                                        <input id="model" name="model" type="text" class="form-control @error('model') is-invalid @enderror" value="{{ old('model') }}">
                                        @error('model')<div class="invalid-feedback">{{ $message }}</div>@enderror
                                    </div>
                                    <div>
                                        <label class="form-label" for="zone">Zone</label>
                                        <select id="zone" name="zone" class="form-select @error('zone') is-invalid @enderror">
                                            <option value="">Zone secin</option>
                                            @foreach ($zoneOptions as $zone)
                                                <option value="{{ $zone['slug'] }}" @selected(old('zone', $selectedZone) === $zone['slug'])>{{ $zone['label'] }}</option>
                                            @endforeach
                                        </select>
                                        @error('zone')<div class="invalid-feedback">{{ $message }}</div>@enderror
                                    </div>
                                    <div>
                                        <label class="form-label" for="location">Location</label>
                                        <input id="location" name="location" type="text" class="form-control" value="{{ old('location') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="port_count">Port Sayisi</label>
                                        <input id="port_count" name="port_count" type="number" min="1" max="128" class="form-control @error('port_count') is-invalid @enderror" value="{{ old('port_count') }}">
                                        @error('port_count')<div class="invalid-feedback">{{ $message }}</div>@enderror
                                    </div>
                                    <div>
                                        <label class="form-label" for="managed">Yonetiliyor mu?</label>
                                        <select id="managed" name="managed" class="form-select">
                                            <option value="1" @selected(old('managed', '1') === '1')>Evet</option>
                                            <option value="0" @selected(old('managed') === '0')>Hayir</option>
                                        </select>
                                    </div>
                                    <div>
                                        <label class="form-label" for="status">Durum</label>
                                        <select id="status" name="status" class="form-select">
                                            <option value="online" @selected(old('status', 'online') === 'online')>Online</option>
                                            <option value="offline" @selected(old('status') === 'offline')>Offline</option>
                                            <option value="warning" @selected(old('status') === 'warning')>Warning</option>
                                            <option value="unmanaged" @selected(old('status') === 'unmanaged')>Unmanaged</option>
                                        </select>
                                    </div>
                                    <div class="field-full">
                                        <label class="form-label" for="description">Aciklama / Not</label>
                                        <textarea id="description" name="description" class="form-control @error('description') is-invalid @enderror">{{ old('description') }}</textarea>
                                        @error('description')<div class="invalid-feedback">{{ $message }}</div>@enderror
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="card-shell">
                            <div class="card-head">
                                <h3>SNMP BILGILERI</h3>
                                <span class="section-note">Version secimine gore alanlar acilir</span>
                            </div>
                            <div class="card-body">
                                <div class="form-grid">
                                    <div>
                                        <label class="form-label" for="snmp_enabled">SNMP Aktif mi?</label>
                                        <select id="snmp_enabled" name="snmp_enabled" class="form-select">
                                            <option value="1" @selected(old('snmp_enabled', '1') === '1')>Evet</option>
                                            <option value="0" @selected(old('snmp_enabled') === '0')>Hayir</option>
                                        </select>
                                    </div>
                                    <div>
                                        <label class="form-label" for="snmp_version">SNMP Version</label>
                                        <select id="snmp_version" name="snmp_version" class="form-select">
                                            <option value="v2c" @selected(old('snmp_version', 'v2c') === 'v2c')>v2c</option>
                                            <option value="v3" @selected(old('snmp_version') === 'v3')>v3</option>
                                        </select>
                                    </div>
                                    <div id="snmp-v2c-group" class="field-full">
                                        <label class="form-label" for="snmp_community">SNMP Community</label>
                                        <input id="snmp_community" name="snmp_community" type="password" class="form-control" value="{{ old('snmp_community') }}">
                                        <!-- Community degeri ileride encrypted saklanacak. -->
                                    </div>
                                    <div>
                                        <label class="form-label" for="snmp_port">SNMP Port</label>
                                        <input id="snmp_port" name="snmp_port" type="number" class="form-control" value="{{ old('snmp_port', '161') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="snmp_timeout">SNMP Timeout</label>
                                        <input id="snmp_timeout" name="snmp_timeout" type="number" class="form-control" value="{{ old('snmp_timeout', '5') }}">
                                    </div>
                                    <div id="snmp-v3-group" class="hidden-section field-full">
                                        <div class="form-grid">
                                            <div>
                                                <label class="form-label" for="snmp_username">SNMP Username</label>
                                                <input id="snmp_username" name="snmp_username" type="text" class="form-control" value="{{ old('snmp_username') }}">
                                            </div>
                                            <div>
                                                <label class="form-label" for="snmp_auth_protocol">SNMP Auth Protocol</label>
                                                <select id="snmp_auth_protocol" name="snmp_auth_protocol" class="form-select">
                                                    <option value="SHA" @selected(old('snmp_auth_protocol', 'SHA') === 'SHA')>SHA</option>
                                                    <option value="MD5" @selected(old('snmp_auth_protocol') === 'MD5')>MD5</option>
                                                </select>
                                            </div>
                                            <div>
                                                <label class="form-label" for="snmp_auth_password">SNMP Auth Password</label>
                                                <input id="snmp_auth_password" name="snmp_auth_password" type="password" class="form-control" value="{{ old('snmp_auth_password') }}">
                                            </div>
                                            <div>
                                                <label class="form-label" for="snmp_privacy_protocol">SNMP Privacy Protocol</label>
                                                <select id="snmp_privacy_protocol" name="snmp_privacy_protocol" class="form-select">
                                                    <option value="AES" @selected(old('snmp_privacy_protocol', 'AES') === 'AES')>AES</option>
                                                    <option value="DES" @selected(old('snmp_privacy_protocol') === 'DES')>DES</option>
                                                </select>
                                            </div>
                                            <div class="field-full">
                                                <label class="form-label" for="snmp_privacy_password">SNMP Privacy Password</label>
                                                <input id="snmp_privacy_password" name="snmp_privacy_password" type="password" class="form-control" value="{{ old('snmp_privacy_password') }}">
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="form-stack">
                        <div class="card-shell">
                            <div class="card-head">
                                <h3>RADIUS / NAC BILGILERI</h3>
                                <span class="section-note">VLAN ve CoA ayarlari</span>
                            </div>
                            <div class="card-body">
                                <div class="form-grid">
                                    <div class="field-full">
                                        <label class="form-label" for="radius_secret">RADIUS Client Secret</label>
                                        <input id="radius_secret" name="radius_secret" type="password" class="form-control" value="{{ old('radius_secret') }}">
                                        <!-- Secret degeri ileride encrypted saklanacak. -->
                                    </div>
                                    <div>
                                        <label class="form-label" for="coa_enabled">CoA Destekli mi?</label>
                                        <select id="coa_enabled" name="coa_enabled" class="form-select">
                                            <option value="1" @selected(old('coa_enabled', '1') === '1')>Evet</option>
                                            <option value="0" @selected(old('coa_enabled') === '0')>Hayir</option>
                                        </select>
                                    </div>
                                    <div>
                                        <label class="form-label" for="coa_port">CoA Port</label>
                                        <input id="coa_port" name="coa_port" type="number" class="form-control" value="{{ old('coa_port', '3799') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="default_vlan">Default VLAN</label>
                                        <input id="default_vlan" name="default_vlan" type="text" class="form-control" value="{{ old('default_vlan') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="guest_vlan">Guest VLAN</label>
                                        <input id="guest_vlan" name="guest_vlan" type="text" class="form-control" value="{{ old('guest_vlan') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="quarantine_vlan">Quarantine VLAN</label>
                                        <input id="quarantine_vlan" name="quarantine_vlan" type="text" class="form-control" value="{{ old('quarantine_vlan') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="reject_vlan">Reject VLAN</label>
                                        <input id="reject_vlan" name="reject_vlan" type="text" class="form-control" value="{{ old('reject_vlan') }}">
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="card-shell">
                            <div class="card-head">
                                <h3>SSH / API ERISIM BILGILERI</h3>
                                <span class="section-note">SSH ve API baglanti alanlari</span>
                            </div>
                            <div class="card-body">
                                <div class="form-grid">
                                    <div>
                                        <label class="form-label" for="ssh_enabled">SSH Aktif mi?</label>
                                        <select id="ssh_enabled" name="ssh_enabled" class="form-select">
                                            <option value="1" @selected(old('ssh_enabled', '1') === '1')>Evet</option>
                                            <option value="0" @selected(old('ssh_enabled') === '0')>Hayir</option>
                                        </select>
                                    </div>
                                    <div>
                                        <label class="form-label" for="ssh_port">SSH Port</label>
                                        <input id="ssh_port" name="ssh_port" type="number" class="form-control" value="{{ old('ssh_port', '22') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="ssh_username">SSH Username</label>
                                        <input id="ssh_username" name="ssh_username" type="text" class="form-control" value="{{ old('ssh_username') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="ssh_password">SSH Password</label>
                                        <input id="ssh_password" name="ssh_password" type="password" class="form-control" value="{{ old('ssh_password') }}">
                                    </div>
                                    <div class="field-full">
                                        <label class="form-label" for="enable_password">Enable Password</label>
                                        <input id="enable_password" name="enable_password" type="password" class="form-control" value="{{ old('enable_password') }}">
                                    </div>
                                    <div>
                                        <label class="form-label" for="api_enabled">API Aktif mi?</label>
                                        <select id="api_enabled" name="api_enabled" class="form-select">
                                            <option value="1" @selected(old('api_enabled') === '1')>Evet</option>
                                            <option value="0" @selected(old('api_enabled', '0') === '0')>Hayir</option>
                                        </select>
                                    </div>
                                    <div>
                                        <label class="form-label" for="api_base_url">API Base URL</label>
                                        <input id="api_base_url" name="api_base_url" type="text" class="form-control" value="{{ old('api_base_url') }}">
                                    </div>
                                    <div class="field-full">
                                        <label class="form-label" for="api_token">API Token</label>
                                        <input id="api_token" name="api_token" type="password" class="form-control" value="{{ old('api_token') }}">
                                        <!-- API token ileride encrypted saklanacak. -->
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="actions-row">
                    <a href="{{ route('switches.index') }}" class="ghost-btn"><i class="bi bi-x-circle"></i><span>Iptal</span></a>
                    <button type="submit" name="submit_action" value="save_test" class="secondary-btn"><i class="bi bi-plug"></i><span>Kaydet ve Test Et</span></button>
                    <button type="submit" name="submit_action" value="save" class="primary-btn"><i class="bi bi-check2-circle"></i><span>Kaydet</span></button>
                </div>
            </form>
        </main>
    </div>

    <script>
        const snmpVersion = document.getElementById('snmp_version');
        const snmpV2 = document.getElementById('snmp-v2c-group');
        const snmpV3 = document.getElementById('snmp-v3-group');

        function syncSnmpGroups() {
            const isV3 = snmpVersion.value === 'v3';
            snmpV2.style.display = isV3 ? 'none' : '';
            snmpV3.style.display = isV3 ? 'block' : 'none';
        }

        snmpVersion.addEventListener('change', syncSnmpGroups);
        syncSnmpGroups();
    </script>
</body>
</html>
