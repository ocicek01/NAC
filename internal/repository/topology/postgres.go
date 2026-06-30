package topology

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/topology"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Upsert(ctx context.Context, link domain.Link) (domain.Link, error) {
	query := `
		INSERT INTO topology_links (
			id,
			source_switch_id,
			source_switch_name,
			source_port_name,
			target_switch_id,
			target_switch_name,
			target_port_name,
			discovery_method,
			status,
			last_observed_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (source_switch_id, source_port_name, target_switch_name, target_port_name)
		DO UPDATE SET
			target_switch_id = NULLIF(EXCLUDED.target_switch_id::text, '')::uuid,
			discovery_method = EXCLUDED.discovery_method,
			status = EXCLUDED.status,
			last_observed_at = EXCLUDED.last_observed_at,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		link.ID,
		link.SourceSwitchID,
		link.SourceSwitchName,
		link.SourcePortName,
		link.TargetSwitchID,
		link.TargetSwitchName,
		link.TargetPortName,
		link.DiscoveryMethod,
		link.Status,
		link.LastObservedAt,
		link.CreatedAt,
		link.UpdatedAt,
	)
	if err != nil {
		return domain.Link{}, err
	}

	return link, nil
}

func (r *PostgresRepository) PruneDiscovered(ctx context.Context, sourceSwitchID string, methods []string, observedAt time.Time) error {
	sourceSwitchID = strings.TrimSpace(sourceSwitchID)
	if sourceSwitchID == "" {
		return nil
	}

	filteredMethods := make([]string, 0, len(methods))
	for _, method := range methods {
		method = strings.TrimSpace(method)
		if method != "" {
			filteredMethods = append(filteredMethods, method)
		}
	}
	if len(filteredMethods) == 0 {
		return nil
	}

	args := []any{sourceSwitchID, filteredMethods}
	query := `
		DELETE FROM topology_links
		WHERE source_switch_id = $1
		  AND discovery_method = ANY($2)
		  AND last_observed_at < $3
	`
	args = append(args, observedAt)

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

func (r *PostgresRepository) List(ctx context.Context) ([]domain.Link, error) {
	query := `
		SELECT
			id,
			source_switch_id,
			source_switch_name,
			source_port_name,
			COALESCE(target_switch_id::text, ''),
			target_switch_name,
			target_port_name,
			discovery_method,
			status,
			last_observed_at,
			created_at,
			updated_at
		FROM topology_links
		ORDER BY source_switch_name ASC, source_port_name ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Link
	for rows.Next() {
		var link domain.Link
		if err := rows.Scan(
			&link.ID,
			&link.SourceSwitchID,
			&link.SourceSwitchName,
			&link.SourcePortName,
			&link.TargetSwitchID,
			&link.TargetSwitchName,
			&link.TargetPortName,
			&link.DiscoveryMethod,
			&link.Status,
			&link.LastObservedAt,
			&link.CreatedAt,
			&link.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, link)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *PostgresRepository) HasLinkedInterface(ctx context.Context, switchID, interfaceName string) (bool, error) {
	interfaceName = normalizeTopologyInterfaceName(interfaceName)
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM topology_links
			WHERE (
				source_switch_id = $1
				AND LOWER(source_port_name) = LOWER($2)
			) OR (
				target_switch_id = $1
				AND LOWER(target_port_name) = LOWER($2)
			)
		)
	`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, switchID, interfaceName).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (r *PostgresRepository) FindLinkedSwitchID(ctx context.Context, switchID, interfaceName string) (string, error) {
	interfaceName = normalizeTopologyInterfaceName(interfaceName)
	query := `
		SELECT
			CASE
				WHEN COUNT(DISTINCT linked_switch_id) = 1 THEN MIN(linked_switch_id)
				ELSE ''
			END
		FROM (
			SELECT target_switch_id::text AS linked_switch_id
			FROM topology_links
			WHERE source_switch_id = $1
			  AND LOWER(source_port_name) = LOWER($2)
			  AND COALESCE(target_switch_id::text, '') <> ''

			UNION ALL

			SELECT source_switch_id AS linked_switch_id
			FROM topology_links
			WHERE COALESCE(target_switch_id::text, '') = $1
			  AND LOWER(target_port_name) = LOWER($2)
			  AND source_switch_id <> ''
		) links
	`

	var linkedSwitchID string
	if err := r.pool.QueryRow(ctx, query, switchID, interfaceName).Scan(&linkedSwitchID); err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	return linkedSwitchID, nil
}

func (r *PostgresRepository) CountLinkedSwitches(ctx context.Context, switchID, interfaceName string) (int, error) {
	interfaceName = normalizeTopologyInterfaceName(interfaceName)
	query := `
		SELECT COUNT(DISTINCT linked_switch_id)
		FROM (
			SELECT target_switch_id::text AS linked_switch_id
			FROM topology_links
			WHERE source_switch_id = $1
			  AND LOWER(source_port_name) = LOWER($2)
			  AND COALESCE(target_switch_id::text, '') <> ''

			UNION

			SELECT source_switch_id AS linked_switch_id
			FROM topology_links
			WHERE COALESCE(target_switch_id::text, '') = $1
			  AND LOWER(target_port_name) = LOWER($2)
			  AND source_switch_id <> ''
		) links
	`

	var count int
	if err := r.pool.QueryRow(ctx, query, switchID, interfaceName).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func normalizeTopologyInterfaceName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(value, "port ") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "port "))
	}
	return value
}
