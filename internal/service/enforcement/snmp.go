package enforcement

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	domain "nac/internal/domain/enforcement"
	switchasset "nac/internal/domain/switchasset"
	"nac/internal/snmp"
)

const (
	oidIfAdminStatusPrefix = ".1.3.6.1.2.1.2.2.1.7."
	oidCiscoVLANPrefix     = ".1.3.6.1.4.1.9.9.68.1.2.2.1.2."
	oidQBridgePVIDPrefix   = ".1.3.6.1.2.1.17.7.1.4.5.1.1."
)

type SNMPEnforcer interface {
	PreviewPortVLAN(ctx context.Context, asset switchasset.Switch, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANPlan, error)
	ExecutePortVLAN(ctx context.Context, asset switchasset.Switch, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANExecutionResult, error)
	PreviewPortBounce(ctx context.Context, asset switchasset.Switch, ifIndex int, interfaceName string) (domain.VLANPlan, error)
	ExecutePortBounce(ctx context.Context, asset switchasset.Switch, ifIndex int, interfaceName string) (domain.VLANExecutionResult, error)
}

type snmpEnforcer struct {
	client snmp.Client
}

func NewSNMPEnforcer() SNMPEnforcer {
	return &snmpEnforcer{client: snmp.NewClient()}
}

func (s *Service) PreviewSNMPPortVLAN(ctx context.Context, switchID string, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANPlan, error) {
	asset, err := s.resolveSwitch(ctx, switchID)
	if err != nil {
		return domain.VLANPlan{}, err
	}
	if s.snmp == nil {
		return domain.VLANPlan{}, fmt.Errorf("snmp enforcer is not configured")
	}
	return s.snmp.PreviewPortVLAN(ctx, *asset, bridgePort, ifIndex, interfaceName, vlanID, skipPortBounce)
}

func (s *Service) ExecuteSNMPPortVLAN(ctx context.Context, switchID string, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANExecutionResult, error) {
	asset, err := s.resolveSwitch(ctx, switchID)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}
	if s.snmp == nil {
		return domain.VLANExecutionResult{}, fmt.Errorf("snmp enforcer is not configured")
	}
	return s.snmp.ExecutePortVLAN(ctx, *asset, bridgePort, ifIndex, interfaceName, vlanID, skipPortBounce)
}

func (s *Service) PreviewSNMPPortBounce(ctx context.Context, switchID string, ifIndex int, interfaceName string) (domain.VLANPlan, error) {
	asset, err := s.resolveSwitch(ctx, switchID)
	if err != nil {
		return domain.VLANPlan{}, err
	}
	if s.snmp == nil {
		return domain.VLANPlan{}, fmt.Errorf("snmp enforcer is not configured")
	}
	return s.snmp.PreviewPortBounce(ctx, *asset, ifIndex, interfaceName)
}

func (s *Service) ExecuteSNMPPortBounce(ctx context.Context, switchID string, ifIndex int, interfaceName string) (domain.VLANExecutionResult, error) {
	asset, err := s.resolveSwitch(ctx, switchID)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}
	if s.snmp == nil {
		return domain.VLANExecutionResult{}, fmt.Errorf("snmp enforcer is not configured")
	}
	return s.snmp.ExecutePortBounce(ctx, *asset, ifIndex, interfaceName)
}

func (e *snmpEnforcer) PreviewPortVLAN(_ context.Context, asset switchasset.Switch, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANPlan, error) {
	plan, _, err := buildSNMPPlan(asset, bridgePort, ifIndex, interfaceName, vlanID, skipPortBounce)
	return plan, err
}

func (e *snmpEnforcer) PreviewPortBounce(_ context.Context, asset switchasset.Switch, ifIndex int, interfaceName string) (domain.VLANPlan, error) {
	plan, _, err := buildSNMPBouncePlan(asset, ifIndex, interfaceName)
	return plan, err
}

func (e *snmpEnforcer) ExecutePortVLAN(ctx context.Context, asset switchasset.Switch, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANExecutionResult, error) {
	plan, target, err := buildSNMPPlan(asset, bridgePort, ifIndex, interfaceName, vlanID, skipPortBounce)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}

	if e.client == nil {
		return domain.VLANExecutionResult{}, fmt.Errorf("snmp client is not configured")
	}

	for _, action := range parsePlanActions(plan) {
		var setErr error
		if action.UseGauge32 {
			setErr = e.client.SetUint(ctx, target, action.OID, uint32(action.Value))
		} else {
			setErr = e.client.SetInt(ctx, target, action.OID, action.Value)
		}
		if setErr != nil {
			return domain.VLANExecutionResult{
				Plan:     plan,
				Executed: false,
				Output:   fmt.Sprintf("snmp set failed for %s: %v", action.OID, setErr),
			}, setErr
		}
		if action.Delay > 0 {
			select {
			case <-ctx.Done():
				return domain.VLANExecutionResult{Plan: plan, Executed: false, Output: ctx.Err().Error()}, ctx.Err()
			case <-time.After(action.Delay):
			}
		}
	}

	return domain.VLANExecutionResult{
		Plan:     plan,
		Executed: true,
		Output:   "snmp vlan move executed",
	}, nil
}

