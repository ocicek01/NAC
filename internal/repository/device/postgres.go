package device

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/device"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

var deviceSelectColumns = `
		id,
		mac_address,
		COALESCE((
			SELECT dobs.ip_address::text
			FROM device_observations dobs
			WHERE dobs.device_id = devices.id
			  AND dobs.ip_address IS NOT NULL
			ORDER BY dobs.observed_at DESC, dobs.created_at DESC
			LIMIT 1
		), (
			SELECT COALESCE(HOST(de.your_ip), HOST(de.requested_ip), HOST(de.client_ip), '')
			FROM dhcp_events de
			WHERE ` + normalizedMACExpression("de.mac_address") + ` = ` + normalizedMACExpression("devices.mac_address") + `
			  AND (de.your_ip IS NOT NULL OR de.requested_ip IS NOT NULL OR de.client_ip IS NOT NULL)
			ORDER BY de.observed_at DESC, de.created_at DESC
			LIMIT 1
		), (
			SELECT HOST(rs.ip_address)
			FROM radius_sessions rs
			WHERE ` + normalizedMACExpression("rs.mac_address") + ` = ` + normalizedMACExpression("devices.mac_address") + `
			  AND rs.ip_address IS NOT NULL
			ORDER BY rs.last_seen_at DESC, rs.updated_at DESC
			LIMIT 1
		), (
			SELECT HOST(mib.ip_address)
			FROM mac_ip_bindings mib
			WHERE ` + normalizedMACExpression("mib.mac_address") + ` = ` + normalizedMACExpression("devices.mac_address") + `
			ORDER BY mib.last_seen_at DESC, mib.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE(device_type, 'unknown'),
		COALESCE(registered_vendor, ''),
		COALESCE(label, ''),
		COALESCE(description, ''),
		hostname,
		vendor_class,
		status,
		COALESCE(approved_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(approved_by, ''),
		COALESCE(expires_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(policy_action, ''),
		COALESCE(policy_reason, ''),
		COALESCE(classification_method, ''),
		COALESCE(trust_level, ''),
		COALESCE(authentication_method, ''),
		COALESCE(authentication_status, ''),
		COALESCE(sophos_username, ''),
		COALESCE(sophos_last_ip::text, ''),
		COALESCE(sophos_last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(last_policy_decision, ''),
		COALESCE(last_policy_evaluated_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(current_switch_id::text, ''),
		COALESCE(current_switch_name, ''),
		COALESCE(current_management_ip::text, ''),
		COALESCE(current_port_id::text, ''),
		COALESCE(current_bridge_port, 0),
		COALESCE(current_if_index, 0),
		COALESCE(current_interface_name, ''),
		COALESCE(current_interface_description, ''),
		COALESCE(current_source_type, ''),
		COALESCE(current_confidence, ''),
		COALESCE((
			SELECT identity_type
			FROM device_identity_snapshots dis
			WHERE dis.device_id = devices.id
			ORDER BY dis.verified_at DESC, dis.created_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT identity_source
			FROM device_identity_snapshots dis
			WHERE dis.device_id = devices.id
			ORDER BY dis.verified_at DESC, dis.created_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT username
			FROM device_identity_snapshots dis
			WHERE dis.device_id = devices.id
			ORDER BY dis.verified_at DESC, dis.created_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT full_name
			FROM device_identity_snapshots dis
			WHERE dis.device_id = devices.id
			ORDER BY dis.verified_at DESC, dis.created_at DESC
			LIMIT 1
		), ''),
		COALESCE(registered_owner, ''),
		COALESCE(owner_username, ''),
		COALESCE(owner_department, ''),
		COALESCE(owner_role, ''),
		COALESCE(default_vlan_id, 0),
		COALESCE(default_vlan_name, ''),
		COALESCE(assigned_policy, ''),
		COALESCE(enrichment_source, ''),
		COALESCE(enrichment_status, ''),
		COALESCE(enrichment_error, ''),
		COALESCE(enriched_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE((
			SELECT policy_action
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT target_vlan
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), 0),
		COALESCE((
			SELECT status
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT es.switch_id::text
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT es.if_index
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), 0),
		COALESCE((
			SELECT COALESCE(es.last_success_at, es.last_attempt_at, es.updated_at)
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE((
			SELECT desired_state
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT applied_state
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT applied_vlan
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), 0),
		COALESCE((
			SELECT last_method
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT last_attempt_at
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE((
			SELECT last_success_at
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE((
			SELECT last_error
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT retry_count
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), 0),
		COALESCE((
			SELECT ip_learning_status
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), ''),
		COALESCE((
			SELECT ip_learning_started_at
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE((
			SELECT ip_learned_at
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE((
			SELECT last_bounce_at
			FROM enforcement_state es
			WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
			  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
			  AND es.if_index = COALESCE(devices.current_if_index, 0)
			ORDER BY es.updated_at DESC
			LIMIT 1
		), '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(first_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
		created_at,
		updated_at
`

