CREATE TABLE IF NOT EXISTS mac_observations (
    id UUID PRIMARY KEY,
    dhcp_event_id UUID NULL,
    mac_address VARCHAR(17) NOT NULL,
    switch_id UUID NOT NULL,
    switch_name TEXT NOT NULL DEFAULT '',
    management_ip INET NULL,
    bridge_port INTEGER NOT NULL DEFAULT 0,
    if_index INTEGER NOT NULL DEFAULT 0,
    interface_name TEXT NOT NULL DEFAULT '',
    interface_description TEXT NOT NULL DEFAULT '',
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mac_observations_recent
    ON mac_observations (mac_address, created_at DESC);
