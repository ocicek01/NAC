package policy

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/policy"
)

func (s *Service) EnsureDefaults(ctx context.Context) error {
	policies, err := s.repository.List(ctx, 1, 0)
	if err != nil || len(policies) > 0 {
		return err
	}
	defaults := []CreateInput{
		{Name: "LDAP Unknown Device Quarantine", Description: "LDAP registry match olmayan ve device type bilinmeyen cihazlari quarantine dry-run olarak isaretle", Priority: 20, Enabled: true, MatchConditions: []domain.Condition{{Field: "enrichment_status", Operator: "equals", Value: "not_found"}, {Field: "device_type", Operator: "equals", Value: "unknown"}}, DecisionType: "quarantine", EnforcementAction: "quarantine", DryRun: true},
		{Name: "Camera Port Mismatch", Description: "Kamera profilli portta kamera disi cihazlari restricted olarak isaretle", Priority: 30, Enabled: true, MatchConditions: []domain.Condition{{Field: "port_profile", Operator: "equals", Value: "camera"}, {Field: "device_type", Operator: "not_equals", Value: "camera"}}, DecisionType: "restricted", EnforcementAction: "restrict", DryRun: true},
		{Name: "Known Managed Device", Description: "Kayitli ve guvenilir cihazlari monitor-only olarak isaretle", Priority: 100, Enabled: true, MatchConditions: []domain.Condition{{Field: "ldap_registry_match", Operator: "equals", Value: "true"}, {Field: "trust_score", Operator: "gte", Value: strconv.Itoa(s.config.ThresholdMonitor)}}, DecisionType: "monitor_only", EnforcementAction: "monitor", DryRun: true},
		{Name: "Default Unknown Device", Description: "Varsayilan guvenli fallback policy", Priority: 999, Enabled: true, MatchConditions: []domain.Condition{{Field: "device_id", Operator: "exists", Value: "true"}}, DecisionType: "quarantine", EnforcementAction: "quarantine", DryRun: true},
	}
	for _, item := range defaults {
		if _, err := s.Create(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Evaluate(ctx context.Context, input EvaluationInput) (EvaluationResult, error) {
	startedAt := time.Now().UTC()
	if s == nil || s.repository == nil {
		return EvaluationResult{}, fmt.Errorf("policy repository is not configured")
	}
	if err := s.EnsureDefaults(ctx); err != nil {
		return EvaluationResult{}, err
	}
	_ = s.auditEvent(ctx, input, "policy_evaluation_started", "info", map[string]any{"port_event_id": strings.TrimSpace(input.PortEventID)})
	trustScore, trustSignals := s.calculateTrustScore(input)
	trustLevel := deriveTrustLevel(trustScore)
	_ = s.auditEvent(ctx, input, "trust_score_calculated", "info", map[string]any{"score": trustScore, "signals": trustSignals})
	policies, err := s.repository.ListActive(ctx)
	if err != nil {
		_ = s.auditEvent(ctx, input, "policy_evaluation_failed", "error", map[string]any{"error": err.Error()})
		return EvaluationResult{}, err
	}
	sort.SliceStable(policies, func(i, j int) bool {
		if policies[i].Priority == policies[j].Priority {
			return policies[i].CreatedAt.Before(policies[j].CreatedAt)
		}
		return policies[i].Priority < policies[j].Priority
	})
	matched := make([]string, 0)
	fields := buildEvaluationFields(input, trustScore, trustLevel)
	var selected *domain.Policy
	for i := range policies {
		if !matchesAll(policies[i].MatchConditions, fields) {
			continue
		}
		matched = append(matched, policies[i].Name)
		if selected == nil {
			p := policies[i]
			selected = &p
		}
	}
	result := buildEvaluationResult(s.config, input, startedAt, trustScore, trustSignals, matched, selected)
	auditAction := "policy_default_applied"
	if selected != nil {
		auditAction = "policy_matched"
	} else {
		_ = s.auditEvent(ctx, input, "policy_no_match", "warning", map[string]any{"decision_type": result.DecisionType})
	}
	_ = s.auditEvent(ctx, input, auditAction, "info", map[string]any{"matched_policies": matched, "decision_type": result.DecisionType})
	if strings.TrimSpace(input.DeviceID) != "" {
		trustResult := domain.TrustScoreResult{ID: uuid.NewString(), DeviceID: strings.TrimSpace(input.DeviceID), Score: result.TrustScore, Signals: result.TrustSignals, CalculatedAt: startedAt, CalculationVersion: "v1"}
		if _, err := s.repository.InsertTrustScoreResult(ctx, trustResult); err != nil {
			_ = s.auditEvent(ctx, input, "policy_evaluation_failed", "error", map[string]any{"error": err.Error()})
			return EvaluationResult{}, err
		}
		decision := domain.Decision{ID: uuid.NewString(), DeviceID: strings.TrimSpace(input.DeviceID), PortEventID: strings.TrimSpace(input.PortEventID), PolicyID: result.PolicyID, PolicyName: result.PolicyName, DecisionType: result.DecisionType, TargetVLAN: result.TargetVLAN, EnforcementAction: result.EnforcementAction, TrustScore: result.TrustScore, TrustSignals: result.TrustSignals, ReasonCodes: result.ReasonCodes, Explanation: result.Explanation, DryRun: result.DryRun, EnforcementStatus: enforcementStatus(result), EvaluationDurationMS: result.EvaluationDurationMS, CreatedAt: startedAt}
		stored, err := s.repository.InsertDecision(ctx, decision)
		if err != nil {
			_ = s.auditEvent(ctx, input, "policy_evaluation_failed", "error", map[string]any{"error": err.Error()})
			return EvaluationResult{}, err
		}
		result.DecisionID = stored.ID
		_ = s.auditEvent(ctx, input, "policy_decision_created", "success", map[string]any{"decision_id": result.DecisionID, "decision_type": result.DecisionType, "dry_run": result.DryRun})
	}
	return result, nil
}

func buildEvaluationResult(cfg Config, input EvaluationInput, startedAt time.Time, trustScore int, trustSignals []domain.TrustSignal, matched []string, selected *domain.Policy) EvaluationResult {
	reasonCodes := deriveReasonCodes(input, trustSignals, trustScore)
	result := EvaluationResult{TrustScore: trustScore, TrustSignals: trustSignals, ReasonCodes: uniqueStrings(reasonCodes), MatchedPolicies: matched, EvaluationDurationMS: time.Since(startedAt).Milliseconds()}
	if selected != nil {
		result.PolicyID, result.PolicyName = selected.ID, selected.Name
		result.DecisionType = normalizeDecisionType(selected.DecisionType)
		result.TargetVLAN = selected.TargetVLAN
		if result.TargetVLAN <= 0 && result.DecisionType == "assign_vlan" {
			result.TargetVLAN = input.DefaultVLANID
		}
		result.EnforcementAction = normalizeEnforcementAction(selected.EnforcementAction, selected.DecisionType)
		result.DryRun = selected.DryRun || !cfg.EnforcementEnabled || cfg.DefaultDryRun
		result.Action, result.Status, result.Reason = legacyPolicyResult(result.DecisionType, result.PolicyName)
	} else {
		result.DecisionType, result.TargetVLAN, result.EnforcementAction = defaultDecision(cfg, input, trustScore)
		result.DryRun = !cfg.EnforcementEnabled || cfg.DefaultDryRun
		result.Action, result.Status, result.Reason = legacyPolicyResult(result.DecisionType, "Default policy")
	}
	result.Explanation = buildExplanation(input, trustScore, result.DecisionType, result.ReasonCodes)
	return result
}

func defaultDecision(cfg Config, input EvaluationInput, trustScore int) (string, int, string) {
	if trustScore >= cfg.ThresholdMonitor && (input.LDAPRegistryMatch || strings.TrimSpace(input.OwnerUsername) != "" || knownDeviceType(input.DeviceType)) {
		if input.DefaultVLANID > 0 {
			return "assign_vlan", input.DefaultVLANID, "assign-vlan"
		}
		return "monitor_only", 0, "monitor"
	}
	if trustScore >= cfg.ThresholdRestricted {
		return "restricted", 0, "restrict"
	}
	if trustScore >= cfg.ThresholdRegistration {
		return "registration", 0, "registration"
	}
	return "quarantine", 0, "quarantine"
}
