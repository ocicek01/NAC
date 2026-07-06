package device

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/device"
	dhcpevent "nac/internal/domain/dhcpevent"
	enforcementdomain "nac/internal/domain/enforcement"
	macipbindingdomain "nac/internal/domain/macipbinding"
	macobservation "nac/internal/domain/macobservation"
	portendpointdomain "nac/internal/domain/portendpoint"
	sessiondomain "nac/internal/domain/session"
	switchportdomain "nac/internal/domain/switchport"
	"nac/internal/normalize"
	enforcementservice "nac/internal/service/enforcement"
	policyservice "nac/internal/service/policy"
)

const (
	sourceTypeRadius                 = "radius"
	confidenceAuthoritative          = "authoritative"
	reasonDerivedObservation         = "Derived from current observation"
	reasonRadiusObservation          = "Derived from RADIUS observation"
	autoEnforcementSuppressionWindow = 2 * time.Minute
)

type PolicyEvaluator interface {
	EnsureDefaults(ctx context.Context) error
	Evaluate(ctx context.Context, input policyservice.EvaluationInput) (policyservice.EvaluationResult, error)
}

type EnforcementRecorder interface {
	RecordDryRun(ctx context.Context, input enforcementservice.Input) (enforcementdomain.Decision, error)
	FindLatestByKey(ctx context.Context, macAddress, switchID, policyAction string, ifIndex int, interfaceName string) (*enforcementdomain.Decision, error)
	AcquireState(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName string, lockedUntil time.Time) (bool, error)
	MarkStateExecuted(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string) error
	MarkStateFailed(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string, lockedUntil time.Time) error
	ClearStateForMAC(ctx context.Context, macAddress string) error
	Approve(ctx context.Context, id string) error
	ExecuteDecision(ctx context.Context, id string, vlanID int, dryRun bool) (enforcementdomain.VLANExecutionResult, error)
	BouncePort(ctx context.Context, switchID string, ifIndex int, interfaceName string) (enforcementdomain.VLANExecutionResult, error)
}

type SwitchPortResolver interface {
	ListBySwitch(ctx context.Context, switchID string) ([]switchportdomain.Port, error)
	FindBySwitchIfIndex(ctx context.Context, switchID string, ifIndex int) (*switchportdomain.Port, error)
}

type PortEndpointResolver interface {
	ListBySwitch(ctx context.Context, switchID string) ([]portendpointdomain.Endpoint, error)
}

type SessionResolver interface {
	FindLatestActiveByMACSwitch(ctx context.Context, macAddress, switchID string) (*sessiondomain.Session, error)
}

type MACIPBindingResolver interface {
	FindLatestByMACSwitch(ctx context.Context, macAddress, switchID string) (*macipbindingdomain.Binding, error)
}

type Service struct {
	repository           domain.Repository
	policies             PolicyEvaluator
	enforcement          EnforcementRecorder
	logger               *slog.Logger
	switchPorts          SwitchPortResolver
	portEndpoints        PortEndpointResolver
	sessions             SessionResolver
	macIPBindings        MACIPBindingResolver
	registrationVLAN     int
	guestVLAN            int
	quarantineVLAN       int
	autoExecute          bool
	ipLearningEnabled    bool
	ipLearningWait       time.Duration
	ipRecheck            time.Duration
	portBounceEnabled    bool
	portBounceDelay      time.Duration
	maxMACCountForBounce int
}

const (
	deviceStatusPending = "pending"
	deviceStatusAllowed = "allowed"
	deviceStatusBlocked = "blocked"
	deviceStatusExpired = "expired"
	deviceStatusRetired = "retired"
)

var ErrInvalidDeviceStatus = errors.New("invalid device status")

type RadiusInventoryInput struct {
	MACAddress           string
	Hostname             string
	VendorClass          string
	SwitchID             string
	SwitchName           string
	ManagementIP         string
	NASPort              string
	NASPortID            string
	ObservedAt           time.Time
	PolicyActionOverride string
	PolicyReasonOverride string
}

func NewService(logger *slog.Logger, repository domain.Repository, policies PolicyEvaluator, enforcement EnforcementRecorder, switchPorts SwitchPortResolver, portEndpoints PortEndpointResolver, sessions SessionResolver, macIPBindings MACIPBindingResolver, registrationVLAN, guestVLAN, quarantineVLAN int, autoExecute bool, ipLearningEnabled bool, ipLearningWait, ipRecheck, portBounceDelay time.Duration, portBounceEnabled bool, maxMACCountForBounce int) *Service {
	return &Service{
		repository:           repository,
		policies:             policies,
		enforcement:          enforcement,
		logger:               logger,
		switchPorts:          switchPorts,
		portEndpoints:        portEndpoints,
		sessions:             sessions,
		macIPBindings:        macIPBindings,
		registrationVLAN:     registrationVLAN,
		guestVLAN:            guestVLAN,
		quarantineVLAN:       quarantineVLAN,
		autoExecute:          autoExecute,
		ipLearningEnabled:    ipLearningEnabled,
		ipLearningWait:       ipLearningWait,
		ipRecheck:            ipRecheck,
		portBounceEnabled:    portBounceEnabled,
		portBounceDelay:      portBounceDelay,
		maxMACCountForBounce: maxMACCountForBounce,
	}
}

func (s *Service) List(ctx context.Context) ([]domain.Device, error) {
	return s.repository.List(ctx)
}

func (s *Service) ListByMAC(ctx context.Context, macAddress string) ([]domain.Device, error) {
	macAddress = normalize.MACAddress(macAddress)
	if macAddress == "" {
		return []domain.Device{}, nil
	}
	return s.repository.ListByMAC(ctx, macAddress)
}

func (s *Service) ListBySwitch(ctx context.Context, switchID string) ([]domain.Device, error) {
	devices, err := s.repository.ListBySwitch(ctx, switchID)
	if err != nil {
		return nil, err
	}

	return s.mergeObservedPortDevices(ctx, switchID, 0, devices)
}

