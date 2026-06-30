package enforcement

import (
	"context"
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
