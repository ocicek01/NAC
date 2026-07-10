package enforcement

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/enforcement"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, decision domain.Decision) (domain.Decision, error) {
	query := `
		INSERT INTO enforcement_decisions (
			id,
			device_mac_address,
			device_hostname,
			policy_action,
			policy_reason,
			decision_action,
			decision_mode,
			selected_method,
			fallback_methods,
			requires_approval,
			approval_status,
			attempt_count,
			max_attempts,
			next_attempt_at,
			last_error,
			switch_id,
			switch_name,
			management_ip,
			bridge_port,
			if_index,
			interface_name,
			interface_description,
			status,
			created_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9,
			$10, $11, $12, $13, $14::timestamptz, $15,
			NULLIF($16, '')::uuid,
			$17,
			NULLIF($18, '')::inet,
			$19,
			$20,
			$21,
			$22,
			$23,
			$24
		)
	`

	var nextAttemptAt any
	if !decision.NextAttemptAt.IsZero() {
		nextAttemptAt = decision.NextAttemptAt
	}

	_, err := r.pool.Exec(ctx, query,
		decision.ID,
		decision.DeviceMACAddress,
		decision.DeviceHostname,
		decision.PolicyAction,
		decision.PolicyReason,
		decision.DecisionAction,
		decision.DecisionMode,
		decision.SelectedMethod,
		strings.Join(decision.FallbackMethods, ","),
		decision.RequiresApproval,
		decision.ApprovalStatus,
		decision.AttemptCount,
		decision.MaxAttempts,
		nextAttemptAt,
		decision.LastError,
		decision.SwitchID,
		decision.SwitchName,
		decision.ManagementIP,
		decision.BridgePort,
		decision.IfIndex,
		decision.InterfaceName,
		decision.InterfaceDescription,
		decision.Status,
		decision.CreatedAt,
	)
	if err != nil {
		return domain.Decision{}, err
	}

	return decision, nil
}

