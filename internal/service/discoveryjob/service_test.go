package discoveryjob

import (
	"context"
	"strings"
	"testing"
	"time"

	arpsnapshot "nac/internal/domain/arpsnapshot"
	domain "nac/internal/domain/discoveryjob"
	macipbinding "nac/internal/domain/macipbinding"
	switchasset "nac/internal/domain/switchasset"
	switchport "nac/internal/domain/switchport"
	topologydomain "nac/internal/domain/topology"
	"nac/internal/snmp"
)

type stubJobRepository struct {
	inserted domain.Job
	found    *domain.Job
	claimed  *domain.Job
}

func (r *stubJobRepository) Insert(_ context.Context, job domain.Job) (domain.Job, error) {
	r.inserted = job
	return job, nil
}

func (r *stubJobRepository) FindByID(_ context.Context, id string) (*domain.Job, error) {
	if r.found == nil || r.found.ID != id {
		return nil, nil
	}
	return r.found, nil
}

func (r *stubJobRepository) Update(_ context.Context, job domain.Job) (domain.Job, error) {
	r.inserted = job
	return job, nil
}

func (r *stubJobRepository) ListBySwitch(_ context.Context, _ string, _ int) ([]domain.Job, error) {
	return nil, nil
}

func (r *stubJobRepository) ClaimNextQueued(_ context.Context, workerID string) (*domain.Job, error) {
	if r.claimed != nil {
		job := *r.claimed
		job.WorkerID = workerID
		return &job, nil
	}
	job := &domain.Job{
		ID:              "job-1",
		SwitchID:        "sw-1",
		Scope:           "ports",
		Status:          "running",
		WorkerID:        workerID,
		CurrentStep:     "claimed",
		ProgressPercent: 5,
	}
	return job, nil
}

func (r *stubJobRepository) ClaimQueuedByID(_ context.Context, id, workerID string) (*domain.Job, error) {
	if r.claimed != nil && r.claimed.ID == id {
		job := *r.claimed
		job.WorkerID = workerID
		return &job, nil
	}
	return nil, nil
}

type stubSwitchRepository struct {
	asset      *switchasset.Switch
	neighborBy map[string]*switchasset.Switch
}

func (r *stubSwitchRepository) Insert(context.Context, switchasset.Switch) (switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) List(context.Context) ([]switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) ListEnabledSNMP(context.Context) ([]switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) FindByID(_ context.Context, id string) (*switchasset.Switch, error) {
	if r.asset != nil && r.asset.ID == id {
		return r.asset, nil
	}
	return nil, nil
}

func (r *stubSwitchRepository) FindByName(context.Context, string) (*switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) FindByManagementIP(context.Context, string) (*switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) FindByBaseMAC(context.Context, string) (*switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) FindByNeighborName(_ context.Context, name string) (*switchasset.Switch, error) {
	return r.neighborBy[strings.TrimSpace(name)], nil
}

func (r *stubSwitchRepository) UpdateIdentity(context.Context, string, string, string, []string) (switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) UpdateRoutingSwitch(context.Context, string, string) (switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) UpdateRadiusSecret(context.Context, string, string) (switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) UpdateSSHConfig(context.Context, string, string, string, int) (switchasset.Switch, error) {
	panic("unexpected call")
}

func (r *stubSwitchRepository) UpdatePollStatus(context.Context, string, time.Time, string) (switchasset.Switch, error) {
	if r.asset != nil {
		return *r.asset, nil
	}
	return switchasset.Switch{}, nil
}

type stubPortRepository struct {
	replaced []switchport.Port
}

func (r *stubPortRepository) ReplaceBySwitch(_ context.Context, _ string, ports []switchport.Port) error {
	r.replaced = append([]switchport.Port{}, ports...)
	return nil
}

func (r *stubPortRepository) ListBySwitch(context.Context, string) ([]switchport.Port, error) {
	return nil, nil
}

func (r *stubPortRepository) FindBySwitchIfIndex(context.Context, string, int) (*switchport.Port, error) {
	return nil, nil
}

type stubARPSnapshotRepository struct {
	items []arpsnapshot.Snapshot
}

func (r *stubARPSnapshotRepository) UpsertBatch(_ context.Context, items []arpsnapshot.Snapshot) error {
	r.items = append([]arpsnapshot.Snapshot{}, items...)
	return nil
}

type stubMACIPBindingRepository struct {
	items []macipbinding.Binding
}

