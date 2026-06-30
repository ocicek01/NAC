package arpsnapshot

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/arpsnapshot"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) UpsertBatch(ctx context.Context, snapshots []domain.Snapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	query := `
		INSERT INTO arp_snapshots (
			id, switch_id, if_index, mac_address, ip_address, source, vlan_id,
			first_seen_at, last_seen_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, NULLIF($5, '')::inet, $6, $7, $8, $9, $10, $11
		)
		ON CONFLICT (switch_id, mac_address, ip_address)
		DO UPDATE SET
			if_index = EXCLUDED.if_index,
			source = EXCLUDED.source,
			vlan_id = EXCLUDED.vlan_id,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = EXCLUDED.updated_at
	`

	for _, snapshot := range snapshots {
		if _, err := r.pool.Exec(
			ctx,
			query,
			snapshot.ID,
			strings.TrimSpace(snapshot.SwitchID),
			snapshot.IfIndex,
			strings.TrimSpace(snapshot.MACAddress),
			strings.TrimSpace(snapshot.IPAddress),
			strings.TrimSpace(snapshot.Source),
			snapshot.VLANID,
			snapshot.FirstSeenAt,
			snapshot.LastSeenAt,
			snapshot.CreatedAt,
			snapshot.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return nil
}
