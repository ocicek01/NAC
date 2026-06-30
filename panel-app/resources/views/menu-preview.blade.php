@php
    $summaryCards = $summaryCards ?? [
        ['label' => 'Toplam Switch', 'value' => 0, 'icon' => 'bi-diagram-3', 'tone' => 'dark'],
        ['label' => 'Aktif Switch', 'value' => 0, 'icon' => 'bi-check-circle', 'tone' => 'success'],
        ['label' => 'Pasif Switch', 'value' => 0, 'icon' => 'bi-x-circle', 'tone' => 'danger'],
        ['label' => 'Toplam Port', 'value' => 0, 'icon' => 'bi-hdd-network', 'tone' => 'dark'],
        ['label' => 'UP Port', 'value' => 0, 'icon' => 'bi-arrow-up-circle', 'tone' => 'success'],
        ['label' => 'DOWN Port', 'value' => 0, 'icon' => 'bi-arrow-down-circle', 'tone' => 'danger'],
        ['label' => 'Toplam Endpoint', 'value' => 0, 'icon' => 'bi-pc-display', 'tone' => 'dark'],
    ];

    $zones = $zones ?? [];

    $navItems = [
        ['label' => 'Dashboard', 'icon' => 'bi-house-door', 'href' => url('/')],
        ['label' => 'Switches', 'icon' => 'bi-diagram-3', 'active' => true, 'href' => route('switches.index')],
        ['label' => 'Endpoints', 'icon' => 'bi-display', 'href' => route('devices.index')],
        ['label' => 'Policies', 'icon' => 'bi-journal-check'],
        ['label' => 'RADIUS Events', 'icon' => 'bi-broadcast-pin'],
        ['label' => 'Guests', 'icon' => 'bi-people'],
        ['label' => 'Investigation', 'icon' => 'bi-search'],
        ['label' => 'Reports', 'icon' => 'bi-bar-chart'],
        ['label' => 'Settings', 'icon' => 'bi-gear'],
        ['label' => 'System', 'icon' => 'bi-gear-wide-connected'],
    ];

    $legendItems = [
        ['label' => 'Up', 'color' => '#2ea44f'],
        ['label' => 'Down', 'color' => '#e03131'],
        ['label' => 'Warning / Unknown', 'color' => '#f08c24'],
        ['label' => 'Offline', 'color' => '#9aa4b2'],
    ];
