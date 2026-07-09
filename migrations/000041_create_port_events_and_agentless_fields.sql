ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS classification_method TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS trust_level TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS authentication_method TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS authentication_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sophos_username TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sophos_last_ip INET NULL,
    ADD COLUMN IF NOT EXISTS sophos_last_seen_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_policy_decision TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_policy_evaluated_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_devices_auth_status_last_seen
    ON devices (authentication_status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_devices_sophos_username_last_seen
    ON devices (sophos_username, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS port_events (
    id UUID PRIMARY KEY,
    switch_id UUID NOT NULL REFERENCES switches(id) ON DELETE CASCADE,
    switch_name TEXT NOT NULL DEFAULT '',
    management_ip INET NULL,
    if_index INTEGER NOT NULL,
    interface_name TEXT NOT NULL DEFAULT '',
    interface_description TEXT NOT NULL DEFAULT '',
    admin_status TEXT NOT NULL DEFAULT '',
    oper_status TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL DEFAULT 'port_observed',
    event_source TEXT NOT NULL DEFAULT '',
    mac_address TEXT NOT NULL DEFAULT '',
    ip_address INET NULL,
    hostname TEXT NOT NULL DEFAULT '',
    vendor_class TEXT NOT NULL DEFAULT '',
    device_type TEXT NOT NULL DEFAULT '',
    policy_action TEXT NOT NULL DEFAULT '',
    policy_reason TEXT NOT NULL DEFAULT '',
    enforcement_action TEXT NOT NULL DEFAULT '',
    trust_level TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_port_events_switch_observed
    ON port_events (switch_id, if_index, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_port_events_mac_observed
    ON port_events (mac_address, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_port_events_event_type_observed
    ON port_events (event_type, observed_at DESC);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY,
    actor TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL DEFAULT '',
    target_id TEXT NOT NULL DEFAULT '',
    switch_id UUID NULL REFERENCES switches(id) ON DELETE SET NULL,
    mac_address TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created
    ON audit_logs (action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_switch_created
    ON audit_logs (switch_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_mac_created
    ON audit_logs (mac_address, created_at DESC);
