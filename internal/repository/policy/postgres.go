package policy

import (
	"context"
	"encoding/json"
	"strings"

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
			enabled,
			match_conditions,
			decision_type,
			target_vlan,
			enforcement_action,
			dry_run,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, $14, $15, $16, $17, $18)
	`
	conditions, err := json.Marshal(normalizeConditions(policy.MatchConditions, policy.MatchField, policy.MatchOperator, policy.MatchValue))
	if err != nil {
		return domain.Policy{}, err
	}
	_, err = r.pool.Exec(
		ctx,
		query,
		policy.ID,
		strings.TrimSpace(policy.Name),
		strings.TrimSpace(policy.Description),
		strings.TrimSpace(policy.Type),
		strings.TrimSpace(policy.Action),
		strings.TrimSpace(policy.MatchField),
		strings.TrimSpace(policy.MatchOperator),
		strings.TrimSpace(policy.MatchValue),
		policy.Priority,
		normalizePolicyStatus(policy.Enabled, policy.Status),
		policy.Enabled,
		conditions,
		strings.TrimSpace(policy.DecisionType),
		policy.TargetVLAN,
		strings.TrimSpace(policy.EnforcementAction),
		policy.DryRun,
		policy.CreatedAt,
		policy.UpdatedAt,
	)
	if err != nil {
		return domain.Policy{}, err
	}
	policy.MatchConditions = normalizeConditions(policy.MatchConditions, policy.MatchField, policy.MatchOperator, policy.MatchValue)
	policy.Status = normalizePolicyStatus(policy.Enabled, policy.Status)
	return policy, nil
}

func (r *PostgresRepository) Update(ctx context.Context, policy domain.Policy) (domain.Policy, error) {
	query := `
		UPDATE policies
		SET name = $2,
			description = $3,
			type = $4,
			action = $5,
			match_field = $6,
			match_operator = $7,
			match_value = $8,
			priority = $9,
			status = $10,
			enabled = $11,
			match_conditions = $12::jsonb,
			decision_type = $13,
			target_vlan = $14,
			enforcement_action = $15,
			dry_run = $16,
			updated_at = $17
		WHERE id = $1
	`
	conditions, err := json.Marshal(normalizeConditions(policy.MatchConditions, policy.MatchField, policy.MatchOperator, policy.MatchValue))
	if err != nil {
		return domain.Policy{}, err
	}
	_, err = r.pool.Exec(
		ctx,
		query,
		policy.ID,
		strings.TrimSpace(policy.Name),
		strings.TrimSpace(policy.Description),
		strings.TrimSpace(policy.Type),
		strings.TrimSpace(policy.Action),
		strings.TrimSpace(policy.MatchField),
		strings.TrimSpace(policy.MatchOperator),
		strings.TrimSpace(policy.MatchValue),
		policy.Priority,
		normalizePolicyStatus(policy.Enabled, policy.Status),
		policy.Enabled,
		conditions,
		strings.TrimSpace(policy.DecisionType),
		policy.TargetVLAN,
		strings.TrimSpace(policy.EnforcementAction),
		policy.DryRun,
		policy.UpdatedAt,
	)
	if err != nil {
		return domain.Policy{}, err
	}
	policy.MatchConditions = normalizeConditions(policy.MatchConditions, policy.MatchField, policy.MatchOperator, policy.MatchValue)
	policy.Status = normalizePolicyStatus(policy.Enabled, policy.Status)
	return policy, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*domain.Policy, error) {
	query := `
		SELECT id, name, description, type, action, match_field, match_operator, match_value,
		       priority, status, enabled, match_conditions, decision_type, target_vlan,
		       enforcement_action, dry_run, created_at, updated_at
		FROM policies
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, strings.TrimSpace(id))
	item, err := scanPolicy(row)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]domain.Policy, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	query := `
		SELECT id, name, description, type, action, match_field, match_operator, match_value,
		       priority, status, enabled, match_conditions, decision_type, target_vlan,
		       enforcement_action, dry_run, created_at, updated_at
		FROM policies
		ORDER BY priority ASC, created_at ASC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Policy, 0, limit)
	for rows.Next() {
		item, scanErr := scanPolicy(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) ListActive(ctx context.Context) ([]domain.Policy, error) {
	query := `
		SELECT id, name, description, type, action, match_field, match_operator, match_value,
		       priority, status, enabled, match_conditions, decision_type, target_vlan,
		       enforcement_action, dry_run, created_at, updated_at
		FROM policies
		WHERE enabled = true AND status = 'active'
		ORDER BY priority ASC, created_at ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Policy
	for rows.Next() {
		item, scanErr := scanPolicy(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) Disable(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE policies
		SET status = 'disabled',
			enabled = false,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(id))
	return err
}

