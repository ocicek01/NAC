package discoveryjob

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	arpsnapshot "nac/internal/domain/arpsnapshot"
	domain "nac/internal/domain/discoveryjob"
	macipbinding "nac/internal/domain/macipbinding"
	switchasset "nac/internal/domain/switchasset"
	switchport "nac/internal/domain/switchport"
	topologydomain "nac/internal/domain/topology"
	"nac/internal/snmp"
)

type Service struct {
	repository domain.Repository
	switches   switchasset.Repository
	ports      switchport.Repository
	snapshots  arpsnapshot.Repository
	bindings   macipbinding.Repository
	client     snmp.Client
	arp        arpDiscoverer
	topology   topologyRunner
}

const uplinkMACThreshold = 8

type topologyRunner interface {
	DiscoverSwitch(ctx context.Context, switchID string) ([]topologydomain.Link, error)
}

type arpDiscoverer interface {
	DiscoverARP(ctx context.Context, target snmp.SwitchTarget) (snmp.ARPDiscovery, error)
}

func NewService(repository domain.Repository, switches switchasset.Repository, ports switchport.Repository, snapshots arpsnapshot.Repository, bindings macipbinding.Repository, client snmp.Client, arp arpDiscoverer, topology topologyRunner) *Service {
	return &Service{
		repository: repository,
		switches:   switches,
		ports:      ports,
		snapshots:  snapshots,
		bindings:   bindings,
		client:     client,
		arp:        arp,
		topology:   topology,
	}
}

func (s *Service) Create(ctx context.Context, job domain.Job) (domain.Job, error) {
	now := time.Now().UTC()

	job.SwitchID = strings.TrimSpace(job.SwitchID)
	job.Scope = normalizeScope(job.Scope)
	job.RequestedSource = strings.TrimSpace(job.RequestedSource)
	job.RequestedBy = strings.TrimSpace(job.RequestedBy)

	if job.Scope == "" {
		return domain.Job{}, fmt.Errorf("scope is required")
	}

	if job.SwitchID != "" && s.switches != nil {
		asset, err := s.switches.FindByID(ctx, job.SwitchID)
		if err != nil {
			return domain.Job{}, err
		}
		if asset == nil {
			return domain.Job{}, fmt.Errorf("switch %q not found", job.SwitchID)
		}
	}

	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if job.RequestedSource == "" {
		job.RequestedSource = "api"
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 3
	}
	if job.Summary == nil {
		job.Summary = map[string]any{}
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	return s.repository.Insert(ctx, job)
}

func (s *Service) FindByID(ctx context.Context, id string) (*domain.Job, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("job id is required")
	}
	return s.repository.FindByID(ctx, id)
}

func (s *Service) StartNext(ctx context.Context, workerID string) (*domain.Job, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = "worker-1"
	}

	job, err := s.repository.ClaimNextQueued(ctx, workerID)
	if err != nil || job == nil {
		return job, err
	}

	return s.executeClaimed(ctx, *job)
}

func (s *Service) StartByID(ctx context.Context, id, workerID string) (*domain.Job, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("job id is required")
	}

	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = "worker-1"
	}

	job, err := s.repository.ClaimQueuedByID(ctx, id, workerID)
	if err != nil || job == nil {
		return job, err
	}

	return s.executeClaimed(ctx, *job)
}

func (s *Service) DispatchByID(ctx context.Context, id, workerID string) (*domain.Job, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("job id is required")
	}

	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = "worker-1"
	}

	job, err := s.repository.ClaimQueuedByID(ctx, id, workerID)
	if err != nil || job == nil {
		return job, err
	}

	go s.executeDetached(*job)

	return job, nil
}

func normalizeScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "ports":
		return "ports"
	case "arp":
		return "arp"
	case "topology":
		return "topology"
	case "full":
		return "full"
	default:
		return ""
	}
}

