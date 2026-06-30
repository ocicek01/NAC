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
