package enforcement

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"nac/internal/config"
	devicedomain "nac/internal/domain/device"
	domain "nac/internal/domain/enforcement"
	policydomain "nac/internal/domain/policy"
	sessiondomain "nac/internal/domain/session"
	switchasset "nac/internal/domain/switchasset"
	switchportdomain "nac/internal/domain/switchport"
)

func TestEnforcePolicyDecisionDryRunCreatesSkippedRequest(t *testing.T) {
	repo := &stubEnforcementRepository{}
	service := NewService(repo, stubSwitchResolver{item: &switchasset.Switch{ID: "sw-1", Name: "sw-1", SupportsSNMPWrite: true, SNMPCommunity: "public", ManagementIP: "10.0.0.1", SNMPPort: 161, SNMPTimeoutMS: 1000}}, nil, config.RadiusConfig{})
	service.ConfigurePhase31(config.EnforcementConfig{Mode: domain.ModeDryRun, AdapterPriority: []string{"mock"}, MockAdapterEnabled: true}, stubPolicyDecisionResolver{item: &policydomain.Decision{ID: "pd-1", DeviceID: "dev-1", PolicyName: "Restricted", DecisionType: "restricted", EnforcementAction: "restrict", TargetVLAN: 300}}, stubDeviceResolver{item: &devicedomain.Device{ID: "dev-1", MACAddress: "AA:BB", CurrentSwitchID: "sw-1", CurrentIfIndex: 10, CurrentInterfaceName: "10"}}, stubPortResolver{item: &switchportdomain.Port{ID: "port-1", SwitchID: "sw-1", IfIndex: 10, InterfaceName: "10", VLANID: 100}}, stubAudit{})

	request, err := service.EnforcePolicyDecision(context.Background(), "pd-1", domain.RequestInput{})
	if err != nil {
		t.Fatalf("EnforcePolicyDecision returned error: %v", err)
	}
	if request.Status != domain.RequestStatusSkipped {
		t.Fatalf("expected skipped request, got %q", request.Status)
	}
	if len(repo.results) != 1 {
		t.Fatalf("expected one skipped result, got %d", len(repo.results))
	}
}

func TestEnforcePolicyDecisionProtectedPortIsSkipped(t *testing.T) {
	repo := &stubEnforcementRepository{}
	service := NewService(repo, stubSwitchResolver{item: &switchasset.Switch{ID: "sw-1", Name: "sw-1", SupportsSNMPWrite: true, SNMPCommunity: "public", ManagementIP: "10.0.0.1", SNMPPort: 161, SNMPTimeoutMS: 1000}}, nil, config.RadiusConfig{})
	service.ConfigurePhase31(config.EnforcementConfig{Mode: domain.ModePilot, AdapterPriority: []string{"mock"}, MockAdapterEnabled: true, AllowedSwitches: []string{"sw-1"}, AllowedPorts: []string{"port-1"}}, stubPolicyDecisionResolver{item: &policydomain.Decision{ID: "pd-1", DeviceID: "dev-1", PolicyName: "Restricted", DecisionType: "restricted", EnforcementAction: "restrict", TargetVLAN: 300}}, stubDeviceResolver{item: &devicedomain.Device{ID: "dev-1", MACAddress: "AA:BB", CurrentSwitchID: "sw-1", CurrentIfIndex: 10, CurrentInterfaceName: "10"}}, stubPortResolver{item: &switchportdomain.Port{ID: "port-1", SwitchID: "sw-1", IfIndex: 10, InterfaceName: "10", VLANID: 100, EnforcementProtected: true}}, stubAudit{})

	request, err := service.EnforcePolicyDecision(context.Background(), "pd-1", domain.RequestInput{})
	if err != nil {
		t.Fatalf("EnforcePolicyDecision returned error: %v", err)
	}
	if request.ErrorCode != "protected_port" {
		t.Fatalf("expected protected_port error code, got %q", request.ErrorCode)
	}
}

func TestProcessNextRequestSucceedsWithMockAdapter(t *testing.T) {
	repo := &stubEnforcementRepository{}
	service := NewService(repo, stubSwitchResolver{item: &switchasset.Switch{ID: "sw-1", Name: "sw-1", SupportsSNMPWrite: true, SNMPCommunity: "public", ManagementIP: "10.0.0.1", SNMPPort: 161, SNMPTimeoutMS: 1000}}, nil, config.RadiusConfig{})
	service.ConfigurePhase31(config.EnforcementConfig{Mode: domain.ModeEnabled, AdapterPriority: []string{"mock"}, MockAdapterEnabled: true}, stubPolicyDecisionResolver{item: &policydomain.Decision{ID: "pd-1", DeviceID: "dev-1", PolicyName: "Restricted", DecisionType: "restricted", EnforcementAction: "restrict", TargetVLAN: 300}}, stubDeviceResolver{item: &devicedomain.Device{ID: "dev-1", MACAddress: "AA:BB", CurrentSwitchID: "sw-1", CurrentIfIndex: 10, CurrentInterfaceName: "10"}}, stubPortResolver{item: &switchportdomain.Port{ID: "port-1", SwitchID: "sw-1", IfIndex: 10, InterfaceName: "10", VLANID: 100}}, stubAudit{})

	request, err := service.EnforcePolicyDecision(context.Background(), "pd-1", domain.RequestInput{})
	if err != nil {
		t.Fatalf("EnforcePolicyDecision returned error: %v", err)
	}
	if request.Status != domain.RequestStatusPending {
		t.Fatalf("expected pending request, got %q", request.Status)
	}
	outcome, err := service.ProcessNextRequest(context.Background())
	if err != nil {
		t.Fatalf("ProcessNextRequest returned error: %v", err)
	}
	if outcome == nil || outcome.Request.Status != domain.RequestStatusSucceeded {
		t.Fatalf("expected succeeded outcome, got %+v", outcome)
	}
}

