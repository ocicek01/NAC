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
        ['label' => 'Online', 'color' => '#41b349'],
        ['label' => 'Uyari', 'color' => '#ff9f1a'],
        ['label' => 'Offline', 'color' => '#ef4444'],
        ['label' => 'Yonetilmiyor', 'color' => '#8b95a7'],
    ];

    $endpointTotal = array_sum(array_column($zone['endpointSegments'], 'value'));
    $portTotal = array_sum(array_column($zone['portSegments'], 'value'));

    $endpointStops = [];
    $offset = 0;
    foreach ($zone['endpointSegments'] as $segment) {
        $size = $endpointTotal > 0 ? round(($segment['value'] / $endpointTotal) * 100, 2) : 0;
        $endpointStops[] = "{$segment['color']} {$offset}% " . ($offset + $size) . '%';
        $offset += $size;
    }
    $endpointChart = 'conic-gradient(' . implode(', ', $endpointStops) . ')';

    $portStops = [];
    $offset = 0;
    foreach ($zone['portSegments'] as $segment) {
        $size = $portTotal > 0 ? round(($segment['value'] / $portTotal) * 100, 2) : 0;
        $portStops[] = "{$segment['color']} {$offset}% " . ($offset + $size) . '%';
        $offset += $size;
    }
    $portChart = 'conic-gradient(' . implode(', ', $portStops) . ')';