func (s *Service) ListBySwitchAndIfIndex(ctx context.Context, switchID string, ifIndex int) ([]domain.Device, error) {
	if ifIndex <= 0 {
		return s.ListBySwitch(ctx, switchID)
	}

	devices, err := s.repository.ListBySwitchAndIfIndex(ctx, switchID, ifIndex)
	if err != nil {
		return nil, err
	}

	return s.mergeObservedPortDevices(ctx, switchID, ifIndex, devices)
}

func (s *Service) mergeObservedPortDevices(ctx context.Context, switchID string, ifIndex int, devices []domain.Device) ([]domain.Device, error) {
	switchID = strings.TrimSpace(switchID)
	if switchID == "" || s.switchPorts == nil {
		return devices, nil
	}

	var ports []switchportdomain.Port
	if ifIndex > 0 {
		port, err := s.switchPorts.FindBySwitchIfIndex(ctx, switchID, ifIndex)
		if err != nil {
			return nil, err
		}
		if port == nil {
			return devices, nil
		}
		ports = []switchportdomain.Port{*port}
	} else {
		var err error
		ports, err = s.switchPorts.ListBySwitch(ctx, switchID)
		if err != nil {
			return nil, err
		}
	}

	endpointByKey := map[string]portendpointdomain.Endpoint{}
	if s.portEndpoints != nil {
		items, err := s.portEndpoints.ListBySwitch(ctx, switchID)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if ifIndex > 0 && item.PortIfIndex != ifIndex {
				continue
			}
			mac := normalize.MACAddress(item.MACAddress)
			if mac == "" {
				continue
			}
			endpointByKey[s.syntheticObservedDeviceKey(switchID, item.PortIfIndex, mac)] = item
		}
	}

	existing := map[string]struct{}{}
	for _, device := range devices {
		mac := normalize.MACAddress(device.MACAddress)
		if mac == "" {
			continue
		}
		existing[s.syntheticObservedDeviceKey(switchID, device.CurrentIfIndex, mac)] = struct{}{}
		existing[mac] = struct{}{}
	}

	merged := append([]domain.Device{}, devices...)
	for _, port := range ports {
		for _, rawMAC := range port.MACAddresses {
			mac := normalize.MACAddress(rawMAC)
			if mac == "" {
				continue
			}
			key := s.syntheticObservedDeviceKey(switchID, port.IfIndex, mac)
			if _, ok := existing[key]; ok {
				continue
			}
			if _, ok := existing[mac]; ok {
				continue
			}
			var endpoint *portendpointdomain.Endpoint
			if item, ok := endpointByKey[key]; ok {
				endpoint = &item
			}
			merged = append(merged, s.syntheticObservedDevice(ctx, switchID, port, endpoint, mac))
			existing[key] = struct{}{}
			existing[mac] = struct{}{}
		}
	}

	return merged, nil
}