func (e *snmpEnforcer) ExecutePortBounce(ctx context.Context, asset switchasset.Switch, ifIndex int, interfaceName string) (domain.VLANExecutionResult, error) {
	plan, target, err := buildSNMPBouncePlan(asset, ifIndex, interfaceName)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}
	if e.client == nil {
		return domain.VLANExecutionResult{}, fmt.Errorf("snmp client is not configured")
	}

	for _, action := range parsePlanActions(plan) {
		if err := e.client.SetInt(ctx, target, action.OID, action.Value); err != nil {
			return domain.VLANExecutionResult{
				Plan:     plan,
				Executed: false,
				Output:   fmt.Sprintf("snmp set failed for %s: %v", action.OID, err),
			}, err
		}
		if action.Delay > 0 {
			select {
			case <-ctx.Done():
				return domain.VLANExecutionResult{Plan: plan, Executed: false, Output: ctx.Err().Error()}, ctx.Err()
			case <-time.After(action.Delay):
			}
		}
	}

	return domain.VLANExecutionResult{
		Plan:     plan,
		Executed: true,
		Output:   "snmp port bounce executed",
	}, nil
}

type snmpAction struct {
	OID        string
	Value      int
	Delay      time.Duration
	UseGauge32 bool
}

func parsePlanActions(plan domain.VLANPlan) []snmpAction {
	actions := make([]snmpAction, 0, len(plan.OIDs))
	for _, raw := range plan.OIDs {
		parts := strings.Split(raw, "=")
		if len(parts) != 2 {
			continue
		}
		value, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}
		action := snmpAction{OID: strings.TrimSpace(parts[0]), Value: value}
		if strings.HasPrefix(action.OID, oidIfAdminStatusPrefix) {
			action.Delay = 2 * time.Second
		}
		if strings.HasPrefix(action.OID, oidQBridgePVIDPrefix) {
			action.UseGauge32 = true
		}
		actions = append(actions, action)
	}
	return actions
}

func buildSNMPPlan(asset switchasset.Switch, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANPlan, snmp.SwitchTarget, error) {
	if vlanID <= 0 {
		return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("vlan_id must be greater than zero")
	}
	if strings.TrimSpace(asset.ManagementIP) == "" {
		return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("switch management_ip is empty")
	}
	if strings.TrimSpace(asset.SNMPCommunity) == "" {
		return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("switch snmp_community is empty")
	}

	profile := selectSNMPProfile(asset)
	if !supportsSNMPWriteStrategy(asset) {
		switch profile.strategy {
		case snmpStrategyExtreme:
			return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("snmp driver %q is not implemented yet for vendor %q model %q", profile.name, asset.Vendor, asset.Model)
		case snmpStrategyJuniperAPI:
			return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("juniper vlan move requires api/ssh helper; snmp driver is not supported")
		default:
			return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("snmp strategy %q is not supported", profile.strategy)
		}
	}
	portIndex := bridgePort
	if profile.vlanIndexMode == "ifindex" {
		portIndex = ifIndex
	}
	if portIndex <= 0 && profile.allowIfIndexFallback && ifIndex > 0 {
		portIndex = ifIndex
	}
	if portIndex <= 0 {
		return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("port index is not available for snmp vlan move")
	}
	if ifIndex <= 0 {
		return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("if_index is not available for snmp port control")
	}

	target := snmp.SwitchTarget{
		Address:   asset.ManagementIP,
		Port:      uint16(asset.SNMPPort),
		Community: asset.SNMPCommunity,
		Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
		Retries:   asset.SNMPRetries,
	}

	oids := make([]string, 0, 3)
	requiresPortBounce := profile.requiresPortBounce && !skipPortBounce

	if requiresPortBounce {
		oids = append(oids, fmt.Sprintf("%s%d=%d", oidIfAdminStatusPrefix, ifIndex, 2))
	}
	vlanOID := fmt.Sprintf("%s%d=%d", profile.vlanOIDPrefix, portIndex, vlanID)
	oids = append(oids, vlanOID)
	if requiresPortBounce {
		oids = append(oids, fmt.Sprintf("%s%d=%d", oidIfAdminStatusPrefix, ifIndex, 1))
	}

	return domain.VLANPlan{
		SwitchID:       asset.ID,
		SwitchName:     asset.Name,
		ManagementIP:   asset.ManagementIP,
		BridgePort:     bridgePort,
		IfIndex:        ifIndex,
		InterfaceName:  strings.TrimSpace(interfaceName),
		VLANID:         vlanID,
		SelectedMethod: "snmp-write",
		OIDs:           oids,
	}, target, nil
}

func buildSNMPBouncePlan(asset switchasset.Switch, ifIndex int, interfaceName string) (domain.VLANPlan, snmp.SwitchTarget, error) {
	if strings.TrimSpace(asset.ManagementIP) == "" {
		return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("switch management_ip is empty")
	}
	if strings.TrimSpace(asset.SNMPCommunity) == "" {
		return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("switch snmp_community is empty")
	}
	if ifIndex <= 0 {
		return domain.VLANPlan{}, snmp.SwitchTarget{}, fmt.Errorf("if_index is not available for snmp port bounce")
	}

	target := snmp.SwitchTarget{
		Address:   asset.ManagementIP,
		Port:      uint16(asset.SNMPPort),
		Community: asset.SNMPCommunity,
		Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
		Retries:   asset.SNMPRetries,
	}

	return domain.VLANPlan{
		SwitchID:       asset.ID,
		SwitchName:     asset.Name,
		ManagementIP:   asset.ManagementIP,
		IfIndex:        ifIndex,
		InterfaceName:  strings.TrimSpace(interfaceName),
		VLANID:         0,
		SelectedMethod: "snmp-bounce",
		OIDs: []string{
			fmt.Sprintf("%s%d=%d", oidIfAdminStatusPrefix, ifIndex, 2),
			fmt.Sprintf("%s%d=%d", oidIfAdminStatusPrefix, ifIndex, 1),
		},
	}, target, nil
}