@endphp
<!DOCTYPE html>
<html lang="tr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NAC Panel | Zone Detail</title>
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
            --line: #d7e1ee;
            --heading: #13233d;
            --body: #5d6b80;
            --success: #41b349;
            --warning: #ff9f1a;
            --danger: #ef4444;
            --primary: #2f6fec;
            --secondary: #8e59d1;
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
            padding: 0;
            border: 0;
            border-radius: 0;
            background: transparent;
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
            overflow: visible;
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
            top: -5px;
            right: -4px;
            min-width: 18px;
            height: 18px;
            border-radius: 999px;
            background: #295dd8;
            color: #fff;
            font-size: 0.68rem;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            font-weight: 700;
            border: 2px solid #fff;
            box-shadow: 0 2px 6px rgba(19, 35, 61, 0.14);
        }

        .profile-chip,
        .refresh-btn,
        .secondary-btn {
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

        .breadcrumb-line {
            color: var(--body);
            font-size: 0.92rem;
            margin-bottom: 8px;
        }

        .title-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 16px;
            margin-bottom: 10px;
        }

        .zone-heading {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .zone-heading h1 {
            font-size: 1.6rem;
            margin: 0;
            font-weight: 800;
        }

        .health-badge {
            min-width: 92px;
            min-height: 32px;
            border-radius: 10px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            font-weight: 700;
            font-size: 0.92rem;
            border: 1px solid transparent;
        }

        .health-badge.success {
            background: rgba(65, 179, 73, 0.08);
            color: var(--success);
            border-color: rgba(65, 179, 73, 0.22);
        }

        .health-badge.warning {
            background: rgba(255, 159, 26, 0.08);
            color: var(--warning);
            border-color: rgba(255, 159, 26, 0.22);
        }

        .health-badge.danger {
            background: rgba(239, 68, 68, 0.08);
            color: var(--danger);
            border-color: rgba(239, 68, 68, 0.22);
        }

        .summary-row {
            display: grid;
            grid-template-columns: repeat(8, minmax(0, 1fr));
            gap: 0;
            border: 1px solid var(--line);
            border-radius: 14px;
            background: #fff;
            padding: 6px 8px;
            box-shadow: 0 8px 18px rgba(19, 35, 61, 0.04);
            margin-bottom: 12px;
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
        }

        .tone-success .summary-icon,
        .text-success {
            color: var(--success) !important;
        }

        .tone-danger .summary-icon,
        .text-danger {
            color: var(--danger) !important;
        }

        .tone-dark .summary-icon,
        .text-dark {
            color: var(--heading) !important;
        }

        .tone-secondary .summary-icon,
        .text-secondary {
            color: var(--secondary) !important;
        }

        .zone-content {
            display: flex;
            gap: 12px;
            align-items: start;
        }

        .main-column {
            flex: 1 1 auto;
            min-width: 0;
        }

        .side-column {
            flex: 0 0 32%;
            max-width: 32%;
            min-width: 320px;
        }

        .card-shell {
            border: 1px solid var(--line);
            border-radius: 14px;
            background: #fff;
            box-shadow: 0 8px 18px rgba(19, 35, 61, 0.04);
            overflow: hidden;
        }

        .card-head {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            padding: 12px 14px;
            border-bottom: 1px solid var(--line);
        }

        .card-head h2,
        .card-head h3 {
            margin: 0;
            font-size: 1rem;
            font-weight: 800;
        }

        .table-toolbar {
            display: flex;
            align-items: center;
            gap: 10px;
            flex-wrap: wrap;
        }

        .input-inline,
        .filter-select {
            min-height: 38px;
            border: 1px solid var(--line);
            border-radius: 10px;
            padding: 0 12px;
            background: #fff;
            color: var(--heading);
        }

        .input-inline {
            min-width: 220px;
        }

        .table-wrap {
            padding: 0 10px 10px;
            overflow-x: auto;
        }

        .table-zone {
            margin: 0;
            vertical-align: middle;
            min-width: 760px;
            table-layout: fixed;
        }

        .table-zone thead th {
            font-size: 0.84rem;
            font-weight: 700;
            color: #425169;
            background: #fff;
            border-bottom-color: var(--line);
            white-space: nowrap;
        }

        .table-zone tbody tr {
            cursor: pointer;
        }

        .table-zone tbody tr:hover {
            background: #f9fbfe;
        }

        .table-zone td {
            font-size: 0.88rem;
            color: var(--heading);
            white-space: nowrap;
        }

        .table-zone td.model-cell {
            white-space: normal;
            overflow-wrap: anywhere;
            line-height: 1.25;
        }

        .hostname-cell {
            display: flex;
            flex-direction: column;
            gap: 2px;
            line-height: 1.15;
        }

        .hostname-main {
            font-weight: 500;
        }

        .hostname-meta {
            font-size: 0.76rem;
            color: var(--body);
        }

        .hostname-badges {
            display: flex;
            flex-wrap: wrap;
            gap: 6px;
            margin-top: 3px;
        }

        .hostname-badge {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            min-height: 22px;
            padding: 0 8px;
            border-radius: 999px;
            background: #eef4fb;
            border: 1px solid #d7e1ee;
            color: var(--heading);
            font-size: 0.72rem;
            font-weight: 600;
            line-height: 1;
        }

        .hostname-badge.is-uplink {
            background: rgba(255, 159, 26, 0.12);
            border-color: rgba(255, 159, 26, 0.24);
            color: #a05d08;
        }

        .hostname-badge.is-mac {
            background: rgba(47, 111, 236, 0.09);
            border-color: rgba(47, 111, 236, 0.18);
            color: #295dd8;
        }

        .table-zone th:nth-child(1),
        .table-zone td:nth-child(1) {
            width: 56px;
            text-align: center;
        }

        .table-zone th:nth-child(2),
        .table-zone td:nth-child(2) {
            width: 188px;
        }

        .table-zone th:nth-child(3),
        .table-zone td:nth-child(3) {
            width: 102px;
        }

        .table-zone th:nth-child(4),
        .table-zone td:nth-child(4) {
            width: 96px;
        }

        .table-zone th:nth-child(5),
        .table-zone td:nth-child(5) {
            width: 170px;
        }

        .table-zone th:nth-child(6),
        .table-zone td:nth-child(6) {
            width: 104px;
            text-align: center;
        }

        .table-zone th:nth-child(7),
        .table-zone td:nth-child(7) {
            width: 84px;
            text-align: center;
        }

        .status-dot {
            width: 10px;
            height: 10px;
            border-radius: 999px;
            display: inline-block;
        }

        .status-online { background: var(--success); }
        .status-warning { background: var(--warning); }
        .status-offline { background: var(--danger); }
        .status-unmanaged { background: #8b95a7; }

        .row-actions {
            text-align: center;
        }

        .table-action-btn {
            min-height: 28px;
            border-radius: 8px;
            border: 1px solid var(--line);
            background: #fff;
            padding: 0 10px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            color: var(--heading);
            text-decoration: none;
            font-size: 0.8rem;
            font-weight: 600;
            line-height: 1;
        }

        .table-footer {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            padding: 0 10px 10px;
            color: var(--body);
            font-size: 0.88rem;
        }

        .pager {
            display: flex;
            gap: 6px;
        }

        .pager button {
            width: 30px;
            height: 30px;
            border-radius: 8px;
            border: 1px solid var(--line);
            background: #fff;
            color: var(--heading);
        }

        .side-column {
            display: grid;
            gap: 12px;
        }

        .overview-body,
        .chart-body,
        .alarm-body,
        .top-body {
            padding: 12px 14px 14px;
        }

        .progress-line + .progress-line {
            margin-top: 12px;
        }

        .progress-head {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 10px;
            font-size: 0.88rem;
            margin-bottom: 6px;
        }

        .mini-progress {
            height: 8px;
            background: #e9eef5;
            border-radius: 999px;
            overflow: hidden;
        }

        .mini-progress span {
            display: block;
            height: 100%;
            border-radius: inherit;
        }

        .metric-list {
            margin-top: 12px;
            display: grid;
            gap: 8px;
        }

        .metric-item {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            font-size: 0.9rem;
        }

        .chart-layout {
            display: grid;
            grid-template-columns: 150px 1fr;
            gap: 12px;
            align-items: center;
        }

        .donut-chart {
            width: 140px;
            height: 140px;
            border-radius: 50%;
            display: grid;
            place-items: center;
            position: relative;
            margin: 0 auto;
        }

        .donut-chart::before {
            content: "";
            width: 84px;
            height: 84px;
            border-radius: 50%;
            background: #fff;
            box-shadow: inset 0 0 0 1px var(--line);
        }

        .donut-center {
            position: absolute;
            text-align: center;
        }

        .donut-center strong {
            display: block;
            font-size: 1.5rem;
            line-height: 1;
        }

        .donut-center span {
            font-size: 0.85rem;
            color: var(--body);
        }

        .legend-list {
            display: grid;
            gap: 8px;
        }

        .legend-row {
            display: grid;
            grid-template-columns: 12px 1fr auto auto;
            gap: 10px;
            align-items: center;
            font-size: 0.88rem;
        }

        .alarm-list {
            display: grid;
            gap: 10px;
        }

        .alarm-row {
            display: grid;
            grid-template-columns: 10px 1fr auto;
            gap: 10px;
            align-items: center;
            font-size: 0.88rem;
        }

        .alarm-row .bullet {
            width: 10px;
            height: 10px;
            border-radius: 999px;
        }

        .bullet.warning { background: var(--warning); }
        .bullet.danger { background: var(--danger); }

        .see-all {
            margin-top: 10px;
            text-align: right;
        }

        .see-all a {
            text-decoration: none;
            color: var(--heading);
            font-weight: 600;
        }

        .top-switch {
            display: grid;
            grid-template-columns: 1fr minmax(120px, 1fr) auto auto;
            gap: 12px;
            align-items: center;
            font-size: 0.88rem;
        }

        .top-switch + .top-switch {
            margin-top: 10px;
        }

        .top-bar {
            height: 8px;
            border-radius: 999px;
            background: #e9eef5;
            overflow: hidden;
        }

        .top-bar span {
            display: block;
            height: 100%;
            background: var(--success);
            border-radius: inherit;
        }

        @media (max-width: 1499.98px) {
            .summary-row {
                grid-template-columns: repeat(4, minmax(0, 1fr));
            }
        }

        @media (max-width: 1199.98px) {
            .zone-content {
                display: grid;
                grid-template-columns: 1fr;
            }

            .side-column {
                max-width: 100%;
                min-width: 0;
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

            .topbar,
            .title-row {
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
            .summary-row {
                grid-template-columns: repeat(2, minmax(0, 1fr));
            }

            .chart-layout,
            .top-switch {
                grid-template-columns: 1fr;
            }

            .table-wrap {
                overflow-x: auto;
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
                    <a href="#" class="sidebar-link {{ !empty($item['active']) ? 'is-active' : '' }}">
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
                        <div class="breadcrumb-line mb-0">Switches &nbsp;&gt;&nbsp; Zone Detail</div>
                    </div>

                    <div class="toolbar-right">
                        <label class="search-wrap">
                            <input type="search" placeholder="Ara...">
                            <i class="bi bi-search"></i>
                        </label>
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
                    </div>
                </div>

                <div class="title-row">
                    <div class="zone-heading">
                        <i class="bi bi-building fs-2"></i>
                        <h1>{{ $zone['label'] }}</h1>
                        <span class="health-badge {{ $zone['statusClass'] }}">{{ $zone['status'] }}</span>
                    </div>

                    <div class="d-flex gap-2 flex-wrap">
                        <a href="{{ route('switches.create', ['zone' => $zoneSlug]) }}" class="secondary-btn"><i class="bi bi-plus-circle"></i><span>Bu Zone'a Switch Ekle</span></a>
                        <a href="{{ url()->current() }}" class="refresh-btn"><i class="bi bi-arrow-clockwise"></i><span>Yenile</span></a>
                        <a href="#" class="secondary-btn"><i class="bi bi-download"></i><span>Rapor Al</span></a>
                        <a href="#" class="secondary-btn"><i class="bi bi-gear"></i><span>Zone Ayarlari</span></a>
                    </div>
                </div>

                <section class="summary-row">
                    @foreach ($zone['summary'] as $card)
                        <div class="summary-card tone-{{ $card['tone'] }}">
                            <span class="summary-icon"><i class="bi {{ $card['icon'] }}"></i></span>
                            <div>
                                <span class="summary-title">{{ $card['label'] }}</span>
                                <span class="summary-value">{{ is_numeric($card['value']) ? number_format($card['value']) : $card['value'] }}</span>
                            </div>
                        </div>
                    @endforeach
                </section>

                <section class="zone-content">
                    <div class="main-column d-grid gap-3">
                        <div class="card-shell">
                            <div class="card-head">
                                <h2>SWITCH LISTESI</h2>
                                <div class="table-toolbar">
                                    <input id="switch-search" class="input-inline" type="search" placeholder="Switch ara...">
                                    <select id="status-filter" class="filter-select">
                                        <option value="all">Filtrele</option>
                                        <option value="online">Online</option>
                                        <option value="warning">Warning</option>
                                        <option value="offline">Offline</option>
                                        <option value="unmanaged">Yonetilmiyor</option>
                                    </select>
                                </div>
                            </div>

                            <div class="table-wrap">
                                <table class="table table-zone">
                                    <thead>
                                        <tr>
                                            <th>Durum</th>
                                            <th>Hostname</th>
                                            <th>IP Adresi</th>
                                            <th>Vendor</th>
                                            <th>Model</th>
                                            <th>Endpoint Sayisi</th>
                                            <th>Islemler</th>
                                        </tr>
                                    </thead>
                                    <tbody id="switch-table-body">
                                        @foreach ($zone['switches'] as $switch)
                                            <tr
                                                data-status="{{ $switch['state'] }}"
                                                data-search="{{ strtolower($switch['hostname'] . ' ' . $switch['ip'] . ' ' . $switch['vendor'] . ' ' . $switch['model']) }}"
                                                data-href="{{ '/switches/' . $zoneSlug . '/' . strtolower($switch['hostname']) }}"
                                            >
                                                <td><span class="status-dot status-{{ $switch['state'] }}"></span></td>
                                                <td>
                                                    <div class="hostname-cell">
                                                        <span class="hostname-main">{{ $switch['hostname'] }}</span>
                                                        <span class="hostname-meta">{{ $switch['ports'] }} Port | {{ $switch['up'] }} Up | {{ $switch['down'] }} Down</span>
                                                        <div class="hostname-badges">
                                                            <span class="hostname-badge is-uplink">Uplink {{ $switch['uplink_ports'] ?? 0 }}</span>
                                                            <span class="hostname-badge is-mac">MAC {{ $switch['learned_macs'] ?? 0 }}</span>
                                                            @if (($switch['top_mac_count'] ?? 0) > 0)
                                                                <span class="hostname-badge">En Yogun {{ $switch['top_mac_port_name'] }} ({{ $switch['top_mac_count'] }})</span>
                                                            @endif
                                                        </div>
                                                    </div>
                                                </td>
                                                <td>{{ $switch['ip'] }}</td>
                                                <td>{{ $switch['vendor'] }}</td>
                                                <td class="model-cell">{{ $switch['model'] }}</td>
                                                <td>{{ $switch['endpoint'] }}</td>
                                                <td class="row-actions">
                                                    <a href="{{ '/switches/' . $zoneSlug . '/' . strtolower($switch['hostname']) }}" class="table-action-btn">Detay</a>
                                                </td>
                                            </tr>
                                        @endforeach
                                    </tbody>
                                </table>
                            </div>

                            <div class="table-footer">
                                <span>Toplam {{ count($zone['switches']) }} kayit</span>
                                <div class="pager">
                                    <button type="button"><i class="bi bi-chevron-left"></i></button>
                                    <button type="button">1</button>
                                    <button type="button"><i class="bi bi-chevron-right"></i></button>
                                </div>
                            </div>
                        </div>

                        <div class="d-grid" style="grid-template-columns: 1fr 1fr; gap: 12px;">
                            <div class="card-shell">
                                <div class="card-head">
                                    <h3>SON ALARMLAR</h3>
                                </div>
                                <div class="alarm-body">
                                    <div class="alarm-list">
                                        @foreach ($zone['alarms'] as $alarm)
                                            <div class="alarm-row">
                                                <span class="bullet {{ $alarm['tone'] }}"></span>
                                                <span>{{ $alarm['message'] }}</span>
                                                <span>{{ $alarm['time'] }}</span>
                                            </div>
                                        @endforeach
                                    </div>
                                    <div class="see-all"><a href="#">Tumunu Gor <i class="bi bi-chevron-right"></i></a></div>
                                </div>
                            </div>

                            <div class="card-shell">
                                <div class="card-head">
                                    <h3>EN COK KULLANILAN SWITCHLER (TOP 5)</h3>
                                </div>
                                <div class="top-body">
                                    @foreach ($zone['topSwitches'] as $index => $item)
                                        <div class="top-switch">
                                            <strong>{{ $index + 1 }}. {{ $item['hostname'] }}</strong>
                                            <div class="top-bar"><span style="width: {{ $item['ratio'] }}%;"></span></div>
                                            <span>{{ $item['used'] }} / {{ $item['total'] }}</span>
                                            <span>{{ $item['ratio'] }}%</span>
                                        </div>
                                    @endforeach
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="side-column">
                        <div class="card-shell">
                            <div class="card-head"><h3>ZONE GENEL BAKIS</h3></div>
                            <div class="overview-body">
                                @foreach ($zone['overview'] as $metric)
                                    <div class="progress-line">
                                        <div class="progress-head">
                                            <span>{{ $metric['label'] }}</span>
                                            <strong>{{ $metric['value'] }}</strong>
                                        </div>
                                        <div class="mini-progress">
                                            <span style="width: {{ $metric['percent'] }}%; background: {{ $metric['tone'] === 'warning' ? '#ff9f1a' : '#41b349' }};"></span>
                                        </div>
                                    </div>
                                @endforeach

                                <div class="metric-list">
                                    @foreach ($zone['security'] as $item)
                                        <div class="metric-item">
                                            <span>{{ $item['label'] }}</span>
                                            <strong class="text-{{ $item['tone'] }}">{{ $item['value'] }}</strong>
                                        </div>
                                    @endforeach
                                </div>
                            </div>
                        </div>

                        <div class="card-shell">
                            <div class="card-head"><h3>ENDPOINT DAGILIMI</h3></div>
                            <div class="chart-body">
                                <div class="chart-layout">
                                    <div class="donut-chart" style="background: {{ $endpointChart }};">
                                        <div class="donut-center">
                                            <strong>{{ $endpointTotal }}</strong>
                                            <span>Toplam</span>
                                        </div>
                                    </div>
                                    <div class="legend-list">
                                        @foreach ($zone['endpointSegments'] as $segment)
                                            @php
                                                $endpointPercentage = $endpointTotal !== 0
                                                    ? number_format(($segment['value'] / $endpointTotal) * 100, 1)
                                                    : number_format(0, 1);
                                            @endphp
                                            <div class="legend-row">
                                                <span class="legend-dot" style="background: {{ $segment['color'] }};"></span>
                                                <span>{{ $segment['label'] }}</span>
                                                <strong>{{ $segment['value'] }}</strong>
                                                <span>({{ $endpointPercentage }}%)</span>
                                            </div>
                                        @endforeach
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="card-shell">
                            <div class="card-head"><h3>PORT DURUM DAGILIMI</h3></div>
                            <div class="chart-body">
                                <div class="chart-layout">
                                    <div class="donut-chart" style="background: {{ $portChart }};">
                                        <div class="donut-center">
                                            <strong>{{ $portTotal }}</strong>
                                            <span>Toplam Port</span>
                                        </div>
                                    </div>
                                    <div class="legend-list">
                                        @foreach ($zone['portSegments'] as $segment)
                                            @php
                                                $portPercentage = $portTotal !== 0
                                                    ? number_format(($segment['value'] / $portTotal) * 100, 1)
                                                    : number_format(0, 1);
                                            @endphp
                                            <div class="legend-row">
                                                <span class="legend-dot" style="background: {{ $segment['color'] }};"></span>
                                                <span>{{ $segment['label'] }}</span>
                                                <strong>{{ $segment['value'] }}</strong>
                                                <span>({{ $portPercentage }}%)</span>
                                            </div>
                                        @endforeach
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </section>
            </div>
        </main>
    </div>

    <script>
        const searchInput = document.getElementById('switch-search');
        const statusFilter = document.getElementById('status-filter');
        const rows = Array.from(document.querySelectorAll('#switch-table-body tr'));

        function filterRows() {
            const query = (searchInput.value || '').toLowerCase().trim();
            const status = statusFilter.value;

            rows.forEach(function (row) {
                const matchesText = row.dataset.search.includes(query);
                const matchesStatus = status === 'all' || row.dataset.status === status;
                row.style.display = matchesText && matchesStatus ? '' : 'none';
            });
        }

        searchInput.addEventListener('input', filterRows);
        statusFilter.addEventListener('change', filterRows);

        rows.forEach(function (row) {
            row.addEventListener('click', function () {
                window.location.href = row.dataset.href;
            });
        });

        document.querySelectorAll('.table-action-btn').forEach(function (button) {
            button.addEventListener('click', function (event) {
                event.stopPropagation();
            });
        });
    </script>
</body>
</html>
