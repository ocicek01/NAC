package device

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/device"
	dhcpevent "nac/internal/domain/dhcpevent"
	enforcementdomain "nac/internal/domain/enforcement"
	macipbindingdomain "nac/internal/domain/macipbinding"
	macobservation "nac/internal/domain/macobservation"
	policydomain "nac/internal/domain/policy"
	portendpointdomain "nac/internal/domain/portendpoint"
	sessiondomain "nac/internal/domain/session"
	switchportdomain "nac/internal/domain/switchport"
	"nac/internal/normalize"
	auditlogservice "nac/internal/service/auditlog"
	enforcementservice "nac/internal/service/enforcement"
	identitysource "nac/internal/service/identitysource"
	policyservice "nac/internal/service/policy"
)

const (
	sourceTypeRadius                 = "radius"
	confidenceAuthoritative          = "authoritative"
	reasonDerivedObservation         = "Derived from current observation"
	reasonRadiusObservation          = "Derived from RADIUS observation"
	autoEnforcementSuppressionWindow = 2 * time.Minute
	enrichmentSourceOpenLDAP         = "openldap_device_registry"
	enrichmentStatusPending          = "pending"
	enrichmentStatusFound            = "enriched"
	enrichmentStatusNotFound         = "not_found"
	enrichmentStatusLookupFailed     = "failed"
	enrichmentStatusLegacyFailed     = "lookup_failed"
	enrichmentEnqueueRetryDelay      = 250 * time.Millisecond
	enrichmentEnqueueRetryLimit      = 20
	enrichmentBackfillBatchSize      = 50
	enrichmentBackfillItemDelay      = 100 * time.Millisecond
	enrichmentBackfillBatchDelay     = 1 * time.Second
)

type PolicyEvaluator interface {
	EnsureDefaults(ctx context.Context) error
	Evaluate(ctx context.Context, input policyservice.EvaluationInput) (policyservice.EvaluationResult, error)
	ListDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]policydomain.Decision, error)
	EnforcementEnabled() bool
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

type DHCPEventResolver interface {
	FindLatestByMAC(ctx context.Context, macAddress string) (*dhcpevent.Event, error)
}

type LDAPDeviceResolver interface {
	LookupByMAC(ctx context.Context, macAddress string) (*identitysource.LDAPDeviceRecord, error)
}

type Service struct {
	repository           domain.Repository
	policies             PolicyEvaluator
	enforcement          EnforcementRecorder
	audit                *auditlogservice.Service
	logger               *slog.Logger
	switchPorts          SwitchPortResolver
	portEndpoints        PortEndpointResolver
	sessions             SessionResolver
	macIPBindings        MACIPBindingResolver
	dhcpEvents           DHCPEventResolver
	ldapDevices          LDAPDeviceResolver
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
	enrichmentQueue      chan string
	enrichmentMu         sync.Mutex
	enrichmentQueued     map[string]struct{}
	backfillMu           sync.Mutex
	backfillRunning      bool
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

func NewService(logger *slog.Logger, repository domain.Repository, policies PolicyEvaluator, enforcement EnforcementRecorder, switchPorts SwitchPortResolver, portEndpoints PortEndpointResolver, sessions SessionResolver, macIPBindings MACIPBindingResolver, dhcpEvents DHCPEventResolver, ldapDevices LDAPDeviceResolver, audit *auditlogservice.Service, registrationVLAN, guestVLAN, quarantineVLAN int, autoExecute bool, ipLearningEnabled bool, ipLearningWait, ipRecheck, portBounceDelay time.Duration, portBounceEnabled bool, maxMACCountForBounce int) *Service {
	service := &Service{
		repository:           repository,
		policies:             policies,
		enforcement:          enforcement,
		audit:                audit,
		logger:               logger,
		switchPorts:          switchPorts,
		portEndpoints:        portEndpoints,
		sessions:             sessions,
		macIPBindings:        macIPBindings,
		dhcpEvents:           dhcpEvents,
		ldapDevices:          ldapDevices,
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
		enrichmentQueue:      make(chan string, 256),
		enrichmentQueued:     map[string]struct{}{},
	}
	service.startEnrichmentWorkers()
	return service
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]domain.Device, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	devices, err := s.repository.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	s.enqueueMissingEnrichment(devices)
	return devices, nil
}

func (s *Service) ListByMAC(ctx context.Context, macAddress string) ([]domain.Device, error) {
	macAddress = normalize.MACAddress(macAddress)
	if macAddress == "" {
		return []domain.Device{}, nil
	}
	devices, err := s.repository.ListByMAC(ctx, macAddress)
	if err != nil {
		return nil, err
	}
	return s.enrichLDAPDevices(ctx, devices, true), nil
}