var deviceListColumns = `
		id,
		mac_address,
		'',
		COALESCE(device_type, 'unknown'),
		COALESCE(registered_vendor, ''),
		COALESCE(label, ''),
		COALESCE(description, ''),
		COALESCE(hostname, ''),
		COALESCE(vendor_class, ''),
		COALESCE(status, ''),
		COALESCE(approved_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(approved_by, ''),
		COALESCE(expires_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(policy_action, ''),
		COALESCE(policy_reason, ''),
		COALESCE(classification_method, ''),
		COALESCE(trust_level, ''),
		COALESCE(authentication_method, ''),
		COALESCE(authentication_status, ''),
		COALESCE(sophos_username, ''),
		COALESCE(sophos_last_ip::text, ''),
		COALESCE(sophos_last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(last_policy_decision, ''),
		COALESCE(last_policy_evaluated_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(current_switch_id::text, ''),
		COALESCE(current_switch_name, ''),
		COALESCE(current_management_ip::text, ''),
		COALESCE(current_port_id::text, ''),
		COALESCE(current_bridge_port, 0),
		COALESCE(current_if_index, 0),
		COALESCE(current_interface_name, ''),
		COALESCE(current_interface_description, ''),
		COALESCE(current_source_type, ''),
		COALESCE(current_confidence, ''),
		'',
		'',
		'',
		'',
		COALESCE(registered_owner, ''),
		COALESCE(owner_username, ''),
		COALESCE(owner_department, ''),
		COALESCE(owner_role, ''),
		COALESCE(default_vlan_id, 0),
		COALESCE(default_vlan_name, ''),
		COALESCE(assigned_policy, ''),
		COALESCE(enrichment_source, ''),
		COALESCE(enrichment_status, ''),
		COALESCE(enrichment_error, ''),
		COALESCE(enriched_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(last_enforcement_action, ''),
		COALESCE(last_enforcement_vlan, 0),
		COALESCE(last_enforcement_status, ''),
		COALESCE(last_enforcement_switch_id::text, ''),
		COALESCE(last_enforcement_if_index, 0),
		COALESCE(last_enforcement_at, '0001-01-01T00:00:00Z'::timestamptz),
		'',
		'',
		0,
		COALESCE(last_enforcement_method, ''),
		'0001-01-01T00:00:00Z'::timestamptz,
		'0001-01-01T00:00:00Z'::timestamptz,
		'',
		0,
		'',
		'0001-01-01T00:00:00Z'::timestamptz,
		'0001-01-01T00:00:00Z'::timestamptz,
		'0001-01-01T00:00:00Z'::timestamptz,
		COALESCE(first_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
		COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
		created_at,
		updated_at
`

