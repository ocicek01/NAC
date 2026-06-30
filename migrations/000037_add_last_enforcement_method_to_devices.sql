ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS last_enforcement_method TEXT NOT NULL DEFAULT '';