func (r *PostgresRepository) InsertDecision(ctx context.Context, decision domain.Decision) (domain.Decision, error) {
	signals, err := json.Marshal(decision.TrustSignals)
	if err != nil {
		return domain.Decision{}, err
	}
	reasons, err := json.Marshal(decision.ReasonCodes)
	if err != nil {
		return domain.Decision{}, err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO policy_decisions (
			id, device_id, port_event_id, policy_id, policy_name, decision_type, target_vlan,
			enforcement_action, trust_score, trust_signals, reason_codes, explanation,
			dry_run, enforcement_status, evaluation_duration_ms, created_at
		)
		VALUES (
			$1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, $6, $7,
			$8, $9, $10::jsonb, $11::jsonb, $12, $13, $14, $15, $16
		)
	`, decision.ID, decision.DeviceID, decision.PortEventID, decision.PolicyID, decision.PolicyName, decision.DecisionType, decision.TargetVLAN, decision.EnforcementAction, decision.TrustScore, signals, reasons, decision.Explanation, decision.DryRun, decision.EnforcementStatus, decision.EvaluationDurationMS, decision.CreatedAt)
	if err != nil {
		return domain.Decision{}, err
	}
	return decision, nil
}

func (r *PostgresRepository) ListDecisions(ctx context.Context, limit, offset int) ([]domain.Decision, error) {
	return r.listDecisions(ctx, "", limit, offset)
}

func (r *PostgresRepository) ListDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]domain.Decision, error) {
	return r.listDecisions(ctx, strings.TrimSpace(deviceID), limit, offset)
}

func (r *PostgresRepository) listDecisions(ctx context.Context, deviceID string, limit, offset int) ([]domain.Decision, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	query := `
		SELECT id, COALESCE(device_id::text, ''), COALESCE(port_event_id::text, ''), COALESCE(policy_id::text, ''),
		       COALESCE(policy_name, ''), COALESCE(decision_type, ''), COALESCE(target_vlan, 0),
		       COALESCE(enforcement_action, ''), COALESCE(trust_score, 0), COALESCE(trust_signals, '[]'::jsonb),
		       COALESCE(reason_codes, '[]'::jsonb), COALESCE(explanation, ''), COALESCE(dry_run, true),
		       COALESCE(enforcement_status, ''), COALESCE(evaluation_duration_ms, 0), created_at
		FROM policy_decisions
		WHERE ($1 = '' OR device_id = NULLIF($1, '')::uuid)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, deviceID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Decision, 0, limit)
	for rows.Next() {
		item, scanErr := scanDecision(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) InsertTrustScoreResult(ctx context.Context, result domain.TrustScoreResult) (domain.TrustScoreResult, error) {
	signals, err := json.Marshal(result.Signals)
	if err != nil {
		return domain.TrustScoreResult{}, err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO trust_score_results (
			id, device_id, score, signals, calculated_at, calculation_version
		)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4::jsonb, $5, $6)
	`, result.ID, result.DeviceID, result.Score, signals, result.CalculatedAt, result.CalculationVersion)
	if err != nil {
		return domain.TrustScoreResult{}, err
	}
	return result, nil
}

func scanPolicy(scanner interface{ Scan(dest ...any) error }) (domain.Policy, error) {
	var item domain.Policy
	var rawConditions []byte
	err := scanner.Scan(
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
		&item.Enabled,
		&rawConditions,
		&item.DecisionType,
		&item.TargetVLAN,
		&item.EnforcementAction,
		&item.DryRun,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return domain.Policy{}, err
	}
	if len(rawConditions) > 0 {
		_ = json.Unmarshal(rawConditions, &item.MatchConditions)
	}
	item.MatchConditions = normalizeConditions(item.MatchConditions, item.MatchField, item.MatchOperator, item.MatchValue)
	return item, nil
}

func scanDecision(scanner interface{ Scan(dest ...any) error }) (domain.Decision, error) {
	var item domain.Decision
	var rawSignals []byte
	var rawReasons []byte
	err := scanner.Scan(
		&item.ID,
		&item.DeviceID,
		&item.PortEventID,
		&item.PolicyID,
		&item.PolicyName,
		&item.DecisionType,
		&item.TargetVLAN,
		&item.EnforcementAction,
		&item.TrustScore,
		&rawSignals,
		&rawReasons,
		&item.Explanation,
		&item.DryRun,
		&item.EnforcementStatus,
		&item.EvaluationDurationMS,
		&item.CreatedAt,
	)
	if err != nil {
		return domain.Decision{}, err
	}
	if len(rawSignals) > 0 {
		_ = json.Unmarshal(rawSignals, &item.TrustSignals)
	}
	if len(rawReasons) > 0 {
		_ = json.Unmarshal(rawReasons, &item.ReasonCodes)
	}
	return item, nil
}

func normalizeConditions(conditions []domain.Condition, field, operator, value string) []domain.Condition {
	if len(conditions) > 0 {
		out := make([]domain.Condition, 0, len(conditions))
		for _, item := range conditions {
			item.Field = strings.TrimSpace(item.Field)
			item.Operator = strings.TrimSpace(item.Operator)
			item.Value = strings.TrimSpace(item.Value)
			if item.Field == "" || item.Operator == "" {
				continue
			}
			out = append(out, item)
		}
		return out
	}
	if strings.TrimSpace(field) == "" || strings.TrimSpace(operator) == "" {
		return []domain.Condition{}
	}
	return []domain.Condition{{Field: strings.TrimSpace(field), Operator: strings.TrimSpace(operator), Value: strings.TrimSpace(value)}}
}

func normalizePolicyStatus(enabled bool, status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if !enabled {
		return "disabled"
	}
	if status == "" || status == "disabled" {
		return "active"
	}
	return status
}
