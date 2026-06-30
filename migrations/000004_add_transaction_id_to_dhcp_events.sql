ALTER TABLE dhcp_events
    ADD COLUMN IF NOT EXISTS transaction_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_dhcp_events_dedup
    ON dhcp_events (mac_address, message_type, transaction_id, created_at DESC);
