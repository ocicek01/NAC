CREATE TABLE IF NOT EXISTS mac_ip_bindings (
    id UUID PRIMARY KEY,
    switch_id UUID NOT NULL REFERENCES switches(id) ON DELETE CASCADE,
    mac_address TEXT NOT NULL,
    ip_address INET NOT NULL,
    source TEXT NOT NULL DEFAULT 'arp',
    hostname TEXT NOT NULL DEFAULT '',
    vendor_class TEXT NOT NULL DEFAULT '',
    options55 TEXT NOT NULL DEFAULT '',
    vlan_id INTEGER NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (switch_id, mac_address, ip_address, source)
);

CREATE INDEX IF NOT EXISTS idx_mac_ip_bindings_switch_last_seen
    ON mac_ip_bindings (switch_id, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_mac_ip_bindings_mac_last_seen
    ON mac_ip_bindings (mac_address, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_mac_ip_bindings_ip_last_seen
    ON mac_ip_bindings (ip_address, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS arp_snapshots (
    id UUID PRIMARY KEY,
    switch_id UUID NOT NULL REFERENCES switches(id) ON DELETE CASCADE,
    if_index INTEGER NOT NULL DEFAULT 0,
    mac_address TEXT NOT NULL,
    ip_address INET NOT NULL,
    source TEXT NOT NULL DEFAULT 'arp',
    vlan_id INTEGER NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (switch_id, mac_address, ip_address)
);

CREATE INDEX IF NOT EXISTS idx_arp_snapshots_switch_last_seen
    ON arp_snapshots (switch_id, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_arp_snapshots_mac_last_seen
    ON arp_snapshots (mac_address, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_arp_snapshots_ip_last_seen
    ON arp_snapshots (ip_address, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS port_endpoints (
    id UUID PRIMARY KEY,
    switch_id UUID NOT NULL REFERENCES switches(id) ON DELETE CASCADE,
    port_ifindex INTEGER NOT NULL,
    mac_address TEXT NOT NULL,
    ip_address INET NULL,
    hostname TEXT NOT NULL DEFAULT '',
    source_confidence TEXT NOT NULL DEFAULT '',
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (switch_id, port_ifindex, mac_address)
);

CREATE INDEX IF NOT EXISTS idx_port_endpoints_switch_port
    ON port_endpoints (switch_id, port_ifindex);

CREATE INDEX IF NOT EXISTS idx_port_endpoints_mac_last_seen
    ON port_endpoints (mac_address, last_seen_at DESC);
