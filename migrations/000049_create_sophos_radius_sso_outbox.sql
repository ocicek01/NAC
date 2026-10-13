CREATE TABLE IF NOT EXISTS sophos_radius_sso_outbox (
    id UUID PRIMARY KEY,
    source_key TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    ip_address INET NOT NULL,
    mac_address TEXT NOT NULL DEFAULT '',
    acct_status_type TEXT NOT NULL,
    acct_session_id TEXT NOT NULL,
    event_timestamp TIMESTAMPTZ NOT NULL,
    acct_delay_time INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_until TIMESTAMPTZ NULL,
    last_error TEXT NOT NULL DEFAULT '',
    sent_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sophos_radius_sso_status_check CHECK (status IN ('pending', 'processing', 'retry', 'sent', 'dead_letter')),
    CONSTRAINT sophos_radius_sso_acct_status_check CHECK (acct_status_type IN ('Start', 'Interim-Update', 'Stop'))
);

CREATE INDEX IF NOT EXISTS idx_sophos_radius_sso_due
    ON sophos_radius_sso_outbox (next_attempt_at, created_at)
    WHERE status IN ('pending', 'retry', 'processing');

CREATE INDEX IF NOT EXISTS idx_sophos_radius_sso_status_created
    ON sophos_radius_sso_outbox (status, created_at DESC);
