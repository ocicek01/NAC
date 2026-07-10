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
	ListRequests(ctx context.Context, filters domain.RequestFilters) ([]domain.Request, error)
	ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]domain.Request, error)
	ListResultsByRequest(ctx context.Context, requestID string) ([]domain.Result, error)
	FindRequestByID(ctx context.Context, id string) (*domain.Request, error)
	FindActiveRequest(ctx context.Context, policyDecisionID, action string, targetVLAN int) (*domain.Request, error)
	ClaimNextRequest(ctx context.Context, now time.Time, staleBefore time.Time) (*domain.Request, error)
	MarkRequestQueued(ctx context.Context, id string, queuedAt time.Time) error
	MarkRequestStarted(ctx context.Context, id string, adapter string, startedAt time.Time) error
	MarkRequestVerifying(ctx context.Context, id string, verifyingAt time.Time) error
	MarkRequestCompleted(ctx context.Context, id, status, errorCode, errorMessage, verificationStatus string, completedAt, verifiedAt time.Time) error
	MarkRequestRetry(ctx context.Context, id, status, errorCode, errorMessage string, nextAttemptAt time.Time) error
	CancelRequest(ctx context.Context, id, status, errorCode, errorMessage string, completedAt time.Time) error
	CancelSupersededRequests(ctx context.Context, deviceID, keepRequestID, reason string) (int, error)
	UpdatePolicyDecisionEnforcement(ctx context.Context, policyDecisionID, requestID, status, errorMessage string, startedAt, completedAt, enforcedAt time.Time, requested bool) error
	UpdateDeviceEnforcementSnapshot(ctx context.Context, deviceID, requestID, action string, vlanID int, status, errorMessage string, observedAt time.Time, verified bool) error
	WorkerStats(ctx context.Context) (domain.WorkerStats, error)
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
	if cfg.RetryBaseSeconds <= 0 {
		cfg.RetryBaseSeconds = cfg.RetryBackoffSeconds
	}
	if cfg.RetryBaseSeconds <= 0 {
		cfg.RetryBaseSeconds = 5
	}
	if cfg.RetryBackoffSeconds <= 0 {
		cfg.RetryBackoffSeconds = cfg.RetryBaseSeconds
	}
	if cfg.RequestTimeoutSeconds <= 0 {
		cfg.RequestTimeoutSeconds = 15
	}
	if cfg.StaleRunningSeconds <= 0 {
		cfg.StaleRunningSeconds = 300
	}
	if len(cfg.AdapterPriority) == 0 {
		cfg.AdapterPriority = []string{"snmp"}
		if cfg.MockAdapterEnabled {
			cfg.AdapterPriority = []string{"mock", "snmp"}
		}
	}
	return cfg
}

func (s *Service) ListRequests(ctx context.Context, filters domain.RequestFilters) ([]domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return []domain.Request{}, nil
	}
	return repo.ListRequests(ctx, filters)
}

