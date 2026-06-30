package enforcement

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"nac/internal/config"
	domain "nac/internal/domain/enforcement"
	sessiondomain "nac/internal/domain/session"
	switchasset "nac/internal/domain/switchasset"
)

type Repository interface {
	Insert(ctx context.Context, decision domain.Decision) (domain.Decision, error)
	ListRecent(ctx context.Context, limit int) ([]domain.Decision, error)
	ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]domain.Decision, error)
	FindLatestByKey(ctx context.Context, macAddress, switchID, policyAction string, ifIndex int, interfaceName string) (*domain.Decision, error)
	AcquireState(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName string, lockedUntil time.Time) (bool, error)
	MarkStateExecuted(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string) error
	MarkStateFailed(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string, lockedUntil time.Time) error
	ClearStateForMAC(ctx context.Context, macAddress string) error
	FindByID(ctx context.Context, id string) (*domain.Decision, error)
	Approve(ctx context.Context, id string) error
	Reject(ctx context.Context, id string) error
	Retry(ctx context.Context, id string) error
	MarkExecuted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, lastError string) error
}

type Input struct {
	MACAddress           string
	Hostname             string
	PolicyAction         string
	PolicyReason         string
	SourceType           string
	SwitchID             string
	SwitchName           string
	ManagementIP         string
	BridgePort           int
	IfIndex              int
	InterfaceName        string
	InterfaceDescription string
}

type SwitchCapabilityResolver interface {
	FindByID(ctx context.Context, id string) (*switchasset.Switch, error)
}

type SessionResolver interface {
	FindLatestActiveByMACSwitch(ctx context.Context, macAddress, switchID string) (*sessiondomain.Session, error)
}

type Service struct {
	repository Repository
	switches   SwitchCapabilityResolver
	sessions   SessionResolver
	ssh        SSHExecutor
	snmp       SNMPEnforcer
	coa        CoAExecutor
}

func NewService(repository Repository, switches SwitchCapabilityResolver, sessions SessionResolver, radiusCfg config.RadiusConfig) *Service {
	return &Service{
		repository: repository,
		switches:   switches,
		sessions:   sessions,
		ssh:        NewNativeSSHExecutor(),
		snmp:       NewSNMPEnforcer(),
		coa:        NewCoAExecutor(radiusCfg),
	}
}

func (s *Service) RecordDryRun(ctx context.Context, input Input) (domain.Decision, error) {
	now := time.Now().UTC()
	action := deriveDecisionAction(input.PolicyAction)
	status, requiresApproval, approvalStatus, nextAttemptAt := deriveDecisionState(strings.TrimSpace(input.PolicyAction), now)
	selectedMethod, fallbackMethods := s.selectMethods(ctx, action, input.SwitchID, input.SourceType)
	decision := domain.Decision{
		ID:                   uuid.NewString(),
		DeviceMACAddress:     strings.ToUpper(strings.TrimSpace(input.MACAddress)),
		DeviceHostname:       strings.TrimSpace(input.Hostname),
		PolicyAction:         strings.TrimSpace(input.PolicyAction),
		PolicyReason:         strings.TrimSpace(input.PolicyReason),
		DecisionAction:       action,
		DecisionMode:         "dry-run",
		SelectedMethod:       selectedMethod,
		FallbackMethods:      fallbackMethods,
		RequiresApproval:     requiresApproval,
		ApprovalStatus:       approvalStatus,
		AttemptCount:         0,
		MaxAttempts:          3,
		NextAttemptAt:        nextAttemptAt,
		LastError:            "",
		SwitchID:             strings.TrimSpace(input.SwitchID),
		SwitchName:           strings.TrimSpace(input.SwitchName),
		ManagementIP:         strings.TrimSpace(input.ManagementIP),
		BridgePort:           input.BridgePort,
		IfIndex:              input.IfIndex,
		InterfaceName:        strings.TrimSpace(input.InterfaceName),
		InterfaceDescription: strings.TrimSpace(input.InterfaceDescription),
		Status:               status,
		CreatedAt:            now,
	}

	return s.repository.Insert(ctx, decision)
}

func (s *Service) selectMethods(ctx context.Context, action, switchID, sourceType string) (string, []string) {
	if strings.TrimSpace(action) == "" || strings.EqualFold(action, "monitor") || strings.EqualFold(action, "review-required") {
		return "observe-only", nil
	}

	methods := []string{}
	if s.switches != nil && strings.TrimSpace(switchID) != "" {
		if asset, err := s.switches.FindByID(ctx, switchID); err == nil && asset != nil {
			methods = deriveMethodsForSwitch(*asset, sourceType)
		}
	}

	if len(methods) == 0 {
		methods = []string{"manual-review"}
	}

	return methods[0], methods[1:]
}