func (s *Service) executeClaimed(ctx context.Context, job domain.Job) (*domain.Job, error) {
	now := time.Now().UTC()
	job.ProgressPercent = 10
	job.CurrentStep = "resolving-switch"
	job.UpdatedAt = now

	updated, err := s.repository.Update(ctx, job)
	if err != nil {
		return nil, err
	}
	job = updated

	if job.Scope != "ports" && job.Scope != "arp" && job.Scope != "full" && job.Scope != "topology" {
		job.Status = "failed"
		job.ErrorMessage = fmt.Sprintf("scope %q is not implemented yet", job.Scope)
		job.CurrentStep = "failed"
		job.CompletedAt = time.Now().UTC()
		job.UpdatedAt = job.CompletedAt
		job.ProgressPercent = 100
		failed, updateErr := s.repository.Update(ctx, job)
		if updateErr != nil {
			return nil, updateErr
		}
		return &failed, nil
	}

	asset, err := s.switches.FindByID(ctx, job.SwitchID)
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	if asset == nil {
		return s.failJob(ctx, job, fmt.Errorf("switch %q not found", job.SwitchID))
	}
	if job.Scope == "topology" {
		if s.topology == nil {
			return s.failJob(ctx, job, fmt.Errorf("topology discovery is not configured"))
		}
		return s.runTopologyDiscovery(ctx, job, *asset)
	}
	if job.Scope == "arp" {
		if s.client == nil || s.snapshots == nil || s.bindings == nil {
			return s.failJob(ctx, job, fmt.Errorf("arp discovery dependencies are not configured"))
		}
		return s.runARPDiscovery(ctx, job, *asset)
	}
	if s.client == nil || s.ports == nil {
		return s.failJob(ctx, job, fmt.Errorf("discovery dependencies are not configured"))
	}

	job.CurrentStep = "walking-interfaces"
	job.ProgressPercent = 30
	job.UpdatedAt = time.Now().UTC()
	job, err = s.repository.Update(ctx, job)
	if err != nil {
		return nil, err
	}

	target := snmp.SwitchTarget{
		Address:   asset.ManagementIP,
		Port:      uint16(asset.SNMPPort),
		Community: asset.SNMPCommunity,
		Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
		Retries:   asset.SNMPRetries,
		Vendor:    asset.Vendor,
		Model:     asset.Model,
	}

	lookupCtx, cancel := context.WithTimeout(ctx, discoveryLookupTimeout(*asset))
	defer cancel()

	interfaceStates, err := s.client.WalkInterfaces(lookupCtx, target)
	if err != nil {
		_, _ = s.switches.UpdatePollStatus(ctx, asset.ID, time.Now().UTC(), err.Error())
		return s.failJob(ctx, job, err)
	}

	trunkPorts, _ := s.client.DiscoverTrunkPorts(lookupCtx, target)
	trunkSet := make(map[int]struct{}, len(trunkPorts))
	for _, trunk := range trunkPorts {
		if trunk.IfIndex > 0 && trunk.State == 1 {
			trunkSet[trunk.IfIndex] = struct{}{}
		}
	}

	neighborsByIfIndex := s.discoverPortNeighbors(lookupCtx, *asset, target)

	fdbDiscovery, _ := s.client.DiscoverFDB(lookupCtx, target)
	fdbEntries := fdbDiscovery.Entries
	macsByIfIndex := make(map[int][]string)
	for _, entry := range fdbEntries {
		if entry.IfIndex <= 0 || strings.TrimSpace(entry.MACAddress) == "" {
			continue
		}
		macsByIfIndex[entry.IfIndex] = append(macsByIfIndex[entry.IfIndex], entry.MACAddress)
	}
	for ifIndex := range macsByIfIndex {
		macsByIfIndex[ifIndex] = uniqueSortedStrings(macsByIfIndex[ifIndex])
	}

	job.CurrentStep = "persisting-ports"
	job.ProgressPercent = 75
	job.UpdatedAt = time.Now().UTC()
	job, err = s.repository.Update(ctx, job)
	if err != nil {
		return nil, err
	}

	observedAt := time.Now().UTC()
	ports := make([]switchport.Port, 0, len(interfaceStates))
	portsWithMACs := 0
	neighborPortCount := 0
	uplinkPortCount := 0
	for _, item := range interfaceStates {
		if !isPhysicalInterface(item.Name, item.Description, item.Alias) {
			continue
		}

		_, isTrunk := trunkSet[item.IfIndex]
		portMACs := macsByIfIndex[item.IfIndex]
		if len(portMACs) > 0 {
			portsWithMACs++
		}
		neighbor := neighborsByIfIndex[item.IfIndex]
		isUplink := isTrunk || neighbor.hasResolvedSwitch() || len(portMACs) >= uplinkMACThreshold
		if neighbor.Protocol != "" {
			neighborPortCount++
		}
		if isUplink {
			uplinkPortCount++
		}
		ports = append(ports, switchport.Port{
			ID:                   uuid.NewString(),
			SwitchID:             asset.ID,
			IfIndex:              item.IfIndex,
			PortIndex:            inferPortIndex(item.Name, item.IfIndex),
			InterfaceName:        strings.TrimSpace(item.Name),
			InterfaceAlias:       strings.TrimSpace(item.Alias),
			InterfaceDescription: strings.TrimSpace(item.Description),
			PortLabel:            strings.TrimSpace(item.Name),
			InterfaceType:        "physical",
			AdminStatus:          strings.TrimSpace(item.AdminStatus),
			OperStatus:           strings.TrimSpace(item.OperStatus),
			Status:               derivePortStatus(item.AdminStatus, item.OperStatus),
			PortMode:             derivePortMode(isTrunk),
			IsPhysical:           true,
			IsUplink:             isUplink,
			IsTrunk:              isTrunk,
			TrunkSource:          trunkSource(isTrunk, asset.Vendor),
			AllowedVLANs:         []string{},
			MACCount:             len(portMACs),
			MACAddresses:         portMACs,
			NeighborProtocol:     neighbor.Protocol,
			NeighborSwitchID:     neighbor.SwitchID,
			NeighborSwitchName:   neighbor.SwitchName,
			NeighborPortName:     neighbor.PortName,
			NeighborPlatform:     neighbor.Platform,
			NeighborDescription:  neighbor.Description,
			NeighborData:         neighbor.toMap(),
			Metadata:             map[string]any{},
			LastDiscoveredAt:     observedAt,
			CreatedAt:            observedAt,
			UpdatedAt:            observedAt,
		})
	}

	if err := s.ports.ReplaceBySwitch(ctx, asset.ID, ports); err != nil {
		_, _ = s.switches.UpdatePollStatus(ctx, asset.ID, time.Now().UTC(), err.Error())
		return s.failJob(ctx, job, err)
	}

	_, _ = s.switches.UpdatePollStatus(ctx, asset.ID, observedAt, "")

	job.Status = "completed"
	job.CurrentStep = "completed"
	job.ProgressPercent = 100
	job.ErrorMessage = ""
	summary := map[string]any{
		"switch_id":            asset.ID,
		"switch_name":          asset.Name,
		"raw_interface_count":  len(interfaceStates),
		"discovered_ports":     len(ports),
		"trunk_port_count":     len(trunkSet),
		"uplink_port_count":    uplinkPortCount,
		"neighbor_port_count":  neighborPortCount,
		"ports_with_macs":      portsWithMACs,
		"fdb_source":           fdbDiscovery.Source,
		"fdb_entry_count":      len(fdbEntries),
		"fdb_ifindex_count":    len(macsByIfIndex),
		"fdb_dot1d_rows":       fdbDiscovery.Dot1DRows,
		"fdb_qbridge_rows":     fdbDiscovery.QBridgeRows,
		"fdb_bridge_port_rows": fdbDiscovery.BridgePortMapRows,
		"fdb_mapped_entries":   fdbDiscovery.MappedEntries,
		"observed_at":          observedAt.Format(time.RFC3339),
	}

	if job.Scope == "full" {
		job.CurrentStep = "syncing-topology"
		job.ProgressPercent = 90
		job.UpdatedAt = time.Now().UTC()
		job, err = s.repository.Update(ctx, job)
		if err != nil {
			return nil, err
		}

		topologyLinks, topologySummary, topoErr := s.discoverTopologyForSwitch(ctx, asset.ID)
		if topoErr != nil {
			return s.failJob(ctx, job, topoErr)
		}
		for key, value := range topologySummary {
			summary[key] = value
		}
		summary["topology_link_count"] = len(topologyLinks)
	}

	job.Status = "completed"
	job.CurrentStep = "completed"
	job.ProgressPercent = 100
	job.Summary = summary
	job.CompletedAt = observedAt
	job.UpdatedAt = observedAt
	completed, err := s.repository.Update(ctx, job)
	if err != nil {
		return nil, err
	}
	return &completed, nil
}

