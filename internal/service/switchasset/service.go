package switchasset

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"nac/internal/config"
	devicedomain "nac/internal/domain/device"
	domain "nac/internal/domain/switchasset"
	switchport "nac/internal/domain/switchport"
	"nac/internal/snmp"
)

type Service struct {
	repository   domain.Repository
	devices      devicedomain.Repository
	ports        switchport.Repository
	snmpDefaults config.SNMPConfig
	client       snmp.Client
}

func NewService(repository domain.Repository, devices devicedomain.Repository, ports switchport.Repository, snmpDefaults config.SNMPConfig, client snmp.Client) *Service {
	return &Service{
		repository:   repository,
		devices:      devices,
		ports:        ports,
		snmpDefaults: snmpDefaults,
		client:       client,
	}
}

func (s *Service) Create(ctx context.Context, asset domain.Switch) (domain.Switch, error) {
	asset, err := s.normalize(ctx, asset)
	if err != nil {
		return domain.Switch{}, err
	}

	return s.repository.Insert(ctx, asset)
}

func (s *Service) UpdateIdentity(ctx context.Context, id, systemName, baseMAC string, aliases []string) (domain.Switch, error) {
	cleanAliases := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias != "" {
			cleanAliases = append(cleanAliases, alias)
		}
	}
	return s.repository.UpdateIdentity(ctx, strings.TrimSpace(id), strings.TrimSpace(systemName), normalizeBaseMAC(baseMAC), cleanAliases)
}

func (s *Service) UpdateRoutingSwitch(ctx context.Context, id, routingSwitchID string) (domain.Switch, error) {
	id = strings.TrimSpace(id)
	routingSwitchID = strings.TrimSpace(routingSwitchID)
	if id == "" {
		return domain.Switch{}, fmt.Errorf("switch id is required")
	}
	if routingSwitchID == id {
		return domain.Switch{}, fmt.Errorf("routing_switch_id cannot equal switch id")
	}
	if routingSwitchID != "" {
		asset, err := s.repository.FindByID(ctx, routingSwitchID)
		if err != nil {
			return domain.Switch{}, err
		}
		if asset == nil {
			return domain.Switch{}, fmt.Errorf("routing switch %q not found", routingSwitchID)
		}
	}
	return s.repository.UpdateRoutingSwitch(ctx, id, routingSwitchID)
}

func (s *Service) UpdateRadiusSecret(ctx context.Context, id, radiusSecret string) (domain.Switch, error) {
	return s.repository.UpdateRadiusSecret(ctx, strings.TrimSpace(id), strings.TrimSpace(radiusSecret))
}

func (s *Service) UpdateSSHConfig(ctx context.Context, id, username, password string, port int) (domain.Switch, error) {
	return s.repository.UpdateSSHConfig(ctx, strings.TrimSpace(id), strings.TrimSpace(username), strings.TrimSpace(password), port)
}

func (s *Service) CreateBulk(ctx context.Context, defaults domain.Switch, assets []domain.Switch) ([]domain.Switch, error) {
	created := make([]domain.Switch, 0, len(assets))
	for _, asset := range assets {
		merged := mergeSwitchDefaults(defaults, asset)
		normalized, err := s.normalize(ctx, merged)
		if err != nil {
			return nil, err
		}

		inserted, err := s.repository.Insert(ctx, normalized)
		if err != nil {
			return nil, err
		}

		created = append(created, inserted)
	}

	return created, nil
}

