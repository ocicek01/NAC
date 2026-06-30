ALTER TABLE dhcp_events
    ADD COLUMN IF NOT EXISTS relay_switch_id UUID NULL,
    ADD COLUMN IF NOT EXISTS relay_switch_name TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_dhcp_events_relay_switch_id_created_at
    ON dhcp_events (relay_switch_id, created_at DESC);
