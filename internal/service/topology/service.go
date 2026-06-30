package topology

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	switchasset "nac/internal/domain/switchasset"
	domain "nac/internal/domain/topology"
	"nac/internal/snmp"
)

type Service struct {
	repository domain.Repository
	switches   switchasset.Repository
	client     snmp.Client
}

func NewService(repository domain.Repository, switches switchasset.Repository, client snmp.Client) *Service {
	return &Service{
		repository: repository,
		switches:   switches,
		client:     client,
	}
}

func (s *Service) Create(ctx context.Context, link domain.Link) (domain.Link, error) {
	now := time.Now().UTC()

	if link.ID == "" {
		link.ID = uuid.NewString()
	}

	link.SourceSwitchID = strings.TrimSpace(link.SourceSwitchID)
	link.SourceSwitchName = strings.TrimSpace(link.SourceSwitchName)
	link.SourcePortName = strings.TrimSpace(link.SourcePortName)
	link.TargetSwitchID = strings.TrimSpace(link.TargetSwitchID)
	link.TargetSwitchName = strings.TrimSpace(link.TargetSwitchName)
	link.TargetPortName = strings.TrimSpace(link.TargetPortName)
	link.DiscoveryMethod = strings.TrimSpace(link.DiscoveryMethod)
	link.Status = strings.TrimSpace(link.Status)

	if link.SourceSwitchID == "" || link.SourcePortName == "" {
		return domain.Link{}, fmt.Errorf("source_switch_id and source_port_name are required")
	}

	if link.SourceSwitchName == "" {
		link.SourceSwitchName = link.SourceSwitchID
	}
	if link.TargetSwitchName == "" && link.TargetSwitchID != "" {
		link.TargetSwitchName = link.TargetSwitchID
	}
	if link.DiscoveryMethod == "" {
		link.DiscoveryMethod = "manual"
	}
	if link.Status == "" {
		link.Status = "active"
	}
	if link.LastObservedAt.IsZero() {
		link.LastObservedAt = now
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = now
	}
	if link.UpdatedAt.IsZero() {
		link.UpdatedAt = now
	}

	return s.repository.Upsert(ctx, link)
}

func (s *Service) List(ctx context.Context) ([]domain.Link, error) {
	return s.repository.List(ctx)
}

func (s *Service) HasLinkedInterface(ctx context.Context, switchID, interfaceName string) (bool, error) {
	return s.repository.HasLinkedInterface(ctx, switchID, interfaceName)
}

func (s *Service) FindLinkedSwitchID(ctx context.Context, switchID, interfaceName string) (string, error) {
	return s.repository.FindLinkedSwitchID(ctx, switchID, interfaceName)
}

func (s *Service) CountLinkedSwitches(ctx context.Context, switchID, interfaceName string) (int, error) {
	return s.repository.CountLinkedSwitches(ctx, switchID, interfaceName)
}

func (s *Service) DiscoverLLDP(ctx context.Context) ([]domain.Link, error) {
	assets, err := s.switches.ListEnabledSNMP(ctx)
	if err != nil {
		return nil, err
	}

	links := make([]domain.Link, 0)
	for _, asset := range assets {
		discovered, err := s.DiscoverSwitch(ctx, asset.ID)
		if err != nil {
			return nil, err
		}
		links = append(links, discovered...)
	}

	return links, nil
}

func (s *Service) DiscoverSwitch(ctx context.Context, switchID string) ([]domain.Link, error) {
	asset, err := s.switches.FindByID(ctx, strings.TrimSpace(switchID))
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("switch not found")
	}

	now := time.Now().UTC()
	target := snmp.SwitchTarget{
		Address:   asset.ManagementIP,
		Port:      uint16(asset.SNMPPort),
		Community: asset.SNMPCommunity,
		Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
		Retries:   asset.SNMPRetries,
		Vendor:    asset.Vendor,
		Model:     asset.Model,
	}

	allowedPorts := s.discoverTrunkPortSet(ctx, target)
	discovered := s.collectDiscoveredLinks(ctx, *asset, target, allowedPorts, now)
	links := make([]domain.Link, 0, len(discovered))
	for _, link := range discovered {
		stored, storeErr := s.repository.Upsert(ctx, link)
		if storeErr != nil {
			continue
		}
		links = append(links, stored)
	}

	if pruneErr := s.repository.PruneDiscovered(ctx, asset.ID, []string{"cdp", "lldp"}, now); pruneErr != nil {
		return nil, pruneErr
	}

	return links, nil
}

