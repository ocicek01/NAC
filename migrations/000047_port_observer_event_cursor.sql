-- Cursor pagination uses the existing immutable created_at/id tuple. This avoids
-- adding and backfilling a BIGSERIAL column on a potentially large event table.
-- The project migration runner wraps every file in a transaction. Avoid a
-- potentially blocking index build on the existing port_events table here.
-- Create the optional observer cursor index after deploy as documented in
-- docs/port-observer.md.

CREATE TABLE IF NOT EXISTS port_observer_outbox (
    id UUID PRIMARY KEY,
    payload JSONB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_port_observer_outbox_due
    ON port_observer_outbox (available_at, created_at)
    WHERE delivered_at IS NULL AND failed_at IS NULL;

CREATE TABLE IF NOT EXISTS port_event_inbox (
    event_id UUID PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('processing', 'completed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 1,
    last_error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
