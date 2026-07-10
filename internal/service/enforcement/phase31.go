package enforcement

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"nac/internal/config"
	devicedomain "nac/internal/domain/device"
	domain "nac/internal/domain/enforcement"
	policydomain "nac/internal/domain/policy"
	switchasset "nac/internal/domain/switchasset"
	switchportdomain "nac/internal/domain/switchport"
)

type phase31Repository interface {
	InsertRequest(ctx context.Context, request domain.Request) (domain.Request, error)
	InsertResult(ctx context.Context, result domain.Result) (domain.Result, error)
	ListRequests(ctx context.Context, limit, offset int) ([]domain.Request, error)
	ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]domain.Request, error)
	ListResultsByRequest(ctx context.Context, requestID string) ([]domain.Result, error)
	FindRequestByID(ctx context.Context, id string) (*domain.Request, error)
	FindActiveRequest(ctx context.Context, policyDecisionID, action string, targetVLAN int) (*domain.Request, error)
	ClaimNextRequest(ctx context.Context, now time.Time) (*domain.Request, error)
	MarkRequestStarted(ctx context.Context, id string, adapter string, startedAt time.Time) error
	MarkRequestCompleted(ctx context.Context, id, status, errorCode, errorMessage, verificationStatus string, completedAt, verifiedAt time.Time) error
	MarkRequestRetry(ctx context.Context, id, errorCode, errorMessage string, nextAttemptAt time.Time) error
	UpdatePolicyDecisionEnforcement(ctx context.Context, policyDecisionID, requestID, status, errorMessage string, startedAt, completedAt, enforcedAt time.Time, requested bool) error
	UpdateDeviceEnforcementSnapshot(ctx context.Context, deviceID, action string, vlanID int, status, errorMessage string, observedAt time.Time) error
}

type policyDecisionResolver interface {
	FindDecisionByID(ctx context.Context, id string) (*policydomain.Decision, error)
}

type deviceStateResolver interface {
	FindByID(ctx context.Context, id string) (*devicedomain.Device, error)
}

type portResolver interface {
	FindBySwitchIfIndex(ctx context.Context, switchID string, ifIndex int) (*switchportdomain.Port, error)
}

type auditRecorder interface {
	Record(ctx context.Context, action, status, targetType, targetID, switchID, macAddress string, payload map[string]any) error
}

type Adapter interface {
	Name() string
	CanHandle(asset switchasset.Switch, request domain.Request) bool
	Preview(ctx context.Context, asset switchasset.Switch, port *switchportdomain.Port, request domain.Request) (string, error)
	ReadState(ctx context.Context, asset switchasset.Switch, port *switchportdomain.Port, request domain.Request) (domain.PortState, error)
	Execute(ctx context.Context, asset switchasset.Switch, port *switchportdomain.Port, request domain.Request) (map[string]any, error)
}

func (s *Service) ConfigurePhase31(cfg config.EnforcementConfig, policies policyDecisionResolver, devices deviceStateResolver, ports portResolver, audit auditRecorder) {
	if s == nil {
		return
	}
	s.enforcementCfg = normalizeEnforcementConfig(cfg)
	s.policies = policies
	s.devices = devices
	s.ports = ports
	s.audit = audit
	if s.adapters == nil {
		s.adapters = map[string]Adapter{}
	}
	if _, ok := s.adapters["mock"]; !ok {
		s.adapters["mock"] = NewMockAdapter()
	}
	if _, ok := s.adapters["snmp"]; !ok {
		s.adapters["snmp"] = NewSNMPAdapter(s)
	}
}