func TestEnforcePolicyDecisionReturnsActiveDuplicate(t *testing.T) {
	repo := &stubEnforcementRepository{}
	service := NewService(repo, stubSwitchResolver{item: &switchasset.Switch{ID: "sw-1", Name: "sw-1", SupportsSNMPWrite: true, SNMPCommunity: "public", ManagementIP: "10.0.0.1", SNMPPort: 161, SNMPTimeoutMS: 1000}}, nil, config.RadiusConfig{})
	service.ConfigurePhase31(config.EnforcementConfig{Mode: domain.ModeEnabled, AdapterPriority: []string{"mock"}, MockAdapterEnabled: true}, stubPolicyDecisionResolver{item: &policydomain.Decision{ID: "pd-1", DeviceID: "dev-1", PolicyName: "Restricted", DecisionType: "restricted", EnforcementAction: "restrict", TargetVLAN: 300}}, stubDeviceResolver{item: &devicedomain.Device{ID: "dev-1", MACAddress: "AA:BB", CurrentSwitchID: "sw-1", CurrentIfIndex: 10, CurrentInterfaceName: "10"}}, stubPortResolver{item: &switchportdomain.Port{ID: "port-1", SwitchID: "sw-1", IfIndex: 10, InterfaceName: "10", VLANID: 100}}, stubAudit{})

	first, err := service.EnforcePolicyDecision(context.Background(), "pd-1", domain.RequestInput{})
	if err != nil {
		t.Fatalf("first request error: %v", err)
	}
	second, err := service.EnforcePolicyDecision(context.Background(), "pd-1", domain.RequestInput{})
	if err != nil {
		t.Fatalf("second request error: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected duplicate request to reuse active request %q, got %q", first.ID, second.ID)
	}
}

type stubEnforcementRepository struct {
	decisions []domain.Decision
	requests  []domain.Request
	results   []domain.Result
}

