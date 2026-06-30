package enforcement

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	domain "nac/internal/domain/enforcement"
	switchasset "nac/internal/domain/switchasset"

	gossh "golang.org/x/crypto/ssh"
)

type SSHExecutor interface {
	Execute(ctx context.Context, asset switchasset.Switch, commands []string) (string, error)
}

type NativeSSHExecutor struct{}

func NewNativeSSHExecutor() *NativeSSHExecutor {
	return &NativeSSHExecutor{}
}

func (e *NativeSSHExecutor) Execute(ctx context.Context, asset switchasset.Switch, commands []string) (string, error) {
	if strings.TrimSpace(asset.ManagementIP) == "" {
		return "", fmt.Errorf("switch management_ip is empty")
	}
	if strings.TrimSpace(asset.SSHUsername) == "" {
		return "", fmt.Errorf("switch ssh_username is empty")
	}
	if strings.TrimSpace(asset.SSHPassword) == "" {
		return "", fmt.Errorf("switch ssh_password is empty")
	}

	port := asset.SSHPort
	if port <= 0 {
		port = 22
	}

	cfg := &gossh.ClientConfig{
		User:            asset.SSHUsername,
		Auth:            []gossh.AuthMethod{gossh.Password(asset.SSHPassword)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         8 * time.Second,
	}

	addr := net.JoinHostPort(asset.ManagementIP, strconv.Itoa(port))
	client, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", err
	}
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return "", err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	doneOut := make(chan struct{})
	doneErr := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdout, stdoutPipe)
		close(doneOut)
	}()
	go func() {
		_, _ = io.Copy(&stderr, stderrPipe)
		close(doneErr)
	}()

	modes := gossh.TerminalModes{
		gossh.ECHO:          0,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 40, modes); err != nil {
		return "", err
	}
	if err := session.Shell(); err != nil {
		return "", err
	}

	currentOutput := func() string {
		return stdout.String() + "\n" + stderr.String()
	}

	waitForPrompt := func(timeout time.Duration, from int, markers ...string) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			combined := currentOutput()
			if from < 0 || from > len(combined) {
				from = 0
			}
			segment := combined[from:]
			for _, marker := range markers {
				if strings.Contains(segment, marker) {
					return nil
				}
			}
			time.Sleep(150 * time.Millisecond)
		}
		return fmt.Errorf("timed out waiting for prompt %v", markers)
	}

	dismissContinuePrompt := func(timeout time.Duration) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			combined := currentOutput()
			if strings.Contains(combined, "Press any key to continue") {
				if _, err := io.WriteString(stdin, "\n"); err != nil {
					return err
				}
				time.Sleep(250 * time.Millisecond)
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return nil
	}

	writeCommand := func(cmd string, markers ...string) error {
		offset := len(currentOutput())
		if _, err := io.WriteString(stdin, cmd+"\n"); err != nil {
			return err
		}
		if len(markers) == 0 {
			return nil
		}
		return waitForPrompt(5*time.Second, offset, markers...)
	}

	_ = dismissContinuePrompt(2 * time.Second)
	initial := currentOutput()
	if err := waitForPrompt(5*time.Second, len(initial), "#"); err != nil {
		if !strings.Contains(currentOutput(), "#") {
			return strings.TrimSpace(currentOutput()), err
		}
	}
	for _, cmd := range commands {
		if err := writeCommand(cmd, "#"); err != nil {
			return strings.TrimSpace(currentOutput()), fmt.Errorf("command %q failed: %w", cmd, err)
		}
	}
	if err := writeCommand("exit", "#", "Do you want to log out"); err != nil {
		return strings.TrimSpace(currentOutput()), err
	}
	combined := currentOutput()
	if strings.Contains(combined, "Do you want to log out") {
		if _, err := io.WriteString(stdin, "y\n"); err != nil {
			return strings.TrimSpace(currentOutput()), err
		}
	}
	_ = stdin.Close()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		return strings.TrimSpace(currentOutput()), ctx.Err()
	case err := <-waitCh:
		<-doneOut
		<-doneErr
		output := strings.TrimSpace(currentOutput())
		if err != nil {
			if output != "" {
				return output, err
			}
			return "", err
		}
		return output, nil
	}
}

func (s *Service) PreviewSSHPortVLAN(ctx context.Context, switchID, interfaceName string, vlanID int) (domain.VLANPlan, error) {
	asset, err := s.resolveSwitch(ctx, switchID)
	if err != nil {
		return domain.VLANPlan{}, err
	}

	return buildSSHPlan(*asset, interfaceName, vlanID)
}

