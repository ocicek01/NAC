CREATE TABLE IF NOT EXISTS dhcp_events (
    id UUID PRIMARY KEY,
    mac_address VARCHAR(17) NOT NULL,
    transaction_id TEXT NOT NULL DEFAULT '',
    source_ip INET NULL,
    message_type TEXT NOT NULL,
    hostname TEXT NOT NULL DEFAULT '',
    vendor_class TEXT NOT NULL DEFAULT '',
    option82_raw TEXT NOT NULL DEFAULT '',
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dhcp_events_dedup
    ON dhcp_events (mac_address, message_type, transaction_id, created_at DESC);
