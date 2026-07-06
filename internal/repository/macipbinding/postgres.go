package macipbinding

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/macipbinding"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) UpsertBatch(ctx context.Context, bindings []domain.Binding) error {
	if len(bindings) == 0 {
		return nil
	}

	query := `
		INSERT INTO mac_ip_bindings (
			id, switch_id, mac_address, ip_address, source, hostname, vendor_class,
			options55, vlan_id, first_seen_at, last_seen_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, NULLIF($4, '')::inet, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		ON CONFLICT (switch_id, mac_address, ip_address, source)
		DO UPDATE SET
			hostname = EXCLUDED.hostname,
			vendor_class = EXCLUDED.vendor_class,
			options55 = EXCLUDED.options55,
			vlan_id = EXCLUDED.vlan_id,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = EXCLUDED.updated_at
	`

	for _, binding := range bindings {
		if _, err := r.pool.Exec(
			ctx,
			query,
			binding.ID,
			strings.TrimSpace(binding.SwitchID),
			strings.TrimSpace(binding.MACAddress),
			strings.TrimSpace(binding.IPAddress),
			strings.TrimSpace(binding.Source),
			strings.TrimSpace(binding.Hostname),
			strings.TrimSpace(binding.VendorClass),
			strings.TrimSpace(binding.Options55),
			binding.VLANID,
			binding.FirstSeenAt,
			binding.LastSeenAt,
			binding.CreatedAt,
			binding.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresRepository) FindLatestByMACSwitch(ctx context.Context, macAddress, switchID string) (*domain.Binding, error) {
	query := `
		SELECT
			id,
			COALESCE(switch_id::text, ''),
			COALESCE(mac_address, ''),
			COALESCE(HOST(ip_address), ''),
			COALESCE(source, ''),
			COALESCE(hostname, ''),
			COALESCE(vendor_class, ''),
			COALESCE(options55, ''),
			COALESCE(vlan_id, 0),
			COALESCE(first_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			created_at,
			updated_at
		FROM mac_ip_bindings
		WHERE UPPER(mac_address) = UPPER($1)
		  AND switch_id = NULLIF($2, '')::uuid
		ORDER BY last_seen_at DESC NULLS LAST, updated_at DESC
		LIMIT 1
	`

	var item domain.Binding
	if err := r.pool.QueryRow(ctx, query, strings.TrimSpace(macAddress), strings.TrimSpace(switchID)).Scan(
		&item.ID,
		&item.SwitchID,
		&item.MACAddress,
		&item.IPAddress,
		&item.Source,
		&item.Hostname,
		&item.VendorClass,
		&item.Options55,
		&item.VLANID,
		&item.FirstSeenAt,
		&item.LastSeenAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}
