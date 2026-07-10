package enforcement

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "nac/internal/domain/enforcement"
	switchasset "nac/internal/domain/switchasset"
	switchportdomain "nac/internal/domain/switchport"
	"nac/internal/snmp"
)

type MockAdapter struct {
	States map[string]domain.PortState
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{States: map[string]domain.PortState{}}
}

func (a *MockAdapter) Name() string { return "mock" }

func (a *MockAdapter) CanHandle(_ switchasset.Switch, _ domain.Request) bool { return true }

func (a *MockAdapter) Preview(_ context.Context, _ switchasset.Switch, port *switchportdomain.Port, request domain.Request) (string, error) {
	return fmt.Sprintf("mock %s vlan=%d port=%s", request.RequestedAction, request.TargetVLAN, interfaceName(port, request)), nil
}

func (a *MockAdapter) ReadState(_ context.Context, _ switchasset.Switch, port *switchportdomain.Port, request domain.Request) (domain.PortState, error) {
	key := adapterStateKey(request)
	if state, ok := a.States[key]; ok {
		return state, nil
	}
	state := domain.PortState{VLANID: request.PreviousVLAN, AdminStatus: "up", OperStatus: "up", PortMode: "access", InterfaceName: interfaceName(port, request), Protected: port != nil && port.EnforcementProtected}
	if port != nil {
		state.VLANID = port.VLANID
		state.PortMode = port.PortMode
		state.Protected = port.EnforcementProtected || port.IsTrunk || port.IsUplink
	}
	return state, nil
}

func (a *MockAdapter) Execute(ctx context.Context, _ switchasset.Switch, port *switchportdomain.Port, request domain.Request) (map[string]any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	state, _ := a.ReadState(ctx, switchasset.Switch{}, port, request)
	if state.Protected {
		return nil, fmt.Errorf("protected port")
	}
	if requiresVLAN(request.RequestedAction) && request.TargetVLAN <= 0 {
		return nil, fmt.Errorf("invalid vlan")
	}
	switch request.RequestedAction {
	case domain.ActionAssignVLAN, domain.ActionAssignRestrictedVLAN, domain.ActionAssignQuarantineVLAN, domain.ActionRestorePreviousState:
		state.VLANID = request.TargetVLAN
	case domain.ActionShutdownPort:
		state.AdminStatus = "down"
	case domain.ActionEnablePort:
		state.AdminStatus = "up"
	case domain.ActionBouncePort:
		state.AdminStatus = "up"
	}
	a.States[adapterStateKey(request)] = state
	return map[string]any{"adapter": "mock", "changed": true}, nil
}

type SNMPAdapter struct {
	service *Service
	client  snmp.Client
}

func NewSNMPAdapter(service *Service) *SNMPAdapter {
	return &SNMPAdapter{service: service, client: snmp.NewClient()}
}

func (a *SNMPAdapter) Name() string { return "snmp" }

func (a *SNMPAdapter) CanHandle(asset switchasset.Switch, _ domain.Request) bool {
	return asset.SupportsSNMPWrite && strings.TrimSpace(asset.ManagementIP) != "" && strings.TrimSpace(asset.SNMPCommunity) != ""
}

func (a *SNMPAdapter) Preview(ctx context.Context, asset switchasset.Switch, port *switchportdomain.Port, request domain.Request) (string, error) {
	if a.service == nil {
		return request.RequestedAction, nil
	}
	switch request.RequestedAction {
	case domain.ActionAssignVLAN, domain.ActionAssignRestrictedVLAN, domain.ActionAssignQuarantineVLAN, domain.ActionRestorePreviousState:
		plan, err := a.service.PreviewSNMPPortVLAN(ctx, asset.ID, bridgePort(port, request), request.CurrentIfIndex, request.CurrentInterfaceName, request.TargetVLAN, true)
		if err != nil {
			return "", err
		}
		return strings.Join(plan.OIDs, "; "), nil
	case domain.ActionBouncePort:
		plan, err := a.service.PreviewSNMPPortBounce(ctx, asset.ID, request.CurrentIfIndex, request.CurrentInterfaceName)
		if err != nil {
			return "", err
		}
		return strings.Join(plan.OIDs, "; "), nil
	case domain.ActionShutdownPort:
		return fmt.Sprintf("set %s%d=2", oidIfAdminStatusPrefix, request.CurrentIfIndex), nil
	case domain.ActionEnablePort:
		return fmt.Sprintf("set %s%d=1", oidIfAdminStatusPrefix, request.CurrentIfIndex), nil
	default:
		return request.RequestedAction, nil
	}
}

