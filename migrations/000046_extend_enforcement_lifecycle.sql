ALTER TABLE enforcement_results
    ADD COLUMN IF NOT EXISTS attempt_number INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS adapter TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS transport TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS action TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS execution_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS command_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ NULL;

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS last_enforcement_request_id UUID NULL,
    ADD COLUMN IF NOT EXISTS verified_vlan INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS quarantine_status TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_enforcement_results_request_attempt
    ON enforcement_results (enforcement_request_id, attempt_number DESC, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_devices_last_enforcement_request
    ON devices (last_enforcement_request_id);