@endphp
<!DOCTYPE html>
<html lang="tr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NAC Panel | Switches</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.3/font/bootstrap-icons.css" rel="stylesheet">
    <style>
        :root {
            --sidebar-bg: #252525;
            --sidebar-active: #3a3a3a;
            --sidebar-border: rgba(255, 255, 255, 0.08);
            --sidebar-text: #f5f7fc;
            --sidebar-muted: #aeb8cb;
            --page-bg: #f4f7fb;
            --surface: #ffffff;
            --surface-soft: #f8fbfe;
            --surface-muted: #f1f5fa;
            --line: #d7e1ee;
            --heading: #13233d;
            --body: #5d6b80;
            --success: #2ea44f;
            --warning: #f08c24;
            --danger: #e03131;
            --primary: #2558b8;
            --secondary: #7c3aed;
            --shadow: 0 18px 40px rgba(19, 35, 61, 0.08);
        }

        * {
            box-sizing: border-box;
        }

        body {
            margin: 0;
            font-family: "Segoe UI", Tahoma, Geneva, Verdana, sans-serif;
            background:
                radial-gradient(circle at top left, rgba(80, 127, 174, 0.12), transparent 16%),
                linear-gradient(180deg, #f8fafc 0%, var(--page-bg) 100%);
            color: var(--heading);
        }

        .app-shell {
            min-height: 100vh;
            display: flex;
        }

        .app-sidebar {
            width: 228px;
            min-width: 228px;
            background:
                linear-gradient(180deg, rgba(255, 255, 255, 0.02), transparent 18%),
                var(--sidebar-bg);
            color: var(--sidebar-text);
            padding: 18px 12px;
            display: flex;
            flex-direction: column;
            overflow-y: auto;
            position: sticky;
            top: 0;
            height: 100vh;
        }

        .sidebar-brand {
            padding: 6px 6px 18px;
        }

        .brand-row {
            display: flex;
            align-items: center;
            gap: 12px;
            margin-bottom: 18px;
        }

        .brand-icon {
            width: 42px;
            height: 42px;
            border-radius: 12px;
            border: 1px solid rgba(255, 255, 255, 0.12);
            display: inline-flex;
            align-items: center;
            justify-content: center;
            font-size: 1.05rem;
        }

        .brand-title {
            font-size: 0.92rem;
            font-weight: 800;
            letter-spacing: 0.08em;
        }

        .sidebar-nav-label {
            color: rgba(255, 255, 255, 0.48);
            letter-spacing: 0.18em;
            font-size: 0.68rem;
            font-weight: 700;
            padding: 0 14px 8px;
        }

        .sidebar-nav {
            display: grid;
            gap: 4px;
        }

        .sidebar-link {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            color: var(--sidebar-text);
            text-decoration: none;
            padding: 12px 14px;
            border-radius: 16px;
            transition: 0.18s ease;
        }

        .sidebar-link:hover {
            background: rgba(255, 255, 255, 0.04);
            color: #fff;
        }

        .sidebar-link.is-active {
            background: #1f465d;
            box-shadow: inset 0 0 0 1px rgba(93, 198, 255, 0.18);
        }

        .sidebar-link .label-wrap {
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .sidebar-link i {
            font-size: 1.05rem;
            width: 18px;
            text-align: center;
        }

        .sidebar-link .dot {
            width: 9px;
            height: 9px;
            border-radius: 999px;
            background: rgba(255, 255, 255, 0.18);
            flex-shrink: 0;
        }

        .sidebar-link.is-active .dot {
            background: #67d1ff;
            box-shadow: 0 0 0 5px rgba(103, 209, 255, 0.14);
        }

        .legend-box {
            margin-top: 64px;
            border: 1px solid var(--sidebar-border);
            border-radius: 18px;
            padding: 12px 14px;
            background: rgba(255, 255, 255, 0.03);
        }

        .legend-box h6 {
            font-size: 0.96rem;
            margin-bottom: 14px;
        }

        .legend-item {
            display: flex;
            align-items: center;
            gap: 10px;
            color: rgba(255, 255, 255, 0.84);
            font-size: 0.88rem;
        }

        .legend-item + .legend-item {
            margin-top: 10px;
        }

        .legend-dot {
            width: 11px;
            height: 11px;
            border-radius: 999px;
            flex-shrink: 0;
        }

        main {
            flex: 1;
            padding: 10px 12px 14px;
            min-width: 0;
        }

        .workspace {
            border: 1px solid #cfd8e5;
            border-radius: 18px;
            padding: 8px 10px 12px;
            background: rgba(255, 255, 255, 0.4);
        }

        .topbar {
            min-height: 56px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 16px;
            border: 1px solid var(--line);
            border-radius: 14px;
            padding: 8px 12px;
            background: #fff;
            margin-bottom: 10px;
        }

        .toolbar-left,
        .toolbar-right {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .menu-trigger,
        .icon-btn {
            width: 40px;
            height: 40px;
            border-radius: 12px;
            border: 1px solid var(--line);
            background: #fff;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            color: var(--heading);
            text-decoration: none;
            position: relative;
        }

        .search-wrap {
            width: min(100%, 430px);
            display: flex;
            align-items: center;
            gap: 10px;
            border: 1px solid var(--line);
            background: #fff;
            border-radius: 12px;
            min-height: 40px;
            padding: 0 12px;
        }

        .search-wrap input {
            border: 0;
            outline: 0;
            background: transparent;
            width: 100%;
            color: var(--heading);
        }

        .notify-badge {
            position: absolute;
            top: 7px;
            right: 7px;
            min-width: 15px;
            height: 15px;
            border-radius: 999px;
            background: #295dd8;
            color: #fff;
            font-size: 0.67rem;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            font-weight: 700;
        }

        .profile-chip,
        .refresh-btn {
            min-height: 40px;
            border-radius: 12px;
            border: 1px solid var(--line);
            background: #fff;
            padding: 0 12px;
            display: inline-flex;
            align-items: center;
            gap: 10px;
            text-decoration: none;
            color: var(--heading);
            font-weight: 600;
        }

        .page-heading {
            margin: 2px 0 10px;
        }

        .page-heading h1 {
            font-size: 1.08rem;
            font-weight: 800;
            margin: 0 0 2px;
            letter-spacing: 0.04em;
        }

        .page-heading .breadcrumb-line {
            color: var(--body);
            font-size: 0.92rem;
        }

        .summary-row {
            display: grid;
            grid-template-columns: repeat(7, minmax(0, 1fr));
            gap: 0;
            border: 1px solid var(--line);
            border-radius: 14px;
            background: #fff;
            padding: 6px 8px;
            box-shadow: 0 8px 18px rgba(19, 35, 61, 0.04);
        }

        .summary-card {
            display: flex;
            align-items: center;
            gap: 12px;
            padding: 10px 14px;
            min-height: 76px;
            position: relative;
        }

        .summary-card + .summary-card::before {
            content: "";
            position: absolute;
            left: 0;
            top: 18px;
            bottom: 18px;
            width: 1px;
            background: var(--line);
        }

        .summary-icon {
            width: 38px;
            height: 38px;
            border-radius: 10px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            background: var(--surface-soft);
            font-size: 1.16rem;
        }

        .summary-card.tone-success .summary-icon,
        .text-success {
            color: var(--success) !important;
        }

        .summary-card.tone-danger .summary-icon,
        .text-danger {
            color: var(--danger) !important;
        }

        .summary-card.tone-dark .summary-icon,
        .text-dark {
            color: var(--heading) !important;
        }

        .text-primary {
            color: var(--primary) !important;
        }

        .text-secondary {
            color: var(--secondary) !important;
        }

        .summary-title {
            display: block;
            color: var(--heading);
            font-size: 0.85rem;
            font-weight: 600;
            margin-bottom: 2px;
        }

        .summary-value {
            display: block;
            font-size: 1.08rem;
            font-weight: 800;
            color: var(--heading);
            line-height: 1.1;
        }

        .zones-head {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 16px;
            margin: 16px 0 10px;
        }

        .zones-head h2 {
            font-size: 1.14rem;
            font-weight: 800;
            margin: 0;
            letter-spacing: 0.03em;
        }

        .head-actions {
            display: flex;
            gap: 8px;
        }

        .head-actions .mini-btn,
        .head-actions .select-btn {
            min-height: 34px;
            border-radius: 9px;
            border: 1px solid #cfd8e5;
            background: #fff;
            padding: 0 11px;
            display: inline-flex;
            align-items: center;
            gap: 8px;
            color: #415065;
            font-size: 0.9rem;
        }

        .zone-grid {
            display: grid;
            grid-template-columns: repeat(2, minmax(0, 1fr));
            gap: 14px;
        }

        .zone-card {
            background: #fff;
            border: 1px solid var(--line);
            border-radius: 14px;
            padding: 12px 12px 10px;
            box-shadow: 0 6px 16px rgba(19, 35, 61, 0.04);
            min-height: 276px;
            display: flex;
            flex-direction: column;
        }

        .zone-card-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 8px;
            margin-bottom: 9px;
        }

        .zone-title {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 1rem;
            font-weight: 800;
            color: var(--heading);
        }

        .zone-title i {
            font-size: 1rem;
        }

        .status-badge {
            min-width: 78px;
            min-height: 26px;
            border-radius: 8px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            font-size: 0.82rem;
            font-weight: 700;
            border: 1px solid transparent;
            background: #f4f6f9;
        }

        .status-badge.success {
            color: var(--success);
            background: rgba(46, 164, 79, 0.08);
            border-color: rgba(46, 164, 79, 0.22);
        }

        .status-badge.warning {
            color: var(--warning);
            background: rgba(240, 140, 36, 0.08);
            border-color: rgba(240, 140, 36, 0.22);
        }

        .status-badge.danger {
            color: var(--danger);
            background: rgba(224, 49, 49, 0.08);
            border-color: rgba(224, 49, 49, 0.22);
        }

        .stats-panel {
            border: 1px solid var(--line);
            border-radius: 10px;
            overflow: hidden;
            background: #fff;
        }

        .stats-row {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }

        .stats-row + .stats-row {
            border-top: 1px solid var(--line);
        }

        .stat-cell {
            padding: 8px 7px 7px;
            min-height: 52px;
        }

        .stat-cell + .stat-cell {
            border-left: 1px solid var(--line);
        }

        .stat-label {
            display: block;
            font-size: 0.79rem;
            color: #3c4d66;
            margin-bottom: 2px;
        }

        .stat-value {
            display: block;
            font-size: 1rem;
            font-weight: 800;
            line-height: 1.2;
        }

        .switch-section {
            margin-top: 10px;
        }

        .switch-section-title {
            font-size: 0.98rem;
            font-weight: 700;
            color: var(--heading);
            margin-bottom: 7px;
        }

        .switch-row {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-wrap: wrap;
            min-height: 34px;
        }

        .switch-link {
            width: 38px;
            height: 22px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            position: relative;
            text-decoration: none;
        }

        .switch-link svg {
            width: 34px;
            height: 19px;
        }

        .switch-link .switch-body {
            fill: #edf2f8;
            stroke: #5e6b7f;
            transition: fill 0.18s ease, stroke 0.18s ease;
        }

        .switch-link .switch-top,
        .switch-link .switch-port {
            stroke: #2b3646;
            transition: stroke 0.18s ease;
        }

        .switch-link.online .switch-body {
            fill: rgba(46, 164, 79, 0.18);
            stroke: var(--success);
        }

        .switch-link.online .switch-top,
        .switch-link.online .switch-port {
            stroke: var(--success);
        }

        .switch-link.offline .switch-body {
            fill: rgba(224, 49, 49, 0.14);
            stroke: var(--danger);
        }

        .switch-link.offline .switch-top,
        .switch-link.offline .switch-port {
            stroke: var(--danger);
        }

        .switch-link.warning .switch-body {
            fill: rgba(240, 140, 36, 0.14);
            stroke: var(--warning);
        }

        .switch-link.warning .switch-top,
        .switch-link.warning .switch-port {
            stroke: var(--warning);
        }

        .switch-link.unmanaged .switch-body {
            fill: rgba(152, 163, 180, 0.14);
            stroke: #98a3b4;
        }

        .switch-link.unmanaged .switch-top,
        .switch-link.unmanaged .switch-port {
            stroke: #98a3b4;
        }

        .dots {
            color: var(--heading);
            font-weight: 800;
            font-size: 1rem;
            line-height: 1;
        }

        .zone-footer {
            margin-top: 6px;
            display: flex;
            justify-content: flex-end;
        }

        .detail-btn {
            min-height: 34px;
            border-radius: 9px;
            border: 1px solid #cfd8e5;
            background: #fff;
            padding: 0 12px;
            display: inline-flex;
            align-items: center;
            gap: 8px;
            color: var(--heading);
            text-decoration: none;
            font-weight: 600;
            font-size: 0.95rem;
        }

        .bottom-strip {
            margin-top: 12px;
            border: 1px solid var(--line);
            border-radius: 12px;
            padding: 9px 12px;
            background: #fff;
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            font-size: 0.83rem;
            color: var(--body);
        }

        .tooltip {
            --bs-tooltip-bg: #1d2c42;
            --bs-tooltip-max-width: 300px;
        }

        @media (max-width: 1399.98px) {
            .summary-row {
                grid-template-columns: repeat(4, minmax(0, 1fr));
            }

            .zone-grid {
                grid-template-columns: repeat(2, minmax(0, 1fr));
            }
        }

        @media (max-width: 991.98px) {
            .app-shell {
                display: block;
            }

            .app-sidebar {
                width: 100%;
                min-width: 100%;
                height: auto;
                position: static;
            }

            .topbar {
                flex-direction: column;
                align-items: stretch;
            }

            .toolbar-left,
            .toolbar-right {
                width: 100%;
                justify-content: space-between;
                flex-wrap: wrap;
            }

            .search-wrap {
                width: 100%;
            }
        }

        @media (max-width: 767.98px) {
            .workspace {
                padding: 10px;
            }

            .summary-row {
                grid-template-columns: repeat(2, minmax(0, 1fr));
            }

            .zone-grid {
                grid-template-columns: 1fr;
            }

            .stats-row {
                grid-template-columns: repeat(2, minmax(0, 1fr));
            }

            .stat-cell:nth-child(3),
            .stat-cell:nth-child(4) {
                border-top: 1px solid var(--line);
            }

            .stats-row .stat-cell:nth-child(3) {
                border-left: 0;
            }

            .bottom-strip {
                flex-direction: column;
                align-items: flex-start;
            }
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
                    <a href="{{ $item['href'] ?? '#' }}" class="sidebar-link {{ !empty($item['active']) ? 'is-active' : '' }}">
                        <span class="label-wrap">
                            <i class="bi {{ $item['icon'] }}"></i>
                            <span>{{ $item['label'] }}</span>
                        </span>
                        <span class="dot"></span>
                    </a>
                @endforeach
            </nav>

            <div class="legend-box">
                <h6 class="mb-0">Durum Renkleri</h6>
                <div class="mt-3">
                    @foreach ($legendItems as $legend)
                        <div class="legend-item">
                            <span class="legend-dot" style="background: {{ $legend['color'] }};"></span>
                            <span>{{ $legend['label'] }}</span>
                        </div>
                    @endforeach
                </div>
            </div>
        </aside>

        <main>
            <div class="workspace">
                <div class="topbar">
                    <div class="toolbar-left">
                        <a href="#" class="menu-trigger" aria-label="Menu">
                            <i class="bi bi-list fs-4"></i>
                        </a>
                        <label class="search-wrap">
                            <input type="search" placeholder="Ara...">
                            <i class="bi bi-search"></i>
                        </label>
                    </div>

                    <div class="toolbar-right">
                        <a href="#" class="icon-btn" aria-label="Bildirim">
                            <i class="bi bi-bell"></i>
                            <span class="notify-badge">3</span>
                        </a>
                        <a href="#" class="icon-btn" aria-label="Yardim">
                            <i class="bi bi-question-circle"></i>
                        </a>
                        <a href="#" class="profile-chip">
                            <i class="bi bi-person"></i>
                            <span>admin</span>
                            <i class="bi bi-chevron-down small"></i>
                        </a>
                        <a href="{{ route('switches.create') }}" class="secondary-btn">
                            <i class="bi bi-plus-circle"></i>
                            <span>Switch Ekle</span>
                        </a>
                        <a href="{{ route('devices.index') }}" class="refresh-btn">
                            <i class="bi bi-display"></i>
                            <span>Device Registry</span>
                        </a>
                        <a href="{{ url()->current() }}" class="refresh-btn">
                            <i class="bi bi-arrow-clockwise"></i>
                            <span>Yenile</span>
                        </a>
                    </div>
                </div>

                <div class="page-heading">
                    <h1>SWITCHES</h1>
                    <div class="breadcrumb-line">Ana Sayfa &nbsp;&gt;&nbsp; Switches</div>
                </div>

                <section class="summary-row">
                    @foreach ($summaryCards as $card)
                        <div class="summary-card tone-{{ $card['tone'] }}">
                            <span class="summary-icon"><i class="bi {{ $card['icon'] }}"></i></span>
                            <div>
                                <span class="summary-title">{{ $card['label'] }}</span>
                                <span class="summary-value">{{ number_format($card['value']) }}</span>
                            </div>
                        </div>
                    @endforeach
                </section>

                <div class="zones-head">
                    <h2>ZONELER</h2>
                    <div class="head-actions">
                        <button type="button" class="mini-btn"><i class="bi bi-grid"></i></button>
                        <button type="button" class="mini-btn"><i class="bi bi-grid-3x3-gap"></i></button>
                        <button type="button" class="mini-btn"><i class="bi bi-list"></i></button>
                        <button type="button" class="select-btn">Tumu <i class="bi bi-chevron-down"></i></button>
                    </div>
                </div>

                <section class="zone-grid">
                    @foreach ($zones as $zone)
                        <article class="zone-card">
                            <div class="zone-card-header">
                                <div class="zone-title">
                                    <i class="bi bi-building"></i>
                                    <span>{{ $zone['label'] }}</span>
                                </div>
                                <span class="status-badge {{ $zone['statusClass'] }}">{{ $zone['status'] }}</span>
                            </div>

                            <div class="stats-panel">
                                <div class="stats-row">
                                    @foreach (array_slice($zone['stats'], 0, 4) as $stat)
                                        <div class="stat-cell">
                                            <span class="stat-label">{{ $stat['label'] }}</span>
                                            <span class="stat-value text-{{ $stat['tone'] }}">{{ number_format($stat['value']) }}</span>
                                        </div>
                                    @endforeach
                                </div>
                                <div class="stats-row">
                                    @foreach (array_slice($zone['stats'], 4, 4) as $stat)
                                        <div class="stat-cell">
                                            <span class="stat-label">{{ $stat['label'] }}</span>
                                            <span class="stat-value text-{{ $stat['tone'] }}">{{ number_format($stat['value']) }}</span>
                                        </div>
                                    @endforeach
                                </div>
                            </div>

                            <div class="switch-section">
                                <div class="switch-section-title">Switch Listesi</div>
                                <div class="switch-row">
                                    @foreach (array_slice($zone['switches'], 0, 6) as $switch)
                                        <a
                                            href="{{ url('/switches/' . \Illuminate\Support\Str::slug($zone['name']) . '/' . \Illuminate\Support\Str::slug($switch['hostname'])) }}"
                                            class="switch-link {{ $switch['state'] }}"
                                            data-bs-toggle="tooltip"
                                            data-bs-html="true"
                                            data-bs-placement="top"
                                            title="<div class='text-start'>
                                                <div class='fw-bold mb-1'>{{ $switch['hostname'] }}</div>
                                                <div><strong>IP Address:</strong> {{ $switch['ip'] }}</div>
                                                <div><strong>Vendor:</strong> {{ $switch['vendor'] }}</div>
                                                <div><strong>Model:</strong> {{ $switch['model'] }}</div>
                                                <div><strong>Port Sayisi:</strong> {{ $switch['ports'] }}</div>
                                                <div><strong>UP Port:</strong> {{ $switch['up'] }}</div>
                                                <div><strong>DOWN Port:</strong> {{ $switch['down'] }}</div>
                                                <hr class='my-2 border-light opacity-25'>
                                                <div><strong>Uplink Port:</strong> {{ $switch['uplink_ports'] ?? 0 }}</div>
                                                <div><strong>Ogrenilen MAC:</strong> {{ $switch['learned_macs'] ?? 0 }}</div>
                                                @if (($switch['top_mac_count'] ?? 0) > 0)
                                                    <div><strong>En Yogun Port:</strong> {{ $switch['top_mac_port_name'] }} ({{ $switch['top_mac_count'] }})</div>
                                                @endif
                                                <hr class='my-2 border-light opacity-25'>
                                                <div><strong>Last Seen:</strong> {{ $switch['lastSeen'] }}</div>
                                            </div>"
                                        >
                                            <svg viewBox="0 0 54 30" fill="none" xmlns="http://www.w3.org/2000/svg">
                                                <path class="switch-body" d="M6 8.5C6 6.567 7.567 5 9.5 5H44.5C46.433 5 48 6.567 48 8.5V19.5C48 21.433 46.433 23 44.5 23H9.5C7.567 23 6 21.433 6 19.5V8.5Z" fill="#EDF2F8" stroke="#5E6B7F" stroke-width="1.2"/>
                                                <path class="switch-top" d="M9 11H45" stroke="#4C5A6E" stroke-width="1.2" stroke-linecap="round"/>
                                                <path class="switch-port" d="M12 16H17" stroke="#2B3646" stroke-width="2.5" stroke-linecap="round"/>
                                                <path class="switch-port" d="M20 16H25" stroke="#2B3646" stroke-width="2.5" stroke-linecap="round"/>
                                                <path class="switch-port" d="M28 16H33" stroke="#2B3646" stroke-width="2.5" stroke-linecap="round"/>
                                                <path class="switch-port" d="M36 16H41" stroke="#2B3646" stroke-width="2.5" stroke-linecap="round"/>
                                            </svg>
                                        </a>
                                    @endforeach

                                    @if (count($zone['switches']) > 6)
                                        <span class="dots">...</span>
                                    @endif
                                </div>
                            </div>

                            <div class="zone-footer">
                                <a href="{{ url('/switches/' . \Illuminate\Support\Str::slug($zone['name'])) }}" class="detail-btn">
                                    <span>Detay</span>
                                    <i class="bi bi-chevron-right"></i>
                                </a>
                            </div>
                        </article>
                    @endforeach
                </section>

                <section class="bottom-strip">
                    <div>Not: Kart uzerindeki switch ikonlarinin uzerine gelindiginde switch bilgileri gosterilir.</div>
                    <div class="d-flex align-items-center gap-3">
                        <span>Son Guncelleme: 20.05.2025 10:24:35</span>
                        <a href="{{ url()->current() }}" class="text-decoration-none text-dark"><i class="bi bi-arrow-clockwise fs-5"></i></a>
                    </div>
                </section>
            </div>
        </main>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script>
    <script>
        document.querySelectorAll('[data-bs-toggle="tooltip"]').forEach(function (element) {
            new bootstrap.Tooltip(element);
        });
    </script>
</body>
</html>