func (s *Service) syntheticObservedDevice(ctx context.Context, switchID string, port switchportdomain.Port, endpoint *portendpointdomain.Endpoint, macAddress string) domain.Device {
	now := time.Now().UTC()
	device := domain.Device{
		ID:                          s.syntheticObservedDeviceKey(switchID, port.IfIndex, macAddress),
		MACAddress:                  macAddress,
		DeviceType:                  "unknown",
		Status:                      deviceStatusPending,
		CurrentSwitchID:             strings.TrimSpace(switchID),
		CurrentIfIndex:              port.IfIndex,
		CurrentInterfaceName:        strings.TrimSpace(port.InterfaceName),
		CurrentInterfaceDescription: strings.TrimSpace(port.InterfaceDescription),
		CurrentSourceType:           "switch_port",
		CurrentConfidence:           "observed",
		FirstSeenAt:                 now,
		LastSeenAt:                  now,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	if !port.UpdatedAt.IsZero() {
		device.FirstSeenAt = port.UpdatedAt
		device.LastSeenAt = port.UpdatedAt
		device.CreatedAt = port.UpdatedAt
		device.UpdatedAt = port.UpdatedAt
	}

	s.enrichSyntheticObservedDevice(ctx, &device, switchID, endpoint, macAddress)

	if endpoint != nil {
		device.CurrentIPAddress = strings.TrimSpace(endpoint.IPAddress)
		device.Hostname = strings.TrimSpace(endpoint.Hostname)
		if confidence := strings.TrimSpace(endpoint.SourceConfidence); confidence != "" {
			device.CurrentConfidence = confidence
		}
		if !endpoint.LastSeenAt.IsZero() {
			device.LastSeenAt = endpoint.LastSeenAt
			device.UpdatedAt = endpoint.LastSeenAt
		}
		if !endpoint.CreatedAt.IsZero() {
			device.FirstSeenAt = endpoint.CreatedAt
			device.CreatedAt = endpoint.CreatedAt
		}
		device.CurrentSourceType = "port_endpoint"
	}

	return device
}

func (s *Service) enrichSyntheticObservedDevice(ctx context.Context, device *domain.Device, switchID string, endpoint *portendpointdomain.Endpoint, macAddress string) {
	if device == nil {
		return
	}

	if endpoint != nil {
		if device.CurrentIPAddress == "" {
			device.CurrentIPAddress = strings.TrimSpace(endpoint.IPAddress)
		}
		if device.Hostname == "" {
			device.Hostname = strings.TrimSpace(endpoint.Hostname)
		}
	}

	if s.sessions != nil {
		session, err := s.sessions.FindLatestActiveByMACSwitch(ctx, macAddress, switchID)
		if err == nil && session != nil {
			if device.CurrentIPAddress == "" {
				device.CurrentIPAddress = strings.TrimSpace(session.IPAddress)
			}
			if device.Hostname == "" {
				device.Hostname = strings.TrimSpace(session.Hostname)
			}
			if device.IdentityUsername == "" {
				device.IdentityUsername = strings.TrimSpace(session.Username)
			}
			if device.CurrentSwitchName == "" {
				device.CurrentSwitchName = strings.TrimSpace(session.SwitchName)
			}
			if device.CurrentManagementIP == "" {
				device.CurrentManagementIP = strings.TrimSpace(session.ManagementIP)
			}
			if !session.LastSeenAt.IsZero() && session.LastSeenAt.After(device.LastSeenAt) {
				device.LastSeenAt = session.LastSeenAt
				device.UpdatedAt = session.LastSeenAt
			}
			if endpoint == nil {
				device.CurrentSourceType = "radius_session"
			}
		}
	}

	if s.macIPBindings != nil && (device.CurrentIPAddress == "" || device.Hostname == "") {
		binding, err := s.macIPBindings.FindLatestByMACSwitch(ctx, macAddress, switchID)
		if err == nil && binding != nil {
			if device.CurrentIPAddress == "" {
				device.CurrentIPAddress = strings.TrimSpace(binding.IPAddress)
			}
			if device.Hostname == "" {
				device.Hostname = strings.TrimSpace(binding.Hostname)
			}
			if !binding.LastSeenAt.IsZero() && binding.LastSeenAt.After(device.LastSeenAt) {
				device.LastSeenAt = binding.LastSeenAt
				device.UpdatedAt = binding.LastSeenAt
			}
			if endpoint == nil && device.CurrentSourceType == "switch_port" {
				device.CurrentSourceType = "mac_ip_binding"
			}
		}
	}
}
func (s *Service) syntheticObservedDeviceKey(switchID string, ifIndex int, macAddress string) string {
	return fmt.Sprintf("observed:%s:%d:%s", strings.TrimSpace(switchID), ifIndex, strings.ToLower(strings.TrimSpace(macAddress)))
}
func (s *Service) UpdateStatus(ctx context.Context, macAddress, status, approvedBy string, expiresAt time.Time, targetVLAN int) (domain.Device, error) {
	macAddress = normalize.MACAddress(macAddress)
	if macAddress == "" {
		return domain.Device{}, fmt.Errorf("invalid mac address")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case deviceStatusPending, deviceStatusAllowed, deviceStatusBlocked, deviceStatusExpired, deviceStatusRetired:
	default:
		return domain.Device{}, ErrInvalidDeviceStatus
	}

	var approvedAt time.Time
	if status == deviceStatusAllowed {
		approvedAt = time.Now().UTC()
	}

	policyAction, policyReason := statusPolicy(status, approvedBy)
	updated, err := s.repository.UpdateStatus(ctx, macAddress, status, strings.TrimSpace(approvedBy), policyAction, policyReason, approvedAt, expiresAt)
	if err != nil {
		return domain.Device{}, err
	}

	if err := s.executeStatusEnforcement(ctx, updated, targetVLAN); err != nil {
		return domain.Device{}, err
	}

	refreshed, err := s.repository.ListByMAC(ctx, updated.MACAddress)
	if err != nil || len(refreshed) == 0 {
		return updated, err
	}

	return refreshed[0], nil
}

func (s *Service) AddIdentitySnapshot(ctx context.Context, snapshot domain.IdentitySnapshot) (domain.IdentitySnapshot, error) {
	snapshot.IdentityType = strings.TrimSpace(snapshot.IdentityType)
	snapshot.IdentitySource = strings.TrimSpace(snapshot.IdentitySource)
	snapshot.ExternalID = strings.TrimSpace(snapshot.ExternalID)
	snapshot.Username = strings.TrimSpace(snapshot.Username)
	snapshot.FullName = strings.TrimSpace(snapshot.FullName)
	if snapshot.Attributes == nil {
		snapshot.Attributes = map[string]any{}
	}
	if snapshot.VerifiedAt.IsZero() {
		snapshot.VerifiedAt = time.Now().UTC()
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}
	return s.repository.AddIdentitySnapshot(ctx, snapshot)
}

func (s *Service) executeStatusEnforcement(ctx context.Context, device domain.Device, targetVLAN int) error {
	if s.enforcement == nil {
		return nil
	}

	policyAction, policyReason, resolvedVLAN, shouldExecute, err := s.statusEnforcementPlan(device, targetVLAN)
	if err != nil {
		return err
	}
	if !shouldExecute {
		return nil
	}
	if strings.TrimSpace(device.CurrentSwitchID) == "" {
		return fmt.Errorf("device has no current switch context for enforcement")
	}
	if device.CurrentIfIndex <= 0 && strings.TrimSpace(device.CurrentInterfaceName) == "" {
		return fmt.Errorf("device has no current interface context for enforcement")
	}

	ok, err := s.enforcement.AcquireState(
		ctx,
		device.MACAddress,
		device.CurrentSwitchID,
		policyAction,
		device.CurrentIfIndex,
		resolvedVLAN,
		device.CurrentInterfaceName,
		time.Now().UTC().Add(autoEnforcementSuppressionWindow),
	)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	decision, err := s.enforcement.RecordDryRun(ctx, enforcementservice.Input{
		MACAddress:           device.MACAddress,
		Hostname:             device.Hostname,
		PolicyAction:         policyAction,
		PolicyReason:         policyReason,
		SourceType:           device.CurrentSourceType,
		SwitchID:             device.CurrentSwitchID,
		SwitchName:           device.CurrentSwitchName,
		ManagementIP:         device.CurrentManagementIP,
		BridgePort:           device.CurrentBridgePort,
		IfIndex:              device.CurrentIfIndex,
		InterfaceName:        device.CurrentInterfaceName,
		InterfaceDescription: device.CurrentInterfaceDescription,
	})
	if err != nil {
		return err
	}

	method := strings.TrimSpace(decision.SelectedMethod)
	if method == "" || method == "manual-review" || method == "observe-only" {
		return fmt.Errorf("no executable enforcement method available for device %s", device.MACAddress)
	}
	if strings.EqualFold(method, "radius-vlan") && !strings.EqualFold(strings.TrimSpace(device.CurrentSourceType), sourceTypeRadius) {
		return fmt.Errorf("radius-vlan enforcement requires radius-origin context")
	}

	if decision.RequiresApproval {
		if err := s.enforcement.Approve(ctx, decision.ID); err != nil {
			_ = s.enforcement.MarkStateFailed(ctx, decision.DeviceMACAddress, decision.SwitchID, decision.PolicyAction, decision.IfIndex, resolvedVLAN, decision.InterfaceName, decision.ID, decision.SelectedMethod, time.Now().UTC().Add(autoEnforcementSuppressionWindow))
			_ = s.repository.UpdateEnforcementState(ctx, decision.DeviceMACAddress, strings.TrimSpace(decision.PolicyAction), resolvedVLAN, "failed", decision.SwitchID, decision.IfIndex, decision.SelectedMethod, time.Now().UTC())
			return err
		}
	}

	_ = s.repository.UpdateEnforcementState(ctx, decision.DeviceMACAddress, strings.TrimSpace(decision.PolicyAction), resolvedVLAN, "queued", decision.SwitchID, decision.IfIndex, decision.SelectedMethod, time.Now().UTC())
	result, err := s.enforcement.ExecuteDecision(ctx, decision.ID, resolvedVLAN, false)
	if err != nil {
		_ = s.enforcement.MarkStateFailed(ctx, decision.DeviceMACAddress, decision.SwitchID, decision.PolicyAction, decision.IfIndex, resolvedVLAN, decision.InterfaceName, decision.ID, decision.SelectedMethod, time.Now().UTC().Add(autoEnforcementSuppressionWindow))
		_ = s.repository.UpdateEnforcementState(ctx, decision.DeviceMACAddress, strings.TrimSpace(decision.PolicyAction), resolvedVLAN, "failed", decision.SwitchID, decision.IfIndex, decision.SelectedMethod, time.Now().UTC())
		return fmt.Errorf("enforcement execution failed: %w", err)
	}

	_ = result
	if err := s.enforcement.MarkStateExecuted(ctx, decision.DeviceMACAddress, decision.SwitchID, decision.PolicyAction, decision.IfIndex, resolvedVLAN, decision.InterfaceName, decision.ID, decision.SelectedMethod); err != nil {
		return err
	}
	if err := s.repository.UpdateEnforcementState(ctx, decision.DeviceMACAddress, strings.TrimSpace(decision.PolicyAction), resolvedVLAN, "executed", decision.SwitchID, decision.IfIndex, decision.SelectedMethod, time.Now().UTC()); err != nil {
		return err
	}

	s.maybeStartPostEnforcementIPLearning(device, decision, resolvedVLAN)

	return nil
}

func (s *Service) maybeStartPostEnforcementIPLearning(device domain.Device, decision enforcementdomain.Decision, vlanID int) {
	if !s.ipLearningEnabled || s.enforcement == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(device.Status), deviceStatusAllowed) {
		return
	}
	if strings.TrimSpace(decision.PolicyAction) != "active" {
		return
	}
	if strings.TrimSpace(device.CurrentSwitchID) == "" || device.CurrentIfIndex <= 0 {
		return
	}

	startedAt := time.Now().UTC()
	_ = s.repository.UpdateIPLearningState(context.Background(), device.MACAddress, device.CurrentSwitchID, device.CurrentIfIndex, "pending", startedAt, time.Time{}, time.Time{})

	go func(dev domain.Device, dec enforcementdomain.Decision, targetVLAN int) {
		ctx, cancel := context.WithTimeout(context.Background(), s.ipLearningWait+(2*s.ipRecheck)+s.portBounceDelay+(30*time.Second))
		defer cancel()

		if s.waitForIPAddress(ctx, dev.MACAddress, s.ipLearningWait) {
			_ = s.repository.UpdateIPLearningState(context.Background(), dev.MACAddress, dev.CurrentSwitchID, dev.CurrentIfIndex, "learned", startedAt, time.Now().UTC(), time.Time{})
			s.logInfo("post-enforcement ip learned without bounce", "mac_address", dev.MACAddress, "switch_id", dev.CurrentSwitchID, "if_index", dev.CurrentIfIndex, "target_vlan", targetVLAN)
			return
		}

		if !s.portBounceEnabled {
			_ = s.repository.UpdateIPLearningState(context.Background(), dev.MACAddress, dev.CurrentSwitchID, dev.CurrentIfIndex, "timeout", startedAt, time.Time{}, time.Time{})
			s.logInfo("post-enforcement ip learning timed out and bounce is disabled", "mac_address", dev.MACAddress, "switch_id", dev.CurrentSwitchID, "if_index", dev.CurrentIfIndex, "target_vlan", targetVLAN)
			return
		}

		ok, reason := s.bounceEligible(ctx, dev)
		if !ok {
			_ = s.repository.UpdateIPLearningState(context.Background(), dev.MACAddress, dev.CurrentSwitchID, dev.CurrentIfIndex, "bounce_skipped", startedAt, time.Time{}, time.Time{})
			s.logInfo("post-enforcement ip learning bounce skipped", "mac_address", dev.MACAddress, "switch_id", dev.CurrentSwitchID, "if_index", dev.CurrentIfIndex, "reason", reason)
			return
		}

		bouncedAt := time.Now().UTC()
		_ = s.repository.UpdateIPLearningState(context.Background(), dev.MACAddress, dev.CurrentSwitchID, dev.CurrentIfIndex, "bounce_triggered", startedAt, time.Time{}, bouncedAt)
		s.logInfo("post-enforcement ip learning triggering port bounce", "mac_address", dev.MACAddress, "switch_id", dev.CurrentSwitchID, "if_index", dev.CurrentIfIndex, "target_vlan", targetVLAN)
		if _, err := s.enforcement.BouncePort(ctx, dev.CurrentSwitchID, dev.CurrentIfIndex, dev.CurrentInterfaceName); err != nil {
			_ = s.repository.UpdateIPLearningState(context.Background(), dev.MACAddress, dev.CurrentSwitchID, dev.CurrentIfIndex, "bounce_failed", startedAt, time.Time{}, bouncedAt)
			s.logError("post-enforcement port bounce failed", "mac_address", dev.MACAddress, "switch_id", dev.CurrentSwitchID, "if_index", dev.CurrentIfIndex, "error", err)
			return
		}

		if s.portBounceDelay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(s.portBounceDelay):
			}
		}

		if s.waitForIPAddress(ctx, dev.MACAddress, s.ipLearningWait) {
			_ = s.repository.UpdateIPLearningState(context.Background(), dev.MACAddress, dev.CurrentSwitchID, dev.CurrentIfIndex, "learned_after_bounce", startedAt, time.Now().UTC(), bouncedAt)
			s.logInfo("post-enforcement ip learned after port bounce", "mac_address", dev.MACAddress, "switch_id", dev.CurrentSwitchID, "if_index", dev.CurrentIfIndex, "target_vlan", targetVLAN)
			return
		}

		_ = s.repository.UpdateIPLearningState(context.Background(), dev.MACAddress, dev.CurrentSwitchID, dev.CurrentIfIndex, "timeout_after_bounce", startedAt, time.Time{}, bouncedAt)
		s.logInfo("post-enforcement ip learning timed out after port bounce", "mac_address", dev.MACAddress, "switch_id", dev.CurrentSwitchID, "if_index", dev.CurrentIfIndex, "target_vlan", targetVLAN)
	}(device, decision, vlanID)
}

