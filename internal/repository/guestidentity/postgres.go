package guestidentity

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/guestidentity"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]domain.Identity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, external_id, username, full_name, email, phone, status, target_vlan,
		       COALESCE(expires_at, '0001-01-01T00:00:00Z'::timestamptz), created_at, updated_at
		FROM guest_identities
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Identity
	for rows.Next() {
		var item domain.Identity
		if err := rows.Scan(
			&item.ID,
			&item.ExternalID,
			&item.Username,
			&item.FullName,
			&item.Email,
			&item.Phone,
			&item.Status,
			&item.TargetVLAN,
			&item.ExpiresAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Insert(ctx context.Context, identity domain.Identity) (domain.Identity, error) {
	var expiresAt any
	if !identity.ExpiresAt.IsZero() {
		expiresAt = identity.ExpiresAt
	}

	var out domain.Identity
	err := r.pool.QueryRow(ctx, `
		INSERT INTO guest_identities (
			id, external_id, username, full_name, email, phone, status, target_vlan, expires_at, created_at, updated_at
		)
		VALUES (
			NULLIF($1, '')::uuid, $2, $3, $4, $5, $6, $7, $8, $9::timestamptz, $10, $11
		)
		RETURNING id::text, external_id, username, full_name, email, phone, status, target_vlan,
		          COALESCE(expires_at, '0001-01-01T00:00:00Z'::timestamptz), created_at, updated_at
	`,
		identity.ID,
		identity.ExternalID,
		identity.Username,
		identity.FullName,
		identity.Email,
		identity.Phone,
		identity.Status,
		identity.TargetVLAN,
		expiresAt,
		identity.CreatedAt,
		identity.UpdatedAt,
	).Scan(
		&out.ID,
		&out.ExternalID,
		&out.Username,
		&out.FullName,
		&out.Email,
		&out.Phone,
		&out.Status,
		&out.TargetVLAN,
		&out.ExpiresAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return domain.Identity{}, err
	}
	return out, nil
}

func (r *PostgresRepository) FindActiveByIdentifier(ctx context.Context, identifier string) (*domain.Identity, error) {
	identifier = strings.TrimSpace(identifier)
	var item domain.Identity
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, external_id, username, full_name, email, phone, status, target_vlan,
		       COALESCE(expires_at, '0001-01-01T00:00:00Z'::timestamptz), created_at, updated_at
		FROM guest_identities
		WHERE status = 'active'
		  AND (
			  LOWER(username) = LOWER($1)
			  OR LOWER(external_id) = LOWER($1)
		  )
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY updated_at DESC
		LIMIT 1
	`, identifier).Scan(
		&item.ID,
		&item.ExternalID,
		&item.Username,
		&item.FullName,
		&item.Email,
		&item.Phone,
		&item.Status,
		&item.TargetVLAN,
		&item.ExpiresAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
