package switchport

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/switchport"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ReplaceBySwitch(ctx context.Context, switchID string, ports []domain.Port) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `DELETE FROM switch_ports WHERE switch_id = $1`, strings.TrimSpace(switchID)); err != nil {
		return err
	}

	query := `
		INSERT INTO switch_ports (
			id,
			switch_id,
			if_index,
			port_index,
			interface_name,
			interface_alias,
			interface_description,
			port_label,
			interface_type,
			admin_status,
			oper_status,
			status,
			port_mode,
			is_physical,
			is_uplink,
			is_trunk,
			trunk_source,
			vlan_id,
			native_vlan,
			allowed_vlans,
			voice_vlan,
			mac_count,
			mac_addresses,
			speed_bps,
			speed_label,
			duplex,
			poe_enabled,
			poe_power_watts,
			neighbor_protocol,
			neighbor_switch_id,
			neighbor_switch_name,
			neighbor_port_name,
			neighbor_platform,
			neighbor_description,
			neighbor_data,
			metadata,
			last_changed_at,
			last_discovered_at,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23::jsonb, $24, $25, $26, $27, $28::numeric, $29, NULLIF($30, '')::uuid,
			$31, $32, $33, $34, $35::jsonb, $36::jsonb, NULLIF($37, '0001-01-01T00:00:00Z')::timestamptz, $38, $39, $40
		)
	`

	for _, port := range ports {
		macAddresses, err := json.Marshal(port.MACAddresses)
		if err != nil {
			return err
		}
		neighborData, err := json.Marshal(normalizeJSONMap(port.NeighborData))
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(normalizeJSONMap(port.Metadata))
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			ctx,
			query,
			port.ID,
			strings.TrimSpace(switchID),
			port.IfIndex,
			port.PortIndex,
			port.InterfaceName,
			port.InterfaceAlias,
			port.InterfaceDescription,
			port.PortLabel,
			port.InterfaceType,
			port.AdminStatus,
			port.OperStatus,
			port.Status,
			port.PortMode,
			port.IsPhysical,
			port.IsUplink,
			port.IsTrunk,
			port.TrunkSource,
			port.VLANID,
			port.NativeVLAN,
			port.AllowedVLANs,
			port.VoiceVLAN,
			port.MACCount,
			string(macAddresses),
			port.SpeedBPS,
			port.SpeedLabel,
			port.Duplex,
			port.PoEEnabled,
			normalizeNumericText(port.PoEPowerWatts, "0"),
			port.NeighborProtocol,
			port.NeighborSwitchID,
			port.NeighborSwitchName,
			port.NeighborPortName,
			port.NeighborPlatform,
			port.NeighborDescription,
			string(neighborData),
			string(metadata),
			port.LastChangedAt.UTC().Format(timeLayout),
			port.LastDiscoveredAt,
			port.CreatedAt,
			port.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListBySwitch(ctx context.Context, switchID string) ([]domain.Port, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, switch_id, if_index, port_index, interface_name, interface_alias,
			interface_description, port_label, interface_type, admin_status, oper_status,
			status, port_mode, is_physical, is_uplink, is_trunk, trunk_source, vlan_id,
			native_vlan, allowed_vlans, voice_vlan, mac_count, mac_addresses, speed_bps,
			speed_label, duplex, poe_enabled, COALESCE(poe_power_watts::text, '0'),
			neighbor_protocol, COALESCE(neighbor_switch_id::text, ''), neighbor_switch_name,
			neighbor_port_name, neighbor_platform, neighbor_description, neighbor_data,
			metadata, COALESCE(last_changed_at, '0001-01-01T00:00:00Z'::timestamptz),
			last_discovered_at, created_at, updated_at
		FROM switch_ports
		WHERE switch_id = $1
		ORDER BY port_index ASC, if_index ASC
	`, strings.TrimSpace(switchID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPorts(rows)
}

func (r *PostgresRepository) FindBySwitchIfIndex(ctx context.Context, switchID string, ifIndex int) (*domain.Port, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, switch_id, if_index, port_index, interface_name, interface_alias,
			interface_description, port_label, interface_type, admin_status, oper_status,
			status, port_mode, is_physical, is_uplink, is_trunk, trunk_source, vlan_id,
			native_vlan, allowed_vlans, voice_vlan, mac_count, mac_addresses, speed_bps,
			speed_label, duplex, poe_enabled, COALESCE(poe_power_watts::text, '0'),
			neighbor_protocol, COALESCE(neighbor_switch_id::text, ''), neighbor_switch_name,
			neighbor_port_name, neighbor_platform, neighbor_description, neighbor_data,
			metadata, COALESCE(last_changed_at, '0001-01-01T00:00:00Z'::timestamptz),
			last_discovered_at, created_at, updated_at
		FROM switch_ports
		WHERE switch_id = $1 AND if_index = $2
		LIMIT 1
	`, strings.TrimSpace(switchID), ifIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ports, err := scanPorts(rows)
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, nil
	}
	return &ports[0], nil
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanPorts(rows rowScanner) ([]domain.Port, error) {
	result := make([]domain.Port, 0)
	for rows.Next() {
		var port domain.Port
		var macJSON []byte
		var neighborJSON []byte
		var metadataJSON []byte
		var lastChanged time.Time
		if err := rows.Scan(
			&port.ID,
			&port.SwitchID,
			&port.IfIndex,
			&port.PortIndex,
			&port.InterfaceName,
			&port.InterfaceAlias,
			&port.InterfaceDescription,
			&port.PortLabel,
			&port.InterfaceType,
			&port.AdminStatus,
			&port.OperStatus,
			&port.Status,
			&port.PortMode,
			&port.IsPhysical,
			&port.IsUplink,
			&port.IsTrunk,
			&port.TrunkSource,
			&port.VLANID,
			&port.NativeVLAN,
			&port.AllowedVLANs,
			&port.VoiceVLAN,
			&port.MACCount,
			&macJSON,
			&port.SpeedBPS,
			&port.SpeedLabel,
			&port.Duplex,
			&port.PoEEnabled,
			&port.PoEPowerWatts,
			&port.NeighborProtocol,
			&port.NeighborSwitchID,
			&port.NeighborSwitchName,
			&port.NeighborPortName,
			&port.NeighborPlatform,
			&port.NeighborDescription,
			&neighborJSON,
			&metadataJSON,
			&lastChanged,
			&port.LastDiscoveredAt,
			&port.CreatedAt,
			&port.UpdatedAt,
		); err != nil {
			return nil, err
		}

		_ = json.Unmarshal(macJSON, &port.MACAddresses)
		_ = json.Unmarshal(neighborJSON, &port.NeighborData)
		_ = json.Unmarshal(metadataJSON, &port.Metadata)
		if !lastChanged.IsZero() {
			port.LastChangedAt = lastChanged
		}
		result = append(result, port)
	}
	return result, rows.Err()
}

func normalizeJSONMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func normalizeNumericText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
