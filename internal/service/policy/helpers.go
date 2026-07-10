package policy

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	domain "nac/internal/domain/policy"
)

func (s *Service) calculateTrustScore(input EvaluationInput) (int, []domain.TrustSignal) {
	signals := make([]domain.TrustSignal, 0, 8)
	score := s.config.TrustScore.BaseScore
	apply := func(code string, effect int, condition bool) {
		if condition && effect != 0 {
			score += effect
			signals = append(signals, domain.TrustSignal{Code: code, Effect: effect})
		}
	}
	apply("LDAP_REGISTRY_MATCH", s.config.TrustScore.LDAPRegistryMatch, input.LDAPRegistryMatch || strings.EqualFold(strings.TrimSpace(input.EnrichmentStatus), "enriched"))
	apply("OWNER_PRESENT", s.config.TrustScore.RegisteredOwner, strings.TrimSpace(input.OwnerUsername) != "" || strings.TrimSpace(input.RegisteredOwner) != "")
	apply("DEVICE_TYPE_KNOWN", s.config.TrustScore.KnownDeviceType, knownDeviceType(input.DeviceType))
	apply("DEPARTMENT_PRESENT", s.config.TrustScore.DepartmentPresent, strings.TrimSpace(input.OwnerDepartment) != "")
	apply("DEFAULT_VLAN_PRESENT", s.config.TrustScore.DefaultVLANPresent, input.DefaultVLANID > 0)
	apply("STABLE_ATTACHMENT", s.config.TrustScore.StableAttachment, stableAttachment(input))
	apply("LDAP_NOT_FOUND", s.config.TrustScore.LDAPNotFound, strings.EqualFold(strings.TrimSpace(input.EnrichmentStatus), "not_found"))
	apply("DEVICE_TYPE_UNKNOWN", s.config.TrustScore.UnknownDeviceType, !knownDeviceType(input.DeviceType))
	apply("RAPID_PORT_MOVEMENT", s.config.TrustScore.RapidPortMovement, input.RapidPortMovement || input.PortChangeCount >= 3)
	apply("PREVIOUS_QUARANTINE", s.config.TrustScore.PreviousQuarantine, input.PreviousQuarantine || strings.EqualFold(strings.TrimSpace(input.LastEnforcementAction), "blocked"))
	apply("IP_MAC_ANOMALY", s.config.TrustScore.IPMACAnomaly, input.IPMACAnomaly)
	apply("PORT_PROFILE_MISMATCH", s.config.TrustScore.PortProfileMismatch, portProfileMismatch(input))
	apply("ENRICHMENT_REPEATED_FAILURE", s.config.TrustScore.RepeatedEnrichmentError, input.FailedEnrichmentCount > 2 || strings.EqualFold(strings.TrimSpace(input.EnrichmentStatus), "failed"))
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, signals
}

