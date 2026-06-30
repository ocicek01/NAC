CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY,
    mac_address VARCHAR(17) NOT NULL UNIQUE,
    hostname TEXT NOT NULL DEFAULT '',
    vendor_class TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',
    current_switch_id UUID NULL,
    current_port_id UUID NULL,
    first_seen_at TIMESTAMPTZ NULL,
    last_seen_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
