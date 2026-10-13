CREATE INDEX IF NOT EXISTS idx_sophos_radius_sso_session_order
    ON sophos_radius_sso_outbox (
        acct_session_id,
        event_timestamp,
        (CASE acct_status_type WHEN 'Start' THEN 1 WHEN 'Interim-Update' THEN 2 ELSE 3 END),
        created_at,
        id
    )
    WHERE status NOT IN ('sent', 'dead_letter');