func normalizeEnforcementConfig(cfg config.EnforcementConfig) config.EnforcementConfig {
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode == "" {
		cfg.Mode = domain.ModeDryRun
	}
	if cfg.Mode == "dry-run" {
		cfg.Mode = domain.ModeDryRun
	}
	if cfg.Mode != domain.ModeDisabled && cfg.Mode != domain.ModeDryRun && cfg.Mode != domain.ModePilot && cfg.Mode != domain.ModeEnabled {
		cfg.Mode = domain.ModeDryRun
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.WorkerBatchSize <= 0 {
		cfg.WorkerBatchSize = 20
	}
	if cfg.RetryBackoffSeconds <= 0 {
		cfg.RetryBackoffSeconds = 30
	}
	if cfg.RequestTimeoutSeconds <= 0 {
		cfg.RequestTimeoutSeconds = 15
	}
	if len(cfg.AdapterPriority) == 0 {
		cfg.AdapterPriority = []string{"snmp"}
		if cfg.MockAdapterEnabled {
			cfg.AdapterPriority = []string{"mock", "snmp"}
		}
	}
	return cfg
}

func (s *Service) ListRequests(ctx context.Context, limit, offset int) ([]domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return []domain.Request{}, nil
	}
	return repo.ListRequests(ctx, limit, offset)
}

func (s *Service) FindRequestByID(ctx context.Context, id string) (*domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return nil, nil
	}
	return repo.FindRequestByID(ctx, strings.TrimSpace(id))
}

func (s *Service) ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return []domain.Request{}, nil
	}
	return repo.ListDeviceRequests(ctx, strings.TrimSpace(deviceID), limit, offset)
}

func (s *Service) ListResultsByRequest(ctx context.Context, requestID string) ([]domain.Result, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return []domain.Result{}, nil
	}
	return repo.ListResultsByRequest(ctx, strings.TrimSpace(requestID))
}

func (s *Service) EnforcePolicyDecision(ctx context.Context, decisionID string, input domain.RequestInput) (domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return domain.Request{}, fmt.Errorf("phase 3.1 repository methods are not configured")
	}
	if s.policies == nil || s.devices == nil {
		return domain.Request{}, fmt.Errorf("phase 3.1 dependencies are not configured")
	}
	decision, err := s.policies.FindDecisionByID(ctx, strings.TrimSpace(decisionID))
	if err != nil {
		return domain.Request{}, err
	}
	if decision == nil {
		return domain.Request{}, fmt.Errorf("policy decision not found")
	}
	device, err := s.devices.FindByID(ctx, decision.DeviceID)
	if err != nil {
		return domain.Request{}, err
	}
	if device == nil {
		return domain.Request{}, fmt.Errorf("device not found")
	}
	port, _ := s.lookupPort(ctx, *device)
	action := mapDecisionToRequestAction(*decision, input.ActionOverride)
	targetVLAN, err := s.resolveTargetVLAN(*decision, *device, action, input.TargetVLAN)
	if err != nil {
		return domain.Request{}, err
	}
	if active, err := repo.FindActiveRequest(ctx, decision.ID, action, targetVLAN); err == nil && active != nil {
		return *active, nil
	}
	request := buildRequest(*device, *decision, port, action, targetVLAN, input, s.enforcementCfg.Mode)
	request.CommandSummary = s.previewSummary(ctx, request, port)
	status, reasonCode, reasonMessage := s.preflight(request, *device, port)
	request.Status = status
	request.ErrorCode = reasonCode
	request.ErrorMessage = reasonMessage
	stored, err := repo.InsertRequest(ctx, request)
	if err != nil {
		return domain.Request{}, err
	}
	requested := status == domain.RequestStatusPending || status == domain.RequestStatusRunning
	_ = repo.UpdatePolicyDecisionEnforcement(ctx, decision.ID, stored.ID, status, reasonMessage, time.Time{}, completionTimeFor(status), completionTimeFor(status), requested)
	if status != domain.RequestStatusPending {
		_, _ = repo.InsertResult(ctx, buildSkippedResult(stored, reasonCode, reasonMessage))
		_ = repo.UpdateDeviceEnforcementSnapshot(ctx, device.ID, action, targetVLAN, status, reasonMessage, time.Now().UTC())
		s.auditEvent(ctx, auditActionForStatus(status, reasonCode), statusLevel(status), stored, map[string]any{"reason": reasonMessage, "target_vlan": targetVLAN})
		return stored, nil
	}
	s.auditEvent(ctx, "enforcement_requested", "info", stored, map[string]any{"target_vlan": targetVLAN, "mode": stored.Mode})
	return stored, nil
}