func (s *stubEnforcementRepository) Insert(ctx context.Context, decision domain.Decision) (domain.Decision, error) {
	s.decisions = append(s.decisions, decision)
	return decision, nil
}
func (s *stubEnforcementRepository) ListRecent(ctx context.Context, limit int) ([]domain.Decision, error) {
	return append([]domain.Decision{}, s.decisions...), nil
}
func (s *stubEnforcementRepository) ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]domain.Decision, error) {
	return append([]domain.Decision{}, s.decisions...), nil
}
func (s *stubEnforcementRepository) FindLatestByKey(ctx context.Context, macAddress, switchID, policyAction string, ifIndex int, interfaceName string) (*domain.Decision, error) {
	return nil, nil
}
func (s *stubEnforcementRepository) AcquireState(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName string, lockedUntil time.Time) (bool, error) {
	return true, nil
}
func (s *stubEnforcementRepository) MarkStateExecuted(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string) error {
	return nil
}
func (s *stubEnforcementRepository) MarkStateFailed(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string, lockedUntil time.Time) error {
	return nil
}
func (s *stubEnforcementRepository) ClearStateForMAC(ctx context.Context, macAddress string) error {
	return nil
}
func (s *stubEnforcementRepository) FindByID(ctx context.Context, id string) (*domain.Decision, error) {
	return nil, nil
}
func (s *stubEnforcementRepository) Approve(ctx context.Context, id string) error      { return nil }
func (s *stubEnforcementRepository) Reject(ctx context.Context, id string) error       { return nil }
func (s *stubEnforcementRepository) Retry(ctx context.Context, id string) error        { return nil }
func (s *stubEnforcementRepository) MarkExecuted(ctx context.Context, id string) error { return nil }
func (s *stubEnforcementRepository) MarkFailed(ctx context.Context, id, lastError string) error {
	return nil
}
func (s *stubEnforcementRepository) InsertRequest(ctx context.Context, request domain.Request) (domain.Request, error) {
	s.requests = append(s.requests, request)
	return request, nil
}
func (s *stubEnforcementRepository) InsertResult(ctx context.Context, result domain.Result) (domain.Result, error) {
	s.results = append(s.results, result)
	return result, nil
}
func (s *stubEnforcementRepository) ListRequests(ctx context.Context, limit, offset int) ([]domain.Request, error) {
	return append([]domain.Request{}, s.requests...), nil
}
func (s *stubEnforcementRepository) ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]domain.Request, error) {
	items := []domain.Request{}
	for _, item := range s.requests {
		if item.DeviceID == deviceID {
			items = append(items, item)
		}
	}
	return items, nil
}
func (s *stubEnforcementRepository) ListResultsByRequest(ctx context.Context, requestID string) ([]domain.Result, error) {
	items := []domain.Result{}
	for _, item := range s.results {
		if item.EnforcementRequestID == requestID {
			items = append(items, item)
		}
	}
	return items, nil
}
func (s *stubEnforcementRepository) FindRequestByID(ctx context.Context, id string) (*domain.Request, error) {
	for _, item := range s.requests {
		if item.ID == id {
			copyItem := item
			return &copyItem, nil
		}
	}
	return nil, nil
}
func (s *stubEnforcementRepository) FindActiveRequest(ctx context.Context, policyDecisionID, action string, targetVLAN int) (*domain.Request, error) {
	for _, item := range s.requests {
		if item.PolicyDecisionID == policyDecisionID && item.RequestedAction == action && item.TargetVLAN == targetVLAN && (item.Status == domain.RequestStatusPending || item.Status == domain.RequestStatusRunning) {
			copyItem := item
			return &copyItem, nil
		}
	}
	return nil, nil
}
func (s *stubEnforcementRepository) ClaimNextRequest(ctx context.Context, now time.Time) (*domain.Request, error) {
	for i := range s.requests {
		if s.requests[i].Status == domain.RequestStatusPending {
			s.requests[i].Status = domain.RequestStatusRunning
			s.requests[i].StartedAt = now
			copyItem := s.requests[i]
			return &copyItem, nil
		}
	}
	return nil, nil
}
func (s *stubEnforcementRepository) MarkRequestStarted(ctx context.Context, id string, adapter string, startedAt time.Time) error {
	for i := range s.requests {
		if s.requests[i].ID == id {
			s.requests[i].Adapter = adapter
			s.requests[i].StartedAt = startedAt
			s.requests[i].Status = domain.RequestStatusRunning
		}
	}
	return nil
}
func (s *stubEnforcementRepository) MarkRequestCompleted(ctx context.Context, id, status, errorCode, errorMessage, verificationStatus string, completedAt, verifiedAt time.Time) error {
	for i := range s.requests {
		if s.requests[i].ID == id {
			s.requests[i].Status = status
			s.requests[i].ErrorCode = errorCode
			s.requests[i].ErrorMessage = errorMessage
			s.requests[i].VerificationStatus = verificationStatus
			s.requests[i].CompletedAt = completedAt
			s.requests[i].VerifiedAt = verifiedAt
		}
	}
	return nil
}
func (s *stubEnforcementRepository) MarkRequestRetry(ctx context.Context, id, errorCode, errorMessage string, nextAttemptAt time.Time) error {
	for i := range s.requests {
		if s.requests[i].ID == id {
			s.requests[i].Status = domain.RequestStatusPending
			s.requests[i].AttemptCount++
			s.requests[i].ErrorCode = errorCode
			s.requests[i].ErrorMessage = errorMessage
			s.requests[i].RequestedAt = nextAttemptAt
		}
	}
	return nil
}
func (s *stubEnforcementRepository) UpdateRequestPolicyBinding(ctx context.Context, id, policyDecisionID string) error {
	return nil
}
func (s *stubEnforcementRepository) UpdatePolicyDecisionEnforcement(ctx context.Context, decisionID, requestID, status, errorMessage string, startedAt, completedAt, enforcedAt time.Time, requested bool) error {
	return nil
}
func (s *stubEnforcementRepository) UpdateDeviceEnforcementSnapshot(ctx context.Context, deviceID, action string, vlanID int, status, errorMessage string, observedAt time.Time) error {
	return nil
}

type stubSwitchResolver struct{ item *switchasset.Switch }

func (s stubSwitchResolver) FindByID(ctx context.Context, id string) (*switchasset.Switch, error) {
	return s.item, nil
}

type stubPolicyDecisionResolver struct{ item *policydomain.Decision }

func (s stubPolicyDecisionResolver) FindDecisionByID(ctx context.Context, id string) (*policydomain.Decision, error) {
	return s.item, nil
}

type stubDeviceResolver struct{ item *devicedomain.Device }

func (s stubDeviceResolver) FindByID(ctx context.Context, id string) (*devicedomain.Device, error) {
	return s.item, nil
}

type stubPortResolver struct{ item *switchportdomain.Port }

func (s stubPortResolver) FindBySwitchIfIndex(ctx context.Context, switchID string, ifIndex int) (*switchportdomain.Port, error) {
	return s.item, nil
}

type stubAudit struct{}

func (stubAudit) Record(ctx context.Context, action, status, targetType, targetID, switchID, macAddress string, payload map[string]any) error {
	return nil
}

type stubSessionResolver struct{}

func (stubSessionResolver) FindLatestActiveByMACSwitch(ctx context.Context, macAddress, switchID string) (*sessiondomain.Session, error) {
	return nil, nil
}

func TestMain(m *testing.M) {
	_ = slog.New(slog.NewTextHandler(io.Discard, nil))
	m.Run()
}
