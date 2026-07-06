package device

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	devicedomain "nac/internal/domain/device"
	macipbindingdomain "nac/internal/domain/macipbinding"
	portendpointdomain "nac/internal/domain/portendpoint"
	sessiondomain "nac/internal/domain/session"
	switchportdomain "nac/internal/domain/switchport"
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

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, switchPorts, portEndpoints, nil, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

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

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, switchPorts, nil, sessions, bindings, 0, 0, 0, false, false, 0, 0, 0, false, 0)

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

	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), repo, nil, nil, switchPorts, portEndpoints, nil, nil, 0, 0, 0, false, false, 0, 0, 0, false, 0)

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
	bySwitch []devicedomain.Device
	byPort   []devicedomain.Device
}

func (s *stubDeviceRepository) Upsert(ctx context.Context, device devicedomain.Device) (devicedomain.Device, error) {
	return device, nil
}
func (s *stubDeviceRepository) List(ctx context.Context) ([]devicedomain.Device, error) {
	return nil, nil
}
func (s *stubDeviceRepository) ListByMAC(ctx context.Context, macAddress string) ([]devicedomain.Device, error) {
	return nil, nil
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