func (s *Service) RetryRequestByID(ctx context.Context, id string) (*domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return nil, fmt.Errorf("phase 3.1 repository methods are not configured")
	}
	request, err := repo.FindRequestByID(ctx, strings.TrimSpace(id))
	if err != nil || request == nil {
		return request, err
	}
	if request.AttemptCount >= s.enforcementCfg.MaxRetries {
		return nil, fmt.Errorf("max retries exceeded")
	}
	next := time.Now().UTC().Add(time.Duration(s.enforcementCfg.RetryBackoffSeconds) * time.Second)
	if err := repo.MarkRequestRetry(ctx, request.ID, "manual_retry", "manual retry requested", next); err != nil {
		return nil, err
	}
	updated, _ := repo.FindRequestByID(ctx, request.ID)
	if updated != nil {
		s.auditEvent(ctx, "enforcement_retry_scheduled", "warning", *updated, map[string]any{"retry_at": next})
	}
	return updated, nil
}

func (s *Service) RollbackRequestByID(ctx context.Context, id string, input domain.RollbackInput) (domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return domain.Request{}, fmt.Errorf("phase 3.1 repository methods are not configured")
	}
	request, err := repo.FindRequestByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return domain.Request{}, err
	}
	if request == nil {
		return domain.Request{}, fmt.Errorf("enforcement request not found")
	}
	if request.PreviousVLAN <= 0 {
		return domain.Request{}, fmt.Errorf("previous vlan is not available for rollback")
	}
	rollbackInput := domain.RequestInput{RequestedBy: input.RequestedBy, RequestSource: firstNonEmpty(input.RequestSource, "manual"), ForceExecution: input.ForceExecution, Reason: input.Reason, TargetVLAN: request.PreviousVLAN, ActionOverride: domain.ActionRestorePreviousState}
	rollback := domain.Request{ID: uuid.NewString(), DeviceID: request.DeviceID, PolicyDecisionID: request.PolicyDecisionID, SwitchID: request.SwitchID, PortID: request.PortID, RequestedAction: domain.ActionRestorePreviousState, TargetVLAN: request.PreviousVLAN, PreviousVLAN: request.TargetVLAN, RequestedBy: firstNonEmpty(rollbackInput.RequestedBy, "system"), RequestSource: rollbackInput.RequestSource, Mode: s.enforcementCfg.Mode, Status: domain.RequestStatusPending, RollbackOfRequestID: request.ID, CurrentSwitchID: request.CurrentSwitchID, CurrentIfIndex: request.CurrentIfIndex, CurrentInterfaceName: request.CurrentInterfaceName, TargetDeviceMAC: request.TargetDeviceMAC, RequestedAt: time.Now().UTC(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Metadata: map[string]any{"reason": rollbackInput.Reason}}
	stored, err := repo.InsertRequest(ctx, rollback)
	if err != nil {
		return domain.Request{}, err
	}
	s.auditEvent(ctx, "enforcement_rollback_requested", "info", stored, map[string]any{"rollback_of_request_id": request.ID, "target_vlan": stored.TargetVLAN})
	return stored, nil
}

func (s *Service) ProcessNextRequest(ctx context.Context) (*domain.WorkerOutcome, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return nil, nil
	}
	request, err := repo.ClaimNextRequest(ctx, time.Now().UTC())
	if err != nil || request == nil {
		return nil, err
	}
	outcome, err := s.executeRequest(ctx, *request)
	if err != nil {
		return outcome, err
	}
	return outcome, nil
}

func (s *Service) ProcessDueRequests(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = s.enforcementCfg.WorkerBatchSize
	}
	for i := 0; i < limit; i++ {
		outcome, err := s.ProcessNextRequest(ctx)
		if err != nil {
			return err
		}
		if outcome == nil {
			return nil
		}
	}
	return nil
}

