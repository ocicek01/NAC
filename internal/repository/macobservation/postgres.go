package macobservation

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/macobservation"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	query := `
		INSERT INTO mac_observations (
			id,
			dhcp_event_id,
			mac_address,
			source_type,
			confidence,
			switch_id,
			switch_name,
			management_ip,
			bridge_port,
			if_index,
			interface_name,
			interface_description,
			observed_at,
			created_at
		)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, NULLIF($8, '')::inet, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		observation.ID,
		observation.DHCPEventID,
		observation.MACAddress,
		observation.SourceType,
		observation.Confidence,
		observation.SwitchID,
		observation.SwitchName,
		observation.ManagementIP,
		observation.BridgePort,
		observation.IfIndex,
		observation.InterfaceName,
		observation.InterfaceDescription,
		observation.ObservedAt,
		observation.CreatedAt,
	)
	if err != nil {
		return domain.Observation{}, err
	}

	return observation, nil
}

func (r *PostgresRepository) InsertCandidates(ctx context.Context, candidates []domain.Candidate) error {
	if len(candidates) == 0 {
		return nil
	}

	query := `
		INSERT INTO mac_observation_candidates (
			id,
			observation_id,
			dhcp_event_id,
			mac_address,
			source_type,
			confidence,
			switch_id,
			switch_name,
			management_ip,
			bridge_port,
			if_index,
			interface_name,
			interface_description,
			score,
			is_selected,
			observed_at,
			created_at
		)
		VALUES (
			$1,
			NULLIF($2, '')::uuid,
			NULLIF($3, '')::uuid,
			$4,
			$5,
			$6,
			$7,
			$8,
			NULLIF($9, '')::inet,
			$10,
			$11,
			$12,
			$13,
			$14,
			$15,
			$16,
			$17
		)
	`

	batch := &pgx.Batch{}
	for _, candidate := range candidates {
		batch.Queue(
			query,
			candidate.ID,
			candidate.ObservationID,
			candidate.DHCPEventID,
			candidate.MACAddress,
			candidate.SourceType,
			candidate.Confidence,
			candidate.SwitchID,
			candidate.SwitchName,
			candidate.ManagementIP,
			candidate.BridgePort,
			candidate.IfIndex,
			candidate.InterfaceName,
			candidate.InterfaceDescription,
			candidate.Score,
			candidate.IsSelected,
			candidate.ObservedAt,
			candidate.CreatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range candidates {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresRepository) ListRecent(ctx context.Context, limit int) ([]domain.Observation, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT
			mo.id,
			COALESCE(mo.dhcp_event_id::text, ''),
			mo.mac_address,
			COALESCE(mo.source_type, ''),
			COALESCE(mo.confidence, ''),
			mo.switch_id,
			mo.switch_name,
			COALESCE(host(mo.management_ip), ''),
			mo.bridge_port,
			mo.if_index,
			mo.interface_name,
			mo.interface_description,
			COALESCE(stats.candidate_count, 1),
			COALESCE(stats.alternative_summary, ''),
			mo.observed_at,
			mo.created_at
		FROM mac_observations mo
		LEFT JOIN (
			SELECT
				observation_id,
				COUNT(*) AS candidate_count,
				STRING_AGG(
					CASE
						WHEN is_selected THEN NULL
						ELSE switch_name || ' / ' || interface_name
					END,
					' | '
					ORDER BY score DESC
				) AS alternative_summary
			FROM mac_observation_candidates
			GROUP BY observation_id
		) stats ON stats.observation_id = mo.id
		ORDER BY mo.created_at DESC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Observation
	for rows.Next() {
		var item domain.Observation
		if err := rows.Scan(
			&item.ID,
			&item.DHCPEventID,
			&item.MACAddress,
			&item.SourceType,
			&item.Confidence,
			&item.SwitchID,
			&item.SwitchName,
			&item.ManagementIP,
			&item.BridgePort,
			&item.IfIndex,
			&item.InterfaceName,
			&item.InterfaceDescription,
			&item.CandidateCount,
			&item.AlternativeSummary,
			&item.ObservedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