func (r *stubMACIPBindingRepository) UpsertBatch(_ context.Context, items []macipbinding.Binding) error {
	r.items = append([]macipbinding.Binding{}, items...)
	return nil
}

type stubSNMPClient struct{}

type stubTopologyRunner struct {
	links []topologydomain.Link
}

func (r *stubTopologyRunner) DiscoverSwitch(context.Context, string) ([]topologydomain.Link, error) {
	return append([]topologydomain.Link{}, r.links...), nil
}

func (c *stubSNMPClient) LookupMAC(context.Context, snmp.SwitchTarget, string) (snmp.LookupResult, error) {
	return snmp.LookupResult{}, nil
}

func (c *stubSNMPClient) GetSystemName(context.Context, snmp.SwitchTarget) (string, error) {
	return "", nil
}

func (c *stubSNMPClient) GetBaseMAC(context.Context, snmp.SwitchTarget) (string, error) {
	return "", nil
}

func (c *stubSNMPClient) ResolveBridgePort(context.Context, snmp.SwitchTarget, int) (snmp.LookupResult, error) {
	return snmp.LookupResult{}, nil
}

func (c *stubSNMPClient) DiscoverLLDPNeighbors(context.Context, snmp.SwitchTarget) ([]snmp.LLDPNeighbor, error) {
	return []snmp.LLDPNeighbor{
		{LocalPortName: "Gi1/0/11", LocalPortIndex: 11, RemoteSystemName: "core-1", RemotePortName: "Gi0/11", RemoteSystemDesc: "Distribution"},
	}, nil
}

func (c *stubSNMPClient) DiscoverCDPNeighbors(context.Context, snmp.SwitchTarget) ([]snmp.CDPNeighbor, error) {
	return []snmp.CDPNeighbor{
		{LocalIfIndex: 10, LocalPortName: "Gi1/0/10", RemoteSystemName: "core-2", RemotePortName: "Gi0/10", RemotePlatform: "Cisco C9300", RemoteDescription: "Core"},
	}, nil
}

func (c *stubSNMPClient) DiscoverTrunkPorts(context.Context, snmp.SwitchTarget) ([]snmp.TrunkPort, error) {
	return []snmp.TrunkPort{{IfIndex: 10, State: 1}}, nil
}

func (c *stubSNMPClient) DiscoverFDB(context.Context, snmp.SwitchTarget) (snmp.FDBDiscovery, error) {
	return snmp.FDBDiscovery{
		Entries: []snmp.FDBEntry{
			{MACAddress: "00:11:22:33:44:55", IfIndex: 10, BridgePort: 10},
			{MACAddress: "00:11:22:33:44:66", IfIndex: 10, BridgePort: 10},
			{MACAddress: "00:11:22:33:44:77", IfIndex: 11, BridgePort: 11},
		},
		Source:            "dot1d",
		Dot1DRows:         3,
		QBridgeRows:       0,
		BridgePortMapRows: 2,
		MappedEntries:     3,
	}, nil
}

func (c *stubSNMPClient) DiscoverFDBEntries(context.Context, snmp.SwitchTarget) ([]snmp.FDBEntry, error) {
	return []snmp.FDBEntry{
		{MACAddress: "00:11:22:33:44:55", IfIndex: 10, BridgePort: 10},
		{MACAddress: "00:11:22:33:44:66", IfIndex: 10, BridgePort: 10},
		{MACAddress: "00:11:22:33:44:77", IfIndex: 11, BridgePort: 11},
	}, nil
}

func (c *stubSNMPClient) DiscoverARP(context.Context, snmp.SwitchTarget) (snmp.ARPDiscovery, error) {
	return snmp.ARPDiscovery{
		Entries: []snmp.ARPEntry{
			{IfIndex: 2001, IPAddress: "10.10.10.11", MACAddress: "00:11:22:33:44:55", Source: "ipNetToMedia"},
			{IfIndex: 2002, IPAddress: "10.10.10.12", MACAddress: "00:11:22:33:44:66", Source: "ipNetToPhysical"},
		},
		Source:              "merged",
		IPNetToMediaRows:    1,
		IPNetToPhysicalRows: 1,
		MappedEntries:       2,
	}, nil
}

func (c *stubSNMPClient) DiscoverARPEntries(context.Context, snmp.SwitchTarget) ([]snmp.ARPEntry, error) {
	return []snmp.ARPEntry{
		{IfIndex: 2001, IPAddress: "10.10.10.11", MACAddress: "00:11:22:33:44:55", Source: "ipNetToMedia"},
		{IfIndex: 2002, IPAddress: "10.10.10.12", MACAddress: "00:11:22:33:44:66", Source: "ipNetToPhysical"},
	}, nil
}

