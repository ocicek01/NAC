ALTER TABLE enforcement_state
    ADD COLUMN IF NOT EXISTS ip_learning_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ip_learning_started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS ip_learned_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_bounce_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_enforcement_state_ip_learning
    ON enforcement_state (ip_learning_status, updated_at DESC);
