ALTER TABLE devices
    ALTER COLUMN status SET DEFAULT 'pending';

UPDATE devices
SET status = 'pending'
WHERE LOWER(status) = 'unknown';

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS device_type TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS approved_by TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_devices_status_last_seen
    ON devices (status, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS device_identity_snapshots (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    identity_type TEXT NOT NULL DEFAULT '',
    identity_source TEXT NOT NULL DEFAULT '',
    external_id TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    full_name TEXT NOT NULL DEFAULT '',
    attributes_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_device_identity_snapshots_device_verified
    ON device_identity_snapshots (device_id, verified_at DESC);

CREATE INDEX IF NOT EXISTS idx_device_identity_snapshots_external_id
    ON device_identity_snapshots (external_id, verified_at DESC);

CREATE TABLE IF NOT EXISTS device_observations (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    mac_address TEXT NOT NULL,
    ip_address INET NULL,
    switch_id UUID NULL REFERENCES switches(id) ON DELETE SET NULL,
    port_ifindex INTEGER NOT NULL DEFAULT 0,
    vlan_id INTEGER NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT '',
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_device_observations_device_observed
    ON device_observations (device_id, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_device_observations_mac_observed
    ON device_observations (mac_address, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_device_observations_switch_port
    ON device_observations (switch_id, port_ifindex, observed_at DESC);

ALTER TABLE enforcement_state
    ADD COLUMN IF NOT EXISTS desired_state TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS applied_state TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS applied_vlan INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_method TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_success_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_enforcement_state_desired_applied
    ON enforcement_state (desired_state, applied_state, updated_at DESC);
