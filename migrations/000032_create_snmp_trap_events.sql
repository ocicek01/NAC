CREATE TABLE IF NOT EXISTS snmp_trap_events (
    id UUID PRIMARY KEY,
    source_ip INET NOT NULL,
    source_port INTEGER NOT NULL DEFAULT 0,
    switch_id UUID NULL REFERENCES switches(id) ON DELETE SET NULL,
    switch_name TEXT NOT NULL DEFAULT '',
    snmp_version TEXT NOT NULL DEFAULT '',
    community TEXT NOT NULL DEFAULT '',
    trap_oid TEXT NOT NULL DEFAULT '',
    enterprise_oid TEXT NOT NULL DEFAULT '',
    generic_trap INTEGER NOT NULL DEFAULT 0,
    specific_trap INTEGER NOT NULL DEFAULT 0,
    uptime_ticks BIGINT NOT NULL DEFAULT 0,
    varbinds JSONB NOT NULL DEFAULT '[]'::jsonb,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_snmp_trap_events_received_at
    ON snmp_trap_events (received_at DESC);

CREATE INDEX IF NOT EXISTS idx_snmp_trap_events_source_ip
    ON snmp_trap_events (source_ip, received_at DESC);

CREATE INDEX IF NOT EXISTS idx_snmp_trap_events_switch_id
    ON snmp_trap_events (switch_id, received_at DESC);
