CREATE TABLE IF NOT EXISTS discovery_jobs (
    id UUID PRIMARY KEY,
    switch_id UUID NULL REFERENCES switches(id) ON DELETE CASCADE,
    scope TEXT NOT NULL DEFAULT 'full',
    status TEXT NOT NULL DEFAULT 'queued',
    requested_source TEXT NOT NULL DEFAULT '',
    requested_by TEXT NOT NULL DEFAULT '',
    worker_id TEXT NOT NULL DEFAULT '',
    current_step TEXT NOT NULL DEFAULT '',
    progress_percent INTEGER NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    error_message TEXT NOT NULL DEFAULT '',
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    locked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discovery_jobs_status_created_at
    ON discovery_jobs (status, created_at);

CREATE INDEX IF NOT EXISTS idx_discovery_jobs_switch_created_at
    ON discovery_jobs (switch_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_discovery_jobs_locked_at
    ON discovery_jobs (locked_at);
