CREATE TABLE IF NOT EXISTS radius_events (
    id UUID PRIMARY KEY,
    mac_address VARCHAR(17) NOT NULL,
    hostname TEXT NOT NULL DEFAULT '',
    vendor_class TEXT NOT NULL DEFAULT '',
    nas_ip_address INET,
    nas_identifier TEXT NOT NULL DEFAULT '',
    nas_port TEXT NOT NULL DEFAULT '',
    nas_port_id TEXT NOT NULL DEFAULT '',
    called_station_id TEXT NOT NULL DEFAULT '',
    calling_station_id TEXT NOT NULL DEFAULT '',
    decision TEXT NOT NULL DEFAULT '',
    policy_action TEXT NOT NULL DEFAULT '',
    policy_reason TEXT NOT NULL DEFAULT '',
    reply_message TEXT NOT NULL DEFAULT '',
    vlan_id TEXT NOT NULL DEFAULT '',
    reply_attributes TEXT NOT NULL DEFAULT '',
    control_attributes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_radius_events_created_at
    ON radius_events (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_radius_events_mac_created_at
    ON radius_events (mac_address, created_at DESC);
