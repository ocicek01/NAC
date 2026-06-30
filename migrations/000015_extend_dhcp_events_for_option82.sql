ALTER TABLE dhcp_events
    ADD COLUMN IF NOT EXISTS option82_circuit_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS option82_remote_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS option82_vlan TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS relay_ip INET NULL;

CREATE INDEX IF NOT EXISTS idx_dhcp_events_relay_ip_created_at
    ON dhcp_events (relay_ip, created_at DESC);
