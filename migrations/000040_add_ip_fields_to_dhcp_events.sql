ALTER TABLE dhcp_events
    ADD COLUMN IF NOT EXISTS client_ip INET NULL,
    ADD COLUMN IF NOT EXISTS your_ip INET NULL,
    ADD COLUMN IF NOT EXISTS requested_ip INET NULL;

CREATE INDEX IF NOT EXISTS idx_dhcp_events_mac_observed_ip
    ON dhcp_events (mac_address, observed_at DESC);