func (r *PostgresRepository) ListRecent(ctx context.Context, limit int) ([]domain.Decision, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT
			id,
			device_mac_address,
			COALESCE(device_hostname, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			decision_action,
			decision_mode,
			COALESCE(selected_method, ''),
			COALESCE(fallback_methods, ''),
			COALESCE(requires_approval, false),
			COALESCE(approval_status, 'not-required'),
			COALESCE(attempt_count, 0),
			COALESCE(max_attempts, 3),
			COALESCE(next_attempt_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_error, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(host(management_ip), ''),
			COALESCE(bridge_port, 0),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(interface_description, ''),
			status,
			created_at
		FROM enforcement_decisions
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Decision
	for rows.Next() {
		var item domain.Decision
		var fallbackMethods string
		if err := rows.Scan(
			&item.ID,
			&item.DeviceMACAddress,
			&item.DeviceHostname,
			&item.PolicyAction,
			&item.PolicyReason,
			&item.DecisionAction,
			&item.DecisionMode,
			&item.SelectedMethod,
			&fallbackMethods,
			&item.RequiresApproval,
			&item.ApprovalStatus,
			&item.AttemptCount,
			&item.MaxAttempts,
			&item.NextAttemptAt,
			&item.LastError,
			&item.SwitchID,
			&item.SwitchName,
			&item.ManagementIP,
			&item.BridgePort,
			&item.IfIndex,
			&item.InterfaceName,
			&item.InterfaceDescription,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.FallbackMethods = splitCSV(fallbackMethods)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *PostgresRepository) ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]domain.Decision, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT
			id,
			device_mac_address,
			COALESCE(device_hostname, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			decision_action,
			decision_mode,
			COALESCE(selected_method, ''),
			COALESCE(fallback_methods, ''),
			COALESCE(requires_approval, false),
			COALESCE(approval_status, 'not-required'),
			COALESCE(attempt_count, 0),
			COALESCE(max_attempts, 3),
			COALESCE(next_attempt_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_error, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(host(management_ip), ''),
			COALESCE(bridge_port, 0),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(interface_description, ''),
			status,
			created_at
		FROM enforcement_decisions
		WHERE UPPER(device_mac_address) = UPPER($1)
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, macAddress, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Decision
	for rows.Next() {
		var item domain.Decision
		var fallbackMethods string
		if err := rows.Scan(
			&item.ID,
			&item.DeviceMACAddress,
			&item.DeviceHostname,
			&item.PolicyAction,
			&item.PolicyReason,
			&item.DecisionAction,
			&item.DecisionMode,
			&item.SelectedMethod,
			&fallbackMethods,
			&item.RequiresApproval,
			&item.ApprovalStatus,
			&item.AttemptCount,
			&item.MaxAttempts,
			&item.NextAttemptAt,
			&item.LastError,
			&item.SwitchID,
			&item.SwitchName,
			&item.ManagementIP,
			&item.BridgePort,
			&item.IfIndex,
			&item.InterfaceName,
			&item.InterfaceDescription,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.FallbackMethods = splitCSV(fallbackMethods)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *PostgresRepository) FindLatestByKey(ctx context.Context, macAddress, switchID, policyAction string, ifIndex int, interfaceName string) (*domain.Decision, error) {
	query := `
		SELECT
			id,
			device_mac_address,
			COALESCE(device_hostname, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			decision_action,
			decision_mode,
			COALESCE(selected_method, ''),
			COALESCE(fallback_methods, ''),
			COALESCE(requires_approval, false),
			COALESCE(approval_status, 'not-required'),
			COALESCE(attempt_count, 0),
			COALESCE(max_attempts, 3),
			COALESCE(next_attempt_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_error, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(host(management_ip), ''),
			COALESCE(bridge_port, 0),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(interface_description, ''),
			status,
			created_at
		FROM enforcement_decisions
		WHERE UPPER(device_mac_address) = UPPER($1)
		  AND COALESCE(switch_id::text, '') = $2
		  AND COALESCE(policy_action, '') = $3
		  AND COALESCE(if_index, 0) = $4
		  AND COALESCE(interface_name, '') = $5
		ORDER BY created_at DESC
		LIMIT 1
	`

	var item domain.Decision
	var fallbackMethods string
	err := r.pool.QueryRow(ctx, query, macAddress, strings.TrimSpace(switchID), strings.TrimSpace(policyAction), ifIndex, strings.TrimSpace(interfaceName)).Scan(
		&item.ID,
		&item.DeviceMACAddress,
		&item.DeviceHostname,
		&item.PolicyAction,
		&item.PolicyReason,
		&item.DecisionAction,
		&item.DecisionMode,
		&item.SelectedMethod,
		&fallbackMethods,
		&item.RequiresApproval,
		&item.ApprovalStatus,
		&item.AttemptCount,
		&item.MaxAttempts,
		&item.NextAttemptAt,
		&item.LastError,
		&item.SwitchID,
		&item.SwitchName,
		&item.ManagementIP,
		&item.BridgePort,
		&item.IfIndex,
		&item.InterfaceName,
		&item.InterfaceDescription,
		&item.Status,
		&item.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	item.FallbackMethods = splitCSV(fallbackMethods)
	return &item, nil
}

func (r *PostgresRepository) AcquireState(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName string, lockedUntil time.Time) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		SELECT policy_action, target_vlan, status, COALESCE(locked_until, '0001-01-01T00:00:00Z'::timestamptz)
		FROM enforcement_state
		WHERE UPPER(mac_address) = UPPER($1)
		  AND switch_id = NULLIF($2, '')::uuid
		  AND if_index = $3
		FOR UPDATE
	`

	var (
		existingAction string
		existingVLAN   int
		existingStatus string
		existingUntil  time.Time
	)
	err = tx.QueryRow(ctx, query, macAddress, strings.TrimSpace(switchID), ifIndex).Scan(&existingAction, &existingVLAN, &existingStatus, &existingUntil)
	if err != nil && err != pgx.ErrNoRows {
		return false, err
	}

	now := time.Now().UTC()
	if err == nil {
		sameContext := strings.EqualFold(strings.TrimSpace(existingAction), strings.TrimSpace(policyAction)) && existingVLAN == targetVLAN
		status := strings.ToLower(strings.TrimSpace(existingStatus))
		if sameContext {
			if status == "executed" {
				if err := tx.Commit(ctx); err != nil {
					return false, err
				}
				return false, nil
			}
			if (status == "pending" || status == "queued") && !existingUntil.IsZero() && existingUntil.After(now) {
				if err := tx.Commit(ctx); err != nil {
					return false, err
				}
				return false, nil
			}
		}
	}

	upsert := `
		INSERT INTO enforcement_state (
			mac_address, switch_id, if_index, interface_name, policy_action, target_vlan, status, desired_state, locked_until, last_attempt_at, last_error, updated_at
		)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, 'pending', $5, $7, NOW(), '', NOW())
		ON CONFLICT (mac_address, switch_id, if_index)
		DO UPDATE SET
			interface_name = EXCLUDED.interface_name,
			policy_action = EXCLUDED.policy_action,
			target_vlan = EXCLUDED.target_vlan,
			status = 'pending',
			desired_state = EXCLUDED.desired_state,
			locked_until = EXCLUDED.locked_until,
			last_attempt_at = NOW(),
			last_error = '',
			decision_id = NULL,
			updated_at = NOW()
	`
	if _, err := tx.Exec(ctx, upsert, macAddress, strings.TrimSpace(switchID), ifIndex, strings.TrimSpace(interfaceName), strings.TrimSpace(policyAction), targetVLAN, lockedUntil); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *PostgresRepository) MarkStateExecuted(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string) error {
	query := `
		INSERT INTO enforcement_state (
			mac_address, switch_id, if_index, interface_name, policy_action, target_vlan, status, desired_state, applied_state, applied_vlan, locked_until, decision_id, last_method, last_success_at, last_error, updated_at
		)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, 'executed', $5, $5, $6, NULL, NULLIF($7, '')::uuid, $8, NOW(), '', NOW())
		ON CONFLICT (mac_address, switch_id, if_index)
		DO UPDATE SET
			interface_name = EXCLUDED.interface_name,
			policy_action = EXCLUDED.policy_action,
			target_vlan = EXCLUDED.target_vlan,
			status = 'executed',
			desired_state = EXCLUDED.desired_state,
			applied_state = EXCLUDED.applied_state,
			applied_vlan = EXCLUDED.applied_vlan,
			locked_until = NULL,
			decision_id = EXCLUDED.decision_id,
			last_method = EXCLUDED.last_method,
			last_success_at = NOW(),
			last_error = '',
			updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, macAddress, strings.TrimSpace(switchID), ifIndex, strings.TrimSpace(interfaceName), strings.TrimSpace(policyAction), targetVLAN, strings.TrimSpace(decisionID), strings.TrimSpace(method))
	return err
}

func (r *PostgresRepository) MarkStateFailed(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string, lockedUntil time.Time) error {
	query := `
		INSERT INTO enforcement_state (
			mac_address, switch_id, if_index, interface_name, policy_action, target_vlan, status, desired_state, locked_until, decision_id, last_method, last_attempt_at, retry_count, updated_at
		)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, 'failed', $5, $7, NULLIF($8, '')::uuid, $9, NOW(), 1, NOW())
		ON CONFLICT (mac_address, switch_id, if_index)
		DO UPDATE SET
			interface_name = EXCLUDED.interface_name,
			policy_action = EXCLUDED.policy_action,
			target_vlan = EXCLUDED.target_vlan,
			status = 'failed',
			desired_state = EXCLUDED.desired_state,
			locked_until = EXCLUDED.locked_until,
			decision_id = EXCLUDED.decision_id,
			last_method = EXCLUDED.last_method,
			last_attempt_at = NOW(),
			retry_count = enforcement_state.retry_count + 1,
			updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, macAddress, strings.TrimSpace(switchID), ifIndex, strings.TrimSpace(interfaceName), strings.TrimSpace(policyAction), targetVLAN, lockedUntil, strings.TrimSpace(decisionID), strings.TrimSpace(method))
	return err
}

func (r *PostgresRepository) ClearStateForMAC(ctx context.Context, macAddress string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM enforcement_state WHERE UPPER(mac_address) = UPPER($1)`, macAddress)
	return err
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*domain.Decision, error) {
	query := `
		SELECT
			id,
			device_mac_address,
			COALESCE(device_hostname, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			decision_action,
			decision_mode,
			COALESCE(selected_method, ''),
			COALESCE(fallback_methods, ''),
			COALESCE(requires_approval, false),
			COALESCE(approval_status, 'not-required'),
			COALESCE(attempt_count, 0),
			COALESCE(max_attempts, 3),
			COALESCE(next_attempt_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_error, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(host(management_ip), ''),
			COALESCE(bridge_port, 0),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(interface_description, ''),
			status,
			created_at
		FROM enforcement_decisions
		WHERE id = $1
		LIMIT 1
	`

	var item domain.Decision
	var fallbackMethods string
	if err := r.pool.QueryRow(ctx, query, strings.TrimSpace(id)).Scan(
		&item.ID,
		&item.DeviceMACAddress,
		&item.DeviceHostname,
		&item.PolicyAction,
		&item.PolicyReason,
		&item.DecisionAction,
		&item.DecisionMode,
		&item.SelectedMethod,
		&fallbackMethods,
		&item.RequiresApproval,
		&item.ApprovalStatus,
		&item.AttemptCount,
		&item.MaxAttempts,
		&item.NextAttemptAt,
		&item.LastError,
		&item.SwitchID,
		&item.SwitchName,
		&item.ManagementIP,
		&item.BridgePort,
		&item.IfIndex,
		&item.InterfaceName,
		&item.InterfaceDescription,
		&item.Status,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.FallbackMethods = splitCSV(fallbackMethods)
	return &item, nil
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (r *PostgresRepository) Approve(ctx context.Context, id string) error {
	query := `
		UPDATE enforcement_decisions
		SET approval_status = 'approved',
			status = 'queued',
			next_attempt_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

func (r *PostgresRepository) Reject(ctx context.Context, id string) error {
	query := `
		UPDATE enforcement_decisions
		SET approval_status = 'rejected',
			status = 'cancelled'
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

func (r *PostgresRepository) Retry(ctx context.Context, id string) error {
	query := `
		UPDATE enforcement_decisions
		SET attempt_count = attempt_count + 1,
			status = 'queued',
			next_attempt_at = NOW() + INTERVAL '5 minutes',
			last_error = ''
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

func (r *PostgresRepository) MarkExecuted(ctx context.Context, id string) error {
	query := `
		UPDATE enforcement_decisions
		SET status = 'executed',
			last_error = ''
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, strings.TrimSpace(id))
	return err
}

func (r *PostgresRepository) MarkFailed(ctx context.Context, id, lastError string) error {
	query := `
		UPDATE enforcement_decisions
		SET status = 'failed',
			last_error = $2
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, strings.TrimSpace(id), strings.TrimSpace(lastError))
	return err
}

func (r *PostgresRepository) InsertRequest(ctx context.Context, request domain.Request) (domain.Request, error) {
	metadata, err := json.Marshal(normalizeJSONMap(request.Metadata))
	if err != nil {
		return domain.Request{}, err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO enforcement_requests (
			id, device_id, policy_decision_id, switch_id, port_id, requested_action, target_vlan, previous_vlan,
			requested_by, request_source, mode, status, attempt_count, adapter, command_summary, error_code,
			error_message, requested_at, started_at, completed_at, verified_at, rollback_of_request_id,
			verification_status, current_switch_id, current_if_index, current_interface_name, target_device_mac,
			metadata, created_at, updated_at
		)
		VALUES (
			$1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, NULLIF($22, '')::uuid,
			$23, NULLIF($24, '')::uuid, $25, $26, $27,
			$28::jsonb, $29, $30
		)
	`, request.ID, request.DeviceID, request.PolicyDecisionID, request.SwitchID, request.PortID, request.RequestedAction, request.TargetVLAN, request.PreviousVLAN, request.RequestedBy, request.RequestSource, request.Mode, request.Status, request.AttemptCount, request.Adapter, request.CommandSummary, request.ErrorCode, request.ErrorMessage, request.RequestedAt, nullableTime(request.StartedAt), nullableTime(request.CompletedAt), nullableTime(request.VerifiedAt), request.RollbackOfRequestID, request.VerificationStatus, request.CurrentSwitchID, request.CurrentIfIndex, request.CurrentInterfaceName, request.TargetDeviceMAC, metadata, request.CreatedAt, request.UpdatedAt)
	if err != nil {
		return domain.Request{}, err
	}
	if request.Metadata == nil {
		request.Metadata = map[string]any{}
	}
	return request, nil
}

func (r *PostgresRepository) InsertResult(ctx context.Context, result domain.Result) (domain.Result, error) {
	prev, err := json.Marshal(normalizeJSONMap(result.PreviousState))
	if err != nil {
		return domain.Result{}, err
	}
	expected, err := json.Marshal(normalizeJSONMap(result.ExpectedState))
	if err != nil {
		return domain.Result{}, err
	}
	observed, err := json.Marshal(normalizeJSONMap(result.ObservedState))
	if err != nil {
		return domain.Result{}, err
	}
	adapterResponse, err := json.Marshal(normalizeJSONMap(result.AdapterResponse))
	if err != nil {
		return domain.Result{}, err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO enforcement_results (
			id, enforcement_request_id, attempt_number, adapter, transport, action, success, changed, execution_status,
			previous_state, expected_state, observed_state, verification_status, command_summary, adapter_response,
			duration_ms, error_code, error_message, started_at, completed_at, verified_at, created_at
		)
		VALUES (
			$1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8, $9,
			$10::jsonb, $11::jsonb, $12::jsonb, $13, $14, $15::jsonb,
			$16, $17, $18, $19, $20, $21, $22
		)
	`, result.ID, result.EnforcementRequestID, result.AttemptNumber, result.Adapter, result.Transport, result.Action, result.Success, result.Changed, result.ExecutionStatus, prev, expected, observed, result.VerificationStatus, result.CommandSummary, adapterResponse, result.DurationMS, result.ErrorCode, result.ErrorMessage, nullableTime(result.StartedAt), nullableTime(result.CompletedAt), nullableTime(result.VerifiedAt), result.CreatedAt)
	if err != nil {
		return domain.Result{}, err
	}
	if result.PreviousState == nil {
		result.PreviousState = map[string]any{}
	}
	if result.ExpectedState == nil {
		result.ExpectedState = map[string]any{}
	}
	if result.ObservedState == nil {
		result.ObservedState = map[string]any{}
	}
	if result.AdapterResponse == nil {
		result.AdapterResponse = map[string]any{}
	}
	return result, nil
}

func (r *PostgresRepository) ListRequests(ctx context.Context, filters domain.RequestFilters) ([]domain.Request, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}
	query := `
		SELECT id, COALESCE(device_id::text, ''), COALESCE(policy_decision_id::text, ''), COALESCE(switch_id::text, ''), COALESCE(port_id::text, ''),
		       requested_action, target_vlan, previous_vlan, requested_by, request_source, mode, status, attempt_count,
		       COALESCE(adapter, ''), COALESCE(command_summary, ''), COALESCE(error_code, ''), COALESCE(error_message, ''),
		       requested_at, COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz),
		       COALESCE(verified_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(rollback_of_request_id::text, ''), COALESCE(verification_status, ''),
		       COALESCE(current_switch_id::text, ''), COALESCE(current_if_index, 0), COALESCE(current_interface_name, ''), COALESCE(target_device_mac, ''),
		       COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM enforcement_requests
		WHERE ($1 = '' OR device_id = NULLIF($1, '')::uuid)
		  AND ($2 = '' OR switch_id = NULLIF($2, '')::uuid)
		  AND ($3 = '' OR LOWER(status) = LOWER($3))
		  AND ($4 = '' OR LOWER(mode) = LOWER($4))
		  AND ($5 = '' OR LOWER(requested_action) = LOWER($5))
		  AND ($6::timestamptz IS NULL OR created_at >= $6)
		  AND ($7::timestamptz IS NULL OR created_at <= $7)
		ORDER BY requested_at DESC
		LIMIT $8 OFFSET $9
	`
	rows, err := r.pool.Query(ctx, query, strings.TrimSpace(filters.DeviceID), strings.TrimSpace(filters.SwitchID), strings.TrimSpace(filters.Status), strings.TrimSpace(filters.Mode), strings.TrimSpace(filters.Action), nullableTime(filters.DateFrom), nullableTime(filters.DateTo), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Request, 0, limit)
	for rows.Next() {
		item, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]domain.Request, error) {
	return r.ListRequests(ctx, domain.RequestFilters{DeviceID: strings.TrimSpace(deviceID), Limit: limit, Offset: offset})
}

func (r *PostgresRepository) ListResultsByRequest(ctx context.Context, requestID string) ([]domain.Result, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, COALESCE(enforcement_request_id::text, ''), attempt_number, COALESCE(adapter, ''), COALESCE(transport, ''), COALESCE(action, ''), success, changed,
		       COALESCE(execution_status, ''), previous_state, expected_state, observed_state,
		       COALESCE(verification_status, ''), COALESCE(command_summary, ''), COALESCE(adapter_response, '{}'::jsonb), duration_ms,
		       COALESCE(error_code, ''), COALESCE(error_message, ''), COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
		       COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(verified_at, '0001-01-01T00:00:00Z'::timestamptz), created_at
		FROM enforcement_results
		WHERE enforcement_request_id = NULLIF($1, '')::uuid
		ORDER BY attempt_number DESC, created_at DESC
	`, strings.TrimSpace(requestID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.Result
	for rows.Next() {
		item, err := scanResult(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) FindRequestByID(ctx context.Context, id string) (*domain.Request, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, COALESCE(device_id::text, ''), COALESCE(policy_decision_id::text, ''), COALESCE(switch_id::text, ''), COALESCE(port_id::text, ''),
		       requested_action, target_vlan, previous_vlan, requested_by, request_source, mode, status, attempt_count,
		       COALESCE(adapter, ''), COALESCE(command_summary, ''), COALESCE(error_code, ''), COALESCE(error_message, ''),
		       requested_at, COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz),
		       COALESCE(verified_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(rollback_of_request_id::text, ''), COALESCE(verification_status, ''),
		       COALESCE(current_switch_id::text, ''), COALESCE(current_if_index, 0), COALESCE(current_interface_name, ''), COALESCE(target_device_mac, ''),
		       COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM enforcement_requests
		WHERE id = NULLIF($1, '')::uuid
		LIMIT 1
	`, strings.TrimSpace(id))
	item, err := scanRequest(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *PostgresRepository) FindActiveRequest(ctx context.Context, policyDecisionID, action string, targetVLAN int) (*domain.Request, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, COALESCE(device_id::text, ''), COALESCE(policy_decision_id::text, ''), COALESCE(switch_id::text, ''), COALESCE(port_id::text, ''),
		       requested_action, target_vlan, previous_vlan, requested_by, request_source, mode, status, attempt_count,
		       COALESCE(adapter, ''), COALESCE(command_summary, ''), COALESCE(error_code, ''), COALESCE(error_message, ''),
		       requested_at, COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz),
		       COALESCE(verified_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(rollback_of_request_id::text, ''), COALESCE(verification_status, ''),
		       COALESCE(current_switch_id::text, ''), COALESCE(current_if_index, 0), COALESCE(current_interface_name, ''), COALESCE(target_device_mac, ''),
		       COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM enforcement_requests
		WHERE policy_decision_id = NULLIF($1, '')::uuid
		  AND requested_action = $2
		  AND target_vlan = $3
		  AND status IN ('pending', 'queued', 'running', 'verifying', 'retry_scheduled', 'succeeded', 'skipped')
		ORDER BY requested_at DESC
		LIMIT 1
	`, strings.TrimSpace(policyDecisionID), strings.TrimSpace(action), targetVLAN)
	item, err := scanRequest(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *PostgresRepository) ClaimNextRequest(ctx context.Context, now time.Time, staleBefore time.Time) (*domain.Request, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		UPDATE enforcement_requests
		SET status = 'retry_scheduled',
		    attempt_count = attempt_count + 1,
		    error_code = 'stale_running_recovered',
		    error_message = 'stale running request recovered by worker',
		    requested_at = $1,
		    updated_at = $1
		WHERE status IN ('running', 'verifying')
		  AND started_at IS NOT NULL
		  AND started_at < $2
	`, now, staleBefore); err != nil {
		return nil, err
	}
	row := tx.QueryRow(ctx, `
		SELECT id, COALESCE(device_id::text, ''), COALESCE(policy_decision_id::text, ''), COALESCE(switch_id::text, ''), COALESCE(port_id::text, ''),
		       requested_action, target_vlan, previous_vlan, requested_by, request_source, mode, status, attempt_count,
		       COALESCE(adapter, ''), COALESCE(command_summary, ''), COALESCE(error_code, ''), COALESCE(error_message, ''),
		       requested_at, COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(completed_at, '0001-01-01T00:00:00Z'::timestamptz),
		       COALESCE(verified_at, '0001-01-01T00:00:00Z'::timestamptz), COALESCE(rollback_of_request_id::text, ''), COALESCE(verification_status, ''),
		       COALESCE(current_switch_id::text, ''), COALESCE(current_if_index, 0), COALESCE(current_interface_name, ''), COALESCE(target_device_mac, ''),
		       COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		FROM enforcement_requests
		WHERE status IN ('pending', 'queued', 'retry_scheduled', 'rollback_pending')
		  AND requested_at <= $1
		ORDER BY requested_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, now)
	item, err := scanRequest(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE enforcement_requests SET status = 'queued', updated_at = $2 WHERE id = $1`, item.ID, now); err != nil {
		return nil, err
	}
	item.Status = domain.RequestStatusQueued
	item.UpdatedAt = now
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *PostgresRepository) MarkRequestQueued(ctx context.Context, id string, queuedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE enforcement_requests SET status = 'queued', updated_at = $2 WHERE id = NULLIF($1, '')::uuid`, strings.TrimSpace(id), queuedAt)
	return err
}

func (r *PostgresRepository) MarkRequestStarted(ctx context.Context, id string, adapter string, startedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE enforcement_requests SET status = 'running', adapter = $2, started_at = $3, updated_at = $3 WHERE id = NULLIF($1, '')::uuid`, strings.TrimSpace(id), strings.TrimSpace(adapter), startedAt)
	return err
}

func (r *PostgresRepository) MarkRequestVerifying(ctx context.Context, id string, verifyingAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE enforcement_requests SET status = 'verifying', updated_at = $2 WHERE id = NULLIF($1, '')::uuid`, strings.TrimSpace(id), verifyingAt)
	return err
}

func (r *PostgresRepository) MarkRequestCompleted(ctx context.Context, id, status, errorCode, errorMessage, verificationStatus string, completedAt, verifiedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE enforcement_requests
		SET status = $2,
		    error_code = $3,
		    error_message = $4,
		    verification_status = $5,
		    completed_at = $6,
		    verified_at = $7,
		    updated_at = $6
		WHERE id = NULLIF($1, '')::uuid
	`, strings.TrimSpace(id), strings.TrimSpace(status), strings.TrimSpace(errorCode), strings.TrimSpace(errorMessage), strings.TrimSpace(verificationStatus), completedAt, nullableTime(verifiedAt))
	return err
}

func (r *PostgresRepository) MarkRequestRetry(ctx context.Context, id, status, errorCode, errorMessage string, nextAttemptAt time.Time) error {
	if strings.TrimSpace(status) == "" {
		status = domain.RequestStatusRetryScheduled
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE enforcement_requests
		SET status = $2,
		    attempt_count = attempt_count + 1,
		    error_code = $3,
		    error_message = $4,
		    requested_at = $5,
		    updated_at = $5
		WHERE id = NULLIF($1, '')::uuid
	`, strings.TrimSpace(id), strings.TrimSpace(status), strings.TrimSpace(errorCode), strings.TrimSpace(errorMessage), nextAttemptAt)
	return err
}

func (r *PostgresRepository) CancelRequest(ctx context.Context, id, status, errorCode, errorMessage string, completedAt time.Time) error {
	if strings.TrimSpace(status) == "" {
		status = domain.RequestStatusCancelled
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE enforcement_requests
		SET status = $2,
		    error_code = $3,
		    error_message = $4,
		    completed_at = $5,
		    updated_at = $5
		WHERE id = NULLIF($1, '')::uuid
	`, strings.TrimSpace(id), strings.TrimSpace(status), strings.TrimSpace(errorCode), strings.TrimSpace(errorMessage), completedAt)
	return err
}

func (r *PostgresRepository) CancelSupersededRequests(ctx context.Context, deviceID, keepRequestID, reason string) (int, error) {
	commandTag, err := r.pool.Exec(ctx, `
		UPDATE enforcement_requests
		SET status = 'cancelled',
		    error_code = 'superseded_by_newer_decision',
		    error_message = CASE WHEN BTRIM($3) <> '' THEN $3 ELSE 'superseded by newer decision' END,
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE device_id = NULLIF($1, '')::uuid
		  AND id <> NULLIF($2, '')::uuid
		  AND status IN ('pending', 'queued', 'retry_scheduled')
	`, strings.TrimSpace(deviceID), strings.TrimSpace(keepRequestID), strings.TrimSpace(reason))
	if err != nil {
		return 0, err
	}
	return int(commandTag.RowsAffected()), nil
}

func (r *PostgresRepository) UpdateRequestPolicyBinding(ctx context.Context, id, policyDecisionID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE enforcement_requests SET policy_decision_id = NULLIF($2, '')::uuid, updated_at = NOW() WHERE id = NULLIF($1, '')::uuid`, strings.TrimSpace(id), strings.TrimSpace(policyDecisionID))
	return err
}

func (r *PostgresRepository) UpdatePolicyDecisionEnforcement(ctx context.Context, policyDecisionID, requestID, status, errorMessage string, startedAt, completedAt, enforcedAt time.Time, requested bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE policy_decisions
		SET enforcement_requested = $2,
		    enforcement_request_id = NULLIF($3, '')::uuid,
		    enforcement_status = $4,
		    enforcement_started_at = $5,
		    enforcement_completed_at = $6,
		    enforcement_error = $7,
		    enforced_at = $8
		WHERE id = NULLIF($1, '')::uuid
	`, strings.TrimSpace(policyDecisionID), requested, strings.TrimSpace(requestID), strings.TrimSpace(status), nullableTime(startedAt), nullableTime(completedAt), strings.TrimSpace(errorMessage), nullableTime(enforcedAt))
	return err
}

func (r *PostgresRepository) UpdateDeviceEnforcementSnapshot(ctx context.Context, deviceID, requestID, action string, vlanID int, status, errorMessage string, observedAt time.Time, verified bool) error {
	quarantineStatus := ""
	if action == domain.ActionAssignQuarantineVLAN && verified {
		quarantineStatus = "quarantined"
	} else if verified {
		quarantineStatus = "released"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE devices
		SET last_enforcement_action = $2,
		    last_enforcement_vlan = $3,
		    last_enforcement_status = $4,
		    last_enforcement_at = $5,
		    last_enforcement_error = $6,
		    last_enforcement_request_id = NULLIF($7, '')::uuid,
		    applied_enforcement_state = CASE WHEN $8 THEN $2 ELSE applied_enforcement_state END,
		    applied_enforcement_vlan = CASE WHEN $8 THEN $3 ELSE applied_enforcement_vlan END,
		    verified_vlan = CASE WHEN $8 THEN $3 ELSE verified_vlan END,
		    quarantine_status = CASE WHEN $9 <> '' THEN $9 ELSE quarantine_status END,
		    updated_at = NOW()
		WHERE id = NULLIF($1, '')::uuid
	`, strings.TrimSpace(deviceID), strings.TrimSpace(action), vlanID, strings.TrimSpace(status), observedAt, strings.TrimSpace(errorMessage), strings.TrimSpace(requestID), verified, quarantineStatus)
	return err
}

func (r *PostgresRepository) WorkerStats(ctx context.Context) (domain.WorkerStats, error) {
	var stats domain.WorkerStats
	var oldestSeconds int64
	var lastSuccess time.Time
	row := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status IN ('pending', 'queued', 'retry_scheduled') THEN 1 ELSE 0 END), 0) AS queue_depth,
			COALESCE(MAX(CASE WHEN status IN ('running', 'verifying') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN ('running', 'verifying') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN ('failed', 'verification_failed', 'rollback_failed') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'retry_scheduled' THEN 1 ELSE 0 END), 0),
			COALESCE(EXTRACT(EPOCH FROM (NOW() - MIN(requested_at)))::bigint, 0),
			COALESCE(MAX(CASE WHEN status IN ('succeeded', 'rolled_back', 'skipped') THEN completed_at ELSE NULL END), '0001-01-01T00:00:00Z'::timestamptz)
		FROM enforcement_requests
		WHERE status IN ('pending', 'queued', 'retry_scheduled', 'running', 'verifying', 'failed', 'verification_failed', 'rollback_failed', 'succeeded', 'rolled_back', 'skipped')
	`)
	var runningFlag int
	if err := row.Scan(&stats.QueueDepth, &runningFlag, &stats.RunningRequestCount, &stats.FailedRequestCount, &stats.RetryScheduledCount, &oldestSeconds, &lastSuccess); err != nil {
		return domain.WorkerStats{}, err
	}
	stats.Running = runningFlag > 0 || stats.QueueDepth > 0
	stats.OldestPendingAgeSec = oldestSeconds
	stats.LastSuccessfulAt = lastSuccess
	return stats, nil
}

func scanRequest(scanner interface{ Scan(dest ...any) error }) (domain.Request, error) {
	var item domain.Request
	var rawMetadata []byte
	if err := scanner.Scan(&item.ID, &item.DeviceID, &item.PolicyDecisionID, &item.SwitchID, &item.PortID, &item.RequestedAction, &item.TargetVLAN, &item.PreviousVLAN, &item.RequestedBy, &item.RequestSource, &item.Mode, &item.Status, &item.AttemptCount, &item.Adapter, &item.CommandSummary, &item.ErrorCode, &item.ErrorMessage, &item.RequestedAt, &item.StartedAt, &item.CompletedAt, &item.VerifiedAt, &item.RollbackOfRequestID, &item.VerificationStatus, &item.CurrentSwitchID, &item.CurrentIfIndex, &item.CurrentInterfaceName, &item.TargetDeviceMAC, &rawMetadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return domain.Request{}, err
	}
	_ = json.Unmarshal(rawMetadata, &item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	return item, nil
}

func scanResult(scanner interface{ Scan(dest ...any) error }) (domain.Result, error) {
	var item domain.Result
	var prev []byte
	var expected []byte
	var observed []byte
	var adapter []byte
	if err := scanner.Scan(&item.ID, &item.EnforcementRequestID, &item.AttemptNumber, &item.Adapter, &item.Transport, &item.Action, &item.Success, &item.Changed, &item.ExecutionStatus, &prev, &expected, &observed, &item.VerificationStatus, &item.CommandSummary, &adapter, &item.DurationMS, &item.ErrorCode, &item.ErrorMessage, &item.StartedAt, &item.CompletedAt, &item.VerifiedAt, &item.CreatedAt); err != nil {
		return domain.Result{}, err
	}
	_ = json.Unmarshal(prev, &item.PreviousState)
	_ = json.Unmarshal(expected, &item.ExpectedState)
	_ = json.Unmarshal(observed, &item.ObservedState)
	_ = json.Unmarshal(adapter, &item.AdapterResponse)
	if item.PreviousState == nil {
		item.PreviousState = map[string]any{}
	}
	if item.ExpectedState == nil {
		item.ExpectedState = map[string]any{}
	}
	if item.ObservedState == nil {
		item.ObservedState = map[string]any{}
	}
	if item.AdapterResponse == nil {
		item.AdapterResponse = map[string]any{}
	}
	return item, nil
}

func normalizeJSONMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