func deriveMethodsForSwitch(asset switchasset.Switch, sourceType string) []string {
	methods := make([]string, 0, 4)
	radiusOrigin := strings.EqualFold(strings.TrimSpace(sourceType), "radius")
	if radiusOrigin && asset.SupportsCoA {
		methods = append(methods, "coa")
	}
	if radiusOrigin && asset.SupportsRadiusVLAN {
		methods = append(methods, "radius-vlan")
	}
	if preferSNMPWrite(asset) && asset.SupportsSNMPWrite && supportsSNMPWriteStrategy(asset) {
		methods = append(methods, "snmp-write")
	}
	if asset.SupportsRadiusVLAN && !radiusOrigin {
		methods = append(methods, "radius-vlan")
	}
	if asset.SupportsCoA && !radiusOrigin {
		methods = append(methods, "coa")
	}
	if asset.SupportsSNMPWrite && supportsSNMPWriteStrategy(asset) && !preferSNMPWrite(asset) {
		methods = append(methods, "snmp-write")
	}
	if asset.SupportsSSHEnforcement {
		methods = append(methods, "ssh")
	}
	return methods
}

func (s *Service) ListRecent(ctx context.Context, limit int) ([]domain.Decision, error) {
	return s.repository.ListRecent(ctx, limit)
}

func (s *Service) ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]domain.Decision, error) {
	return s.repository.ListRecentByMAC(ctx, macAddress, limit)
}

func (s *Service) FindLatestByKey(ctx context.Context, macAddress, switchID, policyAction string, ifIndex int, interfaceName string) (*domain.Decision, error) {
	return s.repository.FindLatestByKey(ctx, macAddress, switchID, policyAction, ifIndex, interfaceName)
}

func (s *Service) AcquireState(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName string, lockedUntil time.Time) (bool, error) {
	return s.repository.AcquireState(ctx, macAddress, switchID, policyAction, ifIndex, targetVLAN, interfaceName, lockedUntil)
}

func (s *Service) MarkStateExecuted(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string) error {
	return s.repository.MarkStateExecuted(ctx, macAddress, switchID, policyAction, ifIndex, targetVLAN, interfaceName, decisionID, method)
}

func (s *Service) MarkStateFailed(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string, lockedUntil time.Time) error {
	return s.repository.MarkStateFailed(ctx, macAddress, switchID, policyAction, ifIndex, targetVLAN, interfaceName, decisionID, method, lockedUntil)
}

func (s *Service) ClearStateForMAC(ctx context.Context, macAddress string) error {
	return s.repository.ClearStateForMAC(ctx, macAddress)
}

func (s *Service) Approve(ctx context.Context, id string) error {
	return s.repository.Approve(ctx, strings.TrimSpace(id))
}

func (s *Service) Reject(ctx context.Context, id string) error {
	return s.repository.Reject(ctx, strings.TrimSpace(id))
}

func (s *Service) Retry(ctx context.Context, id string) error {
	return s.repository.Retry(ctx, strings.TrimSpace(id))
}

