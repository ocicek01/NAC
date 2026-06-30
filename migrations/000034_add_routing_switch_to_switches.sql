ALTER TABLE switches
    ADD COLUMN IF NOT EXISTS routing_switch_id UUID NULL REFERENCES switches(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_switches_routing_switch_id
    ON switches (routing_switch_id);
