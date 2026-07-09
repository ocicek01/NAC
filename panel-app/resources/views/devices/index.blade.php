@php
    $devices = $devices ?? collect();
    $portEvents = $portEvents ?? collect();
    $auditLogs = $auditLogs ?? collect();
    $summary = $summary ?? [];

    $statusMap = [
        'allowed' => ['label' => 'Allowed', 'tone' => '#2ea44f'],
        'pending' => ['label' => 'Pending', 'tone' => '#f08c24'],
        'blocked' => ['label' => 'Blocked', 'tone' => '#e03131'],
        'expired' => ['label' => 'Expired', 'tone' => '#b7791f'],
        'retired' => ['label' => 'Retired', 'tone' => '#6b7280'],
    ];
@endphp
<!DOCTYPE html>
<html lang="tr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NAC Panel | Device Registry</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.3/font/bootstrap-icons.css" rel="stylesheet">
    <style>
        body { background:#f4f7fb; color:#13233d; font-family:"Segoe UI",Tahoma,Geneva,Verdana,sans-serif; }
        .shell { max-width: 1580px; margin: 0 auto; padding: 18px; }
        .toolbar, .panel, .stat-card { background:#fff; border:1px solid #d7e1ee; border-radius:16px; box-shadow:0 10px 24px rgba(19,35,61,.05); }
        .toolbar { padding:14px 16px; display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:14px; }
        .toolbar a { text-decoration:none; }
        .stats { display:grid; grid-template-columns:repeat(4, minmax(0, 1fr)); gap:12px; margin-bottom:14px; }
        .stat-card { padding:16px; }
        .stat-label { color:#5d6b80; font-size:.82rem; text-transform:uppercase; letter-spacing:.04em; }
        .stat-value { font-size:1.6rem; font-weight:700; margin-top:6px; }
        .table-wrap { overflow:auto; }
        table { width:100%; min-width:2120px; }
        th { font-size:.8rem; letter-spacing:.04em; color:#5d6b80; text-transform:uppercase; }
        td, th { padding:12px 14px; border-bottom:1px solid #e6edf5; vertical-align:middle; }
        tr:last-child td { border-bottom:0; }
        .mac { font-weight:700; letter-spacing:.03em; }
        .muted { color:#6b7b91; font-size:.88rem; }
        .badge-state { display:inline-flex; align-items:center; gap:8px; padding:6px 10px; border-radius:999px; font-weight:700; font-size:.82rem; }
        .dot { width:10px; height:10px; border-radius:999px; }
        .action-group { display:flex; gap:8px; flex-wrap:wrap; }
        .mini-form { display:flex; gap:8px; align-items:center; }
        .mini-form input { width:88px; }
        .pill { display:inline-flex; padding:4px 8px; border-radius:999px; background:#eef4fb; color:#2558b8; font-size:.78rem; font-weight:700; }
        .grid-two { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:14px; margin-top:14px; }
        .panel-head { padding:14px 16px 0 16px; }
        .panel-head h2 { font-size:1rem; margin:0; }
        .mono { font-family:Consolas, Monaco, monospace; }
        @media (max-width: 992px) {
            .stats, .grid-two { grid-template-columns:1fr; }
        }
    </style>
</head>
<body>
    <div class="shell">
        <div class="toolbar">
            <div>
                <h1 class="h4 mb-1">Device Registry</h1>
                <div class="muted">Agentless endpoint inventory, policy state, Sophos korelasyonu ve son olaylar</div>
            </div>
            <div class="d-flex gap-2">
                <a href="{{ route('switches.index') }}" class="btn btn-outline-secondary">
                    <i class="bi bi-diagram-3"></i> Switches
                </a>
                <a href="{{ route('devices.index') }}" class="btn btn-outline-primary">
                    <i class="bi bi-arrow-clockwise"></i> Yenile
                </a>
            </div>
        </div>

        <div class="stats">
            <div class="stat-card">
                <div class="stat-label">Toplam Endpoint</div>
                <div class="stat-value">{{ $summary['device_count'] ?? 0 }}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Pending</div>
                <div class="stat-value">{{ $summary['pending_count'] ?? 0 }}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Sophos Eslesmis</div>
                <div class="stat-value">{{ $summary['authenticated_count'] ?? 0 }}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Son Port Event</div>
                <div class="stat-value">{{ $summary['observed_count'] ?? 0 }}</div>
            </div>
        </div>

        @if (session('success'))
            <div class="alert alert-success">{{ session('success') }}</div>
        @endif
        @if (session('error'))
            <div class="alert alert-danger">{{ session('error') }}</div>
        @endif

        <div class="panel table-wrap">
            <table>
                <thead>
                    <tr>
                        <th>Device</th>
                        <th>Status</th>
                        <th>Attachment</th>
                        <th>Authentication</th>
                        <th>Identity / Approval</th>
                        <th>Classification</th>
                        <th>LDAP Metadata</th>
                        <th>Enforcement</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    @forelse ($devices as $device)
                        @php
                            $status = strtolower((string) ($device['status'] ?? 'pending'));
                            $statusMeta = $statusMap[$status] ?? ['label' => ucfirst($status), 'tone' => '#6b7280'];
                        @endphp
                        <tr>
                            <td>
                                <div class="mac">{{ $device['mac_address'] ?? '-' }}</div>
                                <div class="muted">{{ $device['hostname'] ?: ($device['vendor_class'] ?? 'Unknown endpoint') }}</div>
                                <div class="muted">Type: {{ $device['device_type'] ?? 'unknown' }}</div>
                                <div class="muted">IP: {{ $device['current_ip_address'] ?: '-' }}</div>
                            </td>
                            <td>
                                <span class="badge-state" style="background: {{ $statusMeta['tone'] }}14; color: {{ $statusMeta['tone'] }};">
                                    <span class="dot" style="background: {{ $statusMeta['tone'] }};"></span>
                                    {{ $statusMeta['label'] }}
                                </span>
                                <div class="muted mt-2">Expires: {{ $device['expires_at'] ?? '-' }}</div>
                            </td>
                            <td>
                                <div>{{ $device['current_switch_name'] ?? '-' }}</div>
                                <div class="muted">{{ $device['current_management_ip'] ?? '' }}</div>
                                <div class="pill mt-2">{{ $device['current_interface_name'] ?? ('ifIndex '.($device['current_if_index'] ?? '-')) }}</div>
                                <div class="muted mt-2">Source: {{ $device['current_source_type'] ?: '-' }}</div>
                                <div class="muted">Confidence: {{ $device['current_confidence'] ?: '-' }}</div>
                            </td>
                            <td>
                                <div>Method: <strong>{{ $device['authentication_method'] ?: 'agentless' }}</strong></div>
                                <div class="muted">Status: {{ $device['authentication_status'] ?: 'not_attempted' }}</div>
                                <div class="muted">Sophos User: {{ $device['sophos_username'] ?: '-' }}</div>
                                <div class="muted">Sophos IP: {{ $device['sophos_last_ip'] ?: '-' }}</div>
                            </td>
                            <td>
                                <div>Approved By: <strong>{{ $device['approved_by'] ?: '-' }}</strong></div>
                                <div class="muted">Approved At: {{ $device['approved_at'] ?? '-' }}</div>
                                <div class="muted">Identity: {{ $device['identity_username'] ?: '-' }}</div>
                                <div class="muted">Policy: {{ $device['policy_action'] ?: '-' }}</div>
                            </td>
                            <td>
                                <div>Trust: <strong>{{ $device['trust_level'] ?: '-' }}</strong></div>
                                <div class="muted">Classifier: {{ $device['classification_method'] ?: '-' }}</div>
                                <div class="muted">Decision: {{ $device['last_policy_decision'] ?: '-' }}</div>
                                <div class="muted">Evaluated: {{ $device['last_policy_evaluated_at'] ?: '-' }}</div>
                            </td>
                            <td>
                                <div>Owner: <strong>{{ $device['ldap_owner_dn'] ?: '-' }}</strong></div>
                                <div class="muted">Location: {{ $device['ldap_location_dn'] ?: '-' }}</div>
                                <div class="muted">Dept: {{ $device['ldap_department'] ?: '-' }}</div>
                                <div class="muted">Policy: {{ $device['ldap_policy_name'] ?: '-' }}</div>
                                <div class="muted">VLAN: {{ ($device['ldap_default_vlan_id'] ?? 0) > 0 ? (($device['ldap_default_vlan_id'] ?? 0).' / '.($device['ldap_default_vlan_name'] ?: '-')) : (($device['ldap_vlan_id'] ?? 0) > 0 ? (($device['ldap_vlan_id'] ?? 0).' / '.($device['ldap_vlan_name'] ?: '-')) : '-') }}</div>
                                <div class="muted">Asset: {{ $device['ldap_asset_tag'] ?: '-' }}</div>
                            </td>
                            <td>
                                <div>Action: <strong>{{ $device['last_enforcement_action'] ?: '-' }}</strong></div>
                                <div class="muted">Method: {{ $device['last_enforcement_method'] ?: '-' }}</div>
                                <div class="muted">Applied VLAN: {{ $device['applied_enforcement_vlan'] ?? 0 }}</div>
                                <div class="muted">State: {{ $device['applied_enforcement_state'] ?: '-' }}</div>
                            </td>
                            <td>
                                <div class="action-group">
                                    <form method="POST" action="{{ route('devices.approve', ['mac' => $device['mac_address']]) }}" class="mini-form">
                                        @csrf
                                        <input type="number" name="target_vlan" class="form-control form-control-sm" value="106" min="1">
                                        <button class="btn btn-sm btn-success" type="submit">Allow</button>
                                    </form>
                                    <form method="POST" action="{{ route('devices.block', ['mac' => $device['mac_address']]) }}">
                                        @csrf
                                        <button class="btn btn-sm btn-warning" type="submit">Block</button>
                                    </form>
                                    <form method="POST" action="{{ route('devices.retire', ['mac' => $device['mac_address']]) }}">
                                        @csrf
                                        <button class="btn btn-sm btn-outline-secondary" type="submit">Retire</button>
                                    </form>
                                </div>
                            </td>
                        </tr>
                    @empty
                        <tr>
                            <td colspan="9" class="text-center text-muted py-5">Cihaz kaydi bulunamadi.</td>
                        </tr>
                    @endforelse
                </tbody>
            </table>
        </div>

        <div class="grid-two">
            <div class="panel table-wrap">
                <div class="panel-head">
                    <h2>Son Port Event</h2>
                    <div class="muted">Port up/down, MAC learned ve agentless classification akisi</div>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>Time</th>
                            <th>Switch / Port</th>
                            <th>MAC / IP</th>
                            <th>Event</th>
                            <th>Policy</th>
                        </tr>
                    </thead>
                    <tbody>
                        @forelse ($portEvents as $event)
                            <tr>
                                <td class="mono">{{ $event['observed_at'] ?? '-' }}</td>
                                <td>
                                    <div>{{ $event['switch_name'] ?? '-' }}</div>
                                    <div class="muted">{{ $event['interface_name'] ?? ('ifIndex '.($event['if_index'] ?? '-')) }}</div>
                                </td>
                                <td>
                                    <div class="mac">{{ $event['mac_address'] ?: '-' }}</div>
                                    <div class="muted">{{ $event['ip_address'] ?: '-' }}</div>
                                </td>
                                <td>
                                    <div>{{ $event['event_type'] ?? '-' }}</div>
                                    <div class="muted">{{ $event['event_source'] ?? '-' }}</div>
                                    <div class="muted">Oper: {{ $event['oper_status'] ?? '-' }}</div>
                                </td>
                                <td>
                                    <div>{{ $event['policy_action'] ?: '-' }}</div>
                                    <div class="muted">{{ $event['policy_reason'] ?: '-' }}</div>
                                </td>
                            </tr>
                        @empty
                            <tr>
                                <td colspan="5" class="text-center text-muted py-4">Port event bulunamadi.</td>
                            </tr>
                        @endforelse
                    </tbody>
                </table>
            </div>

            <div class="panel table-wrap">
                <div class="panel-head">
                    <h2>Son Audit Log</h2>
                    <div class="muted">Agentless pipeline ve enforcement kararlarinin backend kaydi</div>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>Time</th>
                            <th>Action</th>
                            <th>Target</th>
                            <th>Status</th>
                            <th>MAC</th>
                        </tr>
                    </thead>
                    <tbody>
                        @forelse ($auditLogs as $log)
                            <tr>
                                <td class="mono">{{ $log['created_at'] ?? '-' }}</td>
                                <td>
                                    <div>{{ $log['action'] ?? '-' }}</div>
                                    <div class="muted">{{ $log['actor'] ?? 'system' }}</div>
                                </td>
                                <td>
                                    <div>{{ $log['target_type'] ?? '-' }}</div>
                                    <div class="muted">{{ $log['target_id'] ?? '-' }}</div>
                                </td>
                                <td>{{ $log['status'] ?: '-' }}</td>
                                <td class="mac">{{ $log['mac_address'] ?: '-' }}</td>
                            </tr>
                        @empty
                            <tr>
                                <td colspan="5" class="text-center text-muted py-4">Audit log bulunamadi.</td>
                            </tr>
                        @endforelse
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</body>
</html>