func buildEvaluationFields(input EvaluationInput, trustScore int, trustLevel string) map[string]string {
	return map[string]string{"device_id": strings.TrimSpace(input.DeviceID), "mac_address": strings.TrimSpace(input.MACAddress), "ip_address": strings.TrimSpace(input.IPAddress), "hostname": strings.TrimSpace(input.Hostname), "vendor_class": strings.TrimSpace(input.VendorClass), "switch_id": strings.TrimSpace(input.SwitchID), "switch_name": strings.TrimSpace(input.SwitchName), "switch_management_ip": strings.TrimSpace(input.SwitchManagementIP), "switch_vendor": strings.TrimSpace(input.SwitchVendor), "port_id": strings.TrimSpace(input.PortID), "interface": strings.TrimSpace(input.Interface), "port_profile": strings.TrimSpace(input.PortProfile), "current_vlan": strconv.Itoa(input.CurrentVLAN), "site": strings.TrimSpace(input.Site), "building": strings.TrimSpace(input.Building), "zone": strings.TrimSpace(input.Zone), "device_type": normalizedDeviceType(input.DeviceType), "operating_system": strings.TrimSpace(input.OperatingSystem), "known_device": boolString(input.KnownDevice), "managed_device": boolString(input.ManagedDevice), "enrichment_status": strings.TrimSpace(input.EnrichmentStatus), "enrichment_source": strings.TrimSpace(input.EnrichmentSource), "registered_owner": strings.TrimSpace(input.RegisteredOwner), "owner_username": strings.TrimSpace(input.OwnerUsername), "owner_department": strings.TrimSpace(input.OwnerDepartment), "owner_role": strings.TrimSpace(input.OwnerRole), "assigned_policy": strings.TrimSpace(input.AssignedPolicy), "registered_vendor": strings.TrimSpace(input.RegisteredVendor), "default_vlan_id": strconv.Itoa(input.DefaultVLANID), "ldap_registry_match": boolString(input.LDAPRegistryMatch || strings.EqualFold(strings.TrimSpace(input.EnrichmentStatus), "enriched")), "previous_quarantine": boolString(input.PreviousQuarantine), "port_change_count": strconv.Itoa(input.PortChangeCount), "rapid_port_movement": boolString(input.RapidPortMovement), "ip_mac_anomaly": boolString(input.IPMACAnomaly), "failed_enrichment_count": strconv.Itoa(input.FailedEnrichmentCount), "unknown_device": boolString(input.UnknownDevice), "last_policy_decision": strings.TrimSpace(input.LastPolicyDecision), "last_violation": strings.TrimSpace(input.LastViolation), "last_enforcement_action": strings.TrimSpace(input.LastEnforcementAction), "last_enforcement_status": strings.TrimSpace(input.LastEnforcementStatus), "authentication_method": strings.TrimSpace(input.AuthenticationMethod), "trust_level": trustLevel, "observation_source": strings.TrimSpace(input.ObservationSource), "trust_score": strconv.Itoa(trustScore)}
}

func matchesAll(conditions []domain.Condition, fields map[string]string) bool {
	conditions = normalizeConditions(conditions)
	if len(conditions) == 0 {
		return false
	}
	for _, c := range conditions {
		if !matches(c.Operator, fields[strings.TrimSpace(c.Field)], c.Value) {
			return false
		}
	}
	return true
}
func matches(operator, value, matchValue string) bool {
	operator = strings.ToLower(strings.TrimSpace(operator))
	value = strings.TrimSpace(value)
	matchValue = strings.TrimSpace(matchValue)
	switch operator {
	case "any":
		return true
	case "exists":
		return value != ""
	case "empty":
		return value == ""
	case "equals":
		return strings.EqualFold(value, matchValue)
	case "not_equals":
		return !strings.EqualFold(value, matchValue)
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(matchValue))
	case "in":
		for _, item := range strings.Split(matchValue, ",") {
			if strings.EqualFold(value, strings.TrimSpace(item)) {
				return true
			}
		}
		return false
	case "gte":
		l, le := strconv.Atoi(value)
		r, re := strconv.Atoi(matchValue)
		return le == nil && re == nil && l >= r
	case "lte":
		l, le := strconv.Atoi(value)
		r, re := strconv.Atoi(matchValue)
		return le == nil && re == nil && l <= r
	default:
		return false
	}
}

func deriveReasonCodes(input EvaluationInput, trustSignals []domain.TrustSignal, trustScore int) []string {
	codes := make([]string, 0, 12)
	if input.LDAPRegistryMatch || strings.EqualFold(strings.TrimSpace(input.EnrichmentStatus), "enriched") {
		codes = append(codes, "LDAP_REGISTRY_MATCH")
	} else if strings.EqualFold(strings.TrimSpace(input.EnrichmentStatus), "not_found") {
		codes = append(codes, "LDAP_NOT_FOUND")
	}
	if strings.TrimSpace(input.OwnerUsername) != "" || strings.TrimSpace(input.RegisteredOwner) != "" {
		codes = append(codes, "OWNER_PRESENT", "DEVICE_KNOWN")
	} else {
		codes = append(codes, "DEVICE_UNKNOWN")
	}
	if knownDeviceType(input.DeviceType) {
		codes = append(codes, "DEVICE_TYPE_KNOWN")
	} else {
		codes = append(codes, "DEVICE_TYPE_UNKNOWN")
	}
	if stableAttachment(input) {
		codes = append(codes, "STABLE_ATTACHMENT")
	}
	if portProfileMismatch(input) {
		codes = append(codes, "PORT_PROFILE_MISMATCH")
	} else if strings.TrimSpace(input.PortProfile) != "" {
		codes = append(codes, "PORT_PROFILE_MATCH")
	}
	if input.PreviousQuarantine || strings.EqualFold(strings.TrimSpace(input.LastEnforcementAction), "blocked") {
		codes = append(codes, "PREVIOUS_QUARANTINE")
	}
	if input.RapidPortMovement || input.PortChangeCount >= 3 {
		codes = append(codes, "RAPID_PORT_MOVEMENT")
	}
	if trustScore >= 80 {
		codes = append(codes, "TRUST_SCORE_HIGH")
	} else if trustScore >= 60 {
		codes = append(codes, "TRUST_SCORE_MEDIUM")
	} else {
		codes = append(codes, "TRUST_SCORE_LOW")
	}
	for _, signal := range trustSignals {
		codes = append(codes, signal.Code)
	}
	return uniqueStrings(codes)
}