func (s *Service) ExecuteDecision(ctx context.Context, id string, vlanID int, dryRun bool) (domain.VLANExecutionResult, error) {
	decision, err := s.repository.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}
	if decision == nil {
		return domain.VLANExecutionResult{}, fmt.Errorf("enforcement decision not found")
	}

	methods := append([]string{strings.TrimSpace(decision.SelectedMethod)}, decision.FallbackMethods...)
	for _, method := range methods {
		switch strings.TrimSpace(method) {
		case "radius-vlan":
			if dryRun {
				plan, err := buildRadiusVLANPlan(*decision, vlanID)
				if err != nil {
					return domain.VLANExecutionResult{}, err
				}
				return domain.VLANExecutionResult{Plan: plan, Executed: false}, nil
			}
			result, execErr := executeRadiusVLANPlan(*decision, vlanID)
			if execErr != nil {
				if s.repository != nil {
					_ = s.repository.MarkFailed(ctx, decision.ID, truncateError(result.Output, execErr))
				}
				return result, execErr
			}
			if s.repository != nil {
				_ = s.repository.MarkExecuted(ctx, decision.ID)
			}
			return result, nil
		case "coa":
			if dryRun {
				plan, err := s.PreviewCoAPortVLAN(ctx, decision.DeviceMACAddress, decision.SwitchID, vlanID)
				if err != nil {
					return domain.VLANExecutionResult{}, err
				}
				return domain.VLANExecutionResult{Plan: plan, Executed: false}, nil
			}
			result, execErr := s.ExecuteCoAPortVLAN(ctx, decision.DeviceMACAddress, decision.SwitchID, vlanID)
			if execErr != nil {
				if s.repository != nil {
					_ = s.repository.MarkFailed(ctx, decision.ID, truncateError(result.Output, execErr))
				}
				return result, execErr
			}
			if s.repository != nil {
				_ = s.repository.MarkExecuted(ctx, decision.ID)
			}
			return result, nil
		case "snmp-write":
			if dryRun {
				plan, err := s.PreviewSNMPPortVLAN(ctx, decision.SwitchID, decision.BridgePort, decision.IfIndex, decision.InterfaceName, vlanID, false)
				if err != nil {
					return domain.VLANExecutionResult{}, err
				}
				return domain.VLANExecutionResult{Plan: plan, Executed: false}, nil
			}
			result, execErr := s.ExecuteSNMPPortVLAN(ctx, decision.SwitchID, decision.BridgePort, decision.IfIndex, decision.InterfaceName, vlanID, false)
			if execErr != nil {
				if s.repository != nil {
					_ = s.repository.MarkFailed(ctx, decision.ID, truncateError(result.Output, execErr))
				}
				return result, execErr
			}
			if s.repository != nil {
				_ = s.repository.MarkExecuted(ctx, decision.ID)
			}
			return result, nil
		case "ssh":
			if dryRun {
				plan, err := s.PreviewSSHPortVLAN(ctx, decision.SwitchID, decision.InterfaceName, vlanID)
				if err != nil {
					return domain.VLANExecutionResult{}, err
				}
				return domain.VLANExecutionResult{
					Plan:     plan,
					Executed: false,
					Output:   "",
				}, nil
			}
			result, execErr := s.ExecuteSSHPortVLAN(ctx, decision.SwitchID, decision.InterfaceName, vlanID)
			if execErr != nil {
				if s.repository != nil {
					_ = s.repository.MarkFailed(ctx, decision.ID, truncateError(result.Output, execErr))
				}
				return result, execErr
			}
			if s.repository != nil {
				_ = s.repository.MarkExecuted(ctx, decision.ID)
			}
			return result, nil
		}
	}

	return domain.VLANExecutionResult{}, fmt.Errorf("no executable method available for decision %q", decision.ID)
}

func (s *Service) BouncePort(ctx context.Context, switchID string, ifIndex int, interfaceName string) (domain.VLANExecutionResult, error) {
	asset, err := s.resolveSwitch(ctx, switchID)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}

	if asset.SupportsSNMPWrite && s.snmp != nil {
		return s.ExecuteSNMPPortBounce(ctx, asset.ID, ifIndex, interfaceName)
	}

	if asset.SupportsSSHEnforcement && s.ssh != nil {
		return s.ExecuteSSHPortBounce(ctx, asset.ID, interfaceName)
	}

	return domain.VLANExecutionResult{}, fmt.Errorf("no executable bounce method available for switch %q", switchID)
}

func truncateError(output string, err error) string {
	message := strings.TrimSpace(output)
	if message == "" && err != nil {
		message = err.Error()
	} else if err != nil {
		message = message + " | " + err.Error()
	}
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func deriveDecisionAction(policyAction string) string {
	switch strings.ToLower(strings.TrimSpace(policyAction)) {
	case "blocked":
		return "quarantine-port"
	case "guest":
		return "move-to-guest"
	case "unknown":
		return "review-required"
	case "observed":
		return "monitor"
	case "active":
		return "allow"
	default:
		return "monitor"
	}
}

func deriveDecisionState(policyAction string, now time.Time) (string, bool, string, time.Time) {
	switch strings.ToLower(strings.TrimSpace(policyAction)) {
	case "blocked", "guest":
		return "awaiting-approval", true, "pending", time.Time{}
	case "unknown":
		return "queued", false, "not-required", now.Add(5 * time.Minute)
	case "observed", "active":
		return "simulated", false, "not-required", time.Time{}
	default:
		return "simulated", false, "not-required", time.Time{}
	}
}
