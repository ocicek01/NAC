package device

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	devicedomain "nac/internal/domain/device"
	policydomain "nac/internal/domain/policy"
	policyservice "nac/internal/service/policy"
)

func TestObserveAgentlessEventCreatesPolicyDecisionContext(t *testing.T) {
	repo := &stubDeviceRepository{}
	policies := &capturingPolicyEvaluator{result: policyservice.EvaluationResult{Status: "observed", Action: "observed", Reason: "ok", DecisionID: "pd-1", DecisionType: "monitor_only", DryRun: true, TrustScore: 55}}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, policies, nil, nil, nil, nil, nil, nil, nil, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	device, err := service.ObserveAgentlessEvent(context.Background(), devicedomain.AgentlessObservationInput{
		MACAddress:    "AA:BB:CC:DD:EE:FF",
		SwitchID:      "sw-1",
		SwitchName:    "sw-1",
		IfIndex:       10,
		InterfaceName: "Gig1/0/10",
		ObservedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ObserveAgentlessEvent returned error: %v", err)
	}
	if len(policies.inputs) < 2 {
		t.Fatalf("expected reevaluation with persisted device context, got %d inputs", len(policies.inputs))
	}
	last := policies.inputs[len(policies.inputs)-1]
	if last.DeviceID == "" {
		t.Fatalf("expected reevaluation input device_id to be set")
	}
	if len(repo.policyEvalCalls) != 1 {
		t.Fatalf("expected one persisted policy evaluation update, got %d", len(repo.policyEvalCalls))
	}
	if repo.policyEvalCalls[0].DeviceID != device.ID {
		t.Fatalf("expected policy eval update for %q, got %q", device.ID, repo.policyEvalCalls[0].DeviceID)
	}
}

func TestRefreshPolicyStateAfterEnrichmentPersistsDecisionContext(t *testing.T) {
	repo := &stubDeviceRepository{
		byMAC: map[string][]devicedomain.Device{
			"AA:BB:CC:DD:EE:11": {{
				ID:                   "dev-1",
				MACAddress:           "AA:BB:CC:DD:EE:11",
				CurrentSwitchID:      "sw-1",
				CurrentSwitchName:    "sw-1",
				CurrentInterfaceName: "Gig1/0/11",
				CurrentSourceType:    "snmp",
			}},
		},
		byID: map[string]devicedomain.Device{
			"dev-1": {
				ID:                   "dev-1",
				MACAddress:           "AA:BB:CC:DD:EE:11",
				CurrentSwitchID:      "sw-1",
				CurrentSwitchName:    "sw-1",
				CurrentInterfaceName: "Gig1/0/11",
				CurrentSourceType:    "snmp",
			},
		},
	}
	policies := &capturingPolicyEvaluator{result: policyservice.EvaluationResult{Status: "blocked", Action: "blocked", Reason: "Quarantine policy", DecisionID: "pd-2", DecisionType: "quarantine", DryRun: true, TrustScore: 10}}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, policies, nil, nil, nil, nil, nil, nil, &stubLDAPDeviceResolver{}, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	repo.byMAC["AA:BB:CC:DD:EE:11"][0].EnrichmentStatus = ""
	repo.byID["dev-1"] = repo.byMAC["AA:BB:CC:DD:EE:11"][0]
	if err := service.EnrichDeviceMetadata(context.Background(), "AA:BB:CC:DD:EE:11"); err != nil {
		t.Fatalf("EnrichDeviceMetadata returned error: %v", err)
	}
	if len(repo.policyEvalCalls) != 1 {
		t.Fatalf("expected one persisted policy evaluation update after enrichment, got %d", len(repo.policyEvalCalls))
	}
	if repo.policyEvalCalls[0].DeviceID != "dev-1" {
		t.Fatalf("expected enrichment reevaluation to target dev-1, got %q", repo.policyEvalCalls[0].DeviceID)
	}
	last := policies.inputs[len(policies.inputs)-1]
	if last.DeviceID != "dev-1" {
		t.Fatalf("expected enrichment reevaluation input device_id dev-1, got %q", last.DeviceID)
	}
}

func TestEvaluatePolicyByIDPersistsActionAsLastPolicyDecision(t *testing.T) {
	repo := &stubDeviceRepository{
		byID: map[string]devicedomain.Device{
			"dev-2": {
				ID:                   "dev-2",
				MACAddress:           "00:08:D1:04:E2:23",
				CurrentSwitchID:      "sw-1",
				CurrentSwitchName:    "sw-1",
				CurrentInterfaceName: "Gig1/0/23",
				CurrentSourceType:    "snmp",
				PolicyAction:         "unknown",
				PolicyReason:         "Default Unknown",
				TrustLevel:           "low",
			},
		},
		byMAC: map[string][]devicedomain.Device{
			"00:08:D1:04:E2:23": {{
				ID:                   "dev-2",
				MACAddress:           "00:08:D1:04:E2:23",
				CurrentSwitchID:      "sw-1",
				CurrentSwitchName:    "sw-1",
				CurrentInterfaceName: "Gig1/0/23",
				CurrentSourceType:    "snmp",
				PolicyAction:         "unknown",
				PolicyReason:         "Default Unknown",
				TrustLevel:           "low",
			}},
		},
	}
	policies := &capturingPolicyEvaluator{result: policyservice.EvaluationResult{Status: "restricted", Action: "unknown", Explanation: "Default Unknown", DecisionID: "pd-3", DecisionType: "restricted", PolicyName: "Default Unknown", DryRun: true, TrustScore: 35}}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, policies, nil, nil, nil, nil, nil, nil, nil, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	result, err := service.EvaluatePolicyByID(context.Background(), "dev-2")
	if err != nil {
		t.Fatalf("EvaluatePolicyByID returned error: %v", err)
	}
	if result.DecisionType != "restricted" {
		t.Fatalf("expected decision_type restricted, got %q", result.DecisionType)
	}
	if len(repo.policyEvalCalls) != 1 {
		t.Fatalf("expected one persisted policy evaluation update, got %d", len(repo.policyEvalCalls))
	}
	if repo.policyEvalCalls[0].LastPolicyDecision != "unknown" {
		t.Fatalf("expected last policy decision to persist action fallback, got %q", repo.policyEvalCalls[0].LastPolicyDecision)
	}
}

func (s *stubDeviceRepository) FindByID(ctx context.Context, id string) (*devicedomain.Device, error) {
	if item, ok := s.byID[id]; ok {
		copyItem := item
		return &copyItem, nil
	}
	for _, item := range s.list {
		if item.ID == id {
			copyItem := item
			return &copyItem, nil
		}
	}
	for _, items := range s.byMAC {
		for _, item := range items {
			if item.ID == id {
				copyItem := item
				return &copyItem, nil
			}
		}
	}
	return nil, nil
}

func (s *stubDeviceRepository) UpdatePolicyEvaluationByID(ctx context.Context, deviceID, status, policyAction, policyReason, trustLevel, lastPolicyDecision string, evaluatedAt time.Time) error {
	s.policyEvalCalls = append(s.policyEvalCalls, policyEvalCall{DeviceID: deviceID, Status: status, PolicyAction: policyAction, PolicyReason: policyReason, TrustLevel: trustLevel, LastPolicyDecision: lastPolicyDecision})
	if s.byID == nil {
		return nil
	}
	item, ok := s.byID[deviceID]
	if !ok {
		return nil
	}
	item.Status = status
	item.PolicyAction = policyAction
	item.PolicyReason = policyReason
	item.TrustLevel = trustLevel
	item.LastPolicyDecision = lastPolicyDecision
	item.LastPolicyEvaluatedAt = evaluatedAt
	s.byID[deviceID] = item
	if items, ok := s.byMAC[item.MACAddress]; ok && len(items) > 0 {
		items[0] = item
		s.byMAC[item.MACAddress] = items
	}
	return nil
}

type capturingPolicyEvaluator struct {
	result policyservice.EvaluationResult
	inputs []policyservice.EvaluationInput
}

func (c *capturingPolicyEvaluator) EnsureDefaults(ctx context.Context) error { return nil }
func (c *capturingPolicyEvaluator) Evaluate(ctx context.Context, input policyservice.EvaluationInput) (policyservice.EvaluationResult, error) {
	c.inputs = append(c.inputs, input)
	return c.result, nil
}
func (c *capturingPolicyEvaluator) ListDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]policydomain.Decision, error) {
	return nil, nil
}
func (c *capturingPolicyEvaluator) EnforcementEnabled() bool { return false }
