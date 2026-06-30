ALTER TABLE topology_links
    ALTER COLUMN target_switch_id DROP NOT NULL;

ALTER TABLE topology_links
    ALTER COLUMN target_port_name SET DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_topology_links_unique
    ON topology_links (source_switch_id, source_port_name, target_switch_name, target_port_name);