func (s *Service) WorkerStats(ctx context.Context) (domain.WorkerStats, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return domain.WorkerStats{}, nil
	}
	stats, err := repo.WorkerStats(ctx)
	if err != nil {
		return domain.WorkerStats{}, err
	}
	s.workerMu.Lock()
	stats.LastWorkerError = s.lastWorkerError
	stats.LastWorkerErrorAt = s.lastWorkerErrorAt
	stats.LastWorkerHeartbeatAt = s.lastWorkerHeartbeatAt
	s.workerMu.Unlock()
	return stats, nil
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
		return domain.Request{}, fmt.Errorf("phase 3.2 repository methods are not configured")
	}
	if s.policies == nil || s.devices == nil {
		return domain.Request{}, fmt.Errorf("phase 3.2 dependencies are not configured")
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
	status, reasonCode, reasonMessage := s.preflight(request)
	request.Status = status
	request.ErrorCode = reasonCode
	request.ErrorMessage = reasonMessage
	stored, err := repo.InsertRequest(ctx, request)
	if err != nil {
		return domain.Request{}, err
	}
	cancelledCount, cancelErr := repo.CancelSupersededRequests(ctx, device.ID, stored.ID, "superseded by newer decision")
	if cancelErr == nil && cancelledCount > 0 {
		s.auditEvent(ctx, "enforcement_request_superseded", "warning", stored, map[string]any{"cancelled_request_count": cancelledCount})
	}
	requested := stored.Status == domain.RequestStatusPending || stored.Status == domain.RequestStatusQueued || stored.Status == domain.RequestStatusRetryScheduled || stored.Status == domain.RequestStatusRollbackPending
	_ = repo.UpdatePolicyDecisionEnforcement(ctx, decision.ID, stored.ID, stored.Status, stored.ErrorMessage, time.Time{}, completionTimeFor(stored.Status), enforcedAtFor(stored.Status, time.Now().UTC()), requested)
	if stored.Status == domain.RequestStatusSkipped || stored.Status == domain.RequestStatusFailed || stored.Status == domain.RequestStatusCancelled {
		result := buildTerminalResult(stored, domain.PortState{}, domain.PortState{}, stored.Status, stored.ErrorCode, stored.ErrorMessage, false, stored.Mode, map[string]any{"reason": stored.ErrorMessage})
		_, _ = repo.InsertResult(ctx, result)
		_ = repo.UpdateDeviceEnforcementSnapshot(ctx, device.ID, stored.ID, action, targetVLAN, stored.Status, stored.ErrorMessage, time.Now().UTC(), false)
		s.auditEvent(ctx, auditActionForStatus(stored.Status, stored.ErrorCode), statusLevel(stored.Status), stored, map[string]any{"reason": stored.ErrorMessage, "target_vlan": targetVLAN})
		return stored, nil
	}
	s.auditEvent(ctx, "enforcement_queued", "info", stored, map[string]any{"target_vlan": targetVLAN, "mode": stored.Mode})
	return stored, nil
}

func (s *Service) RetryRequestByID(ctx context.Context, id string) (*domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return nil, fmt.Errorf("phase 3.2 repository methods are not configured")
	}
	request, err := repo.FindRequestByID(ctx, strings.TrimSpace(id))
	if err != nil || request == nil {
		return request, err
	}
	if request.AttemptCount >= s.enforcementCfg.MaxRetries {
		return nil, fmt.Errorf("max retries exceeded")
	}
	next := time.Now().UTC().Add(s.retryDelay(request.AttemptCount + 1))
	if err := repo.MarkRequestRetry(ctx, request.ID, domain.RequestStatusRetryScheduled, "manual_retry", "manual retry requested", next); err != nil {
		return nil, err
	}
	updated, _ := repo.FindRequestByID(ctx, request.ID)
	if updated != nil {
		s.auditEvent(ctx, "enforcement_retry_scheduled", "warning", *updated, map[string]any{"retry_at": next})
	}
	return updated, nil
}

func (s *Service) CancelRequestByID(ctx context.Context, id string) (*domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return nil, fmt.Errorf("phase 3.2 repository methods are not configured")
	}
	request, err := repo.FindRequestByID(ctx, strings.TrimSpace(id))
	if err != nil || request == nil {
		return request, err
	}
	if request.Status == domain.RequestStatusRunning || request.Status == domain.RequestStatusVerifying {
		return nil, fmt.Errorf("running request cannot be cancelled")
	}
	now := time.Now().UTC()
	if err := repo.CancelRequest(ctx, request.ID, domain.RequestStatusCancelled, "cancelled_by_operator", "manual cancel requested", now); err != nil {
		return nil, err
	}
	updated, _ := repo.FindRequestByID(ctx, request.ID)
	if updated != nil {
		_ = repo.UpdatePolicyDecisionEnforcement(ctx, updated.PolicyDecisionID, updated.ID, domain.RequestStatusCancelled, "manual cancel requested", updated.StartedAt, now, time.Time{}, true)
		s.auditEvent(ctx, "enforcement_request_cancelled", "warning", *updated, map[string]any{"error_code": "cancelled_by_operator"})
	}
	return updated, nil
}

