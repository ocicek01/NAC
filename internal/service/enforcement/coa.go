package enforcement

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"nac/internal/config"
	domain "nac/internal/domain/enforcement"
	sessiondomain "nac/internal/domain/session"
	switchasset "nac/internal/domain/switchasset"
)

type CoAExecutor interface {
	Preview(ctx context.Context, asset switchasset.Switch, session sessiondomain.Session, vlanID int) (domain.VLANPlan, error)
	Execute(ctx context.Context, asset switchasset.Switch, session sessiondomain.Session, vlanID int) (domain.VLANExecutionResult, error)
}

type coaExecutor struct {
	port       int
	clientPath string
}

func NewCoAExecutor(cfg config.RadiusConfig) CoAExecutor {
	port := cfg.CoAPort
	if port <= 0 {
		port = 3799
	}
	clientPath := strings.TrimSpace(cfg.CoAClientPath)
	if clientPath == "" {
		clientPath = "radclient"
	}
	return &coaExecutor{
		port:       port,
		clientPath: clientPath,
	}
}

func (s *Service) PreviewCoAPortVLAN(ctx context.Context, macAddress, switchID string, vlanID int) (domain.VLANPlan, error) {
	asset, session, err := s.resolveCoATarget(ctx, macAddress, switchID)
	if err != nil {
		return domain.VLANPlan{}, err
	}
	if s.coa == nil {
		return domain.VLANPlan{}, fmt.Errorf("coa executor is not configured")
	}
	return s.coa.Preview(ctx, *asset, *session, vlanID)
}

func (s *Service) ExecuteCoAPortVLAN(ctx context.Context, macAddress, switchID string, vlanID int) (domain.VLANExecutionResult, error) {
	asset, session, err := s.resolveCoATarget(ctx, macAddress, switchID)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}
	if s.coa == nil {
		return domain.VLANExecutionResult{}, fmt.Errorf("coa executor is not configured")
	}
	return s.coa.Execute(ctx, *asset, *session, vlanID)
}

func (s *Service) resolveCoATarget(ctx context.Context, macAddress, switchID string) (*switchasset.Switch, *sessiondomain.Session, error) {
	if s.switches == nil {
		return nil, nil, fmt.Errorf("switch resolver is not configured")
	}
	asset, err := s.switches.FindByID(ctx, strings.TrimSpace(switchID))
	if err != nil {
		return nil, nil, err
	}
	if asset == nil {
		return nil, nil, fmt.Errorf("switch not found")
	}
	if s.sessions == nil {
		return nil, nil, fmt.Errorf("session resolver is not configured")
	}
	session, err := s.sessions.FindLatestActiveByMACSwitch(ctx, strings.TrimSpace(macAddress), strings.TrimSpace(switchID))
	if err != nil {
		return nil, nil, err
	}
	if session == nil {
		return nil, nil, fmt.Errorf("active radius session not found for mac %q on switch %q", macAddress, switchID)
	}
	return asset, session, nil
}

func (e *coaExecutor) Preview(_ context.Context, asset switchasset.Switch, session sessiondomain.Session, vlanID int) (domain.VLANPlan, error) {
	plan, _, _, err := buildCoAPlan(asset, session, vlanID, e.port, e.clientPath)
	return plan, err
}

func (e *coaExecutor) Execute(ctx context.Context, asset switchasset.Switch, session sessiondomain.Session, vlanID int) (domain.VLANExecutionResult, error) {
	plan, input, commandLine, err := buildCoAPlan(asset, session, vlanID, e.port, e.clientPath)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}

	cmd := exec.CommandContext(ctx, e.clientPath, "-x", commandLine, "coa", strings.TrimSpace(asset.RadiusSecret))
	cmd.Stdin = strings.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			output = strings.TrimSpace(stdout.String())
		}
		if output == "" {
			output = err.Error()
		}
		return domain.VLANExecutionResult{
			Plan:     plan,
			Executed: false,
			Output:   output,
		}, err
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String())
	}
	if output == "" {
		output = "coa request executed"
	}

	return domain.VLANExecutionResult{
		Plan:     plan,
		Executed: true,
		Output:   output,
	}, nil
}

func buildCoAPlan(asset switchasset.Switch, session sessiondomain.Session, vlanID, coaPort int, clientPath string) (domain.VLANPlan, string, string, error) {
	if strings.TrimSpace(asset.ManagementIP) == "" {
		return domain.VLANPlan{}, "", "", fmt.Errorf("switch management_ip is empty")
	}
	if strings.TrimSpace(asset.RadiusSecret) == "" {
		return domain.VLANPlan{}, "", "", fmt.Errorf("switch radius_secret is empty")
	}
	if strings.TrimSpace(session.AcctSessionID) == "" {
		return domain.VLANPlan{}, "", "", fmt.Errorf("acct_session_id is required for coa request")
	}
	if vlanID <= 0 {
		return domain.VLANPlan{}, "", "", fmt.Errorf("vlan_id must be greater than zero")
	}

	lines := []string{
		fmt.Sprintf(`Acct-Session-Id := "%s"`, session.AcctSessionID),
		fmt.Sprintf(`Calling-Station-Id := "%s"`, firstCoAValue(session.CallingStationID, session.MACAddress)),
	}
	if strings.TrimSpace(session.Username) != "" {
		lines = append(lines, fmt.Sprintf(`User-Name := "%s"`, session.Username))
	}
	if strings.TrimSpace(asset.ManagementIP) != "" {
		lines = append(lines, fmt.Sprintf(`NAS-IP-Address := "%s"`, asset.ManagementIP))
	}
	if strings.TrimSpace(session.NASPort) != "" {
		lines = append(lines, fmt.Sprintf(`NAS-Port := "%s"`, session.NASPort))
	}
	lines = append(lines,
		`Tunnel-Type := 13`,
		`Tunnel-Medium-Type := 6`,
		fmt.Sprintf(`Tunnel-Private-Group-Id := "%d"`, vlanID),
	)

	input := strings.Join(lines, "\n") + "\n"
	commandTarget := fmt.Sprintf("%s:%d", strings.TrimSpace(asset.ManagementIP), coaPort)
	commands := append([]string{fmt.Sprintf(`%s -x %s coa "<radius_secret>"`, clientPath, commandTarget)}, lines...)

	return domain.VLANPlan{
		SwitchID:       asset.ID,
		SwitchName:     asset.Name,
		ManagementIP:   asset.ManagementIP,
		BridgePort:     0,
		IfIndex:        session.IfIndex,
		InterfaceName:  firstCoAValue(session.InterfaceName, session.NASPortID, session.NASPort),
		VLANID:         vlanID,
		SelectedMethod: "coa",
		Commands:       commands,
		OIDs:           nil,
	}, input, commandTarget, nil
}

func firstCoAValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