func (s *Service) normalize(ctx context.Context, asset domain.Switch) (domain.Switch, error) {
	now := time.Now().UTC()

	if asset.ID == "" {
		asset.ID = uuid.NewString()
	}

	asset.Name = strings.TrimSpace(asset.Name)
	asset.ManagementIP = strings.TrimSpace(asset.ManagementIP)
	asset.RoutingSwitchID = strings.TrimSpace(asset.RoutingSwitchID)
	asset.SNMPCommunity = strings.TrimSpace(asset.SNMPCommunity)
	asset.SNMPVersion = strings.TrimSpace(asset.SNMPVersion)
	asset.SystemName = strings.TrimSpace(asset.SystemName)
	asset.BaseMAC = normalizeBaseMAC(asset.BaseMAC)
	asset.RadiusSecret = strings.TrimSpace(asset.RadiusSecret)
	asset.SSHUsername = strings.TrimSpace(asset.SSHUsername)
	asset.SSHPassword = strings.TrimSpace(asset.SSHPassword)

	if ip := net.ParseIP(asset.ManagementIP); ip == nil {
		return domain.Switch{}, fmt.Errorf("management_ip is invalid")
	}

	if parsed, err := netip.ParseAddr(asset.ManagementIP); err == nil {
		asset.ManagementIP = parsed.String()
	}

	if asset.Name == "" {
		asset.Name = defaultSwitchName(asset.ManagementIP)
	}
	if asset.RoutingSwitchID == asset.ID {
		return domain.Switch{}, fmt.Errorf("routing_switch_id cannot equal switch id")
	}
	if asset.RoutingSwitchID != "" {
		related, err := s.repository.FindByID(ctx, asset.RoutingSwitchID)
		if err != nil {
			return domain.Switch{}, err
		}
		if related == nil {
			return domain.Switch{}, fmt.Errorf("routing switch %q not found", asset.RoutingSwitchID)
		}
	}
	if asset.SystemName == "" {
		asset.SystemName = asset.Name
	}

	if asset.Status == "" {
		asset.Status = "active"
	}

	if asset.SNMPVersion == "" {
		asset.SNMPVersion = "2c"
	}

	if asset.SNMPPort == 0 {
		asset.SNMPPort = s.snmpDefaults.Port
	}

	if asset.SNMPTimeoutMS == 0 {
		asset.SNMPTimeoutMS = s.snmpDefaults.TimeoutMS
	}

	if asset.SNMPRetries == 0 {
		asset.SNMPRetries = s.snmpDefaults.Retries
	}

	if asset.SSHPort == 0 {
		asset.SSHPort = 22
	}

	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = now
	}

	if asset.UpdatedAt.IsZero() {
		asset.UpdatedAt = now
	}

	applyDefaultCapabilities(&asset)

	discoveredSystemName, discoveredBaseMAC, aliases := s.discoverIdentity(ctx, asset)
	if (asset.SystemName == "" || asset.SystemName == asset.Name) && discoveredSystemName != "" {
		asset.SystemName = discoveredSystemName
	}
	if asset.BaseMAC == "" && discoveredBaseMAC != "" {
		asset.BaseMAC = discoveredBaseMAC
	}
	if len(asset.Aliases) == 0 {
		asset.Aliases = aliases
	}

	return asset, nil
}

func applyDefaultCapabilities(asset *domain.Switch) {
	if asset == nil {
		return
	}

	if strings.TrimSpace(asset.RadiusSecret) != "" {
		asset.SupportsRadiusVLAN = true
	}
	if strings.TrimSpace(asset.SNMPCommunity) != "" {
		asset.SupportsSNMPWrite = true
	}
	if strings.TrimSpace(asset.ManagementIP) != "" {
		asset.SupportsSSHEnforcement = true
	}
}

func (s *Service) List(ctx context.Context) ([]domain.Switch, error) {
	return s.repository.List(ctx)
}

func (s *Service) FindByID(ctx context.Context, id string) (*domain.Switch, error) {
	return s.repository.FindByID(ctx, strings.TrimSpace(id))
}

func (s *Service) FindByName(ctx context.Context, name string) (*domain.Switch, error) {
	return s.repository.FindByName(ctx, strings.TrimSpace(name))
}

func (s *Service) FindByManagementIP(ctx context.Context, managementIP string) (*domain.Switch, error) {
	return s.repository.FindByManagementIP(ctx, strings.TrimSpace(managementIP))
}

func (s *Service) ListPortsBySwitch(ctx context.Context, switchID string) ([]switchport.Port, error) {
	switchID = strings.TrimSpace(switchID)
	if switchID == "" {
		return nil, fmt.Errorf("switch id is required")
	}
	asset, err := s.repository.FindByID(ctx, switchID)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("switch not found")
	}
	if s.ports == nil {
		return nil, fmt.Errorf("switch port repository is not configured")
	}
	return s.ports.ListBySwitch(ctx, switchID)
}

