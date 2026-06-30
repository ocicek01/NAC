CREATE TABLE IF NOT EXISTS enforcement_decisions (
    id UUID PRIMARY KEY,
    device_mac_address TEXT NOT NULL,
    device_hostname TEXT NOT NULL DEFAULT '',
    policy_action TEXT NOT NULL DEFAULT '',
    policy_reason TEXT NOT NULL DEFAULT '',
    decision_action TEXT NOT NULL,
    decision_mode TEXT NOT NULL DEFAULT 'dry-run',
    switch_id UUID NULL REFERENCES switches(id) ON DELETE SET NULL,
    switch_name TEXT NOT NULL DEFAULT '',
    management_ip INET NULL,
    interface_name TEXT NOT NULL DEFAULT '',
    interface_description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_enforcement_decisions_created_at
    ON enforcement_decisions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_enforcement_decisions_mac
    ON enforcement_decisions (device_mac_address);
