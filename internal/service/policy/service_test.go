package policy

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "nac/internal/domain/policy"
)

func TestEvaluateRegisteredLDAPDevice(t *testing.T) {
	repo := &stubRepository{policies: []domain.Policy{{ID: "p-allow", Name: "Known Managed Device", Priority: 10, Enabled: true, MatchConditions: []domain.Condition{{Field: "ldap_registry_match", Operator: "equals", Value: "true"}, {Field: "trust_score", Operator: "gte", Value: "60"}}, DecisionType: "monitor_only", EnforcementAction: "monitor", DryRun: true, CreatedAt: time.Now().UTC()}}}
	service := NewService(repo, nil, Config{})

	result, err := service.Evaluate(context.Background(), EvaluationInput{DeviceID: "dev-1", MACAddress: "AA:BB:CC:DD:EE:01", DeviceType: "printer", EnrichmentStatus: "enriched", LDAPRegistryMatch: true, RegisteredOwner: "IT", OwnerUsername: "owner1", OwnerDepartment: "IT", DefaultVLANID: 120, FirstSeenAt: time.Now().UTC().Add(-72 * time.Hour), LastSeenAt: time.Now().UTC(), SwitchID: "sw-1", Interface: "Gi1/0/10"})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if result.TrustScore < 80 {
		t.Fatalf("expected high trust score, got %d", result.TrustScore)
	}
	if result.DecisionType != "monitor_only" {
		t.Fatalf("expected monitor_only decision, got %q", result.DecisionType)
	}
	if len(result.ReasonCodes) == 0 {
		t.Fatalf("expected reason codes")
	}
}

func TestEvaluateLDAPNotFoundUnknownDevice(t *testing.T) {
	service := NewService(&stubRepository{}, nil, Config{})
	result, err := service.Evaluate(context.Background(), EvaluationInput{DeviceID: "dev-2", MACAddress: "AA:BB:CC:DD:EE:02", DeviceType: "unknown", EnrichmentStatus: "not_found"})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if result.DecisionType != "quarantine" {
		t.Fatalf("expected quarantine decision, got %q", result.DecisionType)
	}
	if !result.DryRun {
		t.Fatalf("expected dry-run decision")
	}
	assertContains(t, result.ReasonCodes, "LDAP_NOT_FOUND")
}

func TestEvaluatePortProfileMismatch(t *testing.T) {
	service := NewService(&stubRepository{}, nil, Config{})
	result, err := service.Evaluate(context.Background(), EvaluationInput{DeviceID: "dev-3", MACAddress: "AA:BB:CC:DD:EE:03", DeviceType: "laptop", PortProfile: "camera", EnrichmentStatus: "not_found"})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	assertContains(t, result.ReasonCodes, "PORT_PROFILE_MISMATCH")
	if result.TrustScore >= 50 {
		t.Fatalf("expected trust score penalty for mismatch, got %d", result.TrustScore)
	}
}

func TestEvaluateStableKnownDevice(t *testing.T) {
	service := NewService(&stubRepository{}, nil, Config{})
	result, err := service.Evaluate(context.Background(), EvaluationInput{DeviceID: "dev-4", MACAddress: "AA:BB:CC:DD:EE:04", DeviceType: "access-point", EnrichmentStatus: "enriched", LDAPRegistryMatch: true, OwnerUsername: "netops", OwnerDepartment: "IT", FirstSeenAt: time.Now().UTC().Add(-7 * 24 * time.Hour), LastSeenAt: time.Now().UTC(), SwitchID: "sw-1", Interface: "Gi1/0/24"})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	assertContains(t, result.ReasonCodes, "STABLE_ATTACHMENT")
	if result.TrustScore < 80 {
		t.Fatalf("expected stable known device to have high trust score, got %d", result.TrustScore)
	}
}