func (s *Service) waitForIPAddress(ctx context.Context, macAddress string, maxWait time.Duration) bool {
	if maxWait <= 0 {
		maxWait = 30 * time.Second
	}
	interval := s.ipRecheck
	if interval <= 0 {
		interval = 10 * time.Second
	}

	deadline := time.Now().Add(maxWait)
	for {
		devices, err := s.repository.ListByMAC(ctx, macAddress)
		if err == nil && len(devices) > 0 && strings.TrimSpace(devices[0].CurrentIPAddress) != "" {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(interval):
		}
	}
}

func (s *Service) bounceEligible(ctx context.Context, device domain.Device) (bool, string) {
	if s.switchPorts == nil {
		return false, "switch port repository is not configured"
	}
	port, err := s.switchPorts.FindBySwitchIfIndex(ctx, device.CurrentSwitchID, device.CurrentIfIndex)
	if err != nil {
		return false, err.Error()
	}
	if port == nil {
		return false, "switch port not found"
	}
	if port.IsUplink {
		return false, "port is uplink"
	}
	if port.IsTrunk || strings.EqualFold(strings.TrimSpace(port.PortMode), "trunk") {
		return false, "port is trunk"
	}
	if !strings.EqualFold(strings.TrimSpace(port.PortMode), "access") {
		return false, "port is not access"
	}
	if port.MACCount > s.maxMACCountForBounce {
		return false, "port has too many mac addresses"
	}
	return true, ""
}