func (s *Service) PortSummaryBySwitch(ctx context.Context, switchID string) (any, error) {
	switchID = strings.TrimSpace(switchID)
	ports, err := s.ListPortsBySwitch(ctx, switchID)
	if err != nil {
		return nil, err
	}

	summary := map[string]any{
		"switch_id":          switchID,
		"total_ports":        len(ports),
		"uplink_ports":       0,
		"trunk_ports":        0,
		"neighbor_ports":     0,
		"ports_with_macs":    0,
		"total_learned_macs": 0,
		"top_mac_port_index": 0,
		"top_mac_port_name":  "",
		"top_mac_count":      0,
	}

	for _, port := range ports {
		if port.IsUplink {
			summary["uplink_ports"] = summary["uplink_ports"].(int) + 1
		}
		if port.IsTrunk {
			summary["trunk_ports"] = summary["trunk_ports"].(int) + 1
		}
		if strings.TrimSpace(port.NeighborProtocol) != "" {
			summary["neighbor_ports"] = summary["neighbor_ports"].(int) + 1
		}
		if port.MACCount > 0 {
			summary["ports_with_macs"] = summary["ports_with_macs"].(int) + 1
		}
		summary["total_learned_macs"] = summary["total_learned_macs"].(int) + port.MACCount
		if port.MACCount > summary["top_mac_count"].(int) {
			summary["top_mac_count"] = port.MACCount
			summary["top_mac_port_index"] = port.PortIndex
			summary["top_mac_port_name"] = strings.TrimSpace(port.InterfaceName)
		}
	}

	return summary, nil
}

