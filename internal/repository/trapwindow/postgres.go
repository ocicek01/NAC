package trapwindow

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/trapwindow"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) UpsertPending(ctx context.Context, window domain.Window) (domain.Window, error) {
	summary, err := json.Marshal(normalizeSummary(window.Summary))
	if err != nil {
		return domain.Window{}, err
	}

	query := `
		INSERT INTO trap_windows (
			id, dedupe_key, switch_id, scope, category, status, port_ifindex, mac_address, vlan_id,
			event_count, first_seen_at, last_seen_at, available_at, dispatched_at, trap_oid,
			enterprise_oid, source_ip, summary, created_at, updated_at
		)
		VALUES (
			$1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, NULLIF($14, '0001-01-01T00:00:00Z')::timestamptz, $15,
			$16, NULLIF($17, '')::inet, $18::jsonb, $19, $20
		)
		ON CONFLICT (dedupe_key)
		DO UPDATE SET
			status = 'pending',
			event_count = trap_windows.event_count + 1,
			last_seen_at = EXCLUDED.last_seen_at,
			available_at = EXCLUDED.available_at,
			trap_oid = EXCLUDED.trap_oid,
			enterprise_oid = EXCLUDED.enterprise_oid,
			source_ip = EXCLUDED.source_ip,
			summary = EXCLUDED.summary,
			updated_at = EXCLUDED.updated_at
	`

	_, err = r.pool.Exec(
		ctx,
		query,
		window.ID,
		window.DedupeKey,
		window.SwitchID,
		window.Scope,
		window.Category,
		window.Status,
		window.PortIfIndex,
		window.MACAddress,
		window.VLANID,
		window.EventCount,
		window.FirstSeenAt,
		window.LastSeenAt,
		window.AvailableAt,
		window.DispatchedAt.UTC().Format(timeLayout),
		window.TrapOID,
		window.EnterpriseOID,
		window.SourceIP,
		string(summary),
		window.CreatedAt,
		window.UpdatedAt,
	)
	if err != nil {
		return domain.Window{}, err
	}
	return window, nil
}

func (r *PostgresRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]domain.Window, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, dedupe_key, COALESCE(switch_id::text, ''), scope, category, status, port_ifindex, mac_address, vlan_id,
			event_count, first_seen_at, last_seen_at, available_at,
			COALESCE(dispatched_at, '0001-01-01T00:00:00Z'::timestamptz),
			trap_oid, enterprise_oid, COALESCE(host(source_ip), ''), summary, created_at, updated_at
		FROM trap_windows
		WHERE status = 'pending' AND available_at <= $1
		ORDER BY available_at ASC, created_at ASC
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Window, 0)
	for rows.Next() {
		var item domain.Window
		var summary []byte
		if err := rows.Scan(
			&item.ID, &item.DedupeKey, &item.SwitchID, &item.Scope, &item.Category, &item.Status, &item.PortIfIndex, &item.MACAddress, &item.VLANID,
			&item.EventCount, &item.FirstSeenAt, &item.LastSeenAt, &item.AvailableAt, &item.DispatchedAt,
			&item.TrapOID, &item.EnterpriseOID, &item.SourceIP, &summary, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(summary, &item.Summary)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) MarkDispatched(ctx context.Context, id string, dispatchedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE trap_windows
		SET status = 'dispatched',
		    dispatched_at = $2,
		    updated_at = $2
		WHERE id = $1
	`, strings.TrimSpace(id), dispatchedAt)
	return err
}

func (r *PostgresRepository) PruneDispatchedOlderThan(ctx context.Context, cutoff time.Time) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM trap_windows WHERE status = 'dispatched' AND updated_at < $1`, cutoff)
	return err
}

func normalizeSummary(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