func (a *SNMPAdapter) ReadState(ctx context.Context, asset switchasset.Switch, port *switchportdomain.Port, request domain.Request) (domain.PortState, error) {
	if a.client == nil {
		return domain.PortState{}, fmt.Errorf("snmp client is not configured")
	}
	target := snmp.SwitchTarget{Address: asset.ManagementIP, Port: uint16(asset.SNMPPort), Community: asset.SNMPCommunity, Timeout: time.Duration(asset.SNMPTimeoutMS) * time.Millisecond, Retries: asset.SNMPRetries, Vendor: asset.Vendor, Model: asset.Model}
	profile := selectSNMPProfile(asset)
	portIndex := bridgePort(port, request)
	if profile.vlanIndexMode == "ifindex" {
		portIndex = request.CurrentIfIndex
	}
	vlanID := 0
	if requiresVLAN(request.RequestedAction) && portIndex > 0 {
		vlanValue, err := a.client.GetInt(ctx, target, fmt.Sprintf("%s%d", profile.vlanOIDPrefix, portIndex))
		if err == nil {
			vlanID = vlanValue
		}
	}
	adminStatus := "unknown"
	if request.CurrentIfIndex > 0 {
		adminValue, err := a.client.GetInt(ctx, target, fmt.Sprintf("%s%d", oidIfAdminStatusPrefix, request.CurrentIfIndex))
		if err == nil {
			if adminValue == 1 {
				adminStatus = "up"
			} else if adminValue == 2 {
				adminStatus = "down"
			}
		}
	}
	state := domain.PortState{VLANID: vlanID, AdminStatus: adminStatus, OperStatus: "unknown", PortMode: "access", InterfaceName: interfaceName(port, request), Protected: port != nil && isProtectedPort(*port)}
	if port != nil {
		state.PortMode = port.PortMode
		state.Protected = isProtectedPort(*port)
	}
	return state, nil
}

func (a *SNMPAdapter) Execute(ctx context.Context, asset switchasset.Switch, port *switchportdomain.Port, request domain.Request) (map[string]any, error) {
	if a.service == nil {
		return nil, fmt.Errorf("service is not configured")
	}
	switch request.RequestedAction {
	case domain.ActionAssignVLAN, domain.ActionAssignRestrictedVLAN, domain.ActionAssignQuarantineVLAN, domain.ActionRestorePreviousState:
		result, err := a.service.ExecuteSNMPPortVLAN(ctx, asset.ID, bridgePort(port, request), request.CurrentIfIndex, request.CurrentInterfaceName, request.TargetVLAN, true)
		return map[string]any{"output": result.Output, "executed": result.Executed}, err
	case domain.ActionBouncePort:
		result, err := a.service.ExecuteSNMPPortBounce(ctx, asset.ID, request.CurrentIfIndex, request.CurrentInterfaceName)
		return map[string]any{"output": result.Output, "executed": result.Executed}, err
	case domain.ActionShutdownPort:
		return a.setAdminStatus(ctx, asset, request.CurrentIfIndex, 2)
	case domain.ActionEnablePort:
		return a.setAdminStatus(ctx, asset, request.CurrentIfIndex, 1)
	default:
		return map[string]any{"executed": false}, nil
	}
}

func (a *SNMPAdapter) setAdminStatus(ctx context.Context, asset switchasset.Switch, ifIndex int, status int) (map[string]any, error) {
	if a.client == nil {
		return nil, fmt.Errorf("snmp client is not configured")
	}
	if ifIndex <= 0 {
		return nil, fmt.Errorf("if_index is required")
	}
	target := snmp.SwitchTarget{Address: asset.ManagementIP, Port: uint16(asset.SNMPPort), Community: asset.SNMPCommunity, Timeout: time.Duration(asset.SNMPTimeoutMS) * time.Millisecond, Retries: asset.SNMPRetries, Vendor: asset.Vendor, Model: asset.Model}
	err := a.client.SetInt(ctx, target, fmt.Sprintf("%s%d", oidIfAdminStatusPrefix, ifIndex), status)
	return map[string]any{"if_index": ifIndex, "admin_status": status}, err
}

func adapterStateKey(request domain.Request) string {
	return request.SwitchID + "|" + request.PortID + "|" + request.CurrentInterfaceName
}

func interfaceName(port *switchportdomain.Port, request domain.Request) string {
	if port != nil && strings.TrimSpace(port.InterfaceName) != "" {
		return port.InterfaceName
	}
	return request.CurrentInterfaceName
}

func bridgePort(port *switchportdomain.Port, request domain.Request) int {
	if port != nil && port.PortIndex > 0 {
		return port.PortIndex
	}
	return 0
}
