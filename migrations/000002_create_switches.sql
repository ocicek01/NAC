CREATE TABLE IF NOT EXISTS switches (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    management_ip INET NOT NULL UNIQUE,
    vendor TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',
    snmp_version TEXT NOT NULL DEFAULT '2c',
    snmp_community TEXT NOT NULL DEFAULT '',
    snmp_port INTEGER NOT NULL DEFAULT 161,
    snmp_timeout_ms INTEGER NOT NULL DEFAULT 2000,
    snmp_retries INTEGER NOT NULL DEFAULT 1,
    last_polled_at TIMESTAMPTZ NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
