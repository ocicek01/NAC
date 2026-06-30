ALTER TABLE mac_observations
    ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS confidence TEXT NOT NULL DEFAULT '';

ALTER TABLE mac_observation_candidates
    ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS confidence TEXT NOT NULL DEFAULT '';

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS current_source_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS current_confidence TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_mac_observations_source_confidence_created_at
    ON mac_observations (source_type, confidence, created_at DESC);
