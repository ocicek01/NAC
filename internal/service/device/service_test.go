package device

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	devicedomain "nac/internal/domain/device"
	dhcpeventdomain "nac/internal/domain/dhcpevent"
	macipbindingdomain "nac/internal/domain/macipbinding"
	portendpointdomain "nac/internal/domain/portendpoint"
	sessiondomain "nac/internal/domain/session"
	switchportdomain "nac/internal/domain/switchport"
	identitysource "nac/internal/service/identitysource"
	policyservice "nac/internal/service/policy"
)

func TestListBySwitchAndIfIndexFallsBackToObservedPortData(t *testing.T) {
	repo := &stubDeviceRepository{}
	switchPorts := &stubSwitchPortResolver{
		byIfIndex: map[int]switchportdomain.Port{
			32: {
				SwitchID:             "sw-1",
				IfIndex:              32,
				InterfaceName:        "32",
				InterfaceDescription: "32",
				MACAddresses:         []string{"30:9C:23:9B:97:AA"},
				UpdatedAt:            time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	portEndpoints := &stubPortEndpointResolver{
		items: []portendpointdomain.Endpoint{
			{
				SwitchID:         "sw-1",
				PortIfIndex:      32,
				MACAddress:       "30:9C:23:9B:97:AA",
				IPAddress:        "10.6.8.10",
				Hostname:         "pc-32",
				SourceConfidence: "strong",
				LastSeenAt:       time.Date(2026, 7, 6, 12, 5, 0, 0, time.UTC),
				CreatedAt:        time.Date(2026, 7, 6, 11, 55, 0, 0, time.UTC),
			},
		},
	}

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, switchPorts, portEndpoints, nil, nil, nil, nil, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	devices, err := service.ListBySwitchAndIfIndex(context.Background(), "sw-1", 32)
	if err != nil {
		t.Fatalf("ListBySwitchAndIfIndex returned error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].MACAddress != "30:9C:23:9B:97:AA" {
		t.Fatalf("expected MAC fallback, got %q", devices[0].MACAddress)
	}
	if devices[0].CurrentIfIndex != 32 {
		t.Fatalf("expected if_index 32, got %d", devices[0].CurrentIfIndex)
	}
	if devices[0].CurrentIPAddress != "10.6.8.10" {
		t.Fatalf("expected IP fallback, got %q", devices[0].CurrentIPAddress)
	}
	if devices[0].Hostname != "pc-32" {
		t.Fatalf("expected hostname fallback, got %q", devices[0].Hostname)
	}
}

func TestListBySwitchAndIfIndexFallsBackToRadiusSessionAndBindingData(t *testing.T) {
	repo := &stubDeviceRepository{}
	switchPorts := &stubSwitchPortResolver{
		byIfIndex: map[int]switchportdomain.Port{
			32: {
				SwitchID:             "sw-1",
				IfIndex:              32,
				InterfaceName:        "32",
				InterfaceDescription: "32",
				MACAddresses:         []string{"30:9C:23:9B:97:AA"},
				UpdatedAt:            time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	sessions := &stubSessionResolver{
		byMAC: map[string]*sessiondomain.Session{
			"30:9C:23:9B:97:AA|sw-1": {
				MACAddress:   "30:9C:23:9B:97:AA",
				SwitchID:     "sw-1",
				SwitchName:   "sw-1-name",
				ManagementIP: "10.6.8.19",
				IPAddress:    "10.6.8.10",
				Hostname:     "pc-32",
				Username:     "ocicek",
				LastSeenAt:   time.Date(2026, 7, 6, 12, 6, 0, 0, time.UTC),
			},
		},
	}
	bindings := &stubMACIPBindingResolver{
		byMAC: map[string]*macipbindingdomain.Binding{
			"30:9C:23:9B:97:AA|sw-1": {
				MACAddress: "30:9C:23:9B:97:AA",
				SwitchID:   "sw-1",
				IPAddress:  "10.6.8.10",
				Hostname:   "pc-32",
			},
		},
	}

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, switchPorts, nil, sessions, bindings, nil, nil, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	devices, err := service.ListBySwitchAndIfIndex(context.Background(), "sw-1", 32)
	if err != nil {
		t.Fatalf("ListBySwitchAndIfIndex returned error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].CurrentIPAddress != "10.6.8.10" {
		t.Fatalf("expected IP from fallback sources, got %q", devices[0].CurrentIPAddress)
	}
	if devices[0].Hostname != "pc-32" {
		t.Fatalf("expected hostname from fallback sources, got %q", devices[0].Hostname)
	}
	if devices[0].IdentityUsername != "ocicek" {
		t.Fatalf("expected username from radius session, got %q", devices[0].IdentityUsername)
	}
}
func TestListBySwitchAndIfIndexFallsBackToDHCPEventData(t *testing.T) {
	repo := &stubDeviceRepository{}
	switchPorts := &stubSwitchPortResolver{
		byIfIndex: map[int]switchportdomain.Port{
			32: {
				SwitchID:             "sw-1",
				IfIndex:              32,
				InterfaceName:        "32",
				InterfaceDescription: "32",
				MACAddresses:         []string{"30:9C:23:9B:97:AA"},
			},
		},
	}
	dhcpEvents := &stubDHCPEventResolver{
		byMAC: map[string]*dhcpeventdomain.Event{
			"30:9C:23:9B:97:AA": {
				MACAddress: "30:9C:23:9B:97:AA",
				YourIP:     "10.6.8.10",
				Hostname:   "pc-32",
				ObservedAt: time.Date(2026, 7, 6, 12, 7, 0, 0, time.UTC),
			},
		},
	}

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, switchPorts, nil, nil, nil, dhcpEvents, nil, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	devices, err := service.ListBySwitchAndIfIndex(context.Background(), "sw-1", 32)
	if err != nil {
		t.Fatalf("ListBySwitchAndIfIndex returned error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].CurrentIPAddress != "10.6.8.10" {
		t.Fatalf("expected IP from dhcp event, got %q", devices[0].CurrentIPAddress)
	}
	if devices[0].Hostname != "pc-32" {
		t.Fatalf("expected hostname from dhcp event, got %q", devices[0].Hostname)
	}
}
func TestListUsesPaginationWithoutLDAPEnrichment(t *testing.T) {
	repo := &stubDeviceRepository{
		list: []devicedomain.Device{{MACAddress: "AA:BB:CC:DD:EE:FF"}},
	}
	ldap := &stubLDAPDeviceResolver{}

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, nil, nil, nil, nil, nil, ldap, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	devices, err := service.List(context.Background(), 25, 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if repo.listCalls != 1 {
		t.Fatalf("expected repository List to be called once, got %d", repo.listCalls)
	}
	if repo.listLimit != 25 || repo.listOffset != 10 {
		t.Fatalf("expected limit/offset 25/10, got %d/%d", repo.listLimit, repo.listOffset)
	}
	if ldap.lookupCalls != 0 {
		t.Fatalf("expected no LDAP lookups for list endpoint, got %d", ldap.lookupCalls)
	}
}

func TestListQueuesMissingEnrichmentWithoutBlocking(t *testing.T) {
	repo := &stubDeviceRepository{
		list: []devicedomain.Device{{MACAddress: "AA:BB:CC:DD:EE:FF"}},
		byMAC: map[string][]devicedomain.Device{
			"AA:BB:CC:DD:EE:FF": {{ID: "dev-1", MACAddress: "AA:BB:CC:DD:EE:FF", PolicyAction: "unknown", PolicyReason: "unknown", Status: "unknown"}},
		},
	}
	ldap := &stubLDAPDeviceResolver{}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, stubPolicyEvaluator{result: policyservice.EvaluationResult{Status: "unknown", Action: "unknown", Reason: "matched"}}, nil, nil, nil, nil, nil, nil, ldap, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	devices, err := service.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if repo.lastEnrichment.MACAddress == "AA:BB:CC:DD:EE:FF" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if repo.lastEnrichment.MACAddress != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("expected background enrichment to be queued for listed device")
	}
	if ldap.lookupCalls == 0 {
		t.Fatalf("expected background worker to perform LDAP lookup")
	}
}

func TestListRequeuesLookupFailedEnrichment(t *testing.T) {
	repo := &stubDeviceRepository{
		list: []devicedomain.Device{{MACAddress: "AA:BB:CC:DD:EE:11", EnrichmentStatus: enrichmentStatusLookupFailed}},
		byMAC: map[string][]devicedomain.Device{
			"AA:BB:CC:DD:EE:11": {{ID: "dev-2", MACAddress: "AA:BB:CC:DD:EE:11", PolicyAction: "unknown", PolicyReason: "unknown", Status: "unknown"}},
		},
	}
	ldap := &stubLDAPDeviceResolver{}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, stubPolicyEvaluator{result: policyservice.EvaluationResult{Status: "unknown", Action: "unknown", Reason: "matched"}}, nil, nil, nil, nil, nil, nil, ldap, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	_, err := service.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if repo.lastEnrichment.MACAddress == "AA:BB:CC:DD:EE:11" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if repo.lastEnrichment.MACAddress != "AA:BB:CC:DD:EE:11" {
		t.Fatalf("expected lookup_failed device to be requeued")
	}
}

func TestShouldEnqueueEnrichment(t *testing.T) {
	if !shouldEnqueueEnrichment("") {
		t.Fatalf("expected blank enrichment status to be queued")
	}
	if !shouldEnqueueEnrichment(enrichmentStatusLookupFailed) {
		t.Fatalf("expected lookup_failed enrichment status to be queued")
	}
	if shouldEnqueueEnrichment(enrichmentStatusNotFound) {
		t.Fatalf("expected not_found enrichment status to be skipped")
	}
}

func TestEnrichDeviceMetadataAppliesOwnerMetadataAndPolicyGuard(t *testing.T) {
	repo := &stubDeviceRepository{
		byMAC: map[string][]devicedomain.Device{
			"AA:BB:CC:DD:EE:FF": {{
				ID:                   "dev-1",
				MACAddress:           "AA:BB:CC:DD:EE:FF",
				Hostname:             "edge-ap",
				CurrentSwitchID:      "sw-1",
				CurrentSwitchName:    "sw-1",
				CurrentInterfaceName: "10",
				CurrentSourceType:    "snmp",
				PolicyAction:         "active",
				PolicyReason:         "old",
				Status:               "active",
				TrustLevel:           "low",
			}},
		},
	}
	ldap := &stubLDAPDeviceResolver{
		records: map[string]*identitysource.LDAPDeviceRecord{
			"AA:BB:CC:DD:EE:FF": {
				OwnerName:       "Network Team",
				OwnerUsername:   "netops",
				OwnerRole:       "staff",
				Department:      "IT",
				DeviceType:      "access-point",
				Vendor:          "Cisco",
				DefaultVLANID:   120,
				DefaultVLANName: "corp",
				PolicyName:      "managed-ap",
			},
		},
	}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, nil, nil, nil, nil, nil, ldap, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	if err := service.EnrichDeviceMetadata(context.Background(), "AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("EnrichDeviceMetadata returned error: %v", err)
	}
	if repo.lastEnrichment.OwnerUsername != "netops" {
		t.Fatalf("expected owner username netops, got %q", repo.lastEnrichment.OwnerUsername)
	}
	if repo.lastEnrichment.EnrichmentStatus != enrichmentStatusFound {
		t.Fatalf("expected enrichment status found, got %q", repo.lastEnrichment.EnrichmentStatus)
	}
	if repo.lastEnrichment.TrustLevel != "high" {
		t.Fatalf("expected trust level high, got %q", repo.lastEnrichment.TrustLevel)
	}
	if repo.lastEnrichment.PolicyAction != "pending" {
		t.Fatalf("expected policy action pending with default policy flow, got %q", repo.lastEnrichment.PolicyAction)
	}
}

func TestEnrichDeviceMetadataMarksNotFoundWithoutError(t *testing.T) {
	repo := &stubDeviceRepository{
		byMAC: map[string][]devicedomain.Device{
			"AA:BB:CC:DD:EE:11": {{ID: "dev-2", MACAddress: "AA:BB:CC:DD:EE:11", PolicyAction: "unknown", PolicyReason: "unknown", Status: "unknown"}},
		},
	}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, stubPolicyEvaluator{result: policyservice.EvaluationResult{Status: "active", Action: "active", Reason: "matched"}}, nil, nil, nil, nil, nil, nil, &stubLDAPDeviceResolver{}, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	if err := service.EnrichDeviceMetadata(context.Background(), "AA:BB:CC:DD:EE:11"); err != nil {
		t.Fatalf("EnrichDeviceMetadata returned error: %v", err)
	}
	if repo.lastEnrichment.EnrichmentStatus != enrichmentStatusNotFound {
		t.Fatalf("expected not_found status, got %q", repo.lastEnrichment.EnrichmentStatus)
	}
	if repo.lastEnrichment.PolicyAction != "unknown" {
		t.Fatalf("expected unknown policy action for unenriched device, got %q", repo.lastEnrichment.PolicyAction)
	}
}

func TestListBySwitchKeepsRealInventoryAndAppendsObservedFallbacks(t *testing.T) {
	repo := &stubDeviceRepository{
		bySwitch: []devicedomain.Device{
			{
				ID:               "real-1",
				MACAddress:       "AA:BB:CC:DD:EE:01",
				CurrentSwitchID:  "sw-1",
				CurrentIfIndex:   10,
				CurrentIPAddress: "10.6.8.20",
			},
		},
	}
	switchPorts := &stubSwitchPortResolver{
		list: []switchportdomain.Port{
			{SwitchID: "sw-1", IfIndex: 10, InterfaceName: "10", MACAddresses: []string{"AA:BB:CC:DD:EE:01"}},
			{SwitchID: "sw-1", IfIndex: 32, InterfaceName: "32", MACAddresses: []string{"30:9C:23:9B:97:AA"}},
		},
	}
	portEndpoints := &stubPortEndpointResolver{}

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, switchPorts, portEndpoints, nil, nil, nil, nil, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

	devices, err := service.ListBySwitch(context.Background(), "sw-1")
	if err != nil {
		t.Fatalf("ListBySwitch returned error: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}

	foundFallback := false
	for _, device := range devices {
		if device.MACAddress == "30:9C:23:9B:97:AA" && device.CurrentIfIndex == 32 {
			foundFallback = true
		}
	}
	if !foundFallback {
		t.Fatalf("expected synthesized fallback device for port 32")
	}
}

type stubDeviceRepository struct {
	list           []devicedomain.Device
	byMAC          map[string][]devicedomain.Device
	bySwitch       []devicedomain.Device
	byPort         []devicedomain.Device
	listLimit      int
	listOffset     int
	listCalls      int
	lastEnrichment devicedomain.EnrichmentUpdate
}

func (s *stubDeviceRepository) Upsert(ctx context.Context, device devicedomain.Device) (devicedomain.Device, error) {
	return device, nil
}
func (s *stubDeviceRepository) List(ctx context.Context, limit, offset int) ([]devicedomain.Device, error) {
	s.listCalls++
	s.listLimit = limit
	s.listOffset = offset
	return append([]devicedomain.Device{}, s.list...), nil
}
func (s *stubDeviceRepository) ListByMAC(ctx context.Context, macAddress string) ([]devicedomain.Device, error) {
	items, ok := s.byMAC[macAddress]
	if !ok {
		return nil, nil
	}
	return append([]devicedomain.Device{}, items...), nil
}
func (s *stubDeviceRepository) ListBySwitch(ctx context.Context, switchID string) ([]devicedomain.Device, error) {
	return append([]devicedomain.Device{}, s.bySwitch...), nil
}
func (s *stubDeviceRepository) ListBySwitchAndIfIndex(ctx context.Context, switchID string, ifIndex int) ([]devicedomain.Device, error) {
	return append([]devicedomain.Device{}, s.byPort...), nil
}
func (s *stubDeviceRepository) UpdateStatus(ctx context.Context, macAddress, status, approvedBy, policyAction, policyReason string, approvedAt, expiresAt time.Time) (devicedomain.Device, error) {
	return devicedomain.Device{}, nil
}
func (s *stubDeviceRepository) AddIdentitySnapshot(ctx context.Context, snapshot devicedomain.IdentitySnapshot) (devicedomain.IdentitySnapshot, error) {
	return snapshot, nil
}
func (s *stubDeviceRepository) AddObservation(ctx context.Context, observation devicedomain.Observation) (devicedomain.Observation, error) {
	return observation, nil
}
func (s *stubDeviceRepository) UpdateEnrichment(ctx context.Context, update devicedomain.EnrichmentUpdate) (devicedomain.Device, error) {
	s.lastEnrichment = update
	return devicedomain.Device{ID: "updated", MACAddress: update.MACAddress, CurrentSwitchID: "sw-1"}, nil
}
func (s *stubDeviceRepository) UpdateSophosIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error {
	return nil
}
func (s *stubDeviceRepository) UpdateEnforcementState(ctx context.Context, macAddress, action string, vlanID int, status, switchID string, ifIndex int, method string, enforcedAt time.Time) error {
	return nil
}
func (s *stubDeviceRepository) UpdateIPLearningState(ctx context.Context, macAddress, switchID string, ifIndex int, state string, startedAt, learnedAt, lastBounceAt time.Time) error {
	return nil
}

type stubSwitchPortResolver struct {
	list      []switchportdomain.Port
	byIfIndex map[int]switchportdomain.Port
}

func (s *stubSwitchPortResolver) ListBySwitch(ctx context.Context, switchID string) ([]switchportdomain.Port, error) {
	return append([]switchportdomain.Port{}, s.list...), nil
}
func (s *stubSwitchPortResolver) FindBySwitchIfIndex(ctx context.Context, switchID string, ifIndex int) (*switchportdomain.Port, error) {
	port, ok := s.byIfIndex[ifIndex]
	if !ok {
		return nil, nil
	}
	copyPort := port
	return &copyPort, nil
}

type stubPortEndpointResolver struct {
	items []portendpointdomain.Endpoint
}

func (s *stubPortEndpointResolver) ListBySwitch(ctx context.Context, switchID string) ([]portendpointdomain.Endpoint, error) {
	return append([]portendpointdomain.Endpoint{}, s.items...), nil
}

type stubSessionResolver struct {
	byMAC map[string]*sessiondomain.Session
}

func (s *stubSessionResolver) FindLatestActiveByMACSwitch(ctx context.Context, macAddress, switchID string) (*sessiondomain.Session, error) {
	item, ok := s.byMAC[macAddress+"|"+switchID]
	if !ok {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

type stubMACIPBindingResolver struct {
	byMAC map[string]*macipbindingdomain.Binding
}

func (s *stubMACIPBindingResolver) FindLatestByMACSwitch(ctx context.Context, macAddress, switchID string) (*macipbindingdomain.Binding, error) {
	item, ok := s.byMAC[macAddress+"|"+switchID]
	if !ok {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

type stubDHCPEventResolver struct {
	byMAC map[string]*dhcpeventdomain.Event
}

func (s *stubDHCPEventResolver) FindLatestByMAC(ctx context.Context, macAddress string) (*dhcpeventdomain.Event, error) {
	item, ok := s.byMAC[macAddress]
	if !ok {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

type stubLDAPDeviceResolver struct {
	lookupCalls int
	records     map[string]*identitysource.LDAPDeviceRecord
	err         error
}

func (s *stubLDAPDeviceResolver) LookupByMAC(ctx context.Context, macAddress string) (*identitysource.LDAPDeviceRecord, error) {
	s.lookupCalls++
	if s.err != nil {
		return nil, s.err
	}
	if s.records == nil {
		return nil, nil
	}
	return s.records[macAddress], nil
}

type stubPolicyEvaluator struct {
	result policyservice.EvaluationResult
}

func (s stubPolicyEvaluator) EnsureDefaults(ctx context.Context) error {
	return nil
}

func (s stubPolicyEvaluator) Evaluate(ctx context.Context, input policyservice.EvaluationInput) (policyservice.EvaluationResult, error) {
	return s.result, nil
}