func (s *Service) executeDetached(job domain.Job) {
	_, _ = s.executeClaimed(context.Background(), job)
}

func (s *Service) runARPDiscovery(ctx context.Context, job domain.Job, asset switchasset.Switch) (*domain.Job, error) {
	pollAsset, err := s.resolveARPSwitch(ctx, asset)
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	job.Summary = map[string]any{
		"switch_id":       asset.ID,
		"switch_name":     asset.Name,
		"arp_switch_id":   pollAsset.ID,
		"arp_switch_name": pollAsset.Name,
	}

	job.CurrentStep = "walking-arp"
	job.ProgressPercent = 35
	job.UpdatedAt = time.Now().UTC()

	updated, err := s.repository.Update(ctx, job)
	if err != nil {
		return nil, err
	}
	job = updated

	target := snmp.SwitchTarget{
		Address:        pollAsset.ManagementIP,
		Port:           uint16(pollAsset.SNMPPort),
		Community:      pollAsset.SNMPCommunity,
		Timeout:        arpSNMPTimeout(pollAsset),
		Retries:        arpSNMPRetries(pollAsset),
		MaxRepetitions: 10,
		Vendor:         pollAsset.Vendor,
		Model:          pollAsset.Model,
	}

	lookupCtx, cancel := context.WithTimeout(ctx, discoveryLookupTimeout(pollAsset))
	defer cancel()

	arpDiscovery, err := s.discoverARPWithFallback(lookupCtx, pollAsset, target)
	if err != nil {
		_, _ = s.switches.UpdatePollStatus(ctx, pollAsset.ID, time.Now().UTC(), err.Error())
		return s.failJob(ctx, job, err)
	}

	job.CurrentStep = "persisting-arp"
	job.ProgressPercent = 80
	job.UpdatedAt = time.Now().UTC()
	job, err = s.repository.Update(ctx, job)
	if err != nil {
		return nil, err
	}

	observedAt := time.Now().UTC()
	snapshots := make([]arpsnapshot.Snapshot, 0, len(arpDiscovery.Entries))
	bindings := make([]macipbinding.Binding, 0, len(arpDiscovery.Entries))
	arpIfIndexes := make(map[int]struct{}, len(arpDiscovery.Entries))
	for _, entry := range arpDiscovery.Entries {
		arpIfIndexes[entry.IfIndex] = struct{}{}
		snapshots = append(snapshots, arpsnapshot.Snapshot{
			ID:          uuid.NewString(),
			SwitchID:    pollAsset.ID,
			IfIndex:     entry.IfIndex,
			MACAddress:  entry.MACAddress,
			IPAddress:   entry.IPAddress,
			Source:      "arp",
			VLANID:      0,
			FirstSeenAt: observedAt,
			LastSeenAt:  observedAt,
			CreatedAt:   observedAt,
			UpdatedAt:   observedAt,
		})
		bindings = append(bindings, macipbinding.Binding{
			ID:          uuid.NewString(),
			SwitchID:    pollAsset.ID,
			MACAddress:  entry.MACAddress,
			IPAddress:   entry.IPAddress,
			Source:      "arp",
			Hostname:    "",
			VendorClass: "",
			Options55:   "",
			VLANID:      0,
			FirstSeenAt: observedAt,
			LastSeenAt:  observedAt,
			CreatedAt:   observedAt,
			UpdatedAt:   observedAt,
		})
	}

	if err := s.snapshots.UpsertBatch(ctx, snapshots); err != nil {
		_, _ = s.switches.UpdatePollStatus(ctx, pollAsset.ID, time.Now().UTC(), err.Error())
		return s.failJob(ctx, job, err)
	}
	if err := s.bindings.UpsertBatch(ctx, bindings); err != nil {
		_, _ = s.switches.UpdatePollStatus(ctx, pollAsset.ID, time.Now().UTC(), err.Error())
		return s.failJob(ctx, job, err)
	}

	_, _ = s.switches.UpdatePollStatus(ctx, pollAsset.ID, observedAt, "")

	job.Status = "completed"
	job.CurrentStep = "completed"
	job.ProgressPercent = 100
	job.ErrorMessage = ""
	job.Summary = map[string]any{
		"switch_id":               asset.ID,
		"switch_name":             asset.Name,
		"arp_switch_id":           pollAsset.ID,
		"arp_switch_name":         pollAsset.Name,
		"arp_entry_count":         len(arpDiscovery.Entries),
		"arp_ifindex_count":       len(arpIfIndexes),
		"arp_source":              arpDiscovery.Source,
		"ip_net_to_media_rows":    arpDiscovery.IPNetToMediaRows,
		"ip_net_to_physical_rows": arpDiscovery.IPNetToPhysicalRows,
		"mapped_entries":          arpDiscovery.MappedEntries,
		"observed_at":             observedAt.Format(time.RFC3339),
	}
	job.CompletedAt = observedAt
	job.UpdatedAt = observedAt
	completed, err := s.repository.Update(ctx, job)
	if err != nil {
		return nil, err
	}
	return &completed, nil
}

