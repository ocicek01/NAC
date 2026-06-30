ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS last_enforcement_action TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_enforcement_vlan INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_enforcement_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_enforcement_switch_id UUID NULL,
    ADD COLUMN IF NOT EXISTS last_enforcement_if_index INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_enforcement_at TIMESTAMPTZ NULL;
