ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS registered_vendor TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS registered_owner TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS owner_username TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS owner_department TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS owner_role TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS default_vlan_id INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS default_vlan_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS assigned_policy TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS enrichment_source TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS enrichment_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS enrichment_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS enriched_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_devices_enrichment_status_last_seen
    ON devices (enrichment_status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_devices_owner_username_last_seen
    ON devices (owner_username, last_seen_at DESC);
