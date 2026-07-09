package portevent

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/portevent"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, event domain.Event) (domain.Event, error) {
	metadata, err := json.Marshal(normalizeMetadata(event.Metadata))
	if err != nil {
		return domain.Event{}, err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO port_events (
			id, switch_id, switch_name, management_ip, if_index, interface_name, interface_description,
			admin_status, oper_status, event_type, event_source, mac_address, ip_address, hostname,
			vendor_class, device_type, policy_action, policy_reason, enforcement_action, trust_level,
			metadata, observed_at, created_at
		)
		VALUES (
			$1, NULLIF($2, '')::uuid, $3, NULLIF($4, '')::inet, $5, $6, $7,
			$8, $9, $10, $11, $12, NULLIF($13, '')::inet, $14,
			$15, $16, $17, $18, $19, $20,
			$21::jsonb, $22, $23
		)
	`, event.ID, event.SwitchID, event.SwitchName, event.ManagementIP, event.IfIndex, event.InterfaceName, event.InterfaceDescription, event.AdminStatus, event.OperStatus, event.EventType, event.EventSource, event.MACAddress, event.IPAddress, event.Hostname, event.VendorClass, event.DeviceType, event.PolicyAction, event.PolicyReason, event.EnforcementAction, event.TrustLevel, string(metadata), event.ObservedAt, event.CreatedAt)
	if err != nil {
		return domain.Event{}, err
	}

	event.Metadata = normalizeMetadata(event.Metadata)
	return event, nil
}

func (r *PostgresRepository) ListRecent(ctx context.Context, limit int) ([]domain.Event, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, switch_id::text, switch_name, COALESCE(HOST(management_ip), ''), if_index, interface_name,
			interface_description, admin_status, oper_status, event_type, event_source, mac_address,
			COALESCE(HOST(ip_address), ''), hostname, vendor_class, device_type, policy_action,
			policy_reason, enforcement_action, trust_level, metadata, observed_at, created_at
		FROM port_events
		ORDER BY observed_at DESC, created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Event, 0, limit)
	for rows.Next() {
		var item domain.Event
		var metadata []byte
		if err := rows.Scan(
			&item.ID,
			&item.SwitchID,
			&item.SwitchName,
			&item.ManagementIP,
			&item.IfIndex,
			&item.InterfaceName,
			&item.InterfaceDescription,
			&item.AdminStatus,
			&item.OperStatus,
			&item.EventType,
			&item.EventSource,
			&item.MACAddress,
			&item.IPAddress,
			&item.Hostname,
			&item.VendorClass,
			&item.DeviceType,
			&item.PolicyAction,
			&item.PolicyReason,
			&item.EnforcementAction,
			&item.TrustLevel,
			&metadata,
			&item.ObservedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &item.Metadata)
		}
		item.Metadata = normalizeMetadata(item.Metadata)
		result = append(result, item)
	}

	return result, rows.Err()
}

func normalizeMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	return metadata
}
