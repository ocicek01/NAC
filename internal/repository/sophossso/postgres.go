package sophossso

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/sophossso"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Enqueue(ctx context.Context, item domain.Item) (domain.Item, bool, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO sophos_radius_sso_outbox (
			id, source_key, username, ip_address, mac_address, acct_status_type,
			acct_session_id, event_timestamp, acct_delay_time, status,
			next_attempt_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4::inet,$5,$6,$7,$8,$9,'pending',$10,$11,$11)
		ON CONFLICT (source_key) DO NOTHING
		RETURNING id, source_key, username, host(ip_address), mac_address,
			acct_status_type, acct_session_id, event_timestamp, acct_delay_time,
			status, attempt_count, next_attempt_at, lease_until, last_error,
			created_at, updated_at, sent_at
	`, item.ID, item.SourceKey, item.Username, item.IPAddress, item.MACAddress,
		item.AcctStatusType, item.AcctSessionID, item.EventTimestamp,
		item.AcctDelayTime, item.NextAttemptAt, item.CreatedAt)
	stored, err := scanItem(row)
	if err == pgx.ErrNoRows {
		existing, findErr := r.findBySourceKey(ctx, item.SourceKey)
		return existing, false, findErr
	}
	return stored, err == nil, err
}

func (r *PostgresRepository) ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]domain.Item, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		WITH due AS (
			SELECT id FROM sophos_radius_sso_outbox
			WHERE ((status IN ('pending','retry') AND next_attempt_at <= $1)
				OR (status = 'processing' AND lease_until < $1))
			ORDER BY next_attempt_at, created_at
			FOR UPDATE SKIP LOCKED LIMIT $2
		)
		UPDATE sophos_radius_sso_outbox o
		SET status='processing', lease_until=$3, attempt_count=o.attempt_count+1, updated_at=$1
		FROM due WHERE o.id=due.id
		RETURNING o.id, o.source_key, o.username, host(o.ip_address), o.mac_address,
			o.acct_status_type, o.acct_session_id, o.event_timestamp, o.acct_delay_time,
			o.status, o.attempt_count, o.next_attempt_at, o.lease_until, o.last_error,
			o.created_at, o.updated_at, o.sent_at
	`, now, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Item, 0, limit)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) MarkSent(ctx context.Context, id string, leaseUntil, sentAt time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE sophos_radius_sso_outbox SET status='sent', sent_at=$3, lease_until=NULL, last_error='', updated_at=$3 WHERE id=$1 AND status='processing' AND lease_until=$2`, id, leaseUntil, sentAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("Sophos SSO lease lost while marking %s sent", id)
	}
	return nil
}

func (r *PostgresRepository) MarkRetry(ctx context.Context, id string, leaseUntil time.Time, lastError string, nextAttemptAt time.Time, deadLetter bool) error {
	status := "retry"
	if deadLetter {
		status = "dead_letter"
	}
	tag, err := r.pool.Exec(ctx, `UPDATE sophos_radius_sso_outbox SET status=$3, next_attempt_at=$4, lease_until=NULL, last_error=$5, updated_at=NOW() WHERE id=$1 AND status='processing' AND lease_until=$2`, id, leaseUntil, status, nextAttemptAt, lastError)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("Sophos SSO lease lost while rescheduling %s", id)
	}
	return nil
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]domain.Item, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, source_key, username, host(ip_address), mac_address,
			acct_status_type, acct_session_id, event_timestamp, acct_delay_time,
			status, attempt_count, next_attempt_at, lease_until, last_error,
			created_at, updated_at, sent_at
		FROM sophos_radius_sso_outbox ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Item, 0, limit)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Health(ctx context.Context, enabled bool) (domain.Health, error) {
	var h domain.Health
	h.Enabled = enabled
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status='pending'),
		       count(*) FILTER (WHERE status='retry'),
		       count(*) FILTER (WHERE status='processing'),
		       count(*) FILTER (WHERE status='dead_letter'),
		       max(sent_at) FILTER (WHERE status='sent')
		FROM sophos_radius_sso_outbox
	`).Scan(&h.QueueDepth, &h.RetryCount, &h.ProcessingCount, &h.DeadLetterCount, &h.LastSentAt)
	return h, err
}

type scanner interface{ Scan(...any) error }

func scanItem(row scanner) (domain.Item, error) {
	var item domain.Item
	err := row.Scan(&item.ID, &item.SourceKey, &item.Username, &item.IPAddress,
		&item.MACAddress, &item.AcctStatusType, &item.AcctSessionID,
		&item.EventTimestamp, &item.AcctDelayTime, &item.Status,
		&item.AttemptCount, &item.NextAttemptAt, &item.LeaseUntil,
		&item.LastError, &item.CreatedAt, &item.UpdatedAt, &item.SentAt)
	return item, err
}

func (r *PostgresRepository) findBySourceKey(ctx context.Context, key string) (domain.Item, error) {
	return scanItem(r.pool.QueryRow(ctx, `
		SELECT id, source_key, username, host(ip_address), mac_address,
			acct_status_type, acct_session_id, event_timestamp, acct_delay_time,
			status, attempt_count, next_attempt_at, lease_until, last_error,
			created_at, updated_at, sent_at
		FROM sophos_radius_sso_outbox WHERE source_key=$1
	`, key))
}
