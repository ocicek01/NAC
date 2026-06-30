package topology

import (
	"context"
	"testing"
	"time"

	switchassetdomain "nac/internal/domain/switchasset"
	topologydomain "nac/internal/domain/topology"
	"nac/internal/snmp"
)

type stubTopologyRepository struct {
	links          []topologydomain.Link
	prunedSwitchID string
	prunedMethods  []string
	prunedObserved time.Time
}

func (r *stubTopologyRepository) Upsert(_ context.Context, link topologydomain.Link) (topologydomain.Link, error) {
	r.links = append(r.links, link)
	return link, nil
}

func (r *stubTopologyRepository) PruneDiscovered(_ context.Context, sourceSwitchID string, methods []string, observedAt time.Time) error {
	r.prunedSwitchID = sourceSwitchID
	r.prunedMethods = append([]string{}, methods...)
	r.prunedObserved = observedAt
	return nil
}

func (r *stubTopologyRepository) List(_ context.Context) ([]topologydomain.Link, error) {
	return r.links, nil
}

func (r *stubTopologyRepository) HasLinkedInterface(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (r *stubTopologyRepository) FindLinkedSwitchID(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (r *stubTopologyRepository) CountLinkedSwitches(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

type stubSwitchRepository struct {
	assets       []switchassetdomain.Switch
	neighborByID map[string]*switchassetdomain.Switch
}

func (r *stubSwitchRepository) Insert(_ context.Context, asset switchassetdomain.Switch) (switchassetdomain.Switch, error) {
	return asset, nil
}

func (r *stubSwitchRepository) List(_ context.Context) ([]switchassetdomain.Switch, error) {
	return r.assets, nil
}

func (r *stubSwitchRepository) ListEnabledSNMP(_ context.Context) ([]switchassetdomain.Switch, error) {
	return r.assets, nil
}

func (r *stubSwitchRepository) FindByID(_ context.Context, _ string) (*switchassetdomain.Switch, error) {
	for _, asset := range r.assets {
		copyAsset := asset
		return &copyAsset, nil
	}
	return nil, nil
}

func (r *stubSwitchRepository) FindByName(_ context.Context, _ string) (*switchassetdomain.Switch, error) {
	return nil, nil
}

func (r *stubSwitchRepository) FindByManagementIP(_ context.Context, _ string) (*switchassetdomain.Switch, error) {
	return nil, nil
}

func (r *stubSwitchRepository) FindByBaseMAC(_ context.Context, _ string) (*switchassetdomain.Switch, error) {
	return nil, nil
}

func (r *stubSwitchRepository) FindByNeighborName(_ context.Context, name string) (*switchassetdomain.Switch, error) {
	return r.neighborByID[name], nil
}

func (r *stubSwitchRepository) UpdateIdentity(_ context.Context, _ string, _ string, _ string, _ []string) (switchassetdomain.Switch, error) {
	return switchassetdomain.Switch{}, nil
}

func (r *stubSwitchRepository) UpdateRoutingSwitch(_ context.Context, _ string, _ string) (switchassetdomain.Switch, error) {
	return switchassetdomain.Switch{}, nil
}

func (r *stubSwitchRepository) UpdateRadiusSecret(_ context.Context, _ string, _ string) (switchassetdomain.Switch, error) {
	return switchassetdomain.Switch{}, nil
}

func (r *stubSwitchRepository) UpdatePollStatus(_ context.Context, _ string, _ time.Time, _ string) (switchassetdomain.Switch, error) {
	return switchassetdomain.Switch{}, nil
}

func (r *stubSwitchRepository) UpdateSSHConfig(_ context.Context, _ string, _ string, _ string, _ int) (switchassetdomain.Switch, error) {
	return switchassetdomain.Switch{}, nil
}

type stubSNMPClient struct {
	lldpNeighbors []snmp.LLDPNeighbor
	cdpNeighbors  []snmp.CDPNeighbor
	trunkPorts    []snmp.TrunkPort
	lldpErr       error
	cdpErr        error
	trunkErr      error
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
	return c.lldpNeighbors, c.lldpErr
}

func (c *stubSNMPClient) DiscoverCDPNeighbors(context.Context, snmp.SwitchTarget) ([]snmp.CDPNeighbor, error) {
	return c.cdpNeighbors, c.cdpErr
}

func (c *stubSNMPClient) DiscoverTrunkPorts(context.Context, snmp.SwitchTarget) ([]snmp.TrunkPort, error) {
	return c.trunkPorts, c.trunkErr
}

func (c *stubSNMPClient) DiscoverFDB(context.Context, snmp.SwitchTarget) (snmp.FDBDiscovery, error) {
	return snmp.FDBDiscovery{}, nil
}

func (c *stubSNMPClient) DiscoverFDBEntries(context.Context, snmp.SwitchTarget) ([]snmp.FDBEntry, error) {
	return nil, nil
}

func (c *stubSNMPClient) DiscoverARP(context.Context, snmp.SwitchTarget) (snmp.ARPDiscovery, error) {
	return snmp.ARPDiscovery{}, nil
}

func (c *stubSNMPClient) DiscoverARPEntries(context.Context, snmp.SwitchTarget) ([]snmp.ARPEntry, error) {
	return nil, nil
}

func (c *stubSNMPClient) WalkInterfaces(context.Context, snmp.SwitchTarget) ([]snmp.InterfaceState, error) {
	return nil, nil
}

func (c *stubSNMPClient) SetInt(context.Context, snmp.SwitchTarget, string, int) error {
	return nil
}

func (c *stubSNMPClient) SetUint(context.Context, snmp.SwitchTarget, string, uint32) error {
	return nil
}

func TestDiscoverLLDPPrefersCDPAndFiltersToTrunkPorts(t *testing.T) {
	t.Parallel()

	asset := switchassetdomain.Switch{
		ID:            "sw-1",
		Name:          "edge-1",
		ManagementIP:  "10.0.0.10",
		SNMPCommunity: "public",
		SNMPPort:      161,
		SNMPTimeoutMS: 1000,
		SNMPRetries:   1,
		Vendor:        "Cisco",
		Model:         "C9300",
	}
	remote := &switchassetdomain.Switch{ID: "sw-2", Name: "core-1"}

	repo := &stubTopologyRepository{}
	switchRepo := &stubSwitchRepository{
		assets: []switchassetdomain.Switch{asset},
		neighborByID: map[string]*switchassetdomain.Switch{
			"core-1": remote,
		},
	}
	client := &stubSNMPClient{
		trunkPorts: []snmp.TrunkPort{
			{IfIndex: 10101, State: 1, Vendor: "Cisco"},
		},
		cdpNeighbors: []snmp.CDPNeighbor{
			{LocalIfIndex: 10101, LocalPortName: "Gi1/0/1", RemoteSystemName: "core-1", RemotePortName: "Gi0/1"},
		},
		lldpNeighbors: []snmp.LLDPNeighbor{
			{LocalPortIndex: 1, LocalPortName: "Gi1/0/1", RemoteSystemName: "core-1", RemotePortName: "Gi0/1"},
			{LocalPortIndex: 2, LocalPortName: "Gi1/0/2", RemoteSystemName: "IP212P", RemotePortName: "WAN Port", RemoteSystemDesc: "SIP Phone"},
		},
	}

	service := NewService(repo, switchRepo, client)
	links, err := service.DiscoverLLDP(context.Background())
	if err != nil {
		t.Fatalf("DiscoverLLDP returned error: %v", err)
	}

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}

	link := links[0]
	if link.DiscoveryMethod != "cdp" {
		t.Fatalf("expected CDP to win on duplicate source port, got %q", link.DiscoveryMethod)
	}
	if link.SourcePortName != "Gi1/0/1" {
		t.Fatalf("unexpected source port: %q", link.SourcePortName)
	}
	if link.TargetSwitchID != "sw-2" {
		t.Fatalf("expected resolved target switch id sw-2, got %q", link.TargetSwitchID)
	}
	if repo.prunedSwitchID != "sw-1" {
		t.Fatalf("expected prune to run for sw-1, got %q", repo.prunedSwitchID)
	}
	if repo.prunedObserved.IsZero() {
		t.Fatal("expected prune observed time to be set")
	}
}

func TestCollectDiscoveredLinksFallsBackToLLDPWhenNoTrunkDataExists(t *testing.T) {
	t.Parallel()

	remote := &switchassetdomain.Switch{ID: "sw-2", Name: "dist-1"}
	service := NewService(&stubTopologyRepository{}, &stubSwitchRepository{
		neighborByID: map[string]*switchassetdomain.Switch{
			"dist-1": remote,
		},
	}, &stubSNMPClient{
		lldpNeighbors: []snmp.LLDPNeighbor{
			{LocalPortIndex: 24, LocalPortName: "1/0/24", RemoteSystemName: "dist-1", RemotePortName: "1/0/1"},
		},
	})

	asset := switchassetdomain.Switch{
		ID:   "sw-1",
		Name: "access-1",
	}
	target := snmp.SwitchTarget{}
	links := service.collectDiscoveredLinks(context.Background(), asset, target, nil, time.Now().UTC())
	if len(links) != 1 {
		t.Fatalf("expected one lldp link without trunk filter, got %d", len(links))
	}
	if links[0].DiscoveryMethod != "lldp" {
		t.Fatalf("expected lldp method, got %q", links[0].DiscoveryMethod)
	}
	if links[0].TargetSwitchID != "sw-2" {
		t.Fatalf("expected resolved target switch id sw-2, got %q", links[0].TargetSwitchID)
	}
}

func TestCollectDiscoveredLinksSkipsEndpointLikeNeighbors(t *testing.T) {
	t.Parallel()

	service := NewService(&stubTopologyRepository{}, &stubSwitchRepository{}, &stubSNMPClient{
		lldpNeighbors: []snmp.LLDPNeighbor{
			{LocalPortIndex: 5, LocalPortName: "5", RemoteSystemName: "IP212P", RemotePortName: "WAN Port", RemoteSystemDesc: "SIP Phone"},
		},
	})

	asset := switchassetdomain.Switch{
		ID:   "sw-1",
		Name: "edge-1",
	}

	links := service.collectDiscoveredLinks(context.Background(), asset, snmp.SwitchTarget{}, nil, time.Now().UTC())
	if len(links) != 0 {
		t.Fatalf("expected endpoint-like neighbors to be skipped, got %d links", len(links))
	}
}

func TestCollectDiscoveredLinksSkipsCCTVNeighbors(t *testing.T) {
	t.Parallel()

	service := NewService(&stubTopologyRepository{}, &stubSwitchRepository{}, &stubSNMPClient{
		cdpNeighbors: []snmp.CDPNeighbor{
			{
				LocalIfIndex:      10020,
				LocalPortName:     "Gi1/0/20",
				RemoteSystemName:  "DIREK1CCTV.hakkari.local",
				RemotePortName:    "GigabitEthernet0/1",
				RemotePlatform:    "IP Camera",
				RemoteDescription: "CCTV Camera",
			},
		},
	})

	asset := switchassetdomain.Switch{
		ID:   "sw-1",
		Name: "edge-1",
	}

	links := service.collectDiscoveredLinks(context.Background(), asset, snmp.SwitchTarget{}, nil, time.Now().UTC())
	if len(links) != 0 {
		t.Fatalf("expected cctv neighbors to be skipped, got %d links", len(links))
	}
}

func TestCollectDiscoveredLinksSkipsUnresolvedNeighbors(t *testing.T) {
	t.Parallel()

	service := NewService(&stubTopologyRepository{}, &stubSwitchRepository{}, &stubSNMPClient{
		cdpNeighbors: []snmp.CDPNeighbor{
			{
				LocalIfIndex:      10046,
				LocalPortName:     "Gi1/0/46",
				RemoteSystemName:  "Switch.hakkari.local",
				RemotePortName:    "GigabitEthernet0/2",
				RemotePlatform:    "Cisco IOS Software",
				RemoteDescription: "Switch",
			},
		},
	})

	asset := switchassetdomain.Switch{
		ID:   "sw-1",
		Name: "edge-1",
	}

	links := service.collectDiscoveredLinks(context.Background(), asset, snmp.SwitchTarget{}, nil, time.Now().UTC())
	if len(links) != 0 {
		t.Fatalf("expected unresolved neighbors to be skipped, got %d links", len(links))
	}
}
