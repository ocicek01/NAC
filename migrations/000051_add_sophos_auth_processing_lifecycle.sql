ALTER TABLE sophos_auth_events
    ADD COLUMN IF NOT EXISTS processing_status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS lease_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS processed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE sophos_auth_events
SET processing_status = CASE
        WHEN event_type = 'ignored' THEN 'ignored'
        WHEN correlation_status = 'pending' THEN 'retry'
        ELSE 'completed'
    END,
    next_attempt_at = CASE WHEN event_type <> 'ignored' AND correlation_status = 'pending' THEN NOW() ELSE NULL END,
    processed_at = CASE WHEN event_type <> 'ignored' AND correlation_status <> 'pending' THEN received_at ELSE NULL END,
    updated_at = NOW()
WHERE processing_status = 'pending';

ALTER TABLE sophos_auth_events
    ADD CONSTRAINT sophos_auth_events_processing_status_check
    CHECK (processing_status IN ('pending', 'processing', 'retry', 'completed', 'dead_letter', 'ignored'));

CREATE INDEX IF NOT EXISTS idx_sophos_auth_events_processing_due
    ON sophos_auth_events (processing_status, next_attempt_at, received_at)
    WHERE processing_status IN ('pending', 'retry', 'processing');