func (s *Service) RollbackRequestByID(ctx context.Context, id string, input domain.RollbackInput) (domain.Request, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return domain.Request{}, fmt.Errorf("phase 3.2 repository methods are not configured")
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
	rollback := domain.Request{
		ID:                   uuid.NewString(),
		DeviceID:             request.DeviceID,
		PolicyDecisionID:     request.PolicyDecisionID,
		SwitchID:             request.SwitchID,
		PortID:               request.PortID,
		RequestedAction:      domain.ActionRestorePreviousState,
		TargetVLAN:           request.PreviousVLAN,
		PreviousVLAN:         request.TargetVLAN,
		RequestedBy:          firstNonEmpty(input.RequestedBy, "system"),
		RequestSource:        firstNonEmpty(input.RequestSource, "manual"),
		Mode:                 s.enforcementCfg.Mode,
		Status:               domain.RequestStatusRollbackPending,
		RollbackOfRequestID:  request.ID,
		CurrentSwitchID:      request.CurrentSwitchID,
		CurrentIfIndex:       request.CurrentIfIndex,
		CurrentInterfaceName: request.CurrentInterfaceName,
		TargetDeviceMAC:      request.TargetDeviceMAC,
		RequestedAt:          time.Now().UTC(),
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
		Metadata:             map[string]any{"reason": input.Reason, "rollback_of_request_id": request.ID},
	}
	stored, err := repo.InsertRequest(ctx, rollback)
	if err != nil {
		return domain.Request{}, err
	}
	if err := repo.MarkRequestRetry(ctx, stored.ID, domain.RequestStatusRollbackPending, "", "", stored.RequestedAt); err != nil {
		return domain.Request{}, err
	}
	updated, _ := repo.FindRequestByID(ctx, stored.ID)
	if updated != nil {
		stored = *updated
	}
	s.auditEvent(ctx, "enforcement_rollback_started", "info", stored, map[string]any{"rollback_of_request_id": request.ID, "target_vlan": stored.TargetVLAN})
	return stored, nil
}

