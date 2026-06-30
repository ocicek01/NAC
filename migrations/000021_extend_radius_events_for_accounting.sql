ALTER TABLE radius_events
    ADD COLUMN IF NOT EXISTS event_type TEXT NOT NULL DEFAULT 'authorize',
    ADD COLUMN IF NOT EXISTS username TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS nas_port_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS acct_status_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS acct_session_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS framed_ip_address INET,
    ADD COLUMN IF NOT EXISTS session_time TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS terminate_cause TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_radius_events_event_type_created_at
    ON radius_events (event_type, created_at DESC);
