CREATE TABLE IF NOT EXISTS guest_identities (
    id UUID PRIMARY KEY,
    external_id TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    full_name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    target_vlan INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_guest_identities_status
    ON guest_identities (status);

CREATE INDEX IF NOT EXISTS idx_guest_identities_username
    ON guest_identities (LOWER(username));

CREATE INDEX IF NOT EXISTS idx_guest_identities_external_id
    ON guest_identities (LOWER(external_id));
