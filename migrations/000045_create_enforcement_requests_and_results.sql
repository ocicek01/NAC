ALTER TABLE switch_ports
    ADD COLUMN IF NOT EXISTS enforcement_protected BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE switch_ports
SET enforcement_protected = CASE
    WHEN is_uplink OR is_trunk THEN TRUE
    WHEN LOWER(COALESCE(port_mode, '')) = 'trunk' THEN TRUE
    WHEN LOWER(COALESCE(interface_type, '')) IN ('trunk', 'uplink') THEN TRUE
    WHEN COALESCE(neighbor_switch_id::text, '') <> '' THEN TRUE
    WHEN LOWER(COALESCE(neighbor_protocol, '')) IN ('cdp', 'lldp') AND LOWER(COALESCE(neighbor_description, '')) LIKE '%switch%' THEN TRUE
    ELSE enforcement_protected
END;

CREATE TABLE IF NOT EXISTS enforcement_requests (
    id UUID PRIMARY KEY,
    device_id UUID NULL REFERENCES devices(id) ON DELETE SET NULL,
    policy_decision_id UUID NULL REFERENCES policy_decisions(id) ON DELETE SET NULL,
    switch_id UUID NULL REFERENCES switches(id) ON DELETE SET NULL,
    port_id UUID NULL REFERENCES switch_ports(id) ON DELETE SET NULL,
    requested_action TEXT NOT NULL DEFAULT 'none',
    target_vlan INTEGER NOT NULL DEFAULT 0,
    previous_vlan INTEGER NOT NULL DEFAULT 0,
    requested_by TEXT NOT NULL DEFAULT 'system',
    request_source TEXT NOT NULL DEFAULT 'policy_engine',
    mode TEXT NOT NULL DEFAULT 'dry_run',
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    adapter TEXT NOT NULL DEFAULT '',
    command_summary TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    verified_at TIMESTAMPTZ NULL,
    rollback_of_request_id UUID NULL REFERENCES enforcement_requests(id) ON DELETE SET NULL,
    verification_status TEXT NOT NULL DEFAULT '',
    current_switch_id UUID NULL REFERENCES switches(id) ON DELETE SET NULL,
    current_if_index INTEGER NOT NULL DEFAULT 0,
    current_interface_name TEXT NOT NULL DEFAULT '',
    target_device_mac TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_enforcement_requests_requested_at
    ON enforcement_requests (requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_enforcement_requests_status_requested
    ON enforcement_requests (status, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_enforcement_requests_device
    ON enforcement_requests (device_id, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_enforcement_requests_policy_decision
    ON enforcement_requests (policy_decision_id, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_enforcement_requests_switch_port_status
    ON enforcement_requests (switch_id, port_id, status, requested_at DESC);

CREATE TABLE IF NOT EXISTS enforcement_results (
    id UUID PRIMARY KEY,
    enforcement_request_id UUID NOT NULL REFERENCES enforcement_requests(id) ON DELETE CASCADE,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    changed BOOLEAN NOT NULL DEFAULT FALSE,
    previous_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    expected_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    observed_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    verification_status TEXT NOT NULL DEFAULT '',
    adapter_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_enforcement_results_request_created
    ON enforcement_results (enforcement_request_id, created_at DESC);

ALTER TABLE policy_decisions
    ADD COLUMN IF NOT EXISTS enforcement_requested BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS enforcement_request_id UUID NULL REFERENCES enforcement_requests(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS enforcement_started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS enforcement_completed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS enforcement_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS enforced_at TIMESTAMPTZ NULL;