func (s *Service) resolveARPSwitch(ctx context.Context, asset switchasset.Switch) (switchasset.Switch, error) {
	if strings.TrimSpace(asset.RoutingSwitchID) == "" {
		return asset, nil
	}
	related, err := s.switches.FindByID(ctx, asset.RoutingSwitchID)
	if err != nil {
		return switchasset.Switch{}, err
	}
	if related == nil {
		return switchasset.Switch{}, fmt.Errorf("routing switch %q not found", asset.RoutingSwitchID)
	}
	return *related, nil
}

func (s *Service) discoverARPWithFallback(ctx context.Context, asset switchasset.Switch, target snmp.SwitchTarget) (snmp.ARPDiscovery, error) {
	preferExternal := prefersExternalARP(asset)

	if preferExternal && s.arp != nil {
		discovery, err := s.arp.DiscoverARP(ctx, target)
		if err == nil {
			return discovery, nil
		}
		if s.client == nil {
			return snmp.ARPDiscovery{}, err
		}
		primary, primaryErr := s.client.DiscoverARP(ctx, target)
		if primaryErr == nil {
			return primary, nil
		}
		return snmp.ARPDiscovery{}, fmt.Errorf("external arp collector failed: %v; gosnmp fallback failed: %v", err, primaryErr)
	}

	if s.client != nil {
		discovery, err := s.client.DiscoverARP(ctx, target)
		if err == nil {
			return discovery, nil
		}
		if s.arp == nil {
			return snmp.ARPDiscovery{}, err
		}
		fallback, fallbackErr := s.arp.DiscoverARP(ctx, target)
		if fallbackErr == nil {
			return fallback, nil
		}
		return snmp.ARPDiscovery{}, fmt.Errorf("gosnmp arp collector failed: %v; external fallback failed: %v", err, fallbackErr)
	}

	if s.arp != nil {
		return s.arp.DiscoverARP(ctx, target)
	}

	return snmp.ARPDiscovery{}, fmt.Errorf("arp discovery dependencies are not configured")
}

