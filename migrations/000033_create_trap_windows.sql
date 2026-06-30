CREATE TABLE IF NOT EXISTS trap_windows (
    id UUID PRIMARY KEY,
    dedupe_key TEXT NOT NULL UNIQUE,
    switch_id UUID NOT NULL REFERENCES switches(id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    port_ifindex INTEGER NOT NULL DEFAULT 0,
    mac_address TEXT NOT NULL DEFAULT '',
    vlan_id INTEGER NOT NULL DEFAULT 0,
    event_count INTEGER NOT NULL DEFAULT 1,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dispatched_at TIMESTAMPTZ NULL,
    trap_oid TEXT NOT NULL DEFAULT '',
    enterprise_oid TEXT NOT NULL DEFAULT '',
    source_ip INET NULL,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_trap_windows_status_available_at
    ON trap_windows (status, available_at);

CREATE INDEX IF NOT EXISTS idx_trap_windows_switch_status
    ON trap_windows (switch_id, status, available_at);

CREATE INDEX IF NOT EXISTS idx_trap_windows_last_seen_at
    ON trap_windows (last_seen_at DESC);