func (s *Service) LivePortLookup(ctx context.Context, switchID string, ifIndex int) (*domain.LivePortLookup, error) {
	switchID = strings.TrimSpace(switchID)
	if switchID == "" {
		return nil, fmt.Errorf("switch id is required")
	}
	if ifIndex <= 0 {
		return nil, fmt.Errorf("if_index must be greater than zero")
	}

	asset, err := s.repository.FindByID(ctx, switchID)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("switch not found")
	}
	if s.client == nil {
		return nil, fmt.Errorf("snmp client is not configured")
	}

	lookupCtx, cancel := context.WithTimeout(ctx, time.Duration(asset.SNMPTimeoutMS+1500)*time.Millisecond)
	defer cancel()

	target := snmp.SwitchTarget{
		Address:   asset.ManagementIP,
		Port:      uint16(asset.SNMPPort),
		Community: asset.SNMPCommunity,
		Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
		Retries:   asset.SNMPRetries,
		Vendor:    asset.Vendor,
		Model:     asset.Model,
	}

	live := &domain.LivePortLookup{
		SwitchID:     asset.ID,
		SwitchName:   asset.Name,
		ManagementIP: asset.ManagementIP,
		IfIndex:      ifIndex,
		ObservedAt:   time.Now().UTC(),
	}

	if bridgePort, bridgeErr := s.client.FindBridgePortByIfIndex(lookupCtx, target, ifIndex); bridgeErr == nil && bridgePort > 0 {
		live.BridgePort = bridgePort
		if resolved, resolveErr := s.client.ResolveBridgePort(lookupCtx, target, bridgePort); resolveErr == nil {
			live.InterfaceName = strings.TrimSpace(resolved.InterfaceName)
			live.InterfaceDescription = strings.TrimSpace(resolved.InterfaceDescription)
		}
	}

	if states, stateErr := s.client.WalkInterfaces(lookupCtx, target); stateErr == nil {
		for _, state := range states {
			if state.IfIndex != ifIndex {
				continue
			}
			if live.InterfaceName == "" {
				live.InterfaceName = strings.TrimSpace(state.Name)
			}
			if live.InterfaceDescription == "" {
				live.InterfaceDescription = strings.TrimSpace(state.Description)
			}
			break
		}
	}

	discovery, err := s.client.DiscoverFDB(lookupCtx, target)
	if err != nil {
		return nil, err
	}
	live.FDBSource = discovery.Source

	seen := make(map[string]struct{})
	for _, entry := range discovery.Entries {
		if entry.IfIndex != ifIndex && (live.BridgePort <= 0 || entry.BridgePort != live.BridgePort) {
			continue
		}
		mac := strings.TrimSpace(entry.MACAddress)
		if mac == "" {
			continue
		}
		if _, ok := seen[mac]; ok {
			continue
		}
		seen[mac] = struct{}{}
		live.MACAddresses = append(live.MACAddresses, mac)
	}

	sort.Strings(live.MACAddresses)
	live.MACCount = len(live.MACAddresses)
	return live, nil
}
func (s *Service) RefreshPortSnapshot(ctx context.Context, switchID string, ifIndex int) (*domain.LivePortLookup, error) {
	switchID = strings.TrimSpace(switchID)
	if switchID == "" {
		return nil, fmt.Errorf("switch id is required")
	}
	if ifIndex <= 0 {
		return nil, fmt.Errorf("if_index must be greater than zero")
	}
	if s.ports == nil {
		return nil, fmt.Errorf("switch port repository is not configured")
	}

	live, err := s.LivePortLookup(ctx, switchID, ifIndex)
	if err != nil {
		return nil, err
	}

	_, err = s.ports.UpdateFDBSnapshot(ctx, switchID, ifIndex, live.InterfaceName, live.InterfaceDescription, live.MACAddresses, live.ObservedAt)
	if err != nil {
		return nil, err
	}

	return live, nil
}
func (s *Service) LiveDetail(ctx context.Context, id string) (*domain.LiveDetail, error) {
	asset, err := s.repository.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("switch not found")
	}
	if s.client == nil {
		return nil, fmt.Errorf("snmp client is not configured")
	}

	lookupCtx, cancel := context.WithTimeout(ctx, time.Duration(asset.SNMPTimeoutMS+1000)*time.Millisecond)
	defer cancel()

	target := snmp.SwitchTarget{
		Address:   asset.ManagementIP,
		Port:      uint16(asset.SNMPPort),
		Community: asset.SNMPCommunity,
		Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
		Retries:   asset.SNMPRetries,
	}

	interfaceStates, err := s.client.WalkInterfaces(lookupCtx, target)
	if err != nil {
		return nil, err
	}

	devices := []devicedomain.Device{}
	if s.devices != nil {
		devices, err = s.devices.ListBySwitch(ctx, asset.ID)
		if err != nil {
			return nil, err
		}
	}

	byInterface := make(map[string][]domain.ConnectedDevice)
	byIfIndex := make(map[int][]domain.ConnectedDevice)
	for _, device := range devices {
		connected := domain.ConnectedDevice{
			MACAddress:    device.MACAddress,
			Hostname:      device.Hostname,
			Status:        device.Status,
			PolicyAction:  device.PolicyAction,
			PolicyReason:  device.PolicyReason,
			FirstSeenAt:   device.FirstSeenAt,
			LastSeenAt:    device.LastSeenAt,
			SourceType:    device.CurrentSourceType,
			Confidence:    device.CurrentConfidence,
			InterfaceName: device.CurrentInterfaceName,
			ManagementIP:  device.CurrentManagementIP,
		}
		if name := strings.TrimSpace(device.CurrentInterfaceName); name != "" {
			byInterface[strings.ToLower(name)] = append(byInterface[strings.ToLower(name)], connected)
		}
		if device.CurrentIfIndex > 0 {
			byIfIndex[device.CurrentIfIndex] = append(byIfIndex[device.CurrentIfIndex], connected)
		}
	}

	interfaces := make([]domain.InterfaceState, 0, len(interfaceStates))
	upCount := 0
	downCount := 0
	totalConnected := 0
	for _, state := range interfaceStates {
		connected := append([]domain.ConnectedDevice{}, byIfIndex[state.IfIndex]...)
		if len(connected) == 0 && strings.TrimSpace(state.Name) != "" {
			connected = append(connected, byInterface[strings.ToLower(strings.TrimSpace(state.Name))]...)
		}
		sort.Slice(connected, func(i, j int) bool {
			return connected[i].MACAddress < connected[j].MACAddress
		})

		item := domain.InterfaceState{
			IfIndex:              state.IfIndex,
			BridgePort:           inferBridgePort(state.Name),
			Name:                 state.Name,
			Description:          state.Description,
			Alias:                state.Alias,
			AdminStatus:          state.AdminStatus,
			OperStatus:           state.OperStatus,
			OperationallyUp:      state.OperStatus == "up",
			OperationallyDown:    state.OperStatus == "down" || state.OperStatus == "lower-layer-down",
			ConnectedDeviceCount: len(connected),
			ConnectedDevices:     connected,
		}
		if item.OperationallyUp {
			upCount++
		}
		if item.OperationallyDown {
			downCount++
		}
		totalConnected += len(connected)
		interfaces = append(interfaces, item)
	}

	return &domain.LiveDetail{
		Switch:               *asset,
		Interfaces:           interfaces,
		TotalInterfaces:      len(interfaces),
		UpInterfaces:         upCount,
		DownInterfaces:       downCount,
		ConnectedDeviceCount: totalConnected,
		ObservedAt:           time.Now().UTC(),
	}, nil
}

func (s *Service) RefreshIdentities(ctx context.Context) ([]domain.Switch, error) {
	assets, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}

	updated := make([]domain.Switch, 0, len(assets))
	for _, asset := range assets {
		systemName, baseMAC, aliases := s.discoverIdentity(ctx, asset)
		if systemName == "" {
			systemName = asset.SystemName
		}
		if baseMAC == "" {
			baseMAC = asset.BaseMAC
		}
		if len(aliases) == 0 {
			aliases = asset.Aliases
		}

		item, err := s.repository.UpdateIdentity(ctx, asset.ID, systemName, baseMAC, aliases)
		if err != nil {
			return nil, err
		}
		updated = append(updated, item)
	}

	return updated, nil
}

