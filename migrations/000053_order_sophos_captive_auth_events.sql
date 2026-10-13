CREATE TABLE IF NOT EXISTS sophos_auth_identity_watermarks (
    username_normalized TEXT NOT NULL,
    source_ip INET NOT NULL,
    last_event_at TIMESTAMPTZ NOT NULL,
    last_event_rank SMALLINT NOT NULL CHECK (last_event_rank IN (1, 2)),
    last_received_at TIMESTAMPTZ NOT NULL,
    last_event_id UUID NOT NULL,
    last_event_type TEXT NOT NULL CHECK (last_event_type IN ('login', 'logout')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (username_normalized, source_ip)
);

INSERT INTO sophos_auth_identity_watermarks (
    username_normalized, source_ip, last_event_at, last_event_rank,
    last_received_at, last_event_id, last_event_type, updated_at
)
SELECT DISTINCT ON (lower(username), source_ip)
    lower(username), source_ip, event_at,
    CASE WHEN event_type='logout' THEN 2 ELSE 1 END,
    received_at, id, event_type, NOW()
FROM sophos_auth_events
WHERE processing_status='completed'
  AND event_type IN ('login', 'logout')
  AND username<>''
  AND source_ip IS NOT NULL
ORDER BY lower(username), source_ip, event_at DESC,
    CASE WHEN event_type='logout' THEN 2 ELSE 1 END DESC,
    received_at DESC, id DESC
ON CONFLICT (username_normalized, source_ip) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_sophos_auth_events_causal_pending
    ON sophos_auth_events (
        lower(username), source_ip, event_at,
        (CASE WHEN event_type='logout' THEN 2 ELSE 1 END),
        received_at, id
    )
    WHERE processing_status IN ('pending', 'retry', 'processing');