func (c *stubSNMPClient) WalkInterfaces(context.Context, snmp.SwitchTarget) ([]snmp.InterfaceState, error) {
	return []snmp.InterfaceState{
		{IfIndex: 10, Name: "Gi1/0/10", AdminStatus: "up", OperStatus: "up"},
		{IfIndex: 11, Name: "Gi1/0/11", AdminStatus: "up", OperStatus: "down"},
		{IfIndex: 1001, Name: "Vlan-interface1", AdminStatus: "up", OperStatus: "up"},
	}, nil
}

func (c *stubSNMPClient) SetInt(context.Context, snmp.SwitchTarget, string, int) error {
	return nil
}

func (c *stubSNMPClient) SetUint(context.Context, snmp.SwitchTarget, string, uint32) error {
	return nil
}

func TestCreateNormalizesAndQueuesJob(t *testing.T) {
	repo := &stubJobRepository{}
	switches := &stubSwitchRepository{
		asset: &switchasset.Switch{ID: "sw-1"},
	}
	service := NewService(repo, switches, nil, nil, nil, nil, nil, nil)

	job, err := service.Create(context.Background(), domain.Job{
		SwitchID: "sw-1",
		Scope:    " Ports ",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if job.ID == "" {
		t.Fatalf("expected generated job id")
	}
	if job.Scope != "ports" {
		t.Fatalf("expected normalized scope ports, got %q", job.Scope)
	}
	if job.Status != "queued" {
		t.Fatalf("expected queued status, got %q", job.Status)
	}
	if job.RequestedSource != "api" {
		t.Fatalf("expected requested_source api, got %q", job.RequestedSource)
	}
	if job.MaxAttempts != 3 {
		t.Fatalf("expected max attempts 3, got %d", job.MaxAttempts)
	}
}

func TestCreateRejectsUnknownScope(t *testing.T) {
	service := NewService(&stubJobRepository{}, nil, nil, nil, nil, nil, nil, nil)

	if _, err := service.Create(context.Background(), domain.Job{Scope: "invalid"}); err == nil {
		t.Fatalf("expected scope validation error")
	}
}

func TestStartNextUsesDefaultWorkerID(t *testing.T) {
	portRepo := &stubPortRepository{}
	service := NewService(&stubJobRepository{}, &stubSwitchRepository{
		asset: &switchasset.Switch{
			ID:            "sw-1",
			ManagementIP:  "10.0.0.1",
			SNMPCommunity: "public",
			SNMPPort:      161,
			SNMPTimeoutMS: 1000,
			SNMPRetries:   1,
			Vendor:        "cisco",
		},
		neighborBy: map[string]*switchasset.Switch{
			"core-1": {ID: "sw-core-1", Name: "core-1"},
			"core-2": {ID: "sw-core-2", Name: "core-2"},
		},
	}, portRepo, nil, nil, &stubSNMPClient{}, nil, &stubTopologyRunner{})

	job, err := service.StartNext(context.Background(), "")
	if err != nil {
		t.Fatalf("StartNext returned error: %v", err)
	}
	if job == nil {
		t.Fatalf("expected claimed job")
	}
	if job.Status != "completed" {
		t.Fatalf("expected completed status, got %q", job.Status)
	}
	if job.WorkerID != "worker-1" {
		t.Fatalf("expected default worker id, got %q", job.WorkerID)
	}
	if job.ProgressPercent != 100 {
		t.Fatalf("expected completed progress, got %d", job.ProgressPercent)
	}
	if len(portRepo.replaced) != 2 {
		t.Fatalf("expected only physical interfaces persisted, got %d", len(portRepo.replaced))
	}
	if portRepo.replaced[0].MACCount != 2 {
		t.Fatalf("expected first port mac_count 2, got %d", portRepo.replaced[0].MACCount)
	}
	if portRepo.replaced[1].MACCount != 1 {
		t.Fatalf("expected second port mac_count 1, got %d", portRepo.replaced[1].MACCount)
	}
	if !portRepo.replaced[1].IsUplink {
		t.Fatalf("expected second port to become uplink from neighbor heuristic")
	}
	if portRepo.replaced[1].NeighborProtocol != "lldp" {
		t.Fatalf("expected second port neighbor protocol lldp, got %q", portRepo.replaced[1].NeighborProtocol)
	}
	if portRepo.replaced[1].NeighborSwitchID != "sw-core-1" {
		t.Fatalf("expected second port neighbor switch id sw-core-1, got %q", portRepo.replaced[1].NeighborSwitchID)
	}
	if got := job.Summary["fdb_source"]; got != "dot1d" {
		t.Fatalf("expected fdb_source dot1d, got %#v", got)
	}
	if got := job.Summary["ports_with_macs"]; got != 2 {
		t.Fatalf("expected ports_with_macs 2, got %#v", got)
	}
	if got := job.Summary["neighbor_port_count"]; got != 2 {
		t.Fatalf("expected neighbor_port_count 2, got %#v", got)
	}
	if got := job.Summary["uplink_port_count"]; got != 2 {
		t.Fatalf("expected uplink_port_count 2, got %#v", got)
	}
}

func TestStartNextFullAddsTopologySummary(t *testing.T) {
	portRepo := &stubPortRepository{}
	jobRepo := &stubJobRepository{
		claimed: &domain.Job{
			ID:              "job-full",
			SwitchID:        "sw-1",
			Scope:           "full",
			Status:          "running",
			CurrentStep:     "claimed",
			ProgressPercent: 5,
		},
	}
	service := NewService(jobRepo, &stubSwitchRepository{
		asset: &switchasset.Switch{
			ID:            "sw-1",
			Name:          "edge-1",
			ManagementIP:  "10.0.0.1",
			SNMPCommunity: "public",
			SNMPPort:      161,
			SNMPTimeoutMS: 1000,
			SNMPRetries:   1,
			Vendor:        "cisco",
		},
		neighborBy: map[string]*switchasset.Switch{
			"core-1": {ID: "sw-core-1", Name: "core-1"},
			"core-2": {ID: "sw-core-2", Name: "core-2"},
		},
	}, portRepo, nil, nil, &stubSNMPClient{}, nil, &stubTopologyRunner{
		links: []topologydomain.Link{
			{DiscoveryMethod: "cdp"},
			{DiscoveryMethod: "lldp"},
		},
	})

	job, err := service.StartNext(context.Background(), "worker-a")
	if err != nil {
		t.Fatalf("StartNext returned error: %v", err)
	}
	if job == nil || job.Status != "completed" {
		t.Fatalf("expected completed full job, got %#v", job)
	}
	if got := job.Summary["topology_link_count"]; got != 2 {
		t.Fatalf("expected topology_link_count 2, got %#v", got)
	}
	if got := job.Summary["cdp_link_count"]; got != 1 {
		t.Fatalf("expected cdp_link_count 1, got %#v", got)
	}
	if got := job.Summary["lldp_link_count"]; got != 1 {
		t.Fatalf("expected lldp_link_count 1, got %#v", got)
	}
}

func TestStartNextARPStoresSnapshotsAndBindings(t *testing.T) {
	jobRepo := &stubJobRepository{
		claimed: &domain.Job{
			ID:              "job-arp",
			SwitchID:        "sw-core",
			Scope:           "arp",
			Status:          "running",
			CurrentStep:     "claimed",
			ProgressPercent: 5,
		},
	}
	snapshotRepo := &stubARPSnapshotRepository{}
	bindingRepo := &stubMACIPBindingRepository{}
	service := NewService(jobRepo, &stubSwitchRepository{
		asset: &switchasset.Switch{
			ID:            "sw-core",
			Name:          "core-1",
			ManagementIP:  "10.0.0.254",
			SNMPCommunity: "public",
			SNMPPort:      161,
			SNMPTimeoutMS: 1000,
			SNMPRetries:   1,
			Vendor:        "cisco",
		},
	}, nil, snapshotRepo, bindingRepo, &stubSNMPClient{}, nil, nil)

	job, err := service.StartNext(context.Background(), "worker-arp")
	if err != nil {
		t.Fatalf("StartNext returned error: %v", err)
	}
	if job == nil || job.Status != "completed" {
		t.Fatalf("expected completed arp job, got %#v", job)
	}
	if len(snapshotRepo.items) != 2 {
		t.Fatalf("expected 2 arp snapshots, got %d", len(snapshotRepo.items))
	}
	if len(bindingRepo.items) != 2 {
		t.Fatalf("expected 2 mac ip bindings, got %d", len(bindingRepo.items))
	}
	if got := job.Summary["arp_source"]; got != "merged" {
		t.Fatalf("expected arp_source merged, got %#v", got)
	}
	if got := job.Summary["arp_entry_count"]; got != 2 {
		t.Fatalf("expected arp_entry_count 2, got %#v", got)
	}
}
