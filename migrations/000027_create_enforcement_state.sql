CREATE TABLE IF NOT EXISTS enforcement_state (
    mac_address TEXT NOT NULL,
    switch_id UUID NOT NULL,
    if_index INTEGER NOT NULL,
    interface_name TEXT NOT NULL DEFAULT '',
    policy_action TEXT NOT NULL DEFAULT '',
    target_vlan INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT '',
    locked_until TIMESTAMPTZ NULL,
    decision_id UUID NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (mac_address, switch_id, if_index)
);

CREATE INDEX IF NOT EXISTS idx_enforcement_state_updated_at
    ON enforcement_state (updated_at DESC);
