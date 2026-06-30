package policy

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/policy"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, policy domain.Policy) (domain.Policy, error) {
	query := `
		INSERT INTO policies (
			id,
			name,
			description,
			type,
			action,
			match_field,
			match_operator,
			match_value,
			priority,
			status,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		policy.ID,
		policy.Name,
		policy.Description,
		policy.Type,
		policy.Action,
		policy.MatchField,
		policy.MatchOperator,
		policy.MatchValue,
		policy.Priority,
		policy.Status,
		policy.CreatedAt,
		policy.UpdatedAt,
	)
	if err != nil {
		return domain.Policy{}, err
	}

	return policy, nil
}

func (r *PostgresRepository) ListActive(ctx context.Context) ([]domain.Policy, error) {
	query := `
		SELECT
			id,
			name,
			description,
			type,
			action,
			match_field,
			match_operator,
			match_value,
			priority,
			status,
			created_at,
			updated_at
		FROM policies
		WHERE status = 'active'
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Policy
	for rows.Next() {
		var item domain.Policy
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Description,
			&item.Type,
			&item.Action,
			&item.MatchField,
			&item.MatchOperator,
			&item.MatchValue,
			&item.Priority,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *PostgresRepository) Disable(ctx context.Context, id string) error {
	query := `
		UPDATE policies
		SET status = 'disabled',
			updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id)
	return err
}
