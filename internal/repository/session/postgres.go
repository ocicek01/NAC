package session

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/session"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Upsert(ctx context.Context, session domain.Session) (domain.Session, error) {
	query := `
		INSERT INTO radius_sessions (
			id,
			active_key,
			device_id,
			switch_id,
			switch_name,
			management_ip,
			port_id,
			nas_port,
			nas_port_id,
			if_index,
			interface_name,
			ip_address,
			mac_address,
			username,
			hostname,
			vendor_class,
			called_station_id,
			calling_station_id,
			acct_session_id,
			authorization_result,
			session_type,
			status,
			policy_action,
			policy_reason,
			assigned_vlan,
			started_at,
			last_seen_at,
			ended_at,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, NULLIF($6, '')::inet, $7, $8, $9, $10,
			$11, NULLIF($12, '')::inet, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30
		)
		ON CONFLICT (active_key) DO UPDATE
		SET device_id = COALESCE(EXCLUDED.device_id, radius_sessions.device_id),
			switch_id = COALESCE(EXCLUDED.switch_id, radius_sessions.switch_id),
			switch_name = CASE WHEN EXCLUDED.switch_name <> '' THEN EXCLUDED.switch_name ELSE radius_sessions.switch_name END,
			management_ip = COALESCE(EXCLUDED.management_ip, radius_sessions.management_ip),
			port_id = CASE WHEN EXCLUDED.port_id <> '' THEN EXCLUDED.port_id ELSE radius_sessions.port_id END,
			nas_port = CASE WHEN EXCLUDED.nas_port <> '' THEN EXCLUDED.nas_port ELSE radius_sessions.nas_port END,
			nas_port_id = CASE WHEN EXCLUDED.nas_port_id <> '' THEN EXCLUDED.nas_port_id ELSE radius_sessions.nas_port_id END,
			if_index = CASE WHEN EXCLUDED.if_index > 0 THEN EXCLUDED.if_index ELSE radius_sessions.if_index END,
			interface_name = CASE WHEN EXCLUDED.interface_name <> '' THEN EXCLUDED.interface_name ELSE radius_sessions.interface_name END,
			ip_address = COALESCE(EXCLUDED.ip_address, radius_sessions.ip_address),
			mac_address = EXCLUDED.mac_address,
			username = CASE WHEN EXCLUDED.username <> '' THEN EXCLUDED.username ELSE radius_sessions.username END,
			hostname = CASE WHEN EXCLUDED.hostname <> '' THEN EXCLUDED.hostname ELSE radius_sessions.hostname END,
			vendor_class = CASE WHEN EXCLUDED.vendor_class <> '' THEN EXCLUDED.vendor_class ELSE radius_sessions.vendor_class END,
			called_station_id = CASE WHEN EXCLUDED.called_station_id <> '' THEN EXCLUDED.called_station_id ELSE radius_sessions.called_station_id END,
			calling_station_id = CASE WHEN EXCLUDED.calling_station_id <> '' THEN EXCLUDED.calling_station_id ELSE radius_sessions.calling_station_id END,
			acct_session_id = CASE WHEN EXCLUDED.acct_session_id <> '' THEN EXCLUDED.acct_session_id ELSE radius_sessions.acct_session_id END,
			authorization_result = CASE WHEN EXCLUDED.authorization_result <> '' THEN EXCLUDED.authorization_result ELSE radius_sessions.authorization_result END,
			session_type = CASE WHEN EXCLUDED.session_type <> '' THEN EXCLUDED.session_type ELSE radius_sessions.session_type END,
			status = CASE WHEN EXCLUDED.status <> '' THEN EXCLUDED.status ELSE radius_sessions.status END,
			policy_action = CASE WHEN EXCLUDED.policy_action <> '' THEN EXCLUDED.policy_action ELSE radius_sessions.policy_action END,
			policy_reason = CASE WHEN EXCLUDED.policy_reason <> '' THEN EXCLUDED.policy_reason ELSE radius_sessions.policy_reason END,
			assigned_vlan = CASE WHEN EXCLUDED.assigned_vlan <> '' THEN EXCLUDED.assigned_vlan ELSE radius_sessions.assigned_vlan END,
			started_at = CASE
				WHEN radius_sessions.started_at <= '0002-01-01T00:00:00Z'::timestamptz AND EXCLUDED.started_at IS NOT NULL THEN EXCLUDED.started_at
				WHEN radius_sessions.started_at IS NULL THEN EXCLUDED.started_at
				WHEN EXCLUDED.started_at IS NULL THEN radius_sessions.started_at
				WHEN EXCLUDED.started_at < radius_sessions.started_at THEN EXCLUDED.started_at
				ELSE radius_sessions.started_at
			END,
			last_seen_at = CASE
				WHEN radius_sessions.last_seen_at IS NULL THEN EXCLUDED.last_seen_at
				WHEN EXCLUDED.last_seen_at > radius_sessions.last_seen_at THEN EXCLUDED.last_seen_at
				ELSE radius_sessions.last_seen_at
			END,
			ended_at = CASE
				WHEN EXCLUDED.ended_at IS NOT NULL THEN EXCLUDED.ended_at
				WHEN EXCLUDED.status IN ('authorized', 'started', 'interim-update') THEN NULL
				ELSE radius_sessions.ended_at
			END,
			updated_at = EXCLUDED.updated_at
		RETURNING
			id,
			active_key,
			COALESCE(device_id::text, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(HOST(management_ip), ''),
			COALESCE(port_id, ''),
			COALESCE(nas_port, ''),
			COALESCE(nas_port_id, ''),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(HOST(ip_address), ''),
			COALESCE(mac_address, ''),
			COALESCE(username, ''),
			COALESCE(hostname, ''),
			COALESCE(vendor_class, ''),
			COALESCE(called_station_id, ''),
			COALESCE(calling_station_id, ''),
			COALESCE(acct_session_id, ''),
			COALESCE(authorization_result, ''),
			COALESCE(session_type, ''),
			COALESCE(status, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			COALESCE(assigned_vlan, ''),
			COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			ended_at,
			created_at,
			updated_at
	`

	var out domain.Session
	if err := r.pool.QueryRow(
		ctx,
		query,
		session.ID,
		session.ActiveKey,
		session.DeviceID,
		session.SwitchID,
		session.SwitchName,
		session.ManagementIP,
		session.PortID,
		session.NASPort,
		session.NASPortID,
		session.IfIndex,
		session.InterfaceName,
		session.IPAddress,
		session.MACAddress,
		session.Username,
		session.Hostname,
		session.VendorClass,
		session.CalledStationID,
		session.CallingStationID,
		session.AcctSessionID,
		session.Authorization,
		session.SessionType,
		session.Status,
		session.PolicyAction,
		session.PolicyReason,
		session.AssignedVLAN,
		nullableTime(session.StartedAt),
		nullableTime(session.LastSeenAt),
		nullableTimePtr(session.EndedAt),
		session.CreatedAt,
		session.UpdatedAt,
	).Scan(
		&out.ID,
		&out.ActiveKey,
		&out.DeviceID,
		&out.SwitchID,
		&out.SwitchName,
		&out.ManagementIP,
		&out.PortID,
		&out.NASPort,
		&out.NASPortID,
		&out.IfIndex,
		&out.InterfaceName,
		&out.IPAddress,
		&out.MACAddress,
		&out.Username,
		&out.Hostname,
		&out.VendorClass,
		&out.CalledStationID,
		&out.CallingStationID,
		&out.AcctSessionID,
		&out.Authorization,
		&out.SessionType,
		&out.Status,
		&out.PolicyAction,
		&out.PolicyReason,
		&out.AssignedVLAN,
		&out.StartedAt,
		&out.LastSeenAt,
		&out.EndedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.Session{}, err
	}

	return out, nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullableTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func (r *PostgresRepository) ListRecent(ctx context.Context, limit int) ([]domain.Session, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT
			id,
			active_key,
			COALESCE(device_id::text, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(HOST(management_ip), ''),
			COALESCE(port_id, ''),
			COALESCE(nas_port, ''),
			COALESCE(nas_port_id, ''),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(HOST(ip_address), ''),
			COALESCE(mac_address, ''),
			COALESCE(username, ''),
			COALESCE(hostname, ''),
			COALESCE(vendor_class, ''),
			COALESCE(called_station_id, ''),
			COALESCE(calling_station_id, ''),
			COALESCE(acct_session_id, ''),
			COALESCE(authorization_result, ''),
			COALESCE(session_type, ''),
			COALESCE(status, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			COALESCE(assigned_vlan, ''),
			COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			ended_at,
			created_at,
			updated_at
		FROM radius_sessions
		ORDER BY last_seen_at DESC NULLS LAST, updated_at DESC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSessions(rows)
}

func (r *PostgresRepository) ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]domain.Session, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT
			id,
			active_key,
			COALESCE(device_id::text, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(HOST(management_ip), ''),
			COALESCE(port_id, ''),
			COALESCE(nas_port, ''),
			COALESCE(nas_port_id, ''),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(HOST(ip_address), ''),
			COALESCE(mac_address, ''),
			COALESCE(username, ''),
			COALESCE(hostname, ''),
			COALESCE(vendor_class, ''),
			COALESCE(called_station_id, ''),
			COALESCE(calling_station_id, ''),
			COALESCE(acct_session_id, ''),
			COALESCE(authorization_result, ''),
			COALESCE(session_type, ''),
			COALESCE(status, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			COALESCE(assigned_vlan, ''),
			COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			ended_at,
			created_at,
			updated_at
		FROM radius_sessions
		WHERE UPPER(mac_address) = UPPER($1)
		ORDER BY last_seen_at DESC NULLS LAST, updated_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, macAddress, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSessions(rows)
}

func (r *PostgresRepository) FindByAcctSession(ctx context.Context, macAddress, switchID, acctSessionID string) (*domain.Session, error) {
	query := `
		SELECT
			id,
			active_key,
			COALESCE(device_id::text, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(HOST(management_ip), ''),
			COALESCE(port_id, ''),
			COALESCE(nas_port, ''),
			COALESCE(nas_port_id, ''),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(HOST(ip_address), ''),
			COALESCE(mac_address, ''),
			COALESCE(username, ''),
			COALESCE(hostname, ''),
			COALESCE(vendor_class, ''),
			COALESCE(called_station_id, ''),
			COALESCE(calling_station_id, ''),
			COALESCE(acct_session_id, ''),
			COALESCE(authorization_result, ''),
			COALESCE(session_type, ''),
			COALESCE(status, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			COALESCE(assigned_vlan, ''),
			COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			ended_at,
			created_at,
			updated_at
		FROM radius_sessions
		WHERE UPPER(mac_address) = UPPER($1)
		  AND switch_id = NULLIF($2, '')::uuid
		  AND acct_session_id = $3
		ORDER BY updated_at DESC
		LIMIT 1
	`

	var item domain.Session
	err := r.pool.QueryRow(ctx, query, macAddress, switchID, acctSessionID).Scan(
		&item.ID,
		&item.ActiveKey,
		&item.DeviceID,
		&item.SwitchID,
		&item.SwitchName,
		&item.ManagementIP,
		&item.PortID,
		&item.NASPort,
		&item.NASPortID,
		&item.IfIndex,
		&item.InterfaceName,
		&item.IPAddress,
		&item.MACAddress,
		&item.Username,
		&item.Hostname,
		&item.VendorClass,
		&item.CalledStationID,
		&item.CallingStationID,
		&item.AcctSessionID,
		&item.Authorization,
		&item.SessionType,
		&item.Status,
		&item.PolicyAction,
		&item.PolicyReason,
		&item.AssignedVLAN,
		&item.StartedAt,
		&item.LastSeenAt,
		&item.EndedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *PostgresRepository) FindLatestActiveByMACSwitch(ctx context.Context, macAddress, switchID string) (*domain.Session, error) {
	query := `
		SELECT
			id,
			active_key,
			COALESCE(device_id::text, ''),
			COALESCE(switch_id::text, ''),
			COALESCE(switch_name, ''),
			COALESCE(HOST(management_ip), ''),
			COALESCE(port_id, ''),
			COALESCE(nas_port, ''),
			COALESCE(nas_port_id, ''),
			COALESCE(if_index, 0),
			COALESCE(interface_name, ''),
			COALESCE(HOST(ip_address), ''),
			COALESCE(mac_address, ''),
			COALESCE(username, ''),
			COALESCE(hostname, ''),
			COALESCE(vendor_class, ''),
			COALESCE(called_station_id, ''),
			COALESCE(calling_station_id, ''),
			COALESCE(acct_session_id, ''),
			COALESCE(authorization_result, ''),
			COALESCE(session_type, ''),
			COALESCE(status, ''),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			COALESCE(assigned_vlan, ''),
			COALESCE(started_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			ended_at,
			created_at,
			updated_at
		FROM radius_sessions
		WHERE UPPER(mac_address) = UPPER($1)
		  AND switch_id = NULLIF($2, '')::uuid
		  AND ended_at IS NULL
		  AND status <> 'stopped'
		  AND status <> 'rejected'
		ORDER BY
			CASE WHEN acct_session_id <> '' THEN 0 ELSE 1 END,
			last_seen_at DESC NULLS LAST,
			updated_at DESC
		LIMIT 1
	`

	var item domain.Session
	err := r.pool.QueryRow(ctx, query, macAddress, switchID).Scan(
		&item.ID,
		&item.ActiveKey,
		&item.DeviceID,
		&item.SwitchID,
		&item.SwitchName,
		&item.ManagementIP,
		&item.PortID,
		&item.NASPort,
		&item.NASPortID,
		&item.IfIndex,
		&item.InterfaceName,
		&item.IPAddress,
		&item.MACAddress,
		&item.Username,
		&item.Hostname,
		&item.VendorClass,
		&item.CalledStationID,
		&item.CallingStationID,
		&item.AcctSessionID,
		&item.Authorization,
		&item.SessionType,
		&item.Status,
		&item.PolicyAction,
		&item.PolicyReason,
		&item.AssignedVLAN,
		&item.StartedAt,
		&item.LastSeenAt,
		&item.EndedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *PostgresRepository) PromoteToAccountingKey(ctx context.Context, oldKey, newKey, acctSessionID string) error {
	if oldKey == "" || newKey == "" || oldKey == newKey {
		return nil
	}
	query := `
		UPDATE radius_sessions
		SET active_key = $2,
			acct_session_id = CASE WHEN $3 <> '' THEN $3 ELSE acct_session_id END,
			updated_at = NOW()
		WHERE active_key = $1
	`
	_, err := r.pool.Exec(ctx, query, oldKey, newKey, acctSessionID)
	return err
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanSessions(rows rowScanner) ([]domain.Session, error) {
	var sessions []domain.Session
	for rows.Next() {
		var item domain.Session
		if err := rows.Scan(
			&item.ID,
			&item.ActiveKey,
			&item.DeviceID,
			&item.SwitchID,
			&item.SwitchName,
			&item.ManagementIP,
			&item.PortID,
			&item.NASPort,
			&item.NASPortID,
			&item.IfIndex,
			&item.InterfaceName,
			&item.IPAddress,
			&item.MACAddress,
			&item.Username,
			&item.Hostname,
			&item.VendorClass,
			&item.CalledStationID,
			&item.CallingStationID,
			&item.AcctSessionID,
			&item.Authorization,
			&item.SessionType,
			&item.Status,
			&item.PolicyAction,
			&item.PolicyReason,
			&item.AssignedVLAN,
			&item.StartedAt,
			&item.LastSeenAt,
			&item.EndedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}