func (s *Service) statusEnforcementPlan(device domain.Device, targetVLAN int) (string, string, int, bool, error) {
	switch strings.ToLower(strings.TrimSpace(device.Status)) {
	case deviceStatusPending, deviceStatusExpired:
		if s.registrationVLAN <= 0 {
			return "", "", 0, false, fmt.Errorf("registration vlan is not configured")
		}
		return "guest", "Registration workflow enforcement", s.registrationVLAN, true, nil
	case deviceStatusBlocked, deviceStatusRetired:
		if s.quarantineVLAN <= 0 {
			return "", "", 0, false, fmt.Errorf("quarantine vlan is not configured")
		}
		return "blocked", "Administrative block enforcement", s.quarantineVLAN, true, nil
	case deviceStatusAllowed:
		if targetVLAN <= 0 {
			return "", "", 0, false, fmt.Errorf("target_vlan is required to enforce allowed status")
		}
		return "active", "Administrative allow enforcement", targetVLAN, true, nil
	default:
		return "", "", 0, false, nil
	}
}

func (s *Service) UpsertFromObservation(ctx context.Context, event dhcpevent.Event, observation macobservation.Observation) error {
	normalizedMAC := normalize.MACAddress(event.MACAddress)
	if normalizedMAC == "" {
		return fmt.Errorf("invalid mac address")
	}
	now := time.Now().UTC()
	hostname := strings.TrimSpace(event.Hostname)
	vendorClass := strings.TrimSpace(event.VendorClass)
	status, policyAction, policyReason := s.evaluatePolicy(ctx, policyservice.EvaluationInput{
		MACAddress:  normalizedMAC,
		Hostname:    hostname,
		VendorClass: vendorClass,
		SwitchName:  observation.SwitchName,
		Interface:   observation.InterfaceName,
	}, observation.SwitchID, false, "", "", reasonDerivedObservation)

	device := domain.Device{
		ID:                          uuid.NewString(),
		MACAddress:                  normalizedMAC,
		Hostname:                    hostname,
		VendorClass:                 vendorClass,
		Status:                      status,
		PolicyAction:                policyAction,
		PolicyReason:                policyReason,
		CurrentSwitchID:             strings.TrimSpace(observation.SwitchID),
		CurrentSwitchName:           strings.TrimSpace(observation.SwitchName),
		CurrentManagementIP:         strings.TrimSpace(observation.ManagementIP),
		CurrentBridgePort:           observation.BridgePort,
		CurrentIfIndex:              observation.IfIndex,
		CurrentInterfaceName:        strings.TrimSpace(observation.InterfaceName),
		CurrentInterfaceDescription: strings.TrimSpace(observation.InterfaceDescription),
		CurrentSourceType:           strings.TrimSpace(observation.SourceType),
		CurrentConfidence:           strings.TrimSpace(observation.Confidence),
		FirstSeenAt:                 event.ObservedAt,
		LastSeenAt:                  event.ObservedAt,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	if device.FirstSeenAt.IsZero() {
		device.FirstSeenAt = now
	}
	if device.LastSeenAt.IsZero() {
		device.LastSeenAt = now
	}

	out, err := s.repository.Upsert(ctx, device)
	if err != nil {
		return err
	}

	if s.enforcement != nil {
		allowed, skipReason := s.acquireEnforcementState(ctx, out)
		if !allowed {
			if skipReason != "" {
				s.logInfo(skipReason, "mac_address", out.MACAddress, "policy_action", out.PolicyAction, "switch_id", out.CurrentSwitchID, "if_index", out.CurrentIfIndex)
			}
			return nil
		}
		decision, err := s.enforcement.RecordDryRun(ctx, enforcementservice.Input{
			MACAddress:           out.MACAddress,
			Hostname:             out.Hostname,
			PolicyAction:         out.PolicyAction,
			PolicyReason:         out.PolicyReason,
			SourceType:           out.CurrentSourceType,
			SwitchID:             out.CurrentSwitchID,
			SwitchName:           out.CurrentSwitchName,
			ManagementIP:         out.CurrentManagementIP,
			BridgePort:           out.CurrentBridgePort,
			IfIndex:              out.CurrentIfIndex,
			InterfaceName:        out.CurrentInterfaceName,
			InterfaceDescription: out.CurrentInterfaceDescription,
		})
		if err != nil {
			log.Printf("device observation enforcement dry-run failed: mac=%q switch=%q err=%v", out.MACAddress, out.CurrentSwitchName, err)
		} else {
			s.maybeAutoExecute(ctx, out, decision)
		}
	}

	return nil
}

func (s *Service) UpsertFromRadius(ctx context.Context, input RadiusInventoryInput) error {
	normalizedMAC := normalize.MACAddress(input.MACAddress)
	if normalizedMAC == "" {
		return fmt.Errorf("invalid mac address")
	}
	now := time.Now().UTC()
	observedAt := input.ObservedAt
	if observedAt.IsZero() {
		observedAt = now
	}

	hostname := strings.TrimSpace(input.Hostname)
	vendorClass := strings.TrimSpace(input.VendorClass)
	interfaceName := strings.TrimSpace(input.NASPortID)
	if interfaceName == "" {
		interfaceName = strings.TrimSpace(input.NASPort)
	}
	status, policyAction, policyReason := s.evaluatePolicy(ctx, policyservice.EvaluationInput{
		MACAddress:  normalizedMAC,
		Hostname:    hostname,
		VendorClass: vendorClass,
		SwitchName:  input.SwitchName,
		Interface:   interfaceName,
	}, input.SwitchID, hostname == "", input.PolicyActionOverride, input.PolicyReasonOverride, reasonRadiusObservation)

	device := domain.Device{
		ID:                          uuid.NewString(),
		MACAddress:                  normalizedMAC,
		Hostname:                    hostname,
		VendorClass:                 vendorClass,
		Status:                      status,
		PolicyAction:                policyAction,
		PolicyReason:                policyReason,
		CurrentSwitchID:             strings.TrimSpace(input.SwitchID),
		CurrentSwitchName:           strings.TrimSpace(input.SwitchName),
		CurrentManagementIP:         strings.TrimSpace(input.ManagementIP),
		CurrentBridgePort:           0,
		CurrentIfIndex:              parseOptionalInt(input.NASPort),
		CurrentInterfaceName:        interfaceName,
		CurrentInterfaceDescription: interfaceName,
		CurrentSourceType:           sourceTypeRadius,
		CurrentConfidence:           confidenceAuthoritative,
		FirstSeenAt:                 observedAt,
		LastSeenAt:                  observedAt,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	out, err := s.repository.Upsert(ctx, device)
	if err != nil {
		return err
	}

	if s.enforcement != nil {
		allowed, skipReason := s.acquireEnforcementState(ctx, out)
		if !allowed {
			if skipReason != "" {
				s.logInfo(skipReason, "mac_address", out.MACAddress, "policy_action", out.PolicyAction, "switch_id", out.CurrentSwitchID, "if_index", out.CurrentIfIndex)
			}
			return nil
		}
		decision, err := s.enforcement.RecordDryRun(ctx, enforcementservice.Input{
			MACAddress:           out.MACAddress,
			Hostname:             out.Hostname,
			PolicyAction:         out.PolicyAction,
			PolicyReason:         out.PolicyReason,
			SourceType:           out.CurrentSourceType,
			SwitchID:             out.CurrentSwitchID,
			SwitchName:           out.CurrentSwitchName,
			ManagementIP:         out.CurrentManagementIP,
			BridgePort:           out.CurrentBridgePort,
			IfIndex:              out.CurrentIfIndex,
			InterfaceName:        out.CurrentInterfaceName,
			InterfaceDescription: out.CurrentInterfaceDescription,
		})
		if err != nil {
			log.Printf("device radius enforcement dry-run failed: mac=%q switch=%q err=%v", out.MACAddress, out.CurrentSwitchName, err)
		} else {
			log.Printf("device radius enforcement dry-run recorded: mac=%q method_context_switch=%q policy_action=%q", out.MACAddress, out.CurrentSwitchName, out.PolicyAction)
			s.maybeAutoExecute(ctx, out, decision)
		}
	}

	return nil
}

func (s *Service) evaluatePolicy(ctx context.Context, input policyservice.EvaluationInput, switchID string, mabFallback bool, actionOverride, reasonOverride, defaultReason string) (string, string, string) {
	status := deriveStatus(input.Hostname, switchID)
	policyAction := status
	policyReason := defaultReason

	if s.policies != nil {
		if err := s.policies.EnsureDefaults(ctx); err == nil {
			if result, evalErr := s.policies.Evaluate(ctx, input); evalErr == nil {
				if strings.TrimSpace(result.Status) != "" {
					status = result.Status
				}
				if strings.TrimSpace(result.Action) != "" {
					policyAction = result.Action
				}
				if strings.TrimSpace(result.Reason) != "" {
					policyReason = result.Reason
				}
			}
		}
	}

	if mabFallback && strings.EqualFold(strings.TrimSpace(policyAction), "observed") {
		status = "pending"
		policyAction = "pending"
		policyReason = "RADIUS MAB fallback requires registration workflow"
	}

	if strings.TrimSpace(actionOverride) != "" {
		policyAction = strings.TrimSpace(actionOverride)
		if strings.EqualFold(policyAction, "unknown") || strings.EqualFold(policyAction, "observed") {
			status = "pending"
			policyAction = "pending"
		}
	}
	if strings.TrimSpace(reasonOverride) != "" {
		policyReason = strings.TrimSpace(reasonOverride)
	}

	return status, policyAction, policyReason
}

func deriveStatus(hostname, switchID string) string {
	return "pending"
}

func statusPolicy(status, approvedBy string) (string, string) {
	status = strings.ToLower(strings.TrimSpace(status))
	approvedBy = strings.ToLower(strings.TrimSpace(approvedBy))

	switch status {
	case deviceStatusPending:
		return "guest", "Registration workflow pending"
	case deviceStatusExpired:
		return "guest", "Registration expired and requires renewal"
	case deviceStatusAllowed:
		if approvedBy == "portal" {
			return "active", "Portal registration approved"
		}
		return "active", "Administrative allow"
	case deviceStatusBlocked:
		if approvedBy == "portal" {
			return "blocked", "Portal registration denied"
		}
		return "blocked", "Administrative block"
	case deviceStatusRetired:
		return "blocked", "Device retired"
	default:
		return status, ""
	}
}

func parseOptionalInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func (s *Service) maybeAutoExecute(ctx context.Context, device domain.Device, decision enforcementdomain.Decision) {
	if s.enforcement == nil || !s.autoExecute {
		return
	}

	vlanID, executable := s.targetVLANForPolicyAction(decision.PolicyAction)
	if !executable {
		return
	}

	if vlanID <= 0 {
		s.logInfo("auto enforcement skipped because vlan is not configured", "mac_address", decision.DeviceMACAddress, "policy_action", decision.PolicyAction)
		return
	}

	if method := strings.TrimSpace(decision.SelectedMethod); method == "" || method == "manual-review" || method == "observe-only" {
		s.logInfo("auto enforcement skipped because no executable method is available", "mac_address", decision.DeviceMACAddress, "selected_method", decision.SelectedMethod, "switch_id", decision.SwitchID)
		return
	}
	if strings.EqualFold(strings.TrimSpace(decision.SelectedMethod), "radius-vlan") && !strings.EqualFold(strings.TrimSpace(device.CurrentSourceType), sourceTypeRadius) {
		s.logInfo("auto enforcement skipped because radius-vlan requires radius-origin context", "mac_address", decision.DeviceMACAddress, "switch_id", decision.SwitchID, "source_type", device.CurrentSourceType)
		return
	}

	if s.shouldSkipDuplicate(device, decision, vlanID) {
		s.logInfo("auto enforcement suppressed as duplicate", "mac_address", decision.DeviceMACAddress, "policy_action", decision.PolicyAction, "vlan_id", vlanID, "switch_id", decision.SwitchID, "if_index", decision.IfIndex)
		return
	}

	if s.alreadyInTargetState(device, decision, vlanID) {
		s.logInfo("auto enforcement skipped because target vlan is already active", "mac_address", decision.DeviceMACAddress, "policy_action", decision.PolicyAction, "vlan_id", vlanID, "switch_id", decision.SwitchID, "if_index", decision.IfIndex)
		return
	}

	if decision.RequiresApproval {
		if err := s.enforcement.Approve(ctx, decision.ID); err != nil {
			_ = s.enforcement.MarkStateFailed(ctx, decision.DeviceMACAddress, decision.SwitchID, decision.PolicyAction, decision.IfIndex, vlanID, decision.InterfaceName, decision.ID, decision.SelectedMethod, time.Now().UTC().Add(autoEnforcementSuppressionWindow))
			s.logError("auto enforcement approve failed", "decision_id", decision.ID, "mac_address", decision.DeviceMACAddress, "error", err)
			return
		}
	}
	_ = s.repository.UpdateEnforcementState(ctx, decision.DeviceMACAddress, strings.TrimSpace(decision.PolicyAction), vlanID, "queued", decision.SwitchID, decision.IfIndex, decision.SelectedMethod, time.Now().UTC())

	result, err := s.enforcement.ExecuteDecision(ctx, decision.ID, vlanID, false)
	if err != nil {
		_ = s.enforcement.MarkStateFailed(ctx, decision.DeviceMACAddress, decision.SwitchID, decision.PolicyAction, decision.IfIndex, vlanID, decision.InterfaceName, decision.ID, decision.SelectedMethod, time.Now().UTC().Add(autoEnforcementSuppressionWindow))
		_ = s.repository.UpdateEnforcementState(ctx, decision.DeviceMACAddress, strings.TrimSpace(decision.PolicyAction), vlanID, "failed", decision.SwitchID, decision.IfIndex, decision.SelectedMethod, time.Now().UTC())
		s.logError("auto enforcement execute failed", "decision_id", decision.ID, "mac_address", decision.DeviceMACAddress, "vlan_id", vlanID, "output", result.Output, "error", err)
		return
	}
	_ = s.enforcement.MarkStateExecuted(ctx, decision.DeviceMACAddress, decision.SwitchID, decision.PolicyAction, decision.IfIndex, vlanID, decision.InterfaceName, decision.ID, decision.SelectedMethod)
	_ = s.repository.UpdateEnforcementState(ctx, decision.DeviceMACAddress, strings.TrimSpace(decision.PolicyAction), vlanID, "executed", decision.SwitchID, decision.IfIndex, decision.SelectedMethod, time.Now().UTC())

	s.logInfo("auto enforcement executed", "decision_id", decision.ID, "mac_address", decision.DeviceMACAddress, "policy_action", decision.PolicyAction, "selected_method", decision.SelectedMethod, "vlan_id", vlanID, "switch_id", decision.SwitchID)
}

func (s *Service) targetVLANForPolicyAction(policyAction string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(policyAction)) {
	case "guest":
		return s.guestVLAN, true
	case "blocked":
		return s.quarantineVLAN, true
	default:
		return 0, false
	}
}

func (s *Service) shouldSkipDuplicate(device domain.Device, decision enforcementdomain.Decision, vlanID int) bool {
	if strings.TrimSpace(device.LastEnforcementStatus) == "" || device.LastEnforcementAt.IsZero() {
		return false
	}
	if !sameEnforcementContext(device, decision, vlanID) {
		return false
	}
	if time.Since(device.LastEnforcementAt) > autoEnforcementSuppressionWindow {
		return false
	}

	status := strings.ToLower(strings.TrimSpace(device.LastEnforcementStatus))
	return status == "queued" || status == "executed"
}

func (s *Service) alreadyInTargetState(device domain.Device, decision enforcementdomain.Decision, vlanID int) bool {
	if !sameEnforcementContext(device, decision, vlanID) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(device.LastEnforcementStatus), "executed")
}

