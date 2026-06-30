CREATE TABLE IF NOT EXISTS radius_sessions (
    id UUID PRIMARY KEY,
    active_key TEXT NOT NULL UNIQUE,
    device_id UUID NULL,
    switch_id UUID NULL,
    switch_name TEXT NOT NULL DEFAULT '',
    management_ip INET NULL,
    port_id TEXT NOT NULL DEFAULT '',
    nas_port TEXT NOT NULL DEFAULT '',
    nas_port_id TEXT NOT NULL DEFAULT '',
    if_index INTEGER NOT NULL DEFAULT 0,
    interface_name TEXT NOT NULL DEFAULT '',
    ip_address INET NULL,
    mac_address VARCHAR(17) NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    hostname TEXT NOT NULL DEFAULT '',
    vendor_class TEXT NOT NULL DEFAULT '',
    called_station_id TEXT NOT NULL DEFAULT '',
    calling_station_id TEXT NOT NULL DEFAULT '',
    acct_session_id TEXT NOT NULL DEFAULT '',
    authorization_result TEXT NOT NULL DEFAULT '',
    session_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    policy_action TEXT NOT NULL DEFAULT '',
    policy_reason TEXT NOT NULL DEFAULT '',
    assigned_vlan TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NULL,
    last_seen_at TIMESTAMPTZ NULL,
    ended_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_radius_sessions_mac_updated_at
    ON radius_sessions (mac_address, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_radius_sessions_switch_ifindex
    ON radius_sessions (switch_id, if_index);

CREATE INDEX IF NOT EXISTS idx_radius_sessions_acct_session_id
    ON radius_sessions (acct_session_id);
