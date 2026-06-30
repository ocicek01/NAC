CREATE TABLE IF NOT EXISTS mac_observation_candidates (
    id UUID PRIMARY KEY,
    observation_id UUID NOT NULL,
    dhcp_event_id UUID NULL,
    mac_address VARCHAR(17) NOT NULL,
    switch_id UUID NOT NULL,
    switch_name TEXT NOT NULL DEFAULT '',
    management_ip INET NULL,
    bridge_port INTEGER NOT NULL DEFAULT 0,
    if_index INTEGER NOT NULL DEFAULT 0,
    interface_name TEXT NOT NULL DEFAULT '',
    interface_description TEXT NOT NULL DEFAULT '',
    score INTEGER NOT NULL DEFAULT 0,
    is_selected BOOLEAN NOT NULL DEFAULT FALSE,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mac_observation_candidates_observation
    ON mac_observation_candidates (observation_id, score DESC);