func (s *Service) executeRequest(ctx context.Context, request domain.Request) (*domain.WorkerOutcome, error) {
	repo := s.repository.(phase31Repository)
	startedAt := time.Now().UTC()
	_ = repo.MarkRequestStarted(ctx, request.ID, "", startedAt)
	request.StartedAt = startedAt
	request.Status = domain.RequestStatusRunning
	device, err := s.devices.FindByID(ctx, request.DeviceID)
	if err != nil || device == nil {
		return s.failRequest(ctx, request, "device_lookup_failed", firstError(err, "device not found"), false)
	}
	asset, err := s.switches.FindByID(ctx, request.SwitchID)
	if err != nil || asset == nil {
		return s.failRequest(ctx, request, "switch_lookup_failed", firstError(err, "switch not found"), false)
	}
	port, _ := s.lookupPort(ctx, *device)
	adapter := s.selectAdapter(*asset, request)
	if adapter == nil {
		return s.failRequest(ctx, request, "unsupported_vendor", "no adapter available", false)
	}
	request.Adapter = adapter.Name()
	_ = repo.MarkRequestStarted(ctx, request.ID, request.Adapter, startedAt)
	initialState, err := adapter.ReadState(ctx, *asset, port, request)
	if err != nil {
		return s.failRequest(ctx, request, "state_read_failed", err.Error(), true)
	}
	if alreadyDesired(request, initialState) {
		result := buildSuccessResult(request, initialState, initialState, false, map[string]any{"reason": "already_in_desired_state", "adapter": adapter.Name()})
		return s.completeRequest(ctx, request, result, domain.RequestStatusSucceeded)
	}
	execResponse, err := adapter.Execute(ctx, *asset, port, request)
	if err != nil {
		return s.failRequest(ctx, request, classifyExecutionError(err), err.Error(), isRetryableError(err))
	}
	verifiedState, err := adapter.ReadState(ctx, *asset, port, request)
	if err != nil {
		return s.failRequest(ctx, request, "verification_failed", err.Error(), true)
	}
	verificationStatus := verifyRequest(request, verifiedState)
	result := buildResultFromExecution(request, initialState, verifiedState, verificationStatus, execResponse)
	if verificationStatus != domain.RequestStatusSucceeded {
		return s.completeRequest(ctx, request, result, domain.RequestStatusVerificationFailed)
	}
	return s.completeRequest(ctx, request, result, domain.RequestStatusSucceeded)
}

func (s *Service) completeRequest(ctx context.Context, request domain.Request, result domain.Result, status string) (*domain.WorkerOutcome, error) {
	repo := s.repository.(phase31Repository)
	now := time.Now().UTC()
	result.CreatedAt = now
	result.EnforcementRequestID = request.ID
	storedResult, err := repo.InsertResult(ctx, result)
	if err != nil {
		return nil, err
	}
	request.Status = status
	request.VerificationStatus = result.VerificationStatus
	request.CompletedAt = now
	request.VerifiedAt = now
	request.ErrorCode = result.ErrorCode
	request.ErrorMessage = result.ErrorMessage
	if err := repo.MarkRequestCompleted(ctx, request.ID, status, result.ErrorCode, result.ErrorMessage, result.VerificationStatus, now, now); err != nil {
		return nil, err
	}
	_ = repo.UpdatePolicyDecisionEnforcement(ctx, request.PolicyDecisionID, request.ID, status, result.ErrorMessage, request.StartedAt, now, enforcedAtFor(status, now), true)
	_ = repo.UpdateDeviceEnforcementSnapshot(ctx, request.DeviceID, request.RequestedAction, request.TargetVLAN, status, result.ErrorMessage, now)
	action := "enforcement_succeeded"
	level := "success"
	if status == domain.RequestStatusVerificationFailed {
		action = "enforcement_verification_failed"
		level = "error"
	}
	s.auditEvent(ctx, action, level, request, map[string]any{"verification_status": result.VerificationStatus, "changed": result.Changed, "target_vlan": request.TargetVLAN})
	return &domain.WorkerOutcome{Request: request, Result: storedResult}, nil
}