func prefersExternalARP(asset switchasset.Switch) bool {
	value := strings.ToLower(strings.TrimSpace(asset.Vendor + " " + asset.Model))
	return strings.Contains(value, "hp") ||
		strings.Contains(value, "hpe") ||
		strings.Contains(value, "aruba") ||
		strings.Contains(value, "procurve") ||
		strings.Contains(value, "zl")
}

func (s *Service) runTopologyDiscovery(ctx context.Context, job domain.Job, asset switchasset.Switch) (*domain.Job, error) {
	topologyLinks, topologySummary, err := s.discoverTopologyForSwitch(ctx, asset.ID)
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	now := time.Now().UTC()
	job.Status = "completed"
	job.CurrentStep = "completed"
	job.ProgressPercent = 100
	job.ErrorMessage = ""
	job.Summary = map[string]any{
		"switch_id":           asset.ID,
		"switch_name":         asset.Name,
		"topology_link_count": len(topologyLinks),
		"observed_at":         now.Format(time.RFC3339),
	}
	for key, value := range topologySummary {
		job.Summary[key] = value
	}
	job.CompletedAt = now
	job.UpdatedAt = now
	completed, updateErr := s.repository.Update(ctx, job)
	if updateErr != nil {
		return nil, updateErr
	}
	return &completed, nil
}

func (s *Service) discoverTopologyForSwitch(ctx context.Context, switchID string) ([]topologydomain.Link, map[string]any, error) {
	if s.topology == nil {
		return nil, nil, fmt.Errorf("topology discovery is not configured")
	}

	links, err := s.topology.DiscoverSwitch(ctx, switchID)
	if err != nil {
		return nil, nil, err
	}

	summary := map[string]any{
		"topology_link_count": len(links),
		"cdp_link_count":      0,
		"lldp_link_count":     0,
	}
	for _, link := range links {
		switch strings.ToLower(strings.TrimSpace(link.DiscoveryMethod)) {
		case "cdp":
			summary["cdp_link_count"] = summary["cdp_link_count"].(int) + 1
		case "lldp":
			summary["lldp_link_count"] = summary["lldp_link_count"].(int) + 1
		}
	}

	return links, summary, nil
}