func (s *Service) ProcessNextRequest(ctx context.Context) (*domain.WorkerOutcome, error) {
	repo, ok := s.repository.(phase31Repository)
	if !ok {
		return nil, nil
	}
	now := time.Now().UTC()
	request, err := repo.ClaimNextRequest(ctx, now, now.Add(-time.Duration(s.enforcementCfg.StaleRunningSeconds)*time.Second))
	if err != nil {
		s.recordWorkerError(err)
		return nil, err
	}
	if request == nil {
		s.recordWorkerHeartbeat()
		return nil, nil
	}
	s.recordWorkerHeartbeat()
	s.auditEvent(ctx, "enforcement_worker_claimed", "info", *request, nil)
	outcome, execErr := s.executeRequest(ctx, *request)
	if execErr != nil {
		s.recordWorkerError(execErr)
		return outcome, execErr
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
	device, err := s.devices.FindByID(ctx, request.DeviceID)
	if err != nil || device == nil {
		return s.failRequest(ctx, request, "device_lookup_failed", firstError(err, "device not found"), false)
	}
	asset, err := s.switches.FindByID(ctx, request.SwitchID)
	if err != nil || asset == nil {
		return s.failRequest(ctx, request, "switch_lookup_failed", firstError(err, "switch not found"), false)
	}
	port, _ := s.lookupPort(ctx, *device)
	validationStatus, validationCode, validationMessage := s.validateRequestForExecution(request, *device, port)
	if validationStatus != "" {
		s.auditEvent(ctx, "enforcement_validation_failed", statusLevel(validationStatus), request, map[string]any{"error_code": validationCode, "error": validationMessage})
		return s.finishWithoutExecution(ctx, request, validationStatus, validationCode, validationMessage)
	}
	adapter := s.selectAdapter(*asset, request)
	adapterName := ""
	if adapter != nil {
		adapterName = adapter.Name()
	}
	if err := repo.MarkRequestStarted(ctx, request.ID, adapterName, startedAt); err != nil {
		return nil, err
	}
	request.Status = domain.RequestStatusRunning
	request.StartedAt = startedAt
	request.Adapter = adapterName
	_ = repo.UpdatePolicyDecisionEnforcement(ctx, request.PolicyDecisionID, request.ID, domain.RequestStatusRunning, "", startedAt, time.Time{}, time.Time{}, true)
	if request.RollbackOfRequestID != "" {
		s.auditEvent(ctx, "enforcement_rollback_started", "info", request, map[string]any{"adapter": adapterName})
	} else {
		s.auditEvent(ctx, "enforcement_started", "info", request, map[string]any{"adapter": adapterName})
	}
	var initialState domain.PortState
	if adapter != nil {
		initialState, err = adapter.ReadState(ctx, *asset, port, request)
		if err != nil {
			return s.failRequest(ctx, request, "state_read_failed", err.Error(), true)
		}
		s.auditEvent(ctx, "enforcement_state_captured", "info", request, map[string]any{"state": stateMap(initialState)})
	}
	if request.Mode == domain.ModeDryRun {
		result := buildTerminalResult(request, initialState, initialState, domain.RequestStatusSkipped, "dry_run", "enforcement mode is dry_run", false, "dry_run", map[string]any{"adapter": firstNonEmpty(adapterName, "dry_run")})
		return s.completeRequest(ctx, request, result, domain.RequestStatusSkipped)
	}
	if adapter == nil {
		return s.failRequest(ctx, request, "unsupported_vendor", "no adapter available", false)
	}
	if alreadyDesired(request, initialState) {
		result := buildTerminalResult(request, initialState, initialState, terminalSuccessStatus(request), "already_in_desired_state", "request already matches target state", false, "noop", map[string]any{"adapter": adapter.Name()})
		return s.completeRequest(ctx, request, result, terminalSuccessStatus(request))
	}
	s.auditEvent(ctx, "enforcement_command_sent", "info", request, map[string]any{"adapter": adapter.Name(), "command_summary": request.CommandSummary})
	execResponse, err := adapter.Execute(ctx, *asset, port, request)
	if err != nil {
		code := classifyExecutionError(err)
		s.auditEvent(ctx, "enforcement_execution_failed", "error", request, map[string]any{"error_code": code, "error": err.Error(), "retryable": isRetryableError(err)})
		return s.failRequest(ctx, request, code, err.Error(), isRetryableError(err))
	}
	s.auditEvent(ctx, "enforcement_execution_succeeded", "success", request, map[string]any{"adapter": adapter.Name()})
	verifyingAt := time.Now().UTC()
	if err := repo.MarkRequestVerifying(ctx, request.ID, verifyingAt); err != nil {
		return nil, err
	}
	request.Status = domain.RequestStatusVerifying
	s.auditEvent(ctx, "enforcement_verification_started", "info", request, map[string]any{"adapter": adapter.Name()})
	verifiedState, err := adapter.ReadState(ctx, *asset, port, request)
	if err != nil {
		return s.failRequest(ctx, request, "verification_temporarily_unavailable", err.Error(), true)
	}
	verificationStatus := verifyRequest(request, verifiedState)
	result := buildResultFromExecution(request, initialState, verifiedState, verificationStatus, execResponse)
	if verificationStatus != domain.RequestStatusSucceeded {
		s.auditEvent(ctx, "enforcement_verification_failed", "error", request, map[string]any{"observed_state": stateMap(verifiedState)})
		return s.completeRequest(ctx, request, result, terminalVerificationFailureStatus(request))
	}
	s.auditEvent(ctx, "enforcement_verification_succeeded", "success", request, map[string]any{"observed_state": stateMap(verifiedState)})
	return s.completeRequest(ctx, request, result, terminalSuccessStatus(request))
}

func (s *Service) finishWithoutExecution(ctx context.Context, request domain.Request, status, code, message string) (*domain.WorkerOutcome, error) {
	repo := s.repository.(phase31Repository)
	now := time.Now().UTC()
	result := buildTerminalResult(request, domain.PortState{}, domain.PortState{}, status, code, message, false, "validation", map[string]any{})
	storedResult, err := repo.InsertResult(ctx, result)
	if err != nil {
		return nil, err
	}
	if err := repo.MarkRequestCompleted(ctx, request.ID, status, code, message, status, now, time.Time{}); err != nil {
		return nil, err
	}
	_ = repo.UpdatePolicyDecisionEnforcement(ctx, request.PolicyDecisionID, request.ID, status, message, request.StartedAt, now, time.Time{}, true)
	_ = repo.UpdateDeviceEnforcementSnapshot(ctx, request.DeviceID, request.ID, request.RequestedAction, request.TargetVLAN, status, message, now, false)
	s.auditEvent(ctx, "enforcement_result_created", statusLevel(status), request, map[string]any{"result_id": storedResult.ID, "status": status})
	return &domain.WorkerOutcome{Request: request, Result: storedResult}, nil
}

func (s *Service) completeRequest(ctx context.Context, request domain.Request, result domain.Result, status string) (*domain.WorkerOutcome, error) {
	repo := s.repository.(phase31Repository)
	now := time.Now().UTC()
	result.CreatedAt = now
	result.EnforcementRequestID = request.ID
	result.AttemptNumber = request.AttemptCount + 1
	storedResult, err := repo.InsertResult(ctx, result)
	if err != nil {
		return nil, err
	}
	verifiedAt := time.Time{}
	if status == domain.RequestStatusSucceeded || status == domain.RequestStatusRolledBack {
		verifiedAt = now
	}
	if err := repo.MarkRequestCompleted(ctx, request.ID, status, result.ErrorCode, result.ErrorMessage, result.VerificationStatus, now, verifiedAt); err != nil {
		return nil, err
	}
	_ = repo.UpdatePolicyDecisionEnforcement(ctx, request.PolicyDecisionID, request.ID, status, result.ErrorMessage, request.StartedAt, now, enforcedAtFor(status, now), true)
	verified := status == domain.RequestStatusSucceeded || status == domain.RequestStatusRolledBack
	_ = repo.UpdateDeviceEnforcementSnapshot(ctx, request.DeviceID, request.ID, request.RequestedAction, request.TargetVLAN, status, result.ErrorMessage, now, verified)
	s.auditEvent(ctx, "enforcement_result_created", statusLevel(status), request, map[string]any{"result_id": storedResult.ID, "status": status})
	request.Status = status
	request.CompletedAt = now
	request.VerifiedAt = verifiedAt
	request.ErrorCode = result.ErrorCode
	request.ErrorMessage = result.ErrorMessage
	request.VerificationStatus = result.VerificationStatus
	return &domain.WorkerOutcome{Request: request, Result: storedResult}, nil
}

func (s *Service) failRequest(ctx context.Context, request domain.Request, code, message string, retryable bool) (*domain.WorkerOutcome, error) {
	repo := s.repository.(phase31Repository)
	now := time.Now().UTC()
	if retryable && request.AttemptCount < s.enforcementCfg.MaxRetries {
		next := now.Add(s.retryDelay(request.AttemptCount + 1))
		_ = repo.MarkRequestRetry(ctx, request.ID, domain.RequestStatusRetryScheduled, code, message, next)
		_ = repo.UpdatePolicyDecisionEnforcement(ctx, request.PolicyDecisionID, request.ID, domain.RequestStatusRetryScheduled, message, request.StartedAt, time.Time{}, time.Time{}, true)
		s.auditEvent(ctx, "enforcement_retry_scheduled", "warning", request, map[string]any{"error_code": code, "error": message, "retry_at": next})
		return &domain.WorkerOutcome{Request: request, Result: domain.Result{}}, nil
	}
	status := terminalFailureStatus(request)
	result := buildTerminalResult(request, domain.PortState{}, domain.PortState{}, status, code, message, false, "failed", map[string]any{})
	storedResult, _ := repo.InsertResult(ctx, result)
	_ = repo.MarkRequestCompleted(ctx, request.ID, status, code, message, status, now, time.Time{})
	_ = repo.UpdatePolicyDecisionEnforcement(ctx, request.PolicyDecisionID, request.ID, status, message, request.StartedAt, now, time.Time{}, true)
	_ = repo.UpdateDeviceEnforcementSnapshot(ctx, request.DeviceID, request.ID, request.RequestedAction, request.TargetVLAN, status, message, now, false)
	if retryable {
		s.auditEvent(ctx, "enforcement_retry_exhausted", "error", request, map[string]any{"error_code": code, "error": message})
	} else {
		s.auditEvent(ctx, "enforcement_execution_failed", "error", request, map[string]any{"error_code": code, "error": message})
	}
	return &domain.WorkerOutcome{Request: request, Result: storedResult}, errors.New(message)
}

func (s *Service) validateRequestForExecution(request domain.Request, device devicedomain.Device, port *switchportdomain.Port) (string, string, string) {
	if s.enforcementCfg.Mode == domain.ModeDisabled {
		return domain.RequestStatusSkipped, "enforcement_disabled", "enforcement mode is disabled"
	}
	if request.RequestedAction == domain.ActionNone || request.RequestedAction == domain.ActionMonitorOnly {
		return domain.RequestStatusSkipped, "monitor_only", "monitor-only decision does not require enforcement"
	}
	if port != nil && isProtectedPort(*port) {
		return domain.RequestStatusSkipped, "protected_port", "protected port"
	}
	if requiresVLAN(request.RequestedAction) && request.TargetVLAN <= 0 {
		return terminalFailureStatus(request), "invalid_target_vlan", "target vlan is invalid"
	}
	if request.Mode == domain.ModePilot && !s.withinPilotScope(request, device, port) {
		return domain.RequestStatusSkipped, "pilot_scope_denied", "request is outside pilot scope"
	}
	if !s.actionAllowed(request.RequestedAction) {
		return terminalFailureStatus(request), "unsupported_action", "action is not allow-listed"
	}
	if !s.vlanAllowed(request.TargetVLAN) {
		return terminalFailureStatus(request), "invalid_target_vlan", "target vlan is not allow-listed"
	}
	return "", "", ""
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

func buildTerminalResult(request domain.Request, previous, observed domain.PortState, status, code, message string, changed bool, executionStatus string, response map[string]any) domain.Result {
	now := time.Now().UTC()
	return domain.Result{ID: uuid.NewString(), EnforcementRequestID: request.ID, AttemptNumber: request.AttemptCount + 1, Adapter: request.Adapter, Transport: request.Adapter, Action: request.RequestedAction, Success: status == domain.RequestStatusSucceeded || status == domain.RequestStatusRolledBack, Changed: changed, ExecutionStatus: executionStatus, PreviousState: stateMap(previous), ExpectedState: expectedStateMap(request), ObservedState: stateMap(observed), VerificationStatus: status, CommandSummary: request.CommandSummary, AdapterResponse: response, DurationMS: 0, ErrorCode: code, ErrorMessage: message, StartedAt: request.StartedAt, CompletedAt: now, CreatedAt: now}
}

func buildResultFromExecution(request domain.Request, previous, observed domain.PortState, verificationStatus string, response map[string]any) domain.Result {
	now := time.Now().UTC()
	res := domain.Result{ID: uuid.NewString(), EnforcementRequestID: request.ID, AttemptNumber: request.AttemptCount + 1, Adapter: request.Adapter, Transport: request.Adapter, Action: request.RequestedAction, Success: verificationStatus == domain.RequestStatusSucceeded, Changed: true, ExecutionStatus: "completed", PreviousState: stateMap(previous), ExpectedState: expectedStateMap(request), ObservedState: stateMap(observed), VerificationStatus: verificationStatus, CommandSummary: request.CommandSummary, AdapterResponse: response, DurationMS: 0, StartedAt: request.StartedAt, CompletedAt: now, CreatedAt: now}
	if verificationStatus == domain.RequestStatusSucceeded {
		res.VerifiedAt = now
		return res
	}
	res.ErrorCode = "verification_failed"
	res.ErrorMessage = "switch state does not match expected state"
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

func (s *Service) preflight(request domain.Request) (string, string, string) {
	if s.enforcementCfg.Mode == domain.ModeDisabled {
		return domain.RequestStatusSkipped, "enforcement_disabled", "enforcement mode is disabled"
	}
	if request.RequestedAction == domain.ActionNone || request.RequestedAction == domain.ActionMonitorOnly {
		return domain.RequestStatusSkipped, "monitor_only", "monitor-only decision does not require enforcement"
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
		return "enforcement_validation_failed"
	}
	if status == domain.RequestStatusSkipped {
		return "enforcement_skipped"
	}
	if status == domain.RequestStatusCancelled {
		return "enforcement_request_cancelled"
	}
	return "enforcement_requested"
}

func statusLevel(status string) string {
	switch status {
	case domain.RequestStatusSkipped, domain.RequestStatusCancelled:
		return "warning"
	case domain.RequestStatusFailed, domain.RequestStatusVerificationFailed, domain.RequestStatusRollbackFailed:
		return "error"
	default:
		return "info"
	}
}

func (s *Service) auditEvent(ctx context.Context, action, status string, request domain.Request, payload map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	data := map[string]any{
		"request_id":         request.ID,
		"device_id":          request.DeviceID,
		"port_id":            request.PortID,
		"policy_decision_id": request.PolicyDecisionID,
		"requested_action":   request.RequestedAction,
		"target_vlan":        request.TargetVLAN,
		"current_if_index":   request.CurrentIfIndex,
		"current_interface":  request.CurrentInterfaceName,
	}
	for k, v := range payload {
		data[k] = v
	}
	_ = s.audit.Record(ctx, action, status, "enforcement_request", request.ID, request.SwitchID, request.TargetDeviceMAC, data)
}

func completionTimeFor(status string) time.Time {
	switch status {
	case domain.RequestStatusPending, domain.RequestStatusQueued, domain.RequestStatusRunning, domain.RequestStatusVerifying, domain.RequestStatusRetryScheduled, domain.RequestStatusRollbackPending:
		return time.Time{}
	default:
		return time.Now().UTC()
	}
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
	case strings.Contains(message, "unsupported"):
		return "unsupported_action"
	default:
		return "execution_failed"
	}
}

func isRetryableError(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "timeout") || strings.Contains(message, "temporary") || strings.Contains(message, "connection") || strings.Contains(message, "unavailable")
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

func terminalSuccessStatus(request domain.Request) string {
	if strings.TrimSpace(request.RollbackOfRequestID) != "" {
		return domain.RequestStatusRolledBack
	}
	return domain.RequestStatusSucceeded
}

func terminalFailureStatus(request domain.Request) string {
	if strings.TrimSpace(request.RollbackOfRequestID) != "" {
		return domain.RequestStatusRollbackFailed
	}
	return domain.RequestStatusFailed
}

func terminalVerificationFailureStatus(request domain.Request) string {
	if strings.TrimSpace(request.RollbackOfRequestID) != "" {
		return domain.RequestStatusRollbackFailed
	}
	return domain.RequestStatusVerificationFailed
}

func (s *Service) retryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	base := s.enforcementCfg.RetryBaseSeconds
	if base <= 0 {
		base = 5
	}
	multiplier := 1 << (attempt - 1)
	return time.Duration(base*multiplier) * time.Second
}