func mergeSwitchDefaults(defaults, asset domain.Switch) domain.Switch {
	if asset.Name == "" {
		asset.Name = defaults.Name
	}
	if asset.Vendor == "" {
		asset.Vendor = defaults.Vendor
	}
	if asset.Model == "" {
		asset.Model = defaults.Model
	}
	if asset.Status == "" {
		asset.Status = defaults.Status
	}
	if asset.SNMPVersion == "" {
		asset.SNMPVersion = defaults.SNMPVersion
	}
	if asset.SNMPCommunity == "" {
		asset.SNMPCommunity = defaults.SNMPCommunity
	}
	if asset.RoutingSwitchID == "" {
		asset.RoutingSwitchID = defaults.RoutingSwitchID
	}
	if asset.RadiusSecret == "" {
		asset.RadiusSecret = defaults.RadiusSecret
	}
	if asset.SSHUsername == "" {
		asset.SSHUsername = defaults.SSHUsername
	}
	if asset.SSHPassword == "" {
		asset.SSHPassword = defaults.SSHPassword
	}
	if asset.SNMPPort == 0 {
		asset.SNMPPort = defaults.SNMPPort
	}
	if asset.SSHPort == 0 {
		asset.SSHPort = defaults.SSHPort
	}
	if asset.SNMPTimeoutMS == 0 {
		asset.SNMPTimeoutMS = defaults.SNMPTimeoutMS
	}
	if asset.SNMPRetries == 0 {
		asset.SNMPRetries = defaults.SNMPRetries
	}
	return asset
}

func defaultSwitchName(managementIP string) string {
	return "sw-" + strings.ReplaceAll(managementIP, ".", "-")
}

func normalizeBaseMAC(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	value = strings.ReplaceAll(value, "-", ":")
	return value
}

func (s *Service) discoverIdentity(ctx context.Context, asset domain.Switch) (string, string, []string) {
	if s.client == nil {
		return "", "", buildIdentityAliases(asset, "")
	}
	if strings.TrimSpace(asset.ManagementIP) == "" || strings.TrimSpace(asset.SNMPCommunity) == "" {
		return "", "", buildIdentityAliases(asset, "")
	}

	lookupCtx, cancel := context.WithTimeout(ctx, time.Duration(asset.SNMPTimeoutMS+1000)*time.Millisecond)
	defer cancel()

	target := snmp.SwitchTarget{
		Address:   asset.ManagementIP,
		Port:      uint16(asset.SNMPPort),
		Community: asset.SNMPCommunity,
		Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
		Retries:   asset.SNMPRetries,
	}

	systemName, _ := s.client.GetSystemName(lookupCtx, target)
	baseMAC, _ := s.client.GetBaseMAC(lookupCtx, target)

	systemName = strings.TrimSpace(systemName)
	baseMAC = normalizeBaseMAC(baseMAC)

	return systemName, baseMAC, buildIdentityAliases(asset, systemName)
}

func buildIdentityAliases(asset domain.Switch, discoveredSystemName string) []string {
	candidates := []string{
		asset.Name,
		asset.SystemName,
		discoveredSystemName,
		asset.Vendor,
		asset.Model,
	}
	candidates = append(candidates, asset.Aliases...)

	modelSuffix := strings.TrimSpace(asset.Model)
	if idx := strings.IndexByte(modelSuffix, ' '); idx > 0 && idx+1 < len(modelSuffix) {
		modelSuffix = strings.TrimSpace(modelSuffix[idx+1:])
		if modelSuffix != "" {
			candidates = append(candidates, modelSuffix)
			if strings.TrimSpace(asset.Vendor) != "" {
				candidates = append(candidates, strings.TrimSpace(asset.Vendor)+"-"+modelSuffix)
			}
		}
	}

	seen := map[string]struct{}{}
	aliases := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		aliases = append(aliases, candidate)
	}

	return aliases
}

func inferBridgePort(name string) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0
	}
	digitsStart := len(name)
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] < '0' || name[i] > '9' {
			break
		}
		digitsStart = i
	}
	if digitsStart >= len(name) {
		return 0
	}
	port, err := strconv.Atoi(strings.TrimSpace(name[digitsStart:]))
	if err != nil {
		return 0
	}
	return port
}
