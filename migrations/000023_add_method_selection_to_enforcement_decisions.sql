ALTER TABLE enforcement_decisions
    ADD COLUMN IF NOT EXISTS selected_method TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS fallback_methods TEXT NOT NULL DEFAULT '';