func (s *Service) ExecuteSSHPortVLAN(ctx context.Context, switchID, interfaceName string, vlanID int) (domain.VLANExecutionResult, error) {
	asset, err := s.resolveSwitch(ctx, switchID)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}

	plan, err := buildSSHPlan(*asset, interfaceName, vlanID)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}

	if s.ssh == nil {
		return domain.VLANExecutionResult{}, fmt.Errorf("ssh executor is not configured")
	}

	output, err := s.ssh.Execute(ctx, *asset, plan.Commands)
	if err != nil {
		return domain.VLANExecutionResult{
			Plan:     plan,
			Executed: false,
			Output:   output,
		}, err
	}

	return domain.VLANExecutionResult{
		Plan:     plan,
		Executed: true,
		Output:   output,
	}, nil
}

func (s *Service) ExecuteSSHPortBounce(ctx context.Context, switchID, interfaceName string) (domain.VLANExecutionResult, error) {
	asset, err := s.resolveSwitch(ctx, switchID)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}

	plan, err := buildSSHBouncePlan(*asset, interfaceName)
	if err != nil {
		return domain.VLANExecutionResult{}, err
	}

	if s.ssh == nil {
		return domain.VLANExecutionResult{}, fmt.Errorf("ssh executor is not configured")
	}

	output, err := s.ssh.Execute(ctx, *asset, plan.Commands)
	if err != nil {
		return domain.VLANExecutionResult{
			Plan:     plan,
			Executed: false,
			Output:   output,
		}, err
	}

	return domain.VLANExecutionResult{
		Plan:     plan,
		Executed: true,
		Output:   output,
	}, nil
}

func (s *Service) resolveSwitch(ctx context.Context, switchID string) (*switchasset.Switch, error) {
	if s.switches == nil {
		return nil, fmt.Errorf("switch repository is not configured")
	}

	asset, err := s.switches.FindByID(ctx, strings.TrimSpace(switchID))
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("switch not found")
	}
	return asset, nil
}

func buildSSHPlan(asset switchasset.Switch, interfaceName string, vlanID int) (domain.VLANPlan, error) {
	if vlanID <= 0 {
		return domain.VLANPlan{}, fmt.Errorf("vlan_id must be greater than zero")
	}

	portID, err := normalizeHPPort(interfaceName)
	if err != nil {
		return domain.VLANPlan{}, err
	}

	commands, method, err := deriveSSHCommands(asset, portID, vlanID)
	if err != nil {
		return domain.VLANPlan{}, err
	}

	return domain.VLANPlan{
		SwitchID:       asset.ID,
		SwitchName:     asset.Name,
		ManagementIP:   asset.ManagementIP,
		InterfaceName:  portID,
		VLANID:         vlanID,
		SelectedMethod: method,
		Commands:       commands,
	}, nil
}

func buildSSHBouncePlan(asset switchasset.Switch, interfaceName string) (domain.VLANPlan, error) {
	portID, err := normalizeHPPort(interfaceName)
	if err != nil {
		return domain.VLANPlan{}, err
	}

	vendor := strings.ToLower(strings.TrimSpace(asset.Vendor))
	model := strings.ToLower(strings.TrimSpace(asset.Model))
	name := strings.ToLower(strings.TrimSpace(asset.Name))

	if strings.Contains(vendor, "hp") || strings.Contains(vendor, "hpe") || strings.Contains(model, "2530") || strings.Contains(name, "2530") {
		return domain.VLANPlan{
			SwitchID:       asset.ID,
			SwitchName:     asset.Name,
			ManagementIP:   asset.ManagementIP,
			InterfaceName:  portID,
			VLANID:         0,
			SelectedMethod: "ssh-bounce",
			Commands: []string{
				"configure terminal",
				fmt.Sprintf("interface %s disable", portID),
				fmt.Sprintf("interface %s enable", portID),
				"write memory",
			},
		}, nil
	}

	return domain.VLANPlan{}, fmt.Errorf("ssh port bounce driver is not implemented for vendor=%q model=%q", asset.Vendor, asset.Model)
}

func deriveSSHCommands(asset switchasset.Switch, portID string, vlanID int) ([]string, string, error) {
	vendor := strings.ToLower(strings.TrimSpace(asset.Vendor))
	model := strings.ToLower(strings.TrimSpace(asset.Model))
	name := strings.ToLower(strings.TrimSpace(asset.Name))

	if strings.Contains(vendor, "hp") || strings.Contains(vendor, "hpe") || strings.Contains(model, "2530") || strings.Contains(name, "2530") {
		return []string{
			"configure terminal",
			fmt.Sprintf("vlan %d", vlanID),
			fmt.Sprintf("untagged %s", portID),
			"exit",
			"write memory",
		}, "ssh", nil
	}

	return nil, "", fmt.Errorf("ssh vlan enforcement driver is not implemented for vendor=%q model=%q", asset.Vendor, asset.Model)
}

func normalizeHPPort(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("interface_name is empty")
	}

	fields := strings.Fields(value)
	if len(fields) > 0 {
		value = fields[len(fields)-1]
	}
	if idx := strings.LastIndex(value, "/"); idx >= 0 && idx+1 < len(value) {
		value = value[idx+1:]
	}

	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return "", fmt.Errorf("unsupported interface_name format: %q", value)
		}
	}

	return value, nil
}
