package enforcement

import (
	"fmt"
	"strings"

	domain "nac/internal/domain/enforcement"
)

func buildRadiusVLANPlan(decision domain.Decision, vlanID int) (domain.VLANPlan, error) {
	if strings.TrimSpace(decision.SwitchID) == "" {
		return domain.VLANPlan{}, fmt.Errorf("switch_id is required for radius-vlan plan")
	}

	commands := make([]string, 0, 4)
	switch strings.ToLower(strings.TrimSpace(decision.PolicyAction)) {
	case "blocked":
		commands = append(commands, `Control:Auth-Type := "Reject"`)
		commands = append(commands, `Reply:Reply-Message := "Blocked by NAC policy"`)
	case "guest":
		if vlanID <= 0 {
			return domain.VLANPlan{}, fmt.Errorf("vlan_id must be greater than zero for guest radius-vlan plan")
		}
		commands = append(commands, `Reply:Reply-Message := "Guest access granted"`)
		commands = append(commands, `Reply:Tunnel-Type := "VLAN"`)
		commands = append(commands, `Reply:Tunnel-Medium-Type := "IEEE-802"`)
		commands = append(commands, fmt.Sprintf(`Reply:Tunnel-Private-Group-Id := "%d"`, vlanID))
	case "unknown":
		if vlanID <= 0 {
			return domain.VLANPlan{}, fmt.Errorf("vlan_id must be greater than zero for unknown/quarantine radius-vlan plan")
		}
		commands = append(commands, `Reply:Reply-Message := "Quarantine access granted"`)
		commands = append(commands, `Reply:Tunnel-Type := "VLAN"`)
		commands = append(commands, `Reply:Tunnel-Medium-Type := "IEEE-802"`)
		commands = append(commands, fmt.Sprintf(`Reply:Tunnel-Private-Group-Id := "%d"`, vlanID))
	default:
		commands = append(commands, `Reply:Reply-Message := "Access granted"`)
	}

	return domain.VLANPlan{
		SwitchID:       decision.SwitchID,
		SwitchName:     decision.SwitchName,
		ManagementIP:   decision.ManagementIP,
		BridgePort:     decision.BridgePort,
		IfIndex:        decision.IfIndex,
		InterfaceName:  decision.InterfaceName,
		VLANID:         vlanID,
		SelectedMethod: "radius-vlan",
		Commands:       commands,
		OIDs:           nil,
	}, nil
}

func executeRadiusVLANPlan(decision domain.Decision, vlanID int) (domain.VLANExecutionResult, error) {
	plan, err := buildRadiusVLANPlan(decision, vlanID)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}

	output := "radius response path recorded; attributes will apply on authorize/reauth"
	switch strings.ToLower(strings.TrimSpace(decision.PolicyAction)) {
	case "blocked":
		output = "radius reject policy recorded; deny will apply on authorize/reauth"
	case "guest":
		output = fmt.Sprintf("radius guest vlan %d recorded; vlan will apply on authorize/reauth", vlanID)
	case "unknown":
		output = fmt.Sprintf("radius quarantine vlan %d recorded; vlan will apply on authorize/reauth", vlanID)
	}

	return domain.VLANExecutionResult{
		Plan:     plan,
		Executed: true,
		Output:   output,
	}, nil
}
