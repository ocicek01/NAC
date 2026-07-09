package auditlog

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/auditlog"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, log domain.Log) (domain.Log, error) {
	payload, err := json.Marshal(normalizePayload(log.Payload))
	if err != nil {
		return domain.Log{}, err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_logs (
			id, actor, action, status, target_type, target_id, switch_id, mac_address, payload, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, $8, $9::jsonb, $10)
	`, log.ID, log.Actor, log.Action, log.Status, log.TargetType, log.TargetID, log.SwitchID, log.MACAddress, string(payload), log.CreatedAt)
	if err != nil {
		return domain.Log{}, err
	}

	log.Payload = normalizePayload(log.Payload)
	return log, nil
}

func (r *PostgresRepository) ListRecent(ctx context.Context, limit int) ([]domain.Log, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, actor, action, status, target_type, target_id, COALESCE(switch_id::text, ''), COALESCE(mac_address, ''), payload, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Log, 0, limit)
	for rows.Next() {
		var item domain.Log
		var payload []byte
		if err := rows.Scan(&item.ID, &item.Actor, &item.Action, &item.Status, &item.TargetType, &item.TargetID, &item.SwitchID, &item.MACAddress, &payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &item.Payload)
		}
		item.Payload = normalizePayload(item.Payload)
		result = append(result, item)
	}

	return result, rows.Err()
}

func normalizePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	return payload
}