func (s *Service) failJob(ctx context.Context, job domain.Job, cause error) (*domain.Job, error) {
	job.Status = "failed"
	job.CurrentStep = "failed"
	job.ProgressPercent = 100
	job.ErrorMessage = cause.Error()
	job.CompletedAt = time.Now().UTC()
	job.UpdatedAt = job.CompletedAt
	failed, err := s.repository.Update(ctx, job)
	if err != nil {
		return nil, err
	}
	return &failed, nil
}

type discoveredNeighbor struct {
	Protocol    string
	SwitchID    string
	SwitchName  string
	PortName    string
	Platform    string
	Description string
}

func (n discoveredNeighbor) hasResolvedSwitch() bool {
	return strings.TrimSpace(n.SwitchID) != ""
}

func (n discoveredNeighbor) toMap() map[string]any {
	if strings.TrimSpace(n.Protocol) == "" &&
		strings.TrimSpace(n.SwitchID) == "" &&
		strings.TrimSpace(n.SwitchName) == "" &&
		strings.TrimSpace(n.PortName) == "" &&
		strings.TrimSpace(n.Platform) == "" &&
		strings.TrimSpace(n.Description) == "" {
		return map[string]any{}
	}

	return map[string]any{
		"protocol":             n.Protocol,
		"neighbor_switch_id":   n.SwitchID,
		"neighbor_switch_name": n.SwitchName,
		"neighbor_port_name":   n.PortName,
		"neighbor_platform":    n.Platform,
		"neighbor_description": n.Description,
	}
}

func (s *Service) discoverPortNeighbors(ctx context.Context, asset switchasset.Switch, target snmp.SwitchTarget) map[int]discoveredNeighbor {
	result := make(map[int]discoveredNeighbor)
	nameIndex := make(map[string]int)

	interfaceStates, err := s.client.WalkInterfaces(ctx, target)
	if err == nil {
		for _, item := range interfaceStates {
			ifIndex := item.IfIndex
			if ifIndex <= 0 {
				continue
			}

			if normalized := normalizePortLookupKey(item.Name); normalized != "" {
				nameIndex[normalized] = ifIndex
			}
			if inferred := inferPortIndex(item.Name, 0); inferred > 0 {
				nameIndex[strconv.Itoa(inferred)] = ifIndex
			}
		}
	}

	cdpNeighbors, err := s.client.DiscoverCDPNeighbors(ctx, target)
	if err == nil {
		for _, neighbor := range cdpNeighbors {
			ifIndex := neighbor.LocalIfIndex
			if ifIndex <= 0 {
				ifIndex = resolvePortIfIndex(nameIndex, neighbor.LocalPortName, 0)
			}
			if ifIndex <= 0 {
				continue
			}

			result[ifIndex] = s.buildDiscoveredNeighbor(ctx, asset, "cdp", neighbor.RemoteSystemName, neighbor.RemotePortName, neighbor.RemotePlatform, neighbor.RemoteDescription)
		}
	}

	lldpNeighbors, err := s.client.DiscoverLLDPNeighbors(ctx, target)
	if err == nil {
		for _, neighbor := range lldpNeighbors {
			ifIndex := resolvePortIfIndex(nameIndex, neighbor.LocalPortName, neighbor.LocalPortIndex)
			if ifIndex <= 0 {
				continue
			}
			if _, exists := result[ifIndex]; exists {
				continue
			}

			result[ifIndex] = s.buildDiscoveredNeighbor(ctx, asset, "lldp", neighbor.RemoteSystemName, neighbor.RemotePortName, neighbor.RemoteSystemDesc, neighbor.RemotePortDesc)
		}
	}

	return result
}