func buildExplanation(input EvaluationInput, trustScore int, decisionType string, reasonCodes []string) string {
	parts := make([]string, 0, 3)
	if strings.EqualFold(strings.TrimSpace(input.EnrichmentStatus), "not_found") {
		parts = append(parts, "Device was not found in the OpenLDAP device registry")
	} else if strings.EqualFold(strings.TrimSpace(input.EnrichmentStatus), "enriched") {
		parts = append(parts, "Device matched the OpenLDAP device registry")
	}
	if knownDeviceType(input.DeviceType) {
		parts = append(parts, fmt.Sprintf("device type is %s", normalizedDeviceType(input.DeviceType)))
	} else {
		parts = append(parts, "device type is unknown")
	}
	if portProfileMismatch(input) {
		parts = append(parts, "port profile does not match the observed device type")
	}
	if len(parts) == 0 {
		parts = append(parts, "policy evaluation completed with default device metadata")
	}
	return fmt.Sprintf("%s. Trust Score %d produced %s decision. Reason codes: %s.", strings.Join(parts, ", "), trustScore, strings.ReplaceAll(decisionType, "_", " "), strings.Join(uniqueStrings(reasonCodes), ", "))
}

func enforcementStatus(result EvaluationResult) string {
	if result.DryRun {
		return "dry-run"
	}
	return "pending"
}
func deriveTrustLevel(score int) string {
	switch {
	case score >= 80:
		return "high"
	case score >= 60:
		return "medium"
	case score >= 40:
		return "low"
	default:
		return "critical"
	}
}
func stableAttachment(input EvaluationInput) bool {
	return !input.FirstSeenAt.IsZero() && !input.LastSeenAt.IsZero() && strings.TrimSpace(input.SwitchID) != "" && strings.TrimSpace(input.Interface) != "" && input.LastSeenAt.Sub(input.FirstSeenAt) >= 24*time.Hour && input.PortChangeCount < 3
}
func portProfileMismatch(input EvaluationInput) bool {
	profile, deviceType := strings.ToLower(strings.TrimSpace(input.PortProfile)), strings.ToLower(normalizedDeviceType(input.DeviceType))
	if profile == "" || deviceType == "" || deviceType == "unknown" {
		return false
	}
	expected := map[string][]string{"camera": {"camera", "ip-camera"}, "printer": {"printer"}, "access-point": {"access-point", "ap", "wireless-ap"}, "voice": {"phone", "ip-phone", "voice"}}
	allowed, ok := expected[profile]
	if !ok {
		return false
	}
	for _, item := range allowed {
		if deviceType == item {
			return false
		}
	}
	return true
}
func knownDeviceType(value string) bool {
	value = normalizedDeviceType(value)
	return value != "" && value != "unknown"
}
func normalizedDeviceType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	return value
}
func normalizeDecisionType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "allow", "monitor_only", "restricted", "registration", "quarantine", "deny", "shutdown_port", "bounce_port", "assign_vlan":
		return value
	default:
		return "monitor_only"
	}
}
func normalizeEnforcementAction(value, decisionType string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value != "" {
		return value
	}
	switch normalizeDecisionType(decisionType) {
	case "assign_vlan":
		return "assign-vlan"
	case "quarantine":
		return "quarantine"
	case "deny":
		return "deny"
	case "shutdown_port":
		return "shutdown-port"
	case "bounce_port":
		return "bounce-port"
	case "restricted":
		return "restrict"
	case "registration":
		return "registration"
	default:
		return "monitor"
	}
}
func normalizeConfig(cfg Config) Config {
	if cfg.ThresholdAllow <= 0 {
		cfg.ThresholdAllow = 80
	}
	if cfg.ThresholdMonitor <= 0 {
		cfg.ThresholdMonitor = 60
	}
	if cfg.ThresholdRestricted <= 0 {
		cfg.ThresholdRestricted = 40
	}
	if cfg.ThresholdRegistration <= 0 {
		cfg.ThresholdRegistration = 20
	}
	if cfg.TrustScore.BaseScore == 0 {
		cfg.TrustScore.BaseScore = 50
	}
	defaults := map[*int]int{&cfg.TrustScore.LDAPRegistryMatch: 20, &cfg.TrustScore.RegisteredOwner: 10, &cfg.TrustScore.KnownDeviceType: 10, &cfg.TrustScore.DepartmentPresent: 5, &cfg.TrustScore.DefaultVLANPresent: 5, &cfg.TrustScore.StableAttachment: 10, &cfg.TrustScore.LDAPNotFound: -15, &cfg.TrustScore.UnknownDeviceType: -10, &cfg.TrustScore.RapidPortMovement: -20, &cfg.TrustScore.PreviousQuarantine: -20, &cfg.TrustScore.IPMACAnomaly: -25, &cfg.TrustScore.PortProfileMismatch: -15, &cfg.TrustScore.RepeatedEnrichmentError: -10}
	for ptr, fallback := range defaults {
		if *ptr == 0 {
			*ptr = fallback
		}
	}
	return cfg
}
func normalizeConditions(conditions []domain.Condition) []domain.Condition {
	out := make([]domain.Condition, 0, len(conditions))
	for _, item := range conditions {
		item.Field, item.Operator, item.Value = strings.TrimSpace(item.Field), strings.TrimSpace(item.Operator), strings.TrimSpace(item.Value)
		if item.Field != "" && item.Operator != "" {
			out = append(out, item)
		}
	}
	return out
}
func firstConditionField(conditions []domain.Condition) string {
	if len(conditions) == 0 {
		return ""
	}
	return strings.TrimSpace(conditions[0].Field)
}
func firstConditionOperator(conditions []domain.Condition) string {
	if len(conditions) == 0 {
		return ""
	}
	return strings.TrimSpace(conditions[0].Operator)
}
func firstConditionValue(conditions []domain.Condition) string {
	if len(conditions) == 0 {
		return ""
	}
	return strings.TrimSpace(conditions[0].Value)
}
func legacyActionForDecision(decisionType string) string {
	action, _, _ := legacyPolicyResult(decisionType, "")
	return action
}
func legacyPolicyResult(decisionType, reason string) (string, string, string) {
	switch normalizeDecisionType(decisionType) {
	case "allow", "assign_vlan":
		return "active", "active", firstNonEmpty(reason, "Trusted device policy")
	case "monitor_only":
		return "observed", "observed", firstNonEmpty(reason, "Monitor only policy")
	case "restricted":
		return "unknown", "unknown", firstNonEmpty(reason, "Restricted device policy")
	case "registration":
		return "pending", "pending", firstNonEmpty(reason, "Registration required")
	case "quarantine", "deny", "shutdown_port":
		return "blocked", "blocked", firstNonEmpty(reason, "Quarantine policy")
	case "bounce_port":
		return "active", "active", firstNonEmpty(reason, "Bounce port policy")
	default:
		return "unknown", "unknown", firstNonEmpty(reason, "Default policy")
	}
}
func policyStatus(enabled bool) string {
	if enabled {
		return "active"
	}
	return "disabled"
}
func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
func (s *Service) auditEvent(ctx context.Context, input EvaluationInput, action, status string, payload map[string]any) error {
	if s == nil || s.audit == nil {
		return nil
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["device_id"] = strings.TrimSpace(input.DeviceID)
	payload["switch_id"] = strings.TrimSpace(input.SwitchID)
	payload["port_profile"] = strings.TrimSpace(input.PortProfile)
	return s.audit.Record(ctx, action, status, "device", strings.TrimSpace(input.DeviceID), strings.TrimSpace(input.SwitchID), strings.TrimSpace(input.MACAddress), payload)
}