func (s *Service) failRequest(ctx context.Context, request domain.Request, code, message string, retryable bool) (*domain.WorkerOutcome, error) {
	repo := s.repository.(phase31Repository)
	now := time.Now().UTC()
	if retryable && request.AttemptCount < s.enforcementCfg.MaxRetries {
		next := now.Add(time.Duration(s.enforcementCfg.RetryBackoffSeconds) * time.Second)
		_ = repo.MarkRequestRetry(ctx, request.ID, code, message, next)
		_ = repo.UpdatePolicyDecisionEnforcement(ctx, request.PolicyDecisionID, request.ID, domain.RequestStatusPending, message, request.StartedAt, time.Time{}, time.Time{}, true)
		s.auditEvent(ctx, "enforcement_retry_scheduled", "warning", request, map[string]any{"error_code": code, "error": message, "retry_at": next})
		return &domain.WorkerOutcome{Request: request, Result: domain.Result{}}, nil
	}
	result := domain.Result{ID: uuid.NewString(), EnforcementRequestID: request.ID, Success: false, Changed: false, PreviousState: map[string]any{}, ExpectedState: map[string]any{}, ObservedState: map[string]any{}, VerificationStatus: domain.RequestStatusFailed, AdapterResponse: map[string]any{}, DurationMS: 0, ErrorCode: code, ErrorMessage: message, CreatedAt: now}
	_, _ = repo.InsertResult(ctx, result)
	_ = repo.MarkRequestCompleted(ctx, request.ID, domain.RequestStatusFailed, code, message, domain.RequestStatusFailed, now, time.Time{})
	_ = repo.UpdatePolicyDecisionEnforcement(ctx, request.PolicyDecisionID, request.ID, domain.RequestStatusFailed, message, request.StartedAt, now, time.Time{}, true)
	_ = repo.UpdateDeviceEnforcementSnapshot(ctx, request.DeviceID, request.RequestedAction, request.TargetVLAN, domain.RequestStatusFailed, message, now)
	s.auditEvent(ctx, "enforcement_failed", "error", request, map[string]any{"error_code": code, "error": message})
	return &domain.WorkerOutcome{Request: request, Result: result}, errors.New(message)
}

func (s *Service) lookupPort(ctx context.Context, device devicedomain.Device) (*switchportdomain.Port, error) {
	if s.ports == nil || strings.TrimSpace(device.CurrentSwitchID) == "" || device.CurrentIfIndex <= 0 {
		return nil, nil
	}
	return s.ports.FindBySwitchIfIndex(ctx, device.CurrentSwitchID, device.CurrentIfIndex)
}

func buildRequest(device devicedomain.Device, decision policydomain.Decision, port *switchportdomain.Port, action string, targetVLAN int, input domain.RequestInput, mode string) domain.Request {
	portID := ""
	previousVLAN := 0
	if port != nil {
		portID = port.ID
		previousVLAN = port.VLANID
	}
	now := time.Now().UTC()
	metadata := map[string]any{"policy_name": decision.PolicyName, "decision_type": decision.DecisionType, "reason": input.Reason}
	return domain.Request{ID: uuid.NewString(), DeviceID: device.ID, PolicyDecisionID: decision.ID, SwitchID: device.CurrentSwitchID, PortID: portID, RequestedAction: action, TargetVLAN: targetVLAN, PreviousVLAN: previousVLAN, RequestedBy: firstNonEmpty(input.RequestedBy, "system"), RequestSource: firstNonEmpty(input.RequestSource, "policy_engine"), Mode: mode, Status: domain.RequestStatusPending, AttemptCount: 0, RequestedAt: now, CurrentSwitchID: device.CurrentSwitchID, CurrentIfIndex: device.CurrentIfIndex, CurrentInterfaceName: device.CurrentInterfaceName, TargetDeviceMAC: device.MACAddress, Metadata: metadata, CreatedAt: now, UpdatedAt: now}
}

func buildSkippedResult(request domain.Request, code, message string) domain.Result {
	now := time.Now().UTC()
	return domain.Result{ID: uuid.NewString(), EnforcementRequestID: request.ID, Success: false, Changed: false, PreviousState: map[string]any{}, ExpectedState: map[string]any{"action": request.RequestedAction, "target_vlan": request.TargetVLAN}, ObservedState: map[string]any{}, VerificationStatus: request.Status, AdapterResponse: map[string]any{}, DurationMS: 0, ErrorCode: code, ErrorMessage: message, CreatedAt: now}
}