func (s *Service) buildDiscoveredNeighbor(ctx context.Context, asset switchasset.Switch, protocol, remoteSystemName, remotePortName, platform, description string) discoveredNeighbor {
	neighbor := discoveredNeighbor{
		Protocol:    strings.TrimSpace(protocol),
		SwitchName:  strings.TrimSpace(remoteSystemName),
		PortName:    strings.TrimSpace(remotePortName),
		Platform:    strings.TrimSpace(platform),
		Description: strings.TrimSpace(description),
	}

	if target, err := s.switches.FindByNeighborName(ctx, neighbor.SwitchName); err == nil && target != nil && target.ID != asset.ID {
		neighbor.SwitchID = target.ID
		neighbor.SwitchName = target.Name
	}

	return neighbor
}

func inferPortIndex(name string, fallback int) int {
	name = strings.TrimSpace(name)
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] < '0' || name[i] > '9' {
			if i == len(name)-1 {
				break
			}
			value, err := strconv.Atoi(name[i+1:])
			if err == nil && value > 0 {
				return value
			}
			break
		}
	}
	return fallback
}

func derivePortStatus(adminStatus, operStatus string) string {
	adminStatus = strings.ToLower(strings.TrimSpace(adminStatus))
	operStatus = strings.ToLower(strings.TrimSpace(operStatus))
	if adminStatus == "down" {
		return "disabled"
	}
	if operStatus == "up" {
		return "up"
	}
	return "down"
}

func derivePortMode(isTrunk bool) string {
	if isTrunk {
		return "trunk"
	}
	return "access"
}

func trunkSource(isTrunk bool, vendor string) string {
	if !isTrunk {
		return ""
	}
	vendor = strings.TrimSpace(strings.ToLower(vendor))
	if vendor == "" {
		return "snmp-trunk"
	}
	return vendor + "-snmp-trunk"
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToUpper(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizePortLookupKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func resolvePortIfIndex(nameIndex map[string]int, localPortName string, localPortIndex int) int {
	if normalized := normalizePortLookupKey(localPortName); normalized != "" {
		if ifIndex := nameIndex[normalized]; ifIndex > 0 {
			return ifIndex
		}
	}

	if localPortIndex > 0 {
		if ifIndex := nameIndex[strconv.Itoa(localPortIndex)]; ifIndex > 0 {
			return ifIndex
		}
		return localPortIndex
	}

	if inferred := inferPortIndex(localPortName, 0); inferred > 0 {
		if ifIndex := nameIndex[strconv.Itoa(inferred)]; ifIndex > 0 {
			return ifIndex
		}
	}

	return 0
}

func isPhysicalInterface(name, description, alias string) bool {
	combined := strings.ToLower(strings.TrimSpace(name + " " + description + " " + alias))
	if combined == "" {
		return false
	}

	excluded := []string{
		"vlan-interface",
		"vlanif",
		"loopback",
		"null",
		"bridge-aggregation",
		"port-channel",
		"trunk",
		"inloopback",
		"stack-port",
		"mgmt",
		"management",
	}
	for _, token := range excluded {
		if strings.Contains(combined, token) {
			return false
		}
	}

	includedPrefixes := []string{
		"gigabitethernet",
		"fastethernet",
		"ethernet",
		"tengigabitethernet",
		"xgigabitethernet",
		"ge",
		"xe",
		"te",
	}
	trimmed := strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range includedPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	return inferPortIndex(name, 0) > 0
}

func discoveryLookupTimeout(asset switchasset.Switch) time.Duration {
	timeout := time.Duration(asset.SNMPTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	retries := asset.SNMPRetries
	if retries < 0 {
		retries = 0
	}

	// Interface/FDB walks are much heavier than a single SNMP get.
	// Give the worker enough time for multiple walks plus retry overhead.
	budget := timeout*time.Duration(retries+1)*8 + 5*time.Second
	if budget < 20*time.Second {
		budget = 20 * time.Second
	}
	if budget > 2*time.Minute {
		budget = 2 * time.Minute
	}

	return budget
}

func arpSNMPTimeout(asset switchasset.Switch) time.Duration {
	timeout := time.Duration(asset.SNMPTimeoutMS) * time.Millisecond
	if timeout < 15*time.Second {
		timeout = 15 * time.Second
	}
	if timeout > 45*time.Second {
		timeout = 45 * time.Second
	}
	return timeout
}

func arpSNMPRetries(asset switchasset.Switch) int {
	if asset.SNMPRetries < 2 {
		return 2
	}
	if asset.SNMPRetries > 4 {
		return 4
	}
	return asset.SNMPRetries
}