func sameEnforcementContext(device domain.Device, decision enforcementdomain.Decision, vlanID int) bool {
	return strings.EqualFold(strings.TrimSpace(device.LastEnforcementAction), strings.TrimSpace(decision.PolicyAction)) &&
		device.LastEnforcementVLAN == vlanID &&
		strings.EqualFold(strings.TrimSpace(device.LastEnforcementSwitchID), strings.TrimSpace(decision.SwitchID)) &&
		device.LastEnforcementIfIndex == decision.IfIndex
}

func (s *Service) shouldSkipDecisionInsert(ctx context.Context, device domain.Device) bool {
	if s.enforcement == nil {
		return false
	}

	vlanID, executable := s.targetVLANForPolicyAction(device.PolicyAction)
	if !executable || vlanID <= 0 {
		return false
	}

	if s.hasAppliedEnforcementState(device, device.PolicyAction, vlanID) {
		s.logInfo(
			"decision insert suppressed because policy is already applied",
			"mac_address", device.MACAddress,
			"policy_action", device.PolicyAction,
			"switch_id", device.CurrentSwitchID,
			"if_index", device.CurrentIfIndex,
			"target_vlan", vlanID,
			"last_enforcement_status", device.LastEnforcementStatus,
		)
		return true
	}

	latest, err := s.enforcement.FindLatestByKey(ctx, device.MACAddress, device.CurrentSwitchID, device.PolicyAction, device.CurrentIfIndex, device.CurrentInterfaceName)
	if err != nil || latest == nil {
		return false
	}

	status := strings.ToLower(strings.TrimSpace(latest.Status))
	if status == "executed" {
		s.logInfo(
			"decision insert suppressed because last enforcement decision is already executed",
			"mac_address", device.MACAddress,
			"policy_action", device.PolicyAction,
			"switch_id", device.CurrentSwitchID,
			"if_index", device.CurrentIfIndex,
			"recent_decision_id", latest.ID,
			"recent_status", latest.Status,
			"target_vlan", vlanID,
		)
		return true
	}

	if time.Since(latest.CreatedAt) > autoEnforcementSuppressionWindow {
		return false
	}

	if status == "queued" || status == "awaiting-approval" {
		s.logInfo("decision insert suppressed as recent duplicate", "mac_address", device.MACAddress, "policy_action", device.PolicyAction, "switch_id", device.CurrentSwitchID, "if_index", device.CurrentIfIndex, "recent_decision_id", latest.ID, "recent_status", latest.Status, "target_vlan", vlanID)
		return true
	}

	return false
}