func buildSuccessResult(request domain.Request, previous, observed domain.PortState, changed bool, response map[string]any) domain.Result {
	now := time.Now().UTC()
	return domain.Result{ID: uuid.NewString(), EnforcementRequestID: request.ID, Success: true, Changed: changed, PreviousState: stateMap(previous), ExpectedState: expectedStateMap(request), ObservedState: stateMap(observed), VerificationStatus: domain.RequestStatusSucceeded, AdapterResponse: response, DurationMS: 0, CreatedAt: now}
}

func buildResultFromExecution(request domain.Request, previous, observed domain.PortState, verificationStatus string, response map[string]any) domain.Result {
	res := buildSuccessResult(request, previous, observed, true, response)
	res.VerificationStatus = verificationStatus
	res.Success = verificationStatus == domain.RequestStatusSucceeded
	if !res.Success {
		res.ErrorCode = "verification_failed"
		res.ErrorMessage = "switch state does not match expected state"
	}
	return res
}

func mapDecisionToRequestAction(decision policydomain.Decision, override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	switch strings.ToLower(strings.TrimSpace(decision.EnforcementAction)) {
	case "monitor", "observe", "monitor_only":
		return domain.ActionNone
	case "restrict":
		return domain.ActionAssignRestrictedVLAN
	case "quarantine":
		return domain.ActionAssignQuarantineVLAN
	case "assign-vlan", "assign_vlan":
		return domain.ActionAssignVLAN
	case "shutdown", "shutdown_port":
		return domain.ActionShutdownPort
	case "enable", "enable_port":
		return domain.ActionEnablePort
	case "bounce", "bounce_port":
		return domain.ActionBouncePort
	}
	switch strings.ToLower(strings.TrimSpace(decision.DecisionType)) {
	case "assign_vlan", "allow":
		return domain.ActionAssignVLAN
	case "restricted":
		return domain.ActionAssignRestrictedVLAN
	case "quarantine", "deny":
		return domain.ActionAssignQuarantineVLAN
	case "monitor_only":
		return domain.ActionNone
	default:
		return domain.ActionNone
	}
}

func (s *Service) resolveTargetVLAN(decision policydomain.Decision, device devicedomain.Device, action string, override int) (int, error) {
	if override > 0 {
		return override, nil
	}
	switch action {
	case domain.ActionAssignVLAN:
		if decision.TargetVLAN > 0 {
			return decision.TargetVLAN, nil
		}
		if device.DefaultVLANID > 0 {
			return device.DefaultVLANID, nil
		}
	case domain.ActionAssignRestrictedVLAN:
		if decision.TargetVLAN > 0 {
			return decision.TargetVLAN, nil
		}
		if s.enforcementCfg.DefaultRestrictedVLAN > 0 {
			return s.enforcementCfg.DefaultRestrictedVLAN, nil
		}
	case domain.ActionAssignQuarantineVLAN:
		if decision.TargetVLAN > 0 {
			return decision.TargetVLAN, nil
		}
		if s.enforcementCfg.DefaultQuarantineVLAN > 0 {
			return s.enforcementCfg.DefaultQuarantineVLAN, nil
		}
	case domain.ActionRestorePreviousState:
		if override > 0 {
			return override, nil
		}
	}
	if requiresVLAN(action) {
		return 0, fmt.Errorf("target vlan is not configured for action %s", action)
	}
	return 0, nil
}