func TestEvaluateUsesDefaultPolicyWhenNothingMatches(t *testing.T) {
	repo := &stubRepository{policies: []domain.Policy{{ID: "p-custom", Name: "Only Printer", Priority: 10, Enabled: true, MatchConditions: []domain.Condition{{Field: "device_type", Operator: "equals", Value: "printer"}}, DecisionType: "assign_vlan", TargetVLAN: 200, EnforcementAction: "assign-vlan", DryRun: true, CreatedAt: time.Now().UTC()}}}
	service := NewService(repo, nil, Config{})
	result, err := service.Evaluate(context.Background(), EvaluationInput{DeviceID: "dev-5", MACAddress: "AA:BB:CC:DD:EE:05", DeviceType: "unknown"})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if result.PolicyID != "" {
		t.Fatalf("expected default policy path without matched policy, got %q", result.PolicyID)
	}
	if result.DecisionType == "" {
		t.Fatalf("expected default decision type")
	}
}

func TestEvaluateRepositoryFailure(t *testing.T) {
	repo := &stubRepository{listActiveErr: errors.New("db down")}
	service := NewService(repo, nil, Config{})
	_, err := service.Evaluate(context.Background(), EvaluationInput{DeviceID: "dev-6", MACAddress: "AA:BB:CC:DD:EE:06"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnforcementEnabledHonorsDryRunDefault(t *testing.T) {
	service := NewService(&stubRepository{}, nil, Config{EnforcementEnabled: true, DefaultDryRun: true})
	if service.EnforcementEnabled() {
		t.Fatalf("expected dry-run default to disable real enforcement")
	}
	service = NewService(&stubRepository{}, nil, Config{EnforcementEnabled: true, DefaultDryRun: false})
	if !service.EnforcementEnabled() {
		t.Fatalf("expected enforcement when feature enabled and dry-run disabled")
	}
}

type stubRepository struct {
	policies      []domain.Policy
	decisions     []domain.Decision
	trustResults  []domain.TrustScoreResult
	listActiveErr error
}

func (s *stubRepository) Insert(ctx context.Context, policy domain.Policy) (domain.Policy, error) {
	s.policies = append(s.policies, policy)
	return policy, nil
}
func (s *stubRepository) Update(ctx context.Context, policy domain.Policy) (domain.Policy, error) {
	for i := range s.policies {
		if s.policies[i].ID == policy.ID {
			s.policies[i] = policy
			return policy, nil
		}
	}
	return policy, nil
}
func (s *stubRepository) FindByID(ctx context.Context, id string) (*domain.Policy, error) {
	for i := range s.policies {
		if s.policies[i].ID == id {
			item := s.policies[i]
			return &item, nil
		}
	}
	return nil, nil
}
func (s *stubRepository) List(ctx context.Context, limit, offset int) ([]domain.Policy, error) {
	return append([]domain.Policy{}, s.policies...), nil
}
func (s *stubRepository) ListActive(ctx context.Context) ([]domain.Policy, error) {
	if s.listActiveErr != nil {
		return nil, s.listActiveErr
	}
	if len(s.policies) == 0 {
		return []domain.Policy{}, nil
	}
	out := make([]domain.Policy, 0, len(s.policies))
	for _, item := range s.policies {
		if item.Enabled {
			out = append(out, item)
		}
	}
	return out, nil
}
func (s *stubRepository) Disable(ctx context.Context, id string) error { return nil }
func (s *stubRepository) InsertDecision(ctx context.Context, decision domain.Decision) (domain.Decision, error) {
	s.decisions = append(s.decisions, decision)
	return decision, nil
}
func (s *stubRepository) ListDecisions(ctx context.Context, limit, offset int) ([]domain.Decision, error) {
	return append([]domain.Decision{}, s.decisions...), nil
}
func (s *stubRepository) ListDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]domain.Decision, error) {
	return append([]domain.Decision{}, s.decisions...), nil
}
func (s *stubRepository) InsertTrustScoreResult(ctx context.Context, result domain.TrustScoreResult) (domain.TrustScoreResult, error) {
	s.trustResults = append(s.trustResults, result)
	return result, nil
}

func assertContains(t *testing.T, items []string, target string) {
	t.Helper()
	for _, item := range items {
		if item == target {
			return
		}
	}
	t.Fatalf("expected %q in %v", target, items)
}
