ALTER TABLE devices
    ALTER COLUMN current_management_ip DROP NOT NULL,
    ALTER COLUMN current_management_ip DROP DEFAULT;

UPDATE devices
SET current_management_ip = NULL
WHERE COALESCE(current_management_ip::text, '') = '';