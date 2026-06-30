CREATE TABLE IF NOT EXISTS switch_ports (
    id UUID PRIMARY KEY,
    switch_id UUID NOT NULL REFERENCES switches(id) ON DELETE CASCADE,
    if_index INTEGER NOT NULL,
    port_index INTEGER NOT NULL DEFAULT 0,
    interface_name TEXT NOT NULL DEFAULT '',
    interface_alias TEXT NOT NULL DEFAULT '',
    interface_description TEXT NOT NULL DEFAULT '',
    port_label TEXT NOT NULL DEFAULT '',
    interface_type TEXT NOT NULL DEFAULT '',
    admin_status TEXT NOT NULL DEFAULT '',
    oper_status TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',
    port_mode TEXT NOT NULL DEFAULT 'unknown',
    is_physical BOOLEAN NOT NULL DEFAULT true,
    is_uplink BOOLEAN NOT NULL DEFAULT false,
    is_trunk BOOLEAN NOT NULL DEFAULT false,
    trunk_source TEXT NOT NULL DEFAULT '',
    vlan_id INTEGER NOT NULL DEFAULT 0,
    native_vlan INTEGER NOT NULL DEFAULT 0,
    allowed_vlans TEXT[] NOT NULL DEFAULT '{}',
    voice_vlan INTEGER NOT NULL DEFAULT 0,
    mac_count INTEGER NOT NULL DEFAULT 0,
    mac_addresses JSONB NOT NULL DEFAULT '[]'::jsonb,
    speed_bps BIGINT NOT NULL DEFAULT 0,
    speed_label TEXT NOT NULL DEFAULT '',
    duplex TEXT NOT NULL DEFAULT '',
    poe_enabled BOOLEAN NOT NULL DEFAULT false,
    poe_power_watts NUMERIC(10,2) NOT NULL DEFAULT 0,
    neighbor_protocol TEXT NOT NULL DEFAULT '',
    neighbor_switch_id UUID NULL REFERENCES switches(id) ON DELETE SET NULL,
    neighbor_switch_name TEXT NOT NULL DEFAULT '',
    neighbor_port_name TEXT NOT NULL DEFAULT '',
    neighbor_platform TEXT NOT NULL DEFAULT '',
    neighbor_description TEXT NOT NULL DEFAULT '',
    neighbor_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_changed_at TIMESTAMPTZ NULL,
    last_discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (switch_id, if_index)
);

CREATE INDEX IF NOT EXISTS idx_switch_ports_switch_port_index
    ON switch_ports (switch_id, port_index);

CREATE INDEX IF NOT EXISTS idx_switch_ports_switch_uplink
    ON switch_ports (switch_id, is_uplink, is_trunk);

CREATE INDEX IF NOT EXISTS idx_switch_ports_neighbor_switch
    ON switch_ports (neighbor_switch_id);

CREATE INDEX IF NOT EXISTS idx_switch_ports_last_discovered_at
    ON switch_ports (last_discovered_at DESC);