func (s *Service) acquireEnforcementState(ctx context.Context, device domain.Device) (bool, string) {
	if s.enforcement == nil {
		return false, ""
	}

	vlanID, executable := s.targetVLANForPolicyAction(device.PolicyAction)
	if !executable || vlanID <= 0 {
		_ = s.enforcement.ClearStateForMAC(ctx, device.MACAddress)
		return true, ""
	}

	if s.hasAppliedEnforcementState(device, device.PolicyAction, vlanID) {
		return false, "decision insert suppressed because policy is already applied"
	}

	ok, err := s.enforcement.AcquireState(
		ctx,
		device.MACAddress,
		device.CurrentSwitchID,
		device.PolicyAction,
		device.CurrentIfIndex,
		vlanID,
		device.CurrentInterfaceName,
		time.Now().UTC().Add(autoEnforcementSuppressionWindow),
	)
	if err != nil {
		s.logError("enforcement state acquire failed", "mac_address", device.MACAddress, "policy_action", device.PolicyAction, "switch_id", device.CurrentSwitchID, "if_index", device.CurrentIfIndex, "error", err)
		return true, ""
	}
	if !ok {
		return false, "decision insert suppressed by enforcement state lock"
	}

	return true, ""
}

func (s *Service) hasAppliedEnforcementState(device domain.Device, policyAction string, vlanID int) bool {
	if !sameDeviceEnforcementContext(device, policyAction, vlanID) {
		return false
	}

	status := strings.ToLower(strings.TrimSpace(device.LastEnforcementStatus))
	return status == "executed"
}

func sameDeviceEnforcementContext(device domain.Device, policyAction string, vlanID int) bool {
	return strings.EqualFold(strings.TrimSpace(device.LastEnforcementAction), strings.TrimSpace(policyAction)) &&
		device.LastEnforcementVLAN == vlanID &&
		strings.EqualFold(strings.TrimSpace(device.LastEnforcementSwitchID), strings.TrimSpace(device.CurrentSwitchID)) &&
		device.LastEnforcementIfIndex == device.CurrentIfIndex
}

func (s *Service) logInfo(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Info(msg, args...)
		return
	}
	log.Println(append([]any{msg}, args...)...)
}

func (s *Service) logError(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Error(msg, args...)
		return
	}
	log.Println(append([]any{msg}, args...)...)
}