func (s *Service) preflight(request domain.Request, device devicedomain.Device, port *switchportdomain.Port) (string, string, string) {
	if s.enforcementCfg.Mode == domain.ModeDisabled {
		return domain.RequestStatusSkipped, "enforcement_disabled", "enforcement mode is disabled"
	}
	if request.RequestedAction == domain.ActionNone || request.RequestedAction == domain.ActionMonitorOnly {
		return domain.RequestStatusSkipped, "dry_run", "monitor-only decision does not require enforcement"
	}
	if port != nil && isProtectedPort(*port) {
		return domain.RequestStatusSkipped, "protected_port", "protected port"
	}
	if requiresVLAN(request.RequestedAction) && request.TargetVLAN <= 0 {
		return domain.RequestStatusSkipped, "invalid_target_vlan", "target vlan is invalid"
	}
	if s.enforcementCfg.Mode == domain.ModeDryRun {
		return domain.RequestStatusSkipped, "dry_run", "enforcement mode is dry_run"
	}
	if s.enforcementCfg.Mode == domain.ModePilot && !s.withinPilotScope(request, device, port) {
		return domain.RequestStatusSkipped, "outside_pilot_scope", "request is outside pilot scope"
	}
	if !s.actionAllowed(request.RequestedAction) {
		return domain.RequestStatusSkipped, "action_not_allowed", "action is not allow-listed"
	}
	if !s.vlanAllowed(request.TargetVLAN) {
		return domain.RequestStatusSkipped, "invalid_target_vlan", "target vlan is not allow-listed"
	}
	return domain.RequestStatusPending, "", ""
}

func (s *Service) previewSummary(ctx context.Context, request domain.Request, port *switchportdomain.Port) string {
	if s.switches == nil {
		return request.RequestedAction
	}
	asset, err := s.switches.FindByID(ctx, request.SwitchID)
	if err != nil || asset == nil {
		return request.RequestedAction
	}
	adapter := s.selectAdapter(*asset, request)
	if adapter == nil {
		return request.RequestedAction
	}
	summary, err := adapter.Preview(ctx, *asset, port, request)
	if err != nil {
		return request.RequestedAction
	}
	return summary
}

func (s *Service) selectAdapter(asset switchasset.Switch, request domain.Request) Adapter {
	if s == nil {
		return nil
	}
	for _, name := range s.enforcementCfg.AdapterPriority {
		adapter, ok := s.adapters[strings.ToLower(strings.TrimSpace(name))]
		if !ok || adapter == nil {
			continue
		}
		if adapter.CanHandle(asset, request) {
			return adapter
		}
	}
	for _, adapter := range s.adapters {
		if adapter != nil && adapter.CanHandle(asset, request) {
			return adapter
		}
	}
	return nil
}

func requiresVLAN(action string) bool {
	switch action {
	case domain.ActionAssignVLAN, domain.ActionAssignRestrictedVLAN, domain.ActionAssignQuarantineVLAN, domain.ActionRestorePreviousState:
		return true
	default:
		return false
	}
}

func isProtectedPort(port switchportdomain.Port) bool {
	if port.EnforcementProtected || port.IsUplink || port.IsTrunk {
		return true
	}
	mode := strings.ToLower(strings.TrimSpace(port.PortMode))
	if mode == "trunk" || mode == "uplink" || mode == "mirror" || mode == "span" {
		return true
	}
	if strings.TrimSpace(port.NeighborSwitchID) != "" {
		return true
	}
	return false
}

func (s *Service) withinPilotScope(request domain.Request, device devicedomain.Device, port *switchportdomain.Port) bool {
	if len(s.enforcementCfg.AllowedSwitches) > 0 && !containsCI(s.enforcementCfg.AllowedSwitches, request.SwitchID) && !containsCI(s.enforcementCfg.AllowedSwitches, device.CurrentManagementIP) {
		return false
	}
	if len(s.enforcementCfg.AllowedDeviceIDs) > 0 && !containsCI(s.enforcementCfg.AllowedDeviceIDs, device.ID) {
		return false
	}
	if len(s.enforcementCfg.AllowedMACs) > 0 && !containsCI(s.enforcementCfg.AllowedMACs, device.MACAddress) {
		return false
	}
	if len(s.enforcementCfg.AllowedPorts) > 0 {
		match := containsCI(s.enforcementCfg.AllowedPorts, request.PortID) || containsCI(s.enforcementCfg.AllowedPorts, request.CurrentInterfaceName)
		if port != nil {
			match = match || containsCI(s.enforcementCfg.AllowedPorts, fmt.Sprintf("%d", port.IfIndex))
		}
		if !match {
			return false
		}
	}
	return true
}