func scanDeviceList(scanner interface {
	Scan(dest ...any) error
}) (domain.Device, error) {
	return scanDevice(scanner)
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func scanDevice(scanner interface {
	Scan(dest ...any) error
}) (domain.Device, error) {
	var item domain.Device
	err := scanner.Scan(
		&item.ID,
		&item.MACAddress,
		&item.CurrentIPAddress,
		&item.DeviceType,
		&item.RegisteredVendor,
		&item.Label,
		&item.Description,
		&item.Hostname,
		&item.VendorClass,
		&item.Status,
		&item.ApprovedAt,
		&item.ApprovedBy,
		&item.ExpiresAt,
		&item.PolicyAction,
		&item.PolicyReason,
		&item.ClassificationMethod,
		&item.TrustLevel,
		&item.AuthenticationMethod,
		&item.AuthenticationStatus,
		&item.SophosUsername,
		&item.SophosLastIP,
		&item.SophosLastSeenAt,
		&item.LastPolicyDecision,
		&item.LastPolicyEvaluatedAt,
		&item.CurrentSwitchID,
		&item.CurrentSwitchName,
		&item.CurrentManagementIP,
		&item.CurrentPortID,
		&item.CurrentBridgePort,
		&item.CurrentIfIndex,
		&item.CurrentInterfaceName,
		&item.CurrentInterfaceDescription,
		&item.CurrentSourceType,
		&item.CurrentConfidence,
		&item.IdentityType,
		&item.IdentitySource,
		&item.IdentityUsername,
		&item.IdentityFullName,
		&item.RegisteredOwner,
		&item.OwnerUsername,
		&item.OwnerDepartment,
		&item.OwnerRole,
		&item.DefaultVLANID,
		&item.DefaultVLANName,
		&item.AssignedPolicy,
		&item.EnrichmentSource,
		&item.EnrichmentStatus,
		&item.EnrichmentError,
		&item.EnrichedAt,
		&item.LastEnforcementAction,
		&item.LastEnforcementVLAN,
		&item.LastEnforcementStatus,
		&item.LastEnforcementSwitchID,
		&item.LastEnforcementIfIndex,
		&item.LastEnforcementAt,
		&item.DesiredEnforcementState,
		&item.AppliedEnforcementState,
		&item.AppliedEnforcementVLAN,
		&item.LastEnforcementMethod,
		&item.LastEnforcementAttemptAt,
		&item.LastEnforcementSuccessAt,
		&item.LastEnforcementError,
		&item.LastEnforcementRetryCount,
		&item.IPLearningStatus,
		&item.IPLearningStartedAt,
		&item.IPLearnedAt,
		&item.LastPortBounceAt,
		&item.FirstSeenAt,
		&item.LastSeenAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func normalizedMACExpression(expression string) string {
	return "regexp_replace(upper(" + expression + "), '[^0-9A-F]', '', 'g')"
}

func (r *PostgresRepository) Upsert(ctx context.Context, device domain.Device) (domain.Device, error) {
	query := `
		INSERT INTO devices (
			id,
			mac_address,
			device_type,
			label,
			description,
			hostname,
			vendor_class,
			status,
			approved_at,
			approved_by,
			expires_at,
			policy_action,
			policy_reason,
			classification_method,
			trust_level,
			authentication_method,
			authentication_status,
			sophos_username,
			sophos_last_ip,
			sophos_last_seen_at,
			last_policy_decision,
			last_policy_evaluated_at,
			current_switch_id,
			current_switch_name,
			current_management_ip,
			current_bridge_port,
			current_if_index,
			current_interface_name,
			current_interface_description,
			current_source_type,
			current_confidence,
			first_seen_at,
			last_seen_at,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9::timestamptz, $10, $11::timestamptz,
			$12, $13, $14, $15, $16, $17, $18, NULLIF($19, '')::inet, $20::timestamptz,
			$21, $22::timestamptz,
			NULLIF($23, '')::uuid, $24, NULLIF($25, '')::inet, $26, $27, $28,
			$29, $30, $31, $32, $33, $34, $35
		)
		ON CONFLICT (mac_address) DO UPDATE
		SET device_type = CASE
				WHEN EXCLUDED.device_type <> '' THEN EXCLUDED.device_type
				ELSE devices.device_type
			END,
			label = CASE
				WHEN EXCLUDED.label <> '' THEN EXCLUDED.label
				ELSE devices.label
			END,
			description = CASE
				WHEN EXCLUDED.description <> '' THEN EXCLUDED.description
				ELSE devices.description
			END,
			hostname = CASE
				WHEN EXCLUDED.hostname <> '' THEN EXCLUDED.hostname
				ELSE devices.hostname
			END,
			vendor_class = CASE
				WHEN EXCLUDED.vendor_class <> '' THEN EXCLUDED.vendor_class
				ELSE devices.vendor_class
			END,
			status = EXCLUDED.status,
			approved_at = COALESCE(EXCLUDED.approved_at, devices.approved_at),
			approved_by = CASE
				WHEN EXCLUDED.approved_by <> '' THEN EXCLUDED.approved_by
				ELSE devices.approved_by
			END,
			expires_at = COALESCE(EXCLUDED.expires_at, devices.expires_at),
			policy_action = EXCLUDED.policy_action,
			policy_reason = EXCLUDED.policy_reason,
			classification_method = EXCLUDED.classification_method,
			trust_level = EXCLUDED.trust_level,
			authentication_method = EXCLUDED.authentication_method,
			authentication_status = EXCLUDED.authentication_status,
			sophos_username = CASE
				WHEN EXCLUDED.sophos_username <> '' THEN EXCLUDED.sophos_username
				ELSE devices.sophos_username
			END,
			sophos_last_ip = COALESCE(EXCLUDED.sophos_last_ip, devices.sophos_last_ip),
			sophos_last_seen_at = COALESCE(EXCLUDED.sophos_last_seen_at, devices.sophos_last_seen_at),
			last_policy_decision = EXCLUDED.last_policy_decision,
			last_policy_evaluated_at = COALESCE(EXCLUDED.last_policy_evaluated_at, devices.last_policy_evaluated_at),
			current_switch_id = EXCLUDED.current_switch_id,
			current_switch_name = EXCLUDED.current_switch_name,
			current_management_ip = EXCLUDED.current_management_ip,
			current_bridge_port = EXCLUDED.current_bridge_port,
			current_if_index = EXCLUDED.current_if_index,
			current_interface_name = EXCLUDED.current_interface_name,
			current_interface_description = EXCLUDED.current_interface_description,
			current_source_type = EXCLUDED.current_source_type,
			current_confidence = EXCLUDED.current_confidence,
			first_seen_at = COALESCE(devices.first_seen_at, EXCLUDED.first_seen_at),
			last_seen_at = CASE
				WHEN devices.last_seen_at IS NULL THEN EXCLUDED.last_seen_at
				WHEN EXCLUDED.last_seen_at > devices.last_seen_at THEN EXCLUDED.last_seen_at
				ELSE devices.last_seen_at
			END,
			updated_at = EXCLUDED.updated_at
		RETURNING
			id,
			mac_address,
			COALESCE((
				SELECT dobs.ip_address::text
				FROM device_observations dobs
				WHERE dobs.device_id = devices.id
				  AND dobs.ip_address IS NOT NULL
				ORDER BY dobs.observed_at DESC, dobs.created_at DESC
				LIMIT 1
			), (
				SELECT COALESCE(HOST(de.your_ip), HOST(de.requested_ip), HOST(de.client_ip), '')
				FROM dhcp_events de
				WHERE ` + normalizedMACExpression("de.mac_address") + ` = ` + normalizedMACExpression("devices.mac_address") + `
				  AND (de.your_ip IS NOT NULL OR de.requested_ip IS NOT NULL OR de.client_ip IS NOT NULL)
				ORDER BY de.observed_at DESC, de.created_at DESC
				LIMIT 1
			), (
				SELECT HOST(rs.ip_address)
				FROM radius_sessions rs
				WHERE ` + normalizedMACExpression("rs.mac_address") + ` = ` + normalizedMACExpression("devices.mac_address") + `
				  AND rs.ip_address IS NOT NULL
				ORDER BY rs.last_seen_at DESC, rs.updated_at DESC
				LIMIT 1
			), (
				SELECT HOST(mib.ip_address)
				FROM mac_ip_bindings mib
				WHERE ` + normalizedMACExpression("mib.mac_address") + ` = ` + normalizedMACExpression("devices.mac_address") + `
				ORDER BY mib.last_seen_at DESC, mib.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE(device_type, 'unknown'),
			COALESCE(label, ''),
			COALESCE(description, ''),
			hostname,
			vendor_class,
			status,
			COALESCE(approved_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(approved_by, ''),
			COALESCE(expires_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(policy_action, ''),
			COALESCE(policy_reason, ''),
			COALESCE(classification_method, ''),
			COALESCE(trust_level, ''),
			COALESCE(authentication_method, ''),
			COALESCE(authentication_status, ''),
			COALESCE(sophos_username, ''),
			COALESCE(sophos_last_ip::text, ''),
			COALESCE(sophos_last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_policy_decision, ''),
			COALESCE(last_policy_evaluated_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(current_switch_id::text, ''),
			COALESCE(current_switch_name, ''),
			COALESCE(current_management_ip::text, ''),
			COALESCE(current_port_id::text, ''),
			COALESCE(current_bridge_port, 0),
			COALESCE(current_if_index, 0),
			COALESCE(current_interface_name, ''),
			COALESCE(current_interface_description, ''),
			COALESCE(current_source_type, ''),
			COALESCE(current_confidence, ''),
			COALESCE((
				SELECT identity_type
				FROM device_identity_snapshots dis
				WHERE dis.device_id = devices.id
				ORDER BY dis.verified_at DESC, dis.created_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT identity_source
				FROM device_identity_snapshots dis
				WHERE dis.device_id = devices.id
				ORDER BY dis.verified_at DESC, dis.created_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT username
				FROM device_identity_snapshots dis
				WHERE dis.device_id = devices.id
				ORDER BY dis.verified_at DESC, dis.created_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT full_name
				FROM device_identity_snapshots dis
				WHERE dis.device_id = devices.id
				ORDER BY dis.verified_at DESC, dis.created_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT policy_action
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT target_vlan
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), 0),
			COALESCE((
				SELECT status
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT es.switch_id::text
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT es.if_index
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), 0),
			COALESCE((
				SELECT COALESCE(es.last_success_at, es.last_attempt_at, es.updated_at)
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE((
				SELECT desired_state
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT applied_state
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT applied_vlan
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), 0),
			COALESCE((
				SELECT last_method
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT last_attempt_at
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE((
				SELECT last_success_at
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE((
				SELECT last_error
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT retry_count
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), 0),
			COALESCE((
				SELECT ip_learning_status
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE((
				SELECT ip_learning_started_at
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE((
				SELECT ip_learned_at
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE((
				SELECT last_bounce_at
				FROM enforcement_state es
				WHERE UPPER(es.mac_address) = UPPER(devices.mac_address)
				  AND COALESCE(es.switch_id::text, '') = COALESCE(devices.current_switch_id::text, '')
				  AND es.if_index = COALESCE(devices.current_if_index, 0)
				ORDER BY es.updated_at DESC
				LIMIT 1
			), '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(first_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			COALESCE(last_seen_at, '0001-01-01T00:00:00Z'::timestamptz),
			created_at,
			updated_at
	`

	out, err := scanDevice(r.pool.QueryRow(
		ctx,
		query,
		device.ID,
		device.MACAddress,
		device.DeviceType,
		device.Label,
		device.Description,
		device.Hostname,
		device.VendorClass,
		device.Status,
		nullableTime(device.ApprovedAt),
		device.ApprovedBy,
		nullableTime(device.ExpiresAt),
		device.PolicyAction,
		device.PolicyReason,
		device.ClassificationMethod,
		device.TrustLevel,
		device.AuthenticationMethod,
		device.AuthenticationStatus,
		device.SophosUsername,
		device.SophosLastIP,
		nullableTime(device.SophosLastSeenAt),
		device.LastPolicyDecision,
		nullableTime(device.LastPolicyEvaluatedAt),
		device.CurrentSwitchID,
		device.CurrentSwitchName,
		device.CurrentManagementIP,
		device.CurrentBridgePort,
		device.CurrentIfIndex,
		device.CurrentInterfaceName,
		device.CurrentInterfaceDescription,
		device.CurrentSourceType,
		device.CurrentConfidence,
		device.FirstSeenAt,
		device.LastSeenAt,
		device.CreatedAt,
		device.UpdatedAt,
	))
	if err != nil {
		return domain.Device{}, err
	}

	return out, nil
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]domain.Device, error) {
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
		SELECT ` + deviceListColumns + `
		FROM devices
		ORDER BY last_seen_at DESC NULLS LAST, created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := make([]domain.Device, 0, limit)
	for rows.Next() {
		item, err := scanDeviceList(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

func (r *PostgresRepository) ListByMAC(ctx context.Context, macAddress string) ([]domain.Device, error) {
	query := `
		SELECT ` + deviceSelectColumns + `
		FROM devices
		WHERE ` + normalizedMACExpression("mac_address") + ` = ` + normalizedMACExpression("$1") + `
		ORDER BY last_seen_at DESC NULLS LAST, created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, macAddress)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		item, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

func (r *PostgresRepository) ListBySwitch(ctx context.Context, switchID string) ([]domain.Device, error) {
	query := `
		SELECT ` + deviceSelectColumns + `
		FROM devices
		WHERE current_switch_id = NULLIF($1, '')::uuid
		ORDER BY current_if_index ASC, last_seen_at DESC NULLS LAST, created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, switchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		item, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

func (r *PostgresRepository) ListBySwitchAndIfIndex(ctx context.Context, switchID string, ifIndex int) ([]domain.Device, error) {
	query := `
		SELECT ` + deviceSelectColumns + `
		FROM devices
		WHERE current_switch_id = NULLIF($1, '')::uuid
		  AND current_if_index = $2
		ORDER BY last_seen_at DESC NULLS LAST, created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, switchID, ifIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		item, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, macAddress, status, approvedBy, policyAction, policyReason string, approvedAt, expiresAt time.Time) (domain.Device, error) {
	query := `
		UPDATE devices
		SET status = $2,
			approved_by = CASE
				WHEN $3 <> '' THEN $3
				ELSE approved_by
			END,
			approved_at = CASE
				WHEN $4::timestamptz IS NOT NULL THEN $4::timestamptz
				ELSE approved_at
			END,
			expires_at = CASE
				WHEN $5::timestamptz IS NOT NULL THEN $5::timestamptz
				ELSE expires_at
			END,
			policy_action = $6,
			policy_reason = $7,
			updated_at = NOW()
		WHERE ` + normalizedMACExpression("mac_address") + ` = ` + normalizedMACExpression("$1") + `
		RETURNING ` + deviceSelectColumns

	out, err := scanDevice(r.pool.QueryRow(
		ctx,
		query,
		macAddress,
		status,
		approvedBy,
		nullableTime(approvedAt),
		nullableTime(expiresAt),
		strings.TrimSpace(policyAction),
		strings.TrimSpace(policyReason),
	))
	if err != nil {
		return domain.Device{}, err
	}
	return out, nil
}

func (r *PostgresRepository) AddIdentitySnapshot(ctx context.Context, snapshot domain.IdentitySnapshot) (domain.IdentitySnapshot, error) {
	query := `
		INSERT INTO device_identity_snapshots (
			id,
			device_id,
			identity_type,
			identity_source,
			external_id,
			username,
			full_name,
			attributes_json,
			verified_at,
			expires_at,
			created_at
		)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9, $10::timestamptz, $11)
		RETURNING id, device_id::text, identity_type, identity_source, external_id, username, full_name, attributes_json, verified_at, COALESCE(expires_at, '0001-01-01T00:00:00Z'::timestamptz), created_at
	`

	attrs, err := json.Marshal(snapshot.Attributes)
	if err != nil {
		return domain.IdentitySnapshot{}, err
	}
	if len(attrs) == 0 {
		attrs = []byte(`{}`)
	}

	var (
		out      domain.IdentitySnapshot
		rawAttrs []byte
	)
	if err := r.pool.QueryRow(
		ctx,
		query,
		snapshot.ID,
		snapshot.DeviceID,
		snapshot.IdentityType,
		snapshot.IdentitySource,
		snapshot.ExternalID,
		snapshot.Username,
		snapshot.FullName,
		string(attrs),
		snapshot.VerifiedAt,
		nullableTime(snapshot.ExpiresAt),
		snapshot.CreatedAt,
	).Scan(
		&out.ID,
		&out.DeviceID,
		&out.IdentityType,
		&out.IdentitySource,
		&out.ExternalID,
		&out.Username,
		&out.FullName,
		&rawAttrs,
		&out.VerifiedAt,
		&out.ExpiresAt,
		&out.CreatedAt,
	); err != nil {
		return domain.IdentitySnapshot{}, err
	}

	if len(rawAttrs) > 0 {
		_ = json.Unmarshal(rawAttrs, &out.Attributes)
	}
	if out.Attributes == nil {
		out.Attributes = map[string]any{}
	}

	return out, nil
}

func (r *PostgresRepository) AddObservation(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO device_observations (
			id, device_id, mac_address, ip_address, switch_id, port_ifindex, vlan_id, source, observed_at, created_at
		)
		VALUES ($1, NULLIF($2, '')::uuid, $3, NULLIF($4, '')::inet, NULLIF($5, '')::uuid, $6, $7, $8, $9, $10)
	`, observation.ID, observation.DeviceID, observation.MACAddress, observation.IPAddress, observation.SwitchID, observation.PortIfIndex, observation.VLANID, observation.Source, observation.ObservedAt, observation.CreatedAt)
	if err != nil {
		return domain.Observation{}, err
	}
	return observation, nil
}

func (r *PostgresRepository) UpdateEnrichment(ctx context.Context, update domain.EnrichmentUpdate) (domain.Device, error) {
	query := "UPDATE devices\n" +
		"SET device_type = CASE WHEN $2 <> '' THEN $2 ELSE device_type END,\n" +
		"    registered_vendor = CASE WHEN $3 <> '' THEN $3 ELSE registered_vendor END,\n" +
		"    description = CASE WHEN $4 <> '' THEN $4 ELSE description END,\n" +
		"    registered_owner = CASE WHEN $5 <> '' THEN $5 ELSE registered_owner END,\n" +
		"    owner_username = CASE WHEN $6 <> '' THEN $6 ELSE owner_username END,\n" +
		"    owner_department = CASE WHEN $7 <> '' THEN $7 ELSE owner_department END,\n" +
		"    owner_role = CASE WHEN $8 <> '' THEN $8 ELSE owner_role END,\n" +
		"    default_vlan_id = $9,\n" +
		"    default_vlan_name = CASE WHEN $10 <> '' THEN $10 ELSE default_vlan_name END,\n" +
		"    assigned_policy = CASE WHEN $11 <> '' THEN $11 ELSE assigned_policy END,\n" +
		"    trust_level = CASE WHEN $12 <> '' THEN $12 ELSE trust_level END,\n" +
		"    enrichment_source = $13,\n" +
		"    enrichment_status = $14,\n" +
		"    enrichment_error = $15,\n" +
		"    enriched_at = $16,\n" +
		"    policy_action = $17,\n" +
		"    policy_reason = $18,\n" +
		"    last_policy_decision = $19,\n" +
		"    last_policy_evaluated_at = $20,\n" +
		"    status = $21,\n" +
		"    classification_method = CASE WHEN $22 <> '' THEN $22 ELSE classification_method END,\n" +
		"    updated_at = NOW()\n" +
		"WHERE " + normalizedMACExpression("mac_address") + " = " + normalizedMACExpression("$1") + "\n" +
		"RETURNING " + deviceSelectColumns

	return scanDevice(r.pool.QueryRow(
		ctx,
		query,
		update.MACAddress,
		strings.TrimSpace(update.DeviceType),
		strings.TrimSpace(update.RegisteredVendor),
		strings.TrimSpace(update.Description),
		strings.TrimSpace(update.RegisteredOwner),
		strings.TrimSpace(update.OwnerUsername),
		strings.TrimSpace(update.OwnerDepartment),
		strings.TrimSpace(update.OwnerRole),
		update.DefaultVLANID,
		strings.TrimSpace(update.DefaultVLANName),
		strings.TrimSpace(update.AssignedPolicy),
		strings.TrimSpace(update.TrustLevel),
		strings.TrimSpace(update.EnrichmentSource),
		strings.TrimSpace(update.EnrichmentStatus),
		strings.TrimSpace(update.EnrichmentError),
		nullableTime(update.EnrichedAt),
		strings.TrimSpace(update.PolicyAction),
		strings.TrimSpace(update.PolicyReason),
		strings.TrimSpace(update.LastPolicyDecision),
		nullableTime(update.LastPolicyEvaluatedAt),
		strings.TrimSpace(update.Status),
		strings.TrimSpace(update.ClassificationMethod),
	))
}

func (r *PostgresRepository) UpdateSophosIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error {
	query := "UPDATE devices\n" +
		"SET sophos_username = $2,\n" +
		"    sophos_last_ip = NULLIF($3, '')::inet,\n" +
		"    sophos_last_seen_at = $4,\n" +
		"    authentication_status = CASE WHEN $2 <> '' THEN 'captive-authenticated' ELSE authentication_status END,\n" +
		"    updated_at = NOW()\n" +
		"WHERE " + normalizedMACExpression("mac_address") + " = " + normalizedMACExpression("$1")

	_, err := r.pool.Exec(ctx, query, macAddress, strings.TrimSpace(username), strings.TrimSpace(ipAddress), nullableTime(seenAt))
	return err
}

func (r *PostgresRepository) UpdateEnforcementState(ctx context.Context, macAddress, action string, vlanID int, status, switchID string, ifIndex int, method string, enforcedAt time.Time) error {
	query := `
		UPDATE devices
		SET last_enforcement_action = $2,
			last_enforcement_vlan = $3,
			last_enforcement_status = $4,
			last_enforcement_switch_id = NULLIF($5, '')::uuid,
			last_enforcement_if_index = $6,
			last_enforcement_at = $7,
			last_enforcement_method = $8,
			updated_at = NOW()
		WHERE ` + normalizedMACExpression("mac_address") + ` = ` + normalizedMACExpression("$1") + `
	`

	_, err := r.pool.Exec(ctx, query, macAddress, action, vlanID, status, switchID, ifIndex, enforcedAt, method)
	return err
}

func (r *PostgresRepository) UpdateIPLearningState(ctx context.Context, macAddress, switchID string, ifIndex int, state string, startedAt, learnedAt, lastBounceAt time.Time) error {
	query := `
		UPDATE enforcement_state
		SET ip_learning_status = $4,
			ip_learning_started_at = CASE
				WHEN $5::timestamptz IS NOT NULL THEN $5::timestamptz
				ELSE ip_learning_started_at
			END,
			ip_learned_at = CASE
				WHEN $6::timestamptz IS NOT NULL THEN $6::timestamptz
				ELSE ip_learned_at
			END,
			last_bounce_at = CASE
				WHEN $7::timestamptz IS NOT NULL THEN $7::timestamptz
				ELSE last_bounce_at
			END,
			updated_at = NOW()
		WHERE ` + normalizedMACExpression("mac_address") + ` = ` + normalizedMACExpression("$1") + `
		  AND switch_id = NULLIF($2, '')::uuid
		  AND if_index = $3
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		macAddress,
		switchID,
		ifIndex,
		strings.TrimSpace(state),
		nullableTime(startedAt),
		nullableTime(learnedAt),
		nullableTime(lastBounceAt),
	)
	return err
}