func (s *Service) collectDiscoveredLinks(ctx context.Context, asset switchasset.Switch, target snmp.SwitchTarget, allowedPorts map[string]struct{}, observedAt time.Time) []domain.Link {
	links := make([]domain.Link, 0)
	seenSourcePorts := make(map[string]struct{})

	cdpNeighbors, err := s.client.DiscoverCDPNeighbors(ctx, target)
	if err == nil {
		for _, neighbor := range cdpNeighbors {
			if !isAllowedTopologyPort(allowedPorts, strings.TrimSpace(neighbor.LocalPortName), neighbor.LocalIfIndex) {
				continue
			}
			resolvedTarget, ok := s.resolveTopologyNeighborTarget(ctx, asset, neighbor.RemoteSystemName, neighbor.RemotePlatform, neighbor.RemoteDescription)
			if !ok {
				continue
			}

			link := s.buildLink(asset, strings.TrimSpace(neighbor.LocalPortName), strings.TrimSpace(neighbor.RemoteSystemName), strings.TrimSpace(neighbor.RemotePortName), "cdp", observedAt, resolvedTarget)
			if link.SourcePortName == "" || link.TargetSwitchName == "" {
				continue
			}

			key := strings.ToLower(link.SourcePortName)
			if _, exists := seenSourcePorts[key]; exists {
				continue
			}
			seenSourcePorts[key] = struct{}{}
			links = append(links, link)
		}
	}

	lldpNeighbors, err := s.client.DiscoverLLDPNeighbors(ctx, target)
	if err == nil {
		for _, neighbor := range lldpNeighbors {
			if !isAllowedTopologyPort(allowedPorts, strings.TrimSpace(neighbor.LocalPortName), neighbor.LocalPortIndex) {
				continue
			}
			resolvedTarget, ok := s.resolveTopologyNeighborTarget(ctx, asset, neighbor.RemoteSystemName, neighbor.RemoteSystemDesc, neighbor.RemotePortDesc)
			if !ok {
				continue
			}

			link := s.buildLink(asset, strings.TrimSpace(neighbor.LocalPortName), strings.TrimSpace(neighbor.RemoteSystemName), strings.TrimSpace(neighbor.RemotePortName), "lldp", observedAt, resolvedTarget)
			if link.SourcePortName == "" || link.TargetSwitchName == "" {
				continue
			}

			key := strings.ToLower(link.SourcePortName)
			if _, exists := seenSourcePorts[key]; exists {
				continue
			}
			seenSourcePorts[key] = struct{}{}
			links = append(links, link)
		}
	}

	return links
}

func (s *Service) discoverTrunkPortSet(ctx context.Context, target snmp.SwitchTarget) map[string]struct{} {
	trunks, err := s.client.DiscoverTrunkPorts(ctx, target)
	if err != nil || len(trunks) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(trunks))
	for _, trunk := range trunks {
		if trunk.IfIndex <= 0 {
			continue
		}
		allowed[strconv.Itoa(trunk.IfIndex)] = struct{}{}
	}

	if len(allowed) == 0 {
		return nil
	}

	return allowed
}

func (s *Service) buildLink(asset switchasset.Switch, sourcePortName, remoteSystemName, remotePortName, method string, observedAt time.Time, target *switchasset.Switch) domain.Link {
	link := domain.Link{
		ID:               uuid.NewString(),
		SourceSwitchID:   asset.ID,
		SourceSwitchName: asset.Name,
		SourcePortName:   sourcePortName,
		TargetSwitchName: remoteSystemName,
		TargetPortName:   remotePortName,
		DiscoveryMethod:  method,
		Status:           "active",
		LastObservedAt:   observedAt,
		CreatedAt:        observedAt,
		UpdatedAt:        observedAt,
	}

	if target != nil {
		if target.ID != asset.ID {
			link.TargetSwitchID = target.ID
			link.TargetSwitchName = target.Name
		}
	}

	return link
}

func (s *Service) resolveTopologyNeighborTarget(ctx context.Context, asset switchasset.Switch, remoteSystemName string, hints ...string) (*switchasset.Switch, bool) {
	remoteSystemName = strings.TrimSpace(remoteSystemName)
	if remoteSystemName == "" {
		return nil, false
	}

	if target, err := s.switches.FindByNeighborName(ctx, remoteSystemName); err == nil && target != nil {
		if target.ID == asset.ID {
			return nil, false
		}
		return target, true
	}

	return nil, false
}

func isAllowedTopologyPort(allowedPorts map[string]struct{}, portName string, portIndex int) bool {
	if len(allowedPorts) == 0 {
		return true
	}

	if _, ok := allowedPorts[strconv.Itoa(portIndex)]; ok {
		return true
	}

	if inferred := inferPortIndexFromName(portName); inferred > 0 {
		_, ok := allowedPorts[strconv.Itoa(inferred)]
		return ok
	}

	return false
}

func inferPortIndexFromName(name string) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0
	}

	end := len(name) - 1
	for end >= 0 && (name[end] < '0' || name[end] > '9') {
		end--
	}
	if end < 0 {
		return 0
	}

	start := end
	for start >= 0 && name[start] >= '0' && name[start] <= '9' {
		start--
	}

	value, err := strconv.Atoi(name[start+1 : end+1])
	if err != nil {
		return 0
	}

	return value
}
