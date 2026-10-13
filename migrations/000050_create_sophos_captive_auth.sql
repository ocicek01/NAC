CREATE TABLE IF NOT EXISTS sophos_auth_events (
    id UUID PRIMARY KEY,
    fingerprint TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL CHECK (event_type IN ('login', 'logout', 'ignored')),
    message_id TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    user_group TEXT NOT NULL DEFAULT '',
    client_used TEXT NOT NULL DEFAULT '',
    auth_mechanism TEXT NOT NULL DEFAULT '',
    auth_status TEXT NOT NULL DEFAULT '',
    source_ip INET NULL,
    source_mac TEXT NOT NULL DEFAULT '',
    correlated_mac TEXT NOT NULL DEFAULT '',
    correlation_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (correlation_status IN ('pending', 'correlated', 'uncorrelated', 'ambiguous', 'ignored')),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    event_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sophos_auth_events_received
    ON sophos_auth_events (received_at DESC);
CREATE INDEX IF NOT EXISTS idx_sophos_auth_events_user_ip
    ON sophos_auth_events (username, source_ip, event_at DESC);
CREATE INDEX IF NOT EXISTS idx_sophos_auth_events_correlation
    ON sophos_auth_events (correlation_status, received_at DESC);

CREATE TABLE IF NOT EXISTS sophos_user_sessions (
    id UUID PRIMARY KEY,
    active_key TEXT UNIQUE,
    username TEXT NOT NULL,
    user_group TEXT NOT NULL DEFAULT '',
    ip_address INET NOT NULL,
    mac_address TEXT NOT NULL DEFAULT '',
    client_used TEXT NOT NULL DEFAULT '',
    auth_mechanism TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('active', 'closed')),
    correlation_status TEXT NOT NULL
        CHECK (correlation_status IN ('correlated', 'uncorrelated', 'ambiguous')),
    started_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sophos_user_sessions_recent
    ON sophos_user_sessions (last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_sophos_user_sessions_user_ip
    ON sophos_user_sessions (username, ip_address, status);
