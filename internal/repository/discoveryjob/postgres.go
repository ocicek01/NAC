package discoveryjob

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/discoveryjob"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, job domain.Job) (domain.Job, error) {
	summary, err := json.Marshal(normalizeSummary(job.Summary))
	if err != nil {
		return domain.Job{}, err
	}

	query := `
		INSERT INTO discovery_jobs (
			id, switch_id, scope, status, requested_source, requested_by, worker_id,
			current_step, progress_percent, attempt_count, max_attempts, error_message,
			summary, started_at, completed_at, locked_at, created_at, updated_at
		)
		VALUES (
			$1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13::jsonb, NULLIF($14, '0001-01-01T00:00:00Z')::timestamptz,
			NULLIF($15, '0001-01-01T00:00:00Z')::timestamptz, NULLIF($16, '0001-01-01T00:00:00Z')::timestamptz, $17, $18
		)
	`

	_, err = r.pool.Exec(
		ctx,
		query,
		job.ID,
		job.SwitchID,
		job.Scope,
		job.Status,
		job.RequestedSource,
		job.RequestedBy,
		job.WorkerID,
		job.CurrentStep,
		job.ProgressPercent,
		job.AttemptCount,
		job.MaxAttempts,
		job.ErrorMessage,
		string(summary),
		job.StartedAt.UTC().Format(timeLayout),
		job.CompletedAt.UTC().Format(timeLayout),
		job.LockedAt.UTC().Format(timeLayout),
		job.CreatedAt,
		job.UpdatedAt,
	)
	if err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*domain.Job, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, COALESCE(switch_id::text, ''), scope, status, requested_source, requested_by,
			worker_id, current_step, progress_percent, attempt_count, max_attempts, error_message,
			summary, COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(locked_at, '0001-01-01T00:00:00Z'::timestamptz),
			created_at, updated_at
		FROM discovery_jobs
		WHERE id = $1
		LIMIT 1
	`, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return &jobs[0], nil
}

func (r *PostgresRepository) Update(ctx context.Context, job domain.Job) (domain.Job, error) {
	summary, err := json.Marshal(normalizeSummary(job.Summary))
	if err != nil {
		return domain.Job{}, err
	}

	query := `
		UPDATE discovery_jobs
		SET switch_id = NULLIF($2, '')::uuid,
		    scope = $3,
		    status = $4,
		    requested_source = $5,
		    requested_by = $6,
		    worker_id = $7,
		    current_step = $8,
		    progress_percent = $9,
		    attempt_count = $10,
		    max_attempts = $11,
		    error_message = $12,
		    summary = $13::jsonb,
		    started_at = NULLIF($14, '0001-01-01T00:00:00Z')::timestamptz,
		    completed_at = NULLIF($15, '0001-01-01T00:00:00Z')::timestamptz,
		    locked_at = NULLIF($16, '0001-01-01T00:00:00Z')::timestamptz,
		    updated_at = $17
		WHERE id = $1
	`

	_, err = r.pool.Exec(
		ctx,
		query,
		job.ID,
		job.SwitchID,
		job.Scope,
		job.Status,
		job.RequestedSource,
		job.RequestedBy,
		job.WorkerID,
		job.CurrentStep,
		job.ProgressPercent,
		job.AttemptCount,
		job.MaxAttempts,
		job.ErrorMessage,
		string(summary),
		job.StartedAt.UTC().Format(timeLayout),
		job.CompletedAt.UTC().Format(timeLayout),
		job.LockedAt.UTC().Format(timeLayout),
		job.UpdatedAt,
	)
	if err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (r *PostgresRepository) ListBySwitch(ctx context.Context, switchID string, limit int) ([]domain.Job, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			id, COALESCE(switch_id::text, ''), scope, status, requested_source, requested_by,
			worker_id, current_step, progress_percent, attempt_count, max_attempts, error_message,
			summary, COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(locked_at, '0001-01-01T00:00:00Z'::timestamptz),
			created_at, updated_at
		FROM discovery_jobs
		WHERE switch_id = NULLIF($1, '')::uuid
		ORDER BY created_at DESC
		LIMIT $2
	`, strings.TrimSpace(switchID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanJobs(rows)
}

func (r *PostgresRepository) ClaimNextQueued(ctx context.Context, workerID string) (*domain.Job, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = "worker-1"
	}

	now := time.Now().UTC()
	rows, err := r.pool.Query(ctx, `
		UPDATE discovery_jobs
		SET status = 'running',
		    worker_id = $1,
		    current_step = 'claimed',
		    progress_percent = CASE WHEN progress_percent <= 0 THEN 5 ELSE progress_percent END,
		    attempt_count = attempt_count + 1,
		    started_at = COALESCE(started_at, $2),
		    locked_at = $2,
		    updated_at = $2
		WHERE id = (
			SELECT id
			FROM discovery_jobs
			WHERE status = 'queued'
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING
			id, COALESCE(switch_id::text, ''), scope, status, requested_source, requested_by,
			worker_id, current_step, progress_percent, attempt_count, max_attempts, error_message,
			summary, COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(locked_at, '0001-01-01T00:00:00Z'::timestamptz),
			created_at, updated_at
	`, workerID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return &jobs[0], nil
}

func (r *PostgresRepository) ClaimQueuedByID(ctx context.Context, id, workerID string) (*domain.Job, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = "worker-1"
	}

	now := time.Now().UTC()
	rows, err := r.pool.Query(ctx, `
		UPDATE discovery_jobs
		SET status = 'running',
		    worker_id = $2,
		    current_step = 'claimed',
		    progress_percent = CASE WHEN progress_percent <= 0 THEN 5 ELSE progress_percent END,
		    attempt_count = attempt_count + 1,
		    started_at = COALESCE(started_at, $3),
		    locked_at = $3,
		    updated_at = $3
		WHERE id = $1
		  AND status = 'queued'
		RETURNING
			id, COALESCE(switch_id::text, ''), scope, status, requested_source, requested_by,
			worker_id, current_step, progress_percent, attempt_count, max_attempts, error_message,
			summary, COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(locked_at, '0001-01-01T00:00:00Z'::timestamptz),
			created_at, updated_at
	`, strings.TrimSpace(id), workerID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return &jobs[0], nil
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanJobs(rows rowScanner) ([]domain.Job, error) {
	result := make([]domain.Job, 0)
	for rows.Next() {
		var job domain.Job
		var summaryJSON []byte
		if err := rows.Scan(
			&job.ID,
			&job.SwitchID,
			&job.Scope,
			&job.Status,
			&job.RequestedSource,
			&job.RequestedBy,
			&job.WorkerID,
			&job.CurrentStep,
			&job.ProgressPercent,
			&job.AttemptCount,
			&job.MaxAttempts,
			&job.ErrorMessage,
			&summaryJSON,
			&job.StartedAt,
			&job.CompletedAt,
			&job.LockedAt,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(summaryJSON, &job.Summary)
		result = append(result, job)
	}
	return result, rows.Err()
}

func normalizeSummary(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
