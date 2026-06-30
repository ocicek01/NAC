package switchasset

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/switchasset"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, asset domain.Switch) (domain.Switch, error) {
	query := `
		INSERT INTO switches (
			id,
			name,
			system_name,
			aliases,
			base_mac,
			management_ip,
			routing_switch_id,
			vendor,
			model,
			status,
			radius_secret,
			ssh_username,
			ssh_password,
			ssh_port,
			snmp_version,
			snmp_community,
			snmp_port,
			snmp_timeout_ms,
			snmp_retries,
			supports_radius_vlan,
			supports_coa,
			supports_ssh_enforcement,
			supports_snmp_write,
			last_error,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::inet, NULLIF($7, '')::uuid, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		asset.ID,
		asset.Name,
		asset.SystemName,
		strings.Join(asset.Aliases, ","),
		asset.BaseMAC,
		asset.ManagementIP,
		asset.RoutingSwitchID,
		asset.Vendor,
		asset.Model,
		asset.Status,
		asset.RadiusSecret,
		asset.SSHUsername,
		asset.SSHPassword,
		asset.SSHPort,
		asset.SNMPVersion,
		asset.SNMPCommunity,
		asset.SNMPPort,
		asset.SNMPTimeoutMS,
		asset.SNMPRetries,
		asset.SupportsRadiusVLAN,
		asset.SupportsCoA,
		asset.SupportsSSHEnforcement,
		asset.SupportsSNMPWrite,
		asset.LastError,
		asset.CreatedAt,
		asset.UpdatedAt,
	)
	if err != nil {
		return domain.Switch{}, err
	}

	return asset, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]domain.Switch, error) {
	query := `
		SELECT
			id,
			name,
			system_name,
			aliases,
			base_mac,
			host(management_ip),
			COALESCE(routing_switch_id::text, ''),
			vendor,
			model,
			status,
			radius_secret,
			COALESCE(ssh_username, ''),
			COALESCE(ssh_password, ''),
			COALESCE(ssh_port, 22),
			snmp_version,
			snmp_community,
			snmp_port,
			snmp_timeout_ms,
			snmp_retries,
			COALESCE(supports_radius_vlan, false),
			COALESCE(supports_coa, false),
			COALESCE(supports_ssh_enforcement, false),
			COALESCE(supports_snmp_write, false),
			COALESCE(last_polled_at, '0001-01-01T00:00:00Z'::timestamptz),
			last_error,
			created_at,
			updated_at
		FROM switches
		ORDER BY name ASC, management_ip ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Switch
	for rows.Next() {
		var asset domain.Switch
		var aliases string
		if err := rows.Scan(
			&asset.ID,
			&asset.Name,
			&asset.SystemName,
			&aliases,
			&asset.BaseMAC,
			&asset.ManagementIP,
			&asset.RoutingSwitchID,
			&asset.Vendor,
			&asset.Model,
			&asset.Status,
			&asset.RadiusSecret,
			&asset.SSHUsername,
			&asset.SSHPassword,
			&asset.SSHPort,
			&asset.SNMPVersion,
			&asset.SNMPCommunity,
			&asset.SNMPPort,
			&asset.SNMPTimeoutMS,
			&asset.SNMPRetries,
			&asset.SupportsRadiusVLAN,
			&asset.SupportsCoA,
			&asset.SupportsSSHEnforcement,
			&asset.SupportsSNMPWrite,
			&asset.LastPolledAt,
			&asset.LastError,
			&asset.CreatedAt,
			&asset.UpdatedAt,
		); err != nil {
			return nil, err
		}
		asset.Aliases = splitAliases(aliases)
		result = append(result, asset)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *PostgresRepository) ListEnabledSNMP(ctx context.Context) ([]domain.Switch, error) {
	query := `
		SELECT
			id,
			name,
			system_name,
			aliases,
			base_mac,
			host(management_ip),
			COALESCE(routing_switch_id::text, ''),
			vendor,
			model,
			status,
			radius_secret,
			COALESCE(ssh_username, ''),
			COALESCE(ssh_password, ''),
			COALESCE(ssh_port, 22),
			snmp_version,
			snmp_community,
			snmp_port,
			snmp_timeout_ms,
			snmp_retries,
			COALESCE(supports_radius_vlan, false),
			COALESCE(supports_coa, false),
			COALESCE(supports_ssh_enforcement, false),
			COALESCE(supports_snmp_write, false),
			COALESCE(last_polled_at, '0001-01-01T00:00:00Z'::timestamptz),
			last_error,
			created_at,
			updated_at
		FROM switches
		WHERE snmp_community <> ''
		ORDER BY name ASC, management_ip ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Switch
	for rows.Next() {
		var asset domain.Switch
		var aliases string
		if err := rows.Scan(
			&asset.ID,
			&asset.Name,
			&asset.SystemName,
			&aliases,
			&asset.BaseMAC,
			&asset.ManagementIP,
			&asset.RoutingSwitchID,
			&asset.Vendor,
			&asset.Model,
			&asset.Status,
			&asset.RadiusSecret,
			&asset.SSHUsername,
			&asset.SSHPassword,
			&asset.SSHPort,
			&asset.SNMPVersion,
			&asset.SNMPCommunity,
			&asset.SNMPPort,
			&asset.SNMPTimeoutMS,
			&asset.SNMPRetries,
			&asset.SupportsRadiusVLAN,
			&asset.SupportsCoA,
			&asset.SupportsSSHEnforcement,
			&asset.SupportsSNMPWrite,
			&asset.LastPolledAt,
			&asset.LastError,
			&asset.CreatedAt,
			&asset.UpdatedAt,
		); err != nil {
			return nil, err
		}
		asset.Aliases = splitAliases(aliases)
		result = append(result, asset)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *PostgresRepository) FindByName(ctx context.Context, name string) (*domain.Switch, error) {
	query := `
		SELECT
			id,
			name,
			system_name,
			aliases,
			base_mac,
			host(management_ip),
			COALESCE(routing_switch_id::text, ''),
			vendor,
			model,
			status,
			radius_secret,
			COALESCE(ssh_username, ''),
			COALESCE(ssh_password, ''),
			COALESCE(ssh_port, 22),
			snmp_version,
			snmp_community,
			snmp_port,
			snmp_timeout_ms,
			snmp_retries,
			COALESCE(supports_radius_vlan, false),
			COALESCE(supports_coa, false),
			COALESCE(supports_ssh_enforcement, false),
			COALESCE(supports_snmp_write, false),
			COALESCE(last_polled_at, '0001-01-01T00:00:00Z'::timestamptz),
			last_error,
			created_at,
			updated_at
		FROM switches
		WHERE LOWER(name) = LOWER($1)
		LIMIT 1
	`

	var asset domain.Switch
	var aliases string
	if err := r.pool.QueryRow(ctx, query, name).Scan(
		&asset.ID,
		&asset.Name,
		&asset.SystemName,
		&aliases,
		&asset.BaseMAC,
		&asset.ManagementIP,
		&asset.RoutingSwitchID,
		&asset.Vendor,
		&asset.Model,
		&asset.Status,
		&asset.RadiusSecret,
		&asset.SSHUsername,
		&asset.SSHPassword,
		&asset.SSHPort,
		&asset.SNMPVersion,
		&asset.SNMPCommunity,
		&asset.SNMPPort,
		&asset.SNMPTimeoutMS,
		&asset.SNMPRetries,
		&asset.SupportsRadiusVLAN,
		&asset.SupportsCoA,
		&asset.SupportsSSHEnforcement,
		&asset.SupportsSNMPWrite,
		&asset.LastPolledAt,
		&asset.LastError,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	asset.Aliases = splitAliases(aliases)

	return &asset, nil
}

func (r *PostgresRepository) FindByManagementIP(ctx context.Context, managementIP string) (*domain.Switch, error) {
	normalized := strings.TrimSpace(managementIP)
	if normalized == "" {
		return nil, nil
	}
	if idx := strings.IndexByte(normalized, '/'); idx > 0 {
		normalized = normalized[:idx]
	}

	query := `
		SELECT
			id,
			name,
			system_name,
			aliases,
			base_mac,
			host(management_ip),
			COALESCE(routing_switch_id::text, ''),
			vendor,
			model,
			status,
			radius_secret,
			COALESCE(ssh_username, ''),
			COALESCE(ssh_password, ''),
			COALESCE(ssh_port, 22),
			snmp_version,
			snmp_community,
			snmp_port,
			snmp_timeout_ms,
			snmp_retries,
			COALESCE(supports_radius_vlan, false),
			COALESCE(supports_coa, false),
			COALESCE(supports_ssh_enforcement, false),
			COALESCE(supports_snmp_write, false),
			COALESCE(last_polled_at, '0001-01-01T00:00:00Z'::timestamptz),
			last_error,
			created_at,
			updated_at
		FROM switches
		WHERE host(management_ip) = $1
		LIMIT 1
	`

	var asset domain.Switch
	var aliases string
	if err := r.pool.QueryRow(ctx, query, normalized).Scan(
		&asset.ID,
		&asset.Name,
		&asset.SystemName,
		&aliases,
		&asset.BaseMAC,
		&asset.ManagementIP,
		&asset.RoutingSwitchID,
		&asset.Vendor,
		&asset.Model,
		&asset.Status,
		&asset.RadiusSecret,
		&asset.SSHUsername,
		&asset.SSHPassword,
		&asset.SSHPort,
		&asset.SNMPVersion,
		&asset.SNMPCommunity,
		&asset.SNMPPort,
		&asset.SNMPTimeoutMS,
		&asset.SNMPRetries,
		&asset.SupportsRadiusVLAN,
		&asset.SupportsCoA,
		&asset.SupportsSSHEnforcement,
		&asset.SupportsSNMPWrite,
		&asset.LastPolledAt,
		&asset.LastError,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	asset.Aliases = splitAliases(aliases)

	return &asset, nil
}

func (r *PostgresRepository) FindByBaseMAC(ctx context.Context, macAddress string) (*domain.Switch, error) {
	normalized := normalizeMAC(macAddress)
	if normalized == "" {
		return nil, nil
	}

	query := `
		SELECT
			id,
			name,
			system_name,
			aliases,
			base_mac,
			host(management_ip),
			COALESCE(routing_switch_id::text, ''),
			vendor,
			model,
			status,
			radius_secret,
			COALESCE(ssh_username, ''),
			COALESCE(ssh_password, ''),
			COALESCE(ssh_port, 22),
			snmp_version,
			snmp_community,
			snmp_port,
			snmp_timeout_ms,
			snmp_retries,
			COALESCE(supports_radius_vlan, false),
			COALESCE(supports_coa, false),
			COALESCE(supports_ssh_enforcement, false),
			COALESCE(supports_snmp_write, false),
			COALESCE(last_polled_at, '0001-01-01T00:00:00Z'::timestamptz),
			last_error,
			created_at,
			updated_at
		FROM switches
		WHERE LOWER(base_mac) = LOWER($1)
		LIMIT 1
	`

	var asset domain.Switch
	var aliases string
	if err := r.pool.QueryRow(ctx, query, normalized).Scan(
		&asset.ID,
		&asset.Name,
		&asset.SystemName,
		&aliases,
		&asset.BaseMAC,
		&asset.ManagementIP,
		&asset.RoutingSwitchID,
		&asset.Vendor,
		&asset.Model,
		&asset.Status,
		&asset.RadiusSecret,
		&asset.SSHUsername,
		&asset.SSHPassword,
		&asset.SSHPort,
		&asset.SNMPVersion,
		&asset.SNMPCommunity,
		&asset.SNMPPort,
		&asset.SNMPTimeoutMS,
		&asset.SNMPRetries,
		&asset.SupportsRadiusVLAN,
		&asset.SupportsCoA,
		&asset.SupportsSSHEnforcement,
		&asset.SupportsSNMPWrite,
		&asset.LastPolledAt,
		&asset.LastError,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	asset.Aliases = splitAliases(aliases)

	return &asset, nil
}

func (r *PostgresRepository) FindByNeighborName(ctx context.Context, name string) (*domain.Switch, error) {
	normalized := normalizeSwitchName(name)
	if normalized == "" {
		return nil, nil
	}

	assets, err := r.ListEnabledSNMP(ctx)
	if err != nil {
		return nil, err
	}

	var matched *domain.Switch
	for _, asset := range assets {
		candidates := []string{asset.Name, asset.SystemName}
		candidates = append(candidates, asset.Aliases...)
		for _, candidate := range candidates {
			if normalizeSwitchName(candidate) == normalized {
				if matched != nil {
					// Ambiguous alias matches are treated as unresolved to avoid
					// linking a neighbor to the wrong switch.
					return nil, nil
				}
				copyAsset := asset
				matched = &copyAsset
				break
			}
		}
	}

	return matched, nil
}

func (r *PostgresRepository) UpdateIdentity(ctx context.Context, id, systemName, baseMAC string, aliases []string) (domain.Switch, error) {
	query := `
		UPDATE switches
		SET system_name = $2,
		    base_mac = $3,
		    aliases = $4,
		    updated_at = NOW()
		WHERE id = $1
	`

	if _, err := r.pool.Exec(ctx, query, id, systemName, baseMAC, strings.Join(aliases, ",")); err != nil {
		return domain.Switch{}, err
	}

	asset, err := r.findByID(ctx, id)
	if err != nil {
		return domain.Switch{}, err
	}
	return *asset, nil
}

func (r *PostgresRepository) UpdateRoutingSwitch(ctx context.Context, id, routingSwitchID string) (domain.Switch, error) {
	query := `
		UPDATE switches
		SET routing_switch_id = NULLIF($2, '')::uuid,
		    updated_at = NOW()
		WHERE id = $1
	`

	if _, err := r.pool.Exec(ctx, query, id, strings.TrimSpace(routingSwitchID)); err != nil {
		return domain.Switch{}, err
	}

	asset, err := r.findByID(ctx, id)
	if err != nil {
		return domain.Switch{}, err
	}
	return *asset, nil
}

func (r *PostgresRepository) UpdateRadiusSecret(ctx context.Context, id, radiusSecret string) (domain.Switch, error) {
	query := `
		UPDATE switches
		SET radius_secret = $2,
		    updated_at = NOW()
		WHERE id = $1
	`

	if _, err := r.pool.Exec(ctx, query, id, strings.TrimSpace(radiusSecret)); err != nil {
		return domain.Switch{}, err
	}

	asset, err := r.findByID(ctx, id)
	if err != nil {
		return domain.Switch{}, err
	}
	return *asset, nil
}

func (r *PostgresRepository) UpdateSSHConfig(ctx context.Context, id, username, password string, port int) (domain.Switch, error) {
	query := `
		UPDATE switches
		SET ssh_username = $2,
		    ssh_password = $3,
		    ssh_port = $4,
		    updated_at = NOW()
		WHERE id = $1
	`

	if port <= 0 {
		port = 22
	}

	if _, err := r.pool.Exec(ctx, query, id, strings.TrimSpace(username), strings.TrimSpace(password), port); err != nil {
		return domain.Switch{}, err
	}

	asset, err := r.findByID(ctx, id)
	if err != nil {
		return domain.Switch{}, err
	}
	return *asset, nil
}

func (r *PostgresRepository) UpdatePollStatus(ctx context.Context, id string, polledAt time.Time, lastError string) (domain.Switch, error) {
	query := `
		UPDATE switches
		SET last_polled_at = $2,
		    last_error = $3,
		    updated_at = NOW()
		WHERE id = $1
	`

	if _, err := r.pool.Exec(ctx, query, strings.TrimSpace(id), polledAt, strings.TrimSpace(lastError)); err != nil {
		return domain.Switch{}, err
	}

	asset, err := r.findByID(ctx, id)
	if err != nil {
		return domain.Switch{}, err
	}
	return *asset, nil
}

func (r *PostgresRepository) findByID(ctx context.Context, id string) (*domain.Switch, error) {
	query := `
		SELECT
			id,
			name,
			system_name,
			aliases,
			base_mac,
			host(management_ip),
			COALESCE(routing_switch_id::text, ''),
			vendor,
			model,
			status,
			radius_secret,
			COALESCE(ssh_username, ''),
			COALESCE(ssh_password, ''),
			COALESCE(ssh_port, 22),
			snmp_version,
			snmp_community,
			snmp_port,
			snmp_timeout_ms,
			snmp_retries,
			COALESCE(supports_radius_vlan, false),
			COALESCE(supports_coa, false),
			COALESCE(supports_ssh_enforcement, false),
			COALESCE(supports_snmp_write, false),
			COALESCE(last_polled_at, '0001-01-01T00:00:00Z'::timestamptz),
			last_error,
			created_at,
			updated_at
		FROM switches
		WHERE id = $1
		LIMIT 1
	`

	var asset domain.Switch
	var aliases string
	if err := r.pool.QueryRow(ctx, query, id).Scan(
		&asset.ID,
		&asset.Name,
		&asset.SystemName,
		&aliases,
		&asset.BaseMAC,
		&asset.ManagementIP,
		&asset.RoutingSwitchID,
		&asset.Vendor,
		&asset.Model,
		&asset.Status,
		&asset.RadiusSecret,
		&asset.SSHUsername,
		&asset.SSHPassword,
		&asset.SSHPort,
		&asset.SNMPVersion,
		&asset.SNMPCommunity,
		&asset.SNMPPort,
		&asset.SNMPTimeoutMS,
		&asset.SNMPRetries,
		&asset.SupportsRadiusVLAN,
		&asset.SupportsCoA,
		&asset.SupportsSSHEnforcement,
		&asset.SupportsSNMPWrite,
		&asset.LastPolledAt,
		&asset.LastError,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		return nil, err
	}
	asset.Aliases = splitAliases(aliases)
	return &asset, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*domain.Switch, error) {
	return r.findByID(ctx, id)
}

func splitAliases(value string) []string {
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

func normalizeSwitchName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if idx := strings.IndexByte(value, '.'); idx > 0 {
		value = value[:idx]
	}
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(value)
}

func normalizeMAC(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}

	value = strings.ReplaceAll(value, "-", ":")
	return value
}
