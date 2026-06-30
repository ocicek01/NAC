package portendpoint

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/portendpoint"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ListBySwitch(ctx context.Context, switchID string) ([]domain.Endpoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			switch_id,
			port_ifindex,
			mac_address,
			COALESCE(host(ip_address), ''),
			hostname,
			source_confidence,
			last_seen_at,
			created_at,
			updated_at
		FROM port_endpoints
		WHERE switch_id = $1
		ORDER BY port_ifindex ASC, mac_address ASC, ip_address ASC
	`, strings.TrimSpace(switchID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Endpoint, 0)
	for rows.Next() {
		var item domain.Endpoint
		if err := rows.Scan(
			&item.ID,
			&item.SwitchID,
			&item.PortIfIndex,
			&item.MACAddress,
			&item.IPAddress,
			&item.Hostname,
			&item.SourceConfidence,
			&item.LastSeenAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}