func (s *Service) ListBySwitch(ctx context.Context, switchID string) ([]domain.Device, error) {
	devices, err := s.repository.ListBySwitch(ctx, switchID)
	if err != nil {
		return nil, err
	}

	devices, err = s.mergeObservedPortDevices(ctx, switchID, 0, devices)
	if err != nil {
		return nil, err
	}
	return s.enrichLDAPDevices(ctx, devices, false), nil
}

func (s *Service) ListBySwitchAndIfIndex(ctx context.Context, switchID string, ifIndex int) ([]domain.Device, error) {
	if ifIndex <= 0 {
		return s.ListBySwitch(ctx, switchID)
	}

	devices, err := s.repository.ListBySwitchAndIfIndex(ctx, switchID, ifIndex)
	if err != nil {
		return nil, err
	}

	devices, err = s.mergeObservedPortDevices(ctx, switchID, ifIndex, devices)
	if err != nil {
		return nil, err
	}
	return s.enrichLDAPDevices(ctx, devices, false), nil
}

func (s *Service) enrichLDAPDevices(ctx context.Context, devices []domain.Device, eager bool) []domain.Device {
	if s.ldapDevices == nil || len(devices) == 0 {
		return devices
	}
	if !eager && len(devices) > 25 {
		return devices
	}

	out := make([]domain.Device, 0, len(devices))
	for _, device := range devices {
		record, err := s.ldapDevices.LookupByMAC(ctx, device.MACAddress)
		if err == nil && record != nil {
			if strings.TrimSpace(record.CommonName) != "" {
				device.LDAPDeviceCN = record.CommonName
			}
			device.RegisteredVendor = firstNonEmpty(device.RegisteredVendor, record.Vendor)
			device.RegisteredOwner = firstNonEmpty(device.RegisteredOwner, record.OwnerName, record.OwnerDN)
			device.OwnerUsername = firstNonEmpty(device.OwnerUsername, record.OwnerUsername)
			device.OwnerDepartment = firstNonEmpty(device.OwnerDepartment, record.Department)
			device.OwnerRole = firstNonEmpty(device.OwnerRole, record.OwnerRole)
			if device.DefaultVLANID == 0 {
				device.DefaultVLANID = record.DefaultVLANID
			}
			device.DefaultVLANName = firstNonEmpty(device.DefaultVLANName, record.DefaultVLANName)
			device.AssignedPolicy = firstNonEmpty(device.AssignedPolicy, record.PolicyName)
			device.EnrichmentSource = firstNonEmpty(device.EnrichmentSource, enrichmentSourceOpenLDAP)
			device.EnrichmentStatus = firstNonEmpty(device.EnrichmentStatus, enrichmentStatusFound)
			device.LDAPOwnerDN = record.OwnerDN
			device.LDAPLocationDN = record.LocationDN
			device.LDAPOwnershipType = record.OwnershipType
			device.LDAPDepartment = record.Department
			device.LDAPAssetTag = record.AssetTag
			device.LDAPPolicyName = record.PolicyName
			device.LDAPVendor = record.Vendor
			device.LDAPModel = record.Model
			device.LDAPDeviceStatus = record.DeviceStatus
			device.LDAPVLANID = record.VLANID
			device.LDAPVLANName = record.VLANName
			device.LDAPDefaultVLANID = record.DefaultVLANID
			device.LDAPDefaultVLANName = record.DefaultVLANName

			if strings.TrimSpace(device.DeviceType) == "" || strings.EqualFold(strings.TrimSpace(device.DeviceType), "unknown") {
				device.DeviceType = record.DeviceType
			}
			if strings.TrimSpace(device.Description) == "" {
				device.Description = record.Description
			}
			if strings.TrimSpace(device.CurrentIPAddress) == "" && strings.TrimSpace(record.IPAddress) != "" {
				device.CurrentIPAddress = record.IPAddress
			}
		}
		out = append(out, device)
	}

	return out
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

	if s.dhcpEvents != nil && (device.CurrentIPAddress == "" || device.Hostname == "") {
		event, err := s.dhcpEvents.FindLatestByMAC(ctx, macAddress)
		if err == nil && event != nil {
			if device.CurrentIPAddress == "" {
				for _, candidate := range []string{event.YourIP, event.RequestedIP, event.ClientIP} {
					candidate = strings.TrimSpace(candidate)
					if candidate != "" {
						device.CurrentIPAddress = candidate
						break
					}
				}
			}
			if device.Hostname == "" {
				device.Hostname = strings.TrimSpace(event.Hostname)
			}
			if !event.ObservedAt.IsZero() && event.ObservedAt.After(device.LastSeenAt) {
				device.LastSeenAt = event.ObservedAt
				device.UpdatedAt = event.ObservedAt
			}
			if endpoint == nil && device.CurrentSourceType == "switch_port" {
				device.CurrentSourceType = "dhcp_event"
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

func (s *Service) ObserveAgentlessEvent(ctx context.Context, input domain.AgentlessObservationInput) (domain.Device, error) {
	macAddress := normalize.MACAddress(input.MACAddress)
	if macAddress == "" {
		return domain.Device{}, fmt.Errorf("invalid mac address")
	}
	if strings.TrimSpace(input.SwitchID) == "" {
		return domain.Device{}, fmt.Errorf("switch id is required")
	}
	if input.IfIndex <= 0 {
		return domain.Device{}, fmt.Errorf("if_index must be greater than zero")
	}

	observedAt := input.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	} else {
		observedAt = observedAt.UTC()
	}
	now := time.Now().UTC()
	trustLevel := deriveTrustLevel(strings.TrimSpace(input.IPAddress), strings.TrimSpace(input.Hostname), strings.TrimSpace(input.DeviceType))
	status, policyAction, policyReason := s.evaluatePolicy(ctx, policyservice.EvaluationInput{
		MACAddress:           macAddress,
		Hostname:             strings.TrimSpace(input.Hostname),
		VendorClass:          strings.TrimSpace(input.VendorClass),
		SwitchID:             strings.TrimSpace(input.SwitchID),
		SwitchName:           strings.TrimSpace(input.SwitchName),
		Interface:            strings.TrimSpace(input.InterfaceName),
		DeviceType:           strings.TrimSpace(input.DeviceType),
		AuthenticationMethod: "agentless",
		TrustLevel:           trustLevel,
		ObservationSource:    strings.TrimSpace(input.SourceType),
	}, strings.TrimSpace(input.SwitchID), false, "", "", "Agentless port observation")

	device := domain.Device{
		ID:                          uuid.NewString(),
		MACAddress:                  macAddress,
		CurrentIPAddress:            strings.TrimSpace(input.IPAddress),
		DeviceType:                  defaultString(strings.TrimSpace(input.DeviceType), "unknown"),
		Hostname:                    strings.TrimSpace(input.Hostname),
		VendorClass:                 strings.TrimSpace(input.VendorClass),
		Status:                      status,
		PolicyAction:                policyAction,
		PolicyReason:                policyReason,
		ClassificationMethod:        "policy-engine",
		TrustLevel:                  trustLevel,
		AuthenticationMethod:        "agentless",
		AuthenticationStatus:        "not_attempted",
		LastPolicyDecision:          policyAction,
		LastPolicyEvaluatedAt:       observedAt,
		CurrentSwitchID:             strings.TrimSpace(input.SwitchID),
		CurrentSwitchName:           strings.TrimSpace(input.SwitchName),
		CurrentManagementIP:         strings.TrimSpace(input.ManagementIP),
		CurrentIfIndex:              input.IfIndex,
		CurrentInterfaceName:        strings.TrimSpace(input.InterfaceName),
		CurrentInterfaceDescription: strings.TrimSpace(input.InterfaceDescription),
		CurrentSourceType:           defaultString(strings.TrimSpace(input.SourceType), "agentless-port-observation"),
		CurrentConfidence:           defaultString(strings.TrimSpace(input.Confidence), "observed"),
		FirstSeenAt:                 observedAt,
		LastSeenAt:                  observedAt,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	stored, err := s.repository.Upsert(ctx, device)
	if err != nil {
		return domain.Device{}, err
	}
	stored = s.refreshPolicyState(ctx, stored)
	s.enqueueEnrichment(stored.MACAddress)

	_, err = s.repository.AddObservation(ctx, domain.Observation{
		ID:          uuid.NewString(),
		DeviceID:    stored.ID,
		MACAddress:  stored.MACAddress,
		IPAddress:   stored.CurrentIPAddress,
		SwitchID:    stored.CurrentSwitchID,
		PortIfIndex: stored.CurrentIfIndex,
		VLANID:      0,
		Source:      stored.CurrentSourceType,
		ObservedAt:  observedAt,
		CreatedAt:   now,
	})
	if err != nil {
		return domain.Device{}, err
	}

	if s.enforcement != nil && !s.shouldSkipDecisionInsert(ctx, stored) {
		decision, err := s.enforcement.RecordDryRun(ctx, enforcementservice.Input{
			MACAddress:           stored.MACAddress,
			Hostname:             stored.Hostname,
			PolicyAction:         stored.PolicyAction,
			PolicyReason:         stored.PolicyReason,
			SourceType:           stored.CurrentSourceType,
			SwitchID:             stored.CurrentSwitchID,
			SwitchName:           stored.CurrentSwitchName,
			ManagementIP:         stored.CurrentManagementIP,
			BridgePort:           stored.CurrentBridgePort,
			IfIndex:              stored.CurrentIfIndex,
			InterfaceName:        stored.CurrentInterfaceName,
			InterfaceDescription: stored.CurrentInterfaceDescription,
		})
		if err == nil {
			stored.LastPolicyDecision = decision.DecisionAction
			s.maybeAutoExecute(ctx, stored, decision)
		}
	}

	return stored, nil
}

func (s *Service) startEnrichmentWorkers() {
	if s == nil || s.ldapDevices == nil || s.enrichmentQueue == nil {
		return
	}
	for i := 0; i < 2; i++ {
		go s.runEnrichmentWorker()
	}
}

func (s *Service) enqueueEnrichment(macAddress string) {
	if s == nil || s.ldapDevices == nil || s.enrichmentQueue == nil {
		return
	}
	macAddress = normalize.MACAddress(macAddress)
	if macAddress == "" {
		return
	}
	s.enrichmentMu.Lock()
	if _, ok := s.enrichmentQueued[macAddress]; ok {
		s.enrichmentMu.Unlock()
		return
	}
	s.enrichmentQueued[macAddress] = struct{}{}
	s.enrichmentMu.Unlock()

	select {
	case s.enrichmentQueue <- macAddress:
		return
	default:
		go s.retryEnqueueEnrichment(macAddress)
	}
}

func (s *Service) retryEnqueueEnrichment(macAddress string) {
	for attempt := 0; attempt < enrichmentEnqueueRetryLimit; attempt++ {
		time.Sleep(enrichmentEnqueueRetryDelay)
		select {
		case s.enrichmentQueue <- macAddress:
			return
		default:
		}
	}

	s.enrichmentMu.Lock()
	delete(s.enrichmentQueued, macAddress)
	s.enrichmentMu.Unlock()
	s.logInfo("device enrichment queue remained full after retries", "mac_address", macAddress)
}

func (s *Service) enqueueMissingEnrichment(devices []domain.Device) {
	if s == nil || s.ldapDevices == nil || len(devices) == 0 {
		return
	}
	for _, device := range devices {
		if !shouldEnqueueEnrichment(device.EnrichmentStatus) {
			continue
		}
		s.enqueueEnrichment(device.MACAddress)
	}
}

func (s *Service) runEnrichmentWorker() {
	for macAddress := range s.enrichmentQueue {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = s.enrichDeviceMetadata(ctx, macAddress)
		cancel()
		s.enrichmentMu.Lock()
		delete(s.enrichmentQueued, macAddress)
		s.enrichmentMu.Unlock()
	}
}

func (s *Service) RunEnrichmentBackfill(ctx context.Context) {
	if s == nil || s.repository == nil || s.ldapDevices == nil {
		return
	}

	s.backfillMu.Lock()
	if s.backfillRunning {
		s.backfillMu.Unlock()
		return
	}
	s.backfillRunning = true
	s.backfillMu.Unlock()
	defer func() {
		s.backfillMu.Lock()
		s.backfillRunning = false
		s.backfillMu.Unlock()
	}()

	startedAt := time.Now().UTC()
	if s.audit != nil {
		_ = s.audit.Record(ctx, "backfill_started", "info", "device_enrichment_backfill", "device-enrichment-backfill", "", "", map[string]any{
			"batch_size": enrichmentBackfillBatchSize,
		})
	}

	processed := 0
	results := map[string]int{}
	for {
		if err := ctx.Err(); err != nil {
			break
		}
		candidates, err := s.repository.ListEnrichmentBackfillCandidates(ctx, enrichmentBackfillBatchSize)
		if err != nil {
			s.logError("device enrichment backfill list failed", "error", err)
			results["failed"]++
			break
		}
		if len(candidates) == 0 {
			break
		}
		for _, candidate := range candidates {
			if err := ctx.Err(); err != nil {
				break
			}
			itemCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			result, itemErr := s.enrichDeviceMetadata(itemCtx, candidate.MACAddress)
			cancel()
			if itemErr != nil && result == "" {
				result = "failed"
			}
			if result == "" {
				result = "skipped"
			}
			if result == "skipped" && strings.TrimSpace(candidate.ID) != "" {
				skippedReason := "device not eligible for enrichment backfill"
				if err := s.repository.UpdateEnrichmentStatusByID(ctx, candidate.ID, enrichmentSourceOpenLDAP, result, skippedReason, time.Now().UTC()); err != nil {
					itemErr = err
					result = "failed"
				} else if itemErr == nil {
					itemErr = errors.New(skippedReason)
				}
			}
			results[result]++
			processed++
			if s.audit != nil {
				statusValue := "info"
				switch result {
				case "enriched":
					statusValue = "success"
				case "not_found", "skipped":
					statusValue = "warning"
				case "failed":
					statusValue = "error"
				}
				payload := map[string]any{"result": result}
				if itemErr != nil {
					payload["error"] = itemErr.Error()
				}
				_ = s.audit.Record(ctx, "backfill_item_processed", statusValue, "device", candidate.ID, candidate.CurrentSwitchID, candidate.MACAddress, payload)
			}
			time.Sleep(enrichmentBackfillItemDelay)
		}
		time.Sleep(enrichmentBackfillBatchDelay)
	}

	if s.audit != nil {
		payload := map[string]any{
			"batch_size":  enrichmentBackfillBatchSize,
			"processed":   processed,
			"duration_ms": time.Since(startedAt).Milliseconds(),
			"results":     results,
		}
		_ = s.audit.Record(ctx, "backfill_completed", "success", "device_enrichment_backfill", "device-enrichment-backfill", "", "", payload)
	}
}

func (s *Service) EnrichDeviceMetadata(ctx context.Context, macAddress string) error {
	_, err := s.enrichDeviceMetadata(ctx, macAddress)
	return err
}

func (s *Service) enrichDeviceMetadata(ctx context.Context, macAddress string) (string, error) {
	if s == nil || s.repository == nil || s.ldapDevices == nil {
		return "skipped", nil
	}
	macAddress = normalize.MACAddress(macAddress)
	if macAddress == "" {
		return "skipped", nil
	}
	devices, err := s.repository.ListByMAC(ctx, macAddress)
	if err != nil {
		return "failed", err
	}
	if len(devices) == 0 {
		return "skipped", nil
	}
	current := devices[0]
	if !shouldEnqueueEnrichment(current.EnrichmentStatus) {
		return "skipped", nil
	}
	currentID := current.ID
	now := time.Now().UTC()
	update := domain.EnrichmentUpdate{
		MACAddress:            macAddress,
		EnrichmentSource:      enrichmentSourceOpenLDAP,
		EnrichedAt:            now,
		ClassificationMethod:  defaultString(current.ClassificationMethod, "policy-engine"),
		LastPolicyEvaluatedAt: now,
	}

	record, lookupErr := s.ldapDevices.LookupByMAC(ctx, macAddress)
	if lookupErr != nil {
		update.EnrichmentStatus = enrichmentStatusLookupFailed
		update.EnrichmentError = strings.TrimSpace(lookupErr.Error())
		update.Status = current.Status
		update.PolicyAction = current.PolicyAction
		update.PolicyReason = current.PolicyReason
		update.LastPolicyDecision = current.LastPolicyDecision
		if _, err := s.repository.UpdateEnrichment(ctx, update); err != nil {
			return "failed", err
		}
		if s.audit != nil {
			_ = s.audit.Record(ctx, "device_enrichment_failed", "error", "device", currentID, current.CurrentSwitchID, macAddress, map[string]any{
				"enrichment_source": enrichmentSourceOpenLDAP,
				"enrichment_status": enrichmentStatusLookupFailed,
				"error":             update.EnrichmentError,
			})
		}
		return "failed", lookupErr
	}

	update.EnrichmentStatus = enrichmentStatusNotFound
	update.EnrichmentError = ""
	result := "not_found"
	if record != nil {
		update = s.applyLDAPDeviceRecord(update, *record)
		update.EnrichmentStatus = enrichmentStatusFound
		result = "enriched"
	}

	status, policyAction, policyReason := s.evaluatePolicy(ctx, policyservice.EvaluationInput{
		MACAddress:        current.MACAddress,
		Hostname:          firstNonEmpty(current.Hostname, update.LDAPDeviceCN),
		VendorClass:       current.VendorClass,
		SwitchID:          current.CurrentSwitchID,
		SwitchName:        current.CurrentSwitchName,
		Interface:         current.CurrentInterfaceName,
		DeviceType:        firstNonEmpty(update.DeviceType, current.DeviceType),
		TrustLevel:        deriveMetadataTrustLevel(current.TrustLevel, update.EnrichmentStatus, update.OwnerUsername, update.DeviceType),
		ObservationSource: current.CurrentSourceType,
		OwnerUsername:     update.OwnerUsername,
		OwnerDepartment:   update.OwnerDepartment,
		OwnerRole:         update.OwnerRole,
		AssignedPolicy:    update.AssignedPolicy,
		RegisteredVendor:  update.RegisteredVendor,
		DefaultVLANID:     update.DefaultVLANID,
		EnrichmentSource:  update.EnrichmentSource,
		EnrichmentStatus:  update.EnrichmentStatus,
	}, current.CurrentSwitchID, false, "", "", "Device registry enrichment")
	status, policyAction, policyReason = s.applyUnknownDeviceGuard(update.EnrichmentStatus, status, policyAction, policyReason)
	update.TrustLevel = deriveMetadataTrustLevel(current.TrustLevel, update.EnrichmentStatus, update.OwnerUsername, update.DeviceType)
	update.Status = status
	update.PolicyAction = policyAction
	update.PolicyReason = policyReason
	update.LastPolicyDecision = policyAction

	updated, err := s.repository.UpdateEnrichment(ctx, update)
	if err != nil {
		return "failed", err
	}
	updated = s.refreshPolicyState(ctx, updated)
	if s.audit != nil {
		action := "device_enrichment_missing"
		statusValue := "warning"
		if update.EnrichmentStatus == enrichmentStatusFound {
			action = "device_enrichment_succeeded"
			statusValue = "success"
		}
		_ = s.audit.Record(ctx, action, statusValue, "device", updated.ID, updated.CurrentSwitchID, updated.MACAddress, map[string]any{
			"enrichment_source": update.EnrichmentSource,
			"enrichment_status": update.EnrichmentStatus,
			"owner_username":    update.OwnerUsername,
			"owner_department":  update.OwnerDepartment,
			"owner_role":        update.OwnerRole,
			"default_vlan_id":   update.DefaultVLANID,
			"assigned_policy":   update.AssignedPolicy,
			"trust_level":       update.TrustLevel,
			"policy_action":     update.PolicyAction,
		})
	}
	return result, nil
}

func (s *Service) applyLDAPDeviceRecord(update domain.EnrichmentUpdate, record identitysource.LDAPDeviceRecord) domain.EnrichmentUpdate {
	update.DeviceType = defaultString(record.DeviceType, update.DeviceType)
	update.RegisteredVendor = strings.TrimSpace(record.Vendor)
	update.Description = strings.TrimSpace(record.Description)
	update.RegisteredOwner = firstNonEmpty(record.OwnerName, record.OwnerDN)
	update.OwnerUsername = strings.TrimSpace(record.OwnerUsername)
	update.OwnerDepartment = strings.TrimSpace(record.Department)
	update.OwnerRole = strings.TrimSpace(record.OwnerRole)
	update.DefaultVLANID = record.DefaultVLANID
	update.DefaultVLANName = strings.TrimSpace(record.DefaultVLANName)
	update.AssignedPolicy = strings.TrimSpace(record.PolicyName)
	update.LDAPDeviceCN = strings.TrimSpace(record.CommonName)
	update.LDAPOwnerDN = strings.TrimSpace(record.OwnerDN)
	update.LDAPLocationDN = strings.TrimSpace(record.LocationDN)
	update.LDAPOwnershipType = strings.TrimSpace(record.OwnershipType)
	update.LDAPDepartment = strings.TrimSpace(record.Department)
	update.LDAPAssetTag = strings.TrimSpace(record.AssetTag)
	update.LDAPPolicyName = strings.TrimSpace(record.PolicyName)
	update.LDAPVendor = strings.TrimSpace(record.Vendor)
	update.LDAPModel = strings.TrimSpace(record.Model)
	update.LDAPDeviceStatus = strings.TrimSpace(record.DeviceStatus)
	update.LDAPVLANID = record.VLANID
	update.LDAPVLANName = strings.TrimSpace(record.VLANName)
	update.LDAPDefaultVLANID = record.DefaultVLANID
	update.LDAPDefaultVLANName = strings.TrimSpace(record.DefaultVLANName)
	return update
}

func (s *Service) applyUnknownDeviceGuard(enrichmentStatus, status, policyAction, policyReason string) (string, string, string) {
	if enrichmentStatus == enrichmentStatusFound {
		return status, policyAction, policyReason
	}
	if strings.EqualFold(strings.TrimSpace(policyAction), "active") {
		return "unknown", "unknown", "Device registry enrichment missing"
	}
	return status, policyAction, policyReason
}

func (s *Service) RecordSophosIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error {
	macAddress = normalize.MACAddress(macAddress)
	if macAddress == "" {
		return fmt.Errorf("invalid mac address")
	}
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	}
	return s.repository.UpdateSophosIdentity(ctx, macAddress, strings.TrimSpace(username), strings.TrimSpace(ipAddress), seenAt.UTC())
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
	out = s.refreshPolicyState(ctx, out)
	s.enqueueEnrichment(out.MACAddress)

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
	out = s.refreshPolicyState(ctx, out)
	s.enqueueEnrichment(out.MACAddress)

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
	if s.policies != nil && !s.policies.EnforcementEnabled() {
		s.logInfo("auto enforcement skipped because policy engine is in dry-run mode", "mac_address", decision.DeviceMACAddress, "policy_action", decision.PolicyAction, "switch_id", decision.SwitchID)
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

func shouldEnqueueEnrichment(status string) bool {
	status = strings.TrimSpace(status)
	return status == "" || strings.EqualFold(status, enrichmentStatusLookupFailed) || strings.EqualFold(status, enrichmentStatusLegacyFailed)
}

func deriveTrustLevel(ipAddress, hostname, deviceType string) string {
	if strings.TrimSpace(deviceType) != "" && !strings.EqualFold(strings.TrimSpace(deviceType), "unknown") && strings.TrimSpace(ipAddress) != "" && strings.TrimSpace(hostname) != "" {
		return "medium"
	}
	if strings.TrimSpace(ipAddress) != "" || strings.TrimSpace(hostname) != "" {
		return "low"
	}
	return "unknown"
}

func deriveMetadataTrustLevel(current, enrichmentStatus, ownerUsername, deviceType string) string {
	if strings.EqualFold(strings.TrimSpace(enrichmentStatus), enrichmentStatusFound) && strings.TrimSpace(ownerUsername) != "" && strings.TrimSpace(deviceType) != "" && !strings.EqualFold(strings.TrimSpace(deviceType), "unknown") {
		return "high"
	}
	if strings.EqualFold(strings.TrimSpace(enrichmentStatus), enrichmentStatusFound) {
		return "medium"
	}
	if strings.TrimSpace(current) != "" {
		return strings.TrimSpace(current)
	}
	return "unknown"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func (s *Service) refreshPolicyState(ctx context.Context, device domain.Device) domain.Device {
	if s == nil || s.repository == nil || s.policies == nil || strings.TrimSpace(device.ID) == "" {
		return device
	}
	if _, err := s.EvaluatePolicyByID(ctx, device.ID); err != nil {
		s.logError("policy reevaluation after device persist failed", "device_id", device.ID, "mac_address", device.MACAddress, "error", err)
		return device
	}
	refreshed, err := s.repository.FindByID(ctx, device.ID)
	if err != nil {
		s.logError("device reload after policy reevaluation failed", "device_id", device.ID, "mac_address", device.MACAddress, "error", err)
		return device
	}
	if refreshed == nil {
		return device
	}
	return *refreshed
}

func (s *Service) FindByID(ctx context.Context, id string) (*domain.Device, error) {
	if s == nil || s.repository == nil {
		return nil, nil
	}
	return s.repository.FindByID(ctx, strings.TrimSpace(id))
}

func (s *Service) ListPolicyDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]policydomain.Decision, error) {
	if s == nil || s.policies == nil {
		return []policydomain.Decision{}, nil
	}
	return s.policies.ListDecisionsByDevice(ctx, strings.TrimSpace(deviceID), limit, offset)
}

func (s *Service) EvaluatePolicyByID(ctx context.Context, deviceID string) (policyservice.EvaluationResult, error) {
	if s == nil || s.repository == nil || s.policies == nil {
		return policyservice.EvaluationResult{}, fmt.Errorf("policy evaluation is not configured")
	}
	device, err := s.repository.FindByID(ctx, strings.TrimSpace(deviceID))
	if err != nil {
		return policyservice.EvaluationResult{}, err
	}
	if device == nil {
		return policyservice.EvaluationResult{}, fmt.Errorf("device not found")
	}
	input := policyservice.EvaluationInput{DeviceID: device.ID, MACAddress: device.MACAddress, IPAddress: device.CurrentIPAddress, Hostname: device.Hostname, VendorClass: device.VendorClass, SwitchID: device.CurrentSwitchID, SwitchName: device.CurrentSwitchName, SwitchManagementIP: device.CurrentManagementIP, Interface: device.CurrentInterfaceName, DeviceType: device.DeviceType, FirstSeenAt: device.FirstSeenAt, LastSeenAt: device.LastSeenAt, KnownDevice: strings.TrimSpace(device.RegisteredOwner) != "" || strings.TrimSpace(device.OwnerUsername) != "", ManagedDevice: strings.TrimSpace(device.AssignedPolicy) != "" || device.DefaultVLANID > 0, EnrichmentSource: device.EnrichmentSource, EnrichmentStatus: device.EnrichmentStatus, RegisteredOwner: device.RegisteredOwner, OwnerUsername: device.OwnerUsername, OwnerDepartment: device.OwnerDepartment, OwnerRole: device.OwnerRole, AssignedPolicy: device.AssignedPolicy, RegisteredVendor: device.RegisteredVendor, DefaultVLANID: device.DefaultVLANID, LDAPRegistryMatch: strings.EqualFold(strings.TrimSpace(device.EnrichmentStatus), enrichmentStatusFound), PreviousQuarantine: strings.EqualFold(strings.TrimSpace(device.LastEnforcementAction), "blocked") || strings.EqualFold(strings.TrimSpace(device.LastEnforcementStatus), "failed"), LastPolicyDecision: device.LastPolicyDecision, LastEnforcementAction: device.LastEnforcementAction, LastEnforcementStatus: device.LastEnforcementStatus, AuthenticationMethod: device.AuthenticationMethod, ObservationSource: device.CurrentSourceType}
	if s.switchPorts != nil && strings.TrimSpace(device.CurrentSwitchID) != "" && device.CurrentIfIndex > 0 {
		if port, portErr := s.switchPorts.FindBySwitchIfIndex(ctx, device.CurrentSwitchID, device.CurrentIfIndex); portErr == nil && port != nil {
			input.PortID = port.ID
			input.PortProfile = derivePortProfile(*port)
			input.CurrentVLAN = port.VLANID
			if port.MACCount > 1 {
				input.PortChangeCount = port.MACCount - 1
			}
		}
	}
	result, err := s.policies.Evaluate(ctx, input)
	if err != nil {
		return policyservice.EvaluationResult{}, err
	}
	trustLevel := result.Action
	if result.TrustScore >= 80 {
		trustLevel = "high"
	} else if result.TrustScore >= 60 {
		trustLevel = "medium"
	} else if result.TrustScore >= 40 {
		trustLevel = "low"
	} else {
		trustLevel = "critical"
	}
	lastPolicyDecision := strings.TrimSpace(result.Action)
	if lastPolicyDecision == "" {
		lastPolicyDecision = strings.TrimSpace(result.DecisionType)
	}
	if lastPolicyDecision == "" {
		lastPolicyDecision = strings.TrimSpace(result.PolicyName)
	}
	if err := s.repository.UpdatePolicyEvaluationByID(ctx, device.ID, result.Status, result.Action, result.Explanation, trustLevel, lastPolicyDecision, time.Now().UTC()); err != nil {
		return policyservice.EvaluationResult{}, err
	}
	if s.audit != nil {
		status := "info"
		action := "policy_enforcement_skipped"
		payload := map[string]any{"decision_id": result.DecisionID, "decision_type": result.DecisionType, "target_vlan": result.TargetVLAN, "dry_run": result.DryRun, "reason_codes": result.ReasonCodes}
		if !result.DryRun && s.autoExecute {
			action = "policy_enforcement_requested"
		}
		if result.DryRun {
			payload["reason"] = "dry_run"
		}
		_ = s.audit.Record(ctx, action, status, "device", device.ID, device.CurrentSwitchID, device.MACAddress, payload)
	}
	return result, nil
}

func derivePortProfile(port switchportdomain.Port) string {
	candidates := []string{port.InterfaceAlias, port.InterfaceDescription, port.PortLabel}
	for _, item := range candidates {
		item = strings.ToLower(strings.TrimSpace(item))
		switch {
		case strings.Contains(item, "camera"):
			return "camera"
		case strings.Contains(item, "printer"):
			return "printer"
		case strings.Contains(item, "voice") || strings.Contains(item, "phone"):
			return "voice"
		case strings.Contains(item, "ap") || strings.Contains(item, "wireless"):
			return "access-point"
		}
	}
	return ""
}