func (s *Service) actionAllowed(action string) bool {
	if len(s.enforcementCfg.AllowedActions) == 0 {
		return true
	}
	return containsCI(s.enforcementCfg.AllowedActions, action)
}

func (s *Service) vlanAllowed(vlanID int) bool {
	if vlanID <= 0 || len(s.enforcementCfg.AllowedVLANs) == 0 {
		return true
	}
	for _, allowed := range s.enforcementCfg.AllowedVLANs {
		if allowed == vlanID {
			return true
		}
	}
	return false
}

func alreadyDesired(request domain.Request, state domain.PortState) bool {
	switch request.RequestedAction {
	case domain.ActionAssignVLAN, domain.ActionAssignRestrictedVLAN, domain.ActionAssignQuarantineVLAN, domain.ActionRestorePreviousState:
		return request.TargetVLAN > 0 && state.VLANID == request.TargetVLAN
	case domain.ActionShutdownPort:
		return strings.EqualFold(state.AdminStatus, "down")
	case domain.ActionEnablePort:
		return strings.EqualFold(state.AdminStatus, "up")
	default:
		return false
	}
}

func verifyRequest(request domain.Request, state domain.PortState) string {
	switch request.RequestedAction {
	case domain.ActionAssignVLAN, domain.ActionAssignRestrictedVLAN, domain.ActionAssignQuarantineVLAN, domain.ActionRestorePreviousState:
		if request.TargetVLAN > 0 && state.VLANID == request.TargetVLAN {
			return domain.RequestStatusSucceeded
		}
	case domain.ActionShutdownPort:
		if strings.EqualFold(state.AdminStatus, "down") {
			return domain.RequestStatusSucceeded
		}
	case domain.ActionEnablePort:
		if strings.EqualFold(state.AdminStatus, "up") {
			return domain.RequestStatusSucceeded
		}
	case domain.ActionBouncePort:
		return domain.RequestStatusSucceeded
	}
	return domain.RequestStatusVerificationFailed
}

func stateMap(state domain.PortState) map[string]any {
	return map[string]any{"vlan_id": state.VLANID, "admin_status": state.AdminStatus, "oper_status": state.OperStatus, "port_mode": state.PortMode, "protected": state.Protected, "interface_name": state.InterfaceName}
}

func expectedStateMap(request domain.Request) map[string]any {
	return map[string]any{"requested_action": request.RequestedAction, "target_vlan": request.TargetVLAN}
}

func containsCI(values []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, item := range values {
		if strings.EqualFold(strings.TrimSpace(item), candidate) {
			return true
		}
	}
	return false
}

func auditActionForStatus(status, code string) string {
	if code == "protected_port" {
		return "protected_port_action_blocked"
	}
	if code == "invalid_target_vlan" {
		return "invalid_target_vlan_blocked"
	}
	if status == domain.RequestStatusSkipped {
		return "enforcement_skipped"
	}
	return "enforcement_requested"
}

func statusLevel(status string) string {
	if status == domain.RequestStatusSkipped {
		return "warning"
	}
	return "info"
}

func (s *Service) auditEvent(ctx context.Context, action, status string, request domain.Request, payload map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	_ = s.audit.Record(ctx, action, status, "enforcement_request", request.ID, request.SwitchID, request.TargetDeviceMAC, payload)
}

func completionTimeFor(status string) time.Time {
	if status == domain.RequestStatusPending || status == domain.RequestStatusRunning {
		return time.Time{}
	}
	return time.Now().UTC()
}

func enforcedAtFor(status string, now time.Time) time.Time {
	if status == domain.RequestStatusSucceeded || status == domain.RequestStatusRolledBack {
		return now
	}
	return time.Time{}
}

func classifyExecutionError(err error) string {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "timeout"):
		return "switch_timeout"
	case strings.Contains(message, "permission"):
		return "permission_denied"
	case strings.Contains(message, "auth"):
		return "authentication_failed"
	default:
		return "execution_failed"
	}
}

func isRetryableError(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "timeout") || strings.Contains(message, "temporary") || strings.Contains(message, "connection")
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstError(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}
