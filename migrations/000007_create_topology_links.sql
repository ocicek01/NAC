CREATE TABLE IF NOT EXISTS topology_links (
    id UUID PRIMARY KEY,
    source_switch_id UUID NOT NULL,
    source_switch_name TEXT NOT NULL DEFAULT '',
    source_port_name TEXT NOT NULL,
    target_switch_id UUID NULL,
    target_switch_name TEXT NOT NULL DEFAULT '',
    target_port_name TEXT NOT NULL DEFAULT '',
    discovery_method TEXT NOT NULL DEFAULT 'manual',
    status TEXT NOT NULL DEFAULT 'active',
    last_observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_topology_links_source
    ON topology_links (source_switch_id, source_port_name);

CREATE INDEX IF NOT EXISTS idx_topology_links_target
    ON topology_links (target_switch_id, target_port_name);

CREATE UNIQUE INDEX IF NOT EXISTS idx_topology_links_unique
    ON topology_links (source_switch_id, source_port_name, target_switch_name, target_port_name);
