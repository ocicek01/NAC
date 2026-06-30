ALTER TABLE switches
    ADD COLUMN IF NOT EXISTS base_mac TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_switches_base_mac
    ON switches (base_mac);
