CREATE TABLE IF NOT EXISTS policies (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT 'classification',
    action TEXT NOT NULL DEFAULT '',
    match_field TEXT NOT NULL DEFAULT '',
    match_operator TEXT NOT NULL DEFAULT '',
    match_value TEXT NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_policies_active_priority
    ON policies (status, priority DESC);

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS policy_action TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS policy_reason TEXT NOT NULL DEFAULT '';
