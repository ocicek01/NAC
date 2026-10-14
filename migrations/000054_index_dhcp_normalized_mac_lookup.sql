CREATE INDEX IF NOT EXISTS idx_dhcp_events_normalized_mac_observed_ip
ON dhcp_events (
    (regexp_replace(upper(mac_address), '[^0-9A-F]', '', 'g')),
    observed_at DESC,
    created_at DESC
)
WHERE your_ip IS NOT NULL OR requested_ip IS NOT NULL OR client_ip IS NOT NULL;
