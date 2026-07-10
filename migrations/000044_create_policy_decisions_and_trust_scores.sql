ALTER TABLE policies
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS match_conditions JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS decision_type TEXT NOT NULL DEFAULT 'monitor_only',
    ADD COLUMN IF NOT EXISTS target_vlan INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS enforcement_action TEXT NOT NULL DEFAULT 'monitor',
    ADD COLUMN IF NOT EXISTS dry_run BOOLEAN NOT NULL DEFAULT true;

UPDATE policies
SET enabled = CASE WHEN status = 'disabled' THEN false ELSE true END,
    match_conditions = CASE
        WHEN jsonb_typeof(match_conditions) = 'array' AND jsonb_array_length(match_conditions) > 0 THEN match_conditions
        WHEN COALESCE(match_field, '') <> '' AND COALESCE(match_operator, '') <> '' THEN jsonb_build_array(jsonb_build_object('field', match_field, 'operator', match_operator, 'value', match_value))
        ELSE '[]'::jsonb
    END,
    decision_type = CASE LOWER(COALESCE(action, ''))
        WHEN 'blocked' THEN 'quarantine'
        WHEN 'guest' THEN 'registration'
        WHEN 'active' THEN 'allow'
        WHEN 'observed' THEN 'monitor_only'
        WHEN 'unknown' THEN 'restricted'
        ELSE decision_type
    END,
    enforcement_action = CASE LOWER(COALESCE(action, ''))
        WHEN 'blocked' THEN 'quarantine'
        WHEN 'guest' THEN 'registration'
        WHEN 'active' THEN 'allow'
        WHEN 'observed' THEN 'monitor'
        WHEN 'unknown' THEN 'restrict'
        ELSE enforcement_action
    END,
    dry_run = true;

CREATE TABLE IF NOT EXISTS policy_decisions (
    id UUID PRIMARY KEY,
    device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
    port_event_id UUID REFERENCES port_events(id) ON DELETE SET NULL,
    policy_id UUID REFERENCES policies(id) ON DELETE SET NULL,
    policy_name TEXT NOT NULL DEFAULT '',
    decision_type TEXT NOT NULL DEFAULT 'monitor_only',
    target_vlan INTEGER NOT NULL DEFAULT 0,
    enforcement_action TEXT NOT NULL DEFAULT 'monitor',
    trust_score INTEGER NOT NULL DEFAULT 0,
    trust_signals JSONB NOT NULL DEFAULT '[]'::jsonb,
    reason_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
    explanation TEXT NOT NULL DEFAULT '',
    dry_run BOOLEAN NOT NULL DEFAULT true,
    enforcement_status TEXT NOT NULL DEFAULT 'dry-run',
    evaluation_duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_policy_decisions_device_created ON policy_decisions (device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_policy_decisions_created ON policy_decisions (created_at DESC);

CREATE TABLE IF NOT EXISTS trust_score_results (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    score INTEGER NOT NULL DEFAULT 0,
    signals JSONB NOT NULL DEFAULT '[]'::jsonb,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    calculation_version TEXT NOT NULL DEFAULT 'v1'
);

CREATE INDEX IF NOT EXISTS idx_trust_score_results_device_calculated ON trust_score_results (device_id, calculated_at DESC);
