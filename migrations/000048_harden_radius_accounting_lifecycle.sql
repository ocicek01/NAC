ALTER TABLE radius_events
    ADD COLUMN IF NOT EXISTS received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS processing_status TEXT NOT NULL DEFAULT 'processed',
    ADD COLUMN IF NOT EXISTS processing_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS processed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS processing_error TEXT NOT NULL DEFAULT '';

ALTER TABLE radius_events
    DROP CONSTRAINT IF EXISTS radius_events_processing_status_check;

ALTER TABLE radius_events
    ADD CONSTRAINT radius_events_processing_status_check
    CHECK (processing_status IN ('processing', 'processed', 'failed'));

CREATE INDEX IF NOT EXISTS idx_radius_events_accounting_session_time
    ON radius_events (acct_session_id, created_at DESC)
    WHERE event_type = 'accounting' AND acct_session_id <> '';

CREATE INDEX IF NOT EXISTS idx_radius_events_processing_status
    ON radius_events (processing_status, processing_started_at)
    WHERE event_type = 'accounting' AND processing_status <> 'processed';

COMMENT ON COLUMN radius_events.created_at IS
    'RADIUS event occurrence time; received_at records NAC ingestion time.';
