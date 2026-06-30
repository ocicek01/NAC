BEGIN;

CREATE TEMP TABLE tmp_device_mac_rank AS
SELECT
    id,
    upper(regexp_replace(mac_address, '[^0-9A-Fa-f]', '', 'g')) AS raw_mac,
    row_number() OVER (
        PARTITION BY upper(regexp_replace(mac_address, '[^0-9A-Fa-f]', '', 'g'))
        ORDER BY updated_at DESC, created_at DESC, id DESC
    ) AS row_num
FROM devices
WHERE mac_address IS NOT NULL
  AND regexp_replace(mac_address, '[^0-9A-Fa-f]', '', 'g') <> '';

CREATE TEMP TABLE tmp_device_mac_keepers AS
SELECT id, raw_mac
FROM tmp_device_mac_rank
WHERE row_num = 1
  AND char_length(raw_mac) = 12;

UPDATE device_identity_snapshots dis
SET device_id = keeper.id
FROM tmp_device_mac_rank ranked
JOIN tmp_device_mac_keepers keeper
  ON keeper.raw_mac = ranked.raw_mac
WHERE dis.device_id = ranked.id
  AND ranked.row_num > 1;

DELETE FROM devices d
USING tmp_device_mac_rank ranked
WHERE d.id = ranked.id
  AND ranked.row_num > 1;

UPDATE devices d
SET mac_address = substr(keeper.raw_mac, 1, 2) || ':' ||
                  substr(keeper.raw_mac, 3, 2) || ':' ||
                  substr(keeper.raw_mac, 5, 2) || ':' ||
                  substr(keeper.raw_mac, 7, 2) || ':' ||
                  substr(keeper.raw_mac, 9, 2) || ':' ||
                  substr(keeper.raw_mac, 11, 2)
FROM tmp_device_mac_keepers keeper
WHERE d.id = keeper.id
  AND d.mac_address IS DISTINCT FROM (
      substr(keeper.raw_mac, 1, 2) || ':' ||
      substr(keeper.raw_mac, 3, 2) || ':' ||
      substr(keeper.raw_mac, 5, 2) || ':' ||
      substr(keeper.raw_mac, 7, 2) || ':' ||
      substr(keeper.raw_mac, 9, 2) || ':' ||
      substr(keeper.raw_mac, 11, 2)
  );

COMMIT;
