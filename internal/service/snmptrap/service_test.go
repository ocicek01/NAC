package snmptrap

import (
	"context"
	"log/slog"
	"testing"

	domain "nac/internal/domain/snmptrap"
	switchasset "nac/internal/domain/switchasset"
	switchport "nac/internal/domain/switchport"
	trapwindowdomain "nac/internal/domain/trapwindow"
)

type stubTrapRepository struct {
	inserted domain.Event
}

func (r *stubTrapRepository) Insert(_ context.Context, event domain.Event) (domain.Event, error) {
	r.inserted = event
	return event, nil
}

type stubTrapSwitchResolver struct {
	byIP map[string]*switchasset.Switch
	byID map[string]*switchasset.Switch
}

func (r *stubTrapSwitchResolver) FindByManagementIP(_ context.Context, managementIP string) (*switchasset.Switch, error) {
	return r.byIP[managementIP], nil
}

func (r *stubTrapSwitchResolver) FindByID(_ context.Context, id string) (*switchasset.Switch, error) {
	return r.byID[id], nil
}

type stubTrapPortResolver struct {
	items []switchport.Port
}

func (r *stubTrapPortResolver) ListBySwitch(context.Context, string) ([]switchport.Port, error) {
	return append([]switchport.Port{}, r.items...), nil
}

type stubTrapWindowRecorder struct {
	inserted []trapwindowdomain.Window
}

func (r *stubTrapWindowRecorder) Record(_ context.Context, window trapwindowdomain.Window) (trapwindowdomain.Window, error) {
	r.inserted = append(r.inserted, window)
	return window, nil
}

type stubPortStatusForwarder struct {
	forwarded []domain.Event
}

func (f *stubPortStatusForwarder) ForwardPortStatus(_ context.Context, event domain.Event) error {
	f.forwarded = append(f.forwarded, event)
	return nil
}

func TestIngestClassifiesLinkTrapAndQueuesPortsJob(t *testing.T) {
	repo := &stubTrapRepository{}
	windows := &stubTrapWindowRecorder{}
	service := NewService(
		slog.Default(),
		repo,
		&stubTrapSwitchResolver{byIP: map[string]*switchasset.Switch{
			"10.6.8.19": {ID: "sw-1", Name: "HP-2530-48G"},
		}},
		&stubTrapPortResolver{},
		windows,
		nil,
	)

	event, err := service.Ingest(context.Background(), domain.Event{
		SourceIP: "10.6.8.19",
		TrapOID:  ".1.3.6.1.6.3.1.1.5.3",
		VarBinds: []domain.VarBind{
			{OID: ".1.3.6.1.2.1.2.2.1.1.10", Value: "10"},
		},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	if event.Category != "link-down" {
		t.Fatalf("expected link-down category, got %q", event.Category)
	}
	if !event.Actionable {
		t.Fatalf("expected event to be actionable")
	}
	if event.IfIndex != 10 {
		t.Fatalf("expected ifIndex 10, got %d", event.IfIndex)
	}
	if len(windows.inserted) != 1 {
		t.Fatalf("expected 1 trap window, got %d", len(windows.inserted))
	}
	if windows.inserted[0].Scope != "ports" {
		t.Fatalf("expected ports scope, got %q", windows.inserted[0].Scope)
	}
}

func TestIngestClassifiesHPPortAccessTrapAndQueuesPortsAndARP(t *testing.T) {
	repo := &stubTrapRepository{}
	windows := &stubTrapWindowRecorder{}
	service := NewService(
		slog.Default(),
		repo,
		&stubTrapSwitchResolver{byIP: map[string]*switchasset.Switch{
			"10.6.8.19": {ID: "sw-1", Name: "HP-2530-48G"},
		}},
		&stubTrapPortResolver{items: []switchport.Port{
			{IfIndex: 10, IsUplink: false},
			{IfIndex: 25, IsUplink: true, NeighborSwitchID: "sw-core-1"},
		}},
		windows,
		nil,
	)

	event, err := service.Ingest(context.Background(), domain.Event{
		SourceIP:      "10.6.8.19",
		EnterpriseOID: ".1.3.6.1.4.1.11.2.14.11.5.1.19",
		VarBinds: []domain.VarBind{
			{OID: ".1.3.6.1.4.1.11.2.14.11.5.1.19.1.12.0", Value: "10"},
			{OID: ".1.3.6.1.4.1.11.2.14.11.5.1.19.1.11.0", Value: "30_9c_23_9b_97_aa"},
			{OID: ".1.3.6.1.4.1.11.2.14.11.5.1.19.1.13.0", Value: "999"},
		},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	if event.Category != "hp-port-access" {
		t.Fatalf("expected hp-port-access category, got %q", event.Category)
	}
	if event.MACAddress != "30:9C:23:9B:97:AA" {
		t.Fatalf("expected normalized mac, got %q", event.MACAddress)
	}
	if event.VLANID != 999 {
		t.Fatalf("expected vlan 999, got %d", event.VLANID)
	}
	if len(windows.inserted) != 2 {
		t.Fatalf("expected 2 trap windows, got %d", len(windows.inserted))
	}
	if windows.inserted[0].Scope != "ports" {
		t.Fatalf("expected first window ports, got %q", windows.inserted[0].Scope)
	}
	if windows.inserted[1].Scope != "arp" || windows.inserted[1].SwitchID != "sw-core-1" {
		t.Fatalf("expected arp window for core switch, got scope=%q switch=%q", windows.inserted[1].Scope, windows.inserted[1].SwitchID)
	}
}

func TestIngestUsesWindowKeyForDeduping(t *testing.T) {
	repo := &stubTrapRepository{}
	windows := &stubTrapWindowRecorder{}
	service := NewService(
		slog.Default(),
		repo,
		&stubTrapSwitchResolver{byIP: map[string]*switchasset.Switch{
			"10.6.8.19": {ID: "sw-1", Name: "HP-2530-48G"},
		}},
		&stubTrapPortResolver{},
		windows,
		nil,
	)

	event, err := service.Ingest(context.Background(), domain.Event{
		SourceIP: "10.6.8.19",
		TrapOID:  ".1.3.6.1.6.3.1.1.5.4",
		VarBinds: []domain.VarBind{
			{OID: ".1.3.6.1.2.1.2.2.1.1.10", Value: "10"},
		},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	if len(windows.inserted) != 1 {
		t.Fatalf("expected a trap window, got %d", len(windows.inserted))
	}
	if got := buildWindowKey("sw-1", "ports", event); windows.inserted[0].DedupeKey != got {
		t.Fatalf("expected dedupe key %q, got %q", got, windows.inserted[0].DedupeKey)
	}
}

func TestIngestForwardsOnlyLinkTrapPortStatuses(t *testing.T) {
	repo := &stubTrapRepository{}
	windows := &stubTrapWindowRecorder{}
	forwarder := &stubPortStatusForwarder{}
	service := NewService(
		slog.Default(),
		repo,
		&stubTrapSwitchResolver{byIP: map[string]*switchasset.Switch{
			"10.6.8.19": {ID: "sw-1", Name: "HP-2530-48G"},
		}},
		&stubTrapPortResolver{},
		windows,
		forwarder,
	)

	_, err := service.Ingest(context.Background(), domain.Event{
		SourceIP: "10.6.8.19",
		TrapOID:  ".1.3.6.1.6.3.1.1.5.4",
		VarBinds: []domain.VarBind{{OID: ".1.3.6.1.2.1.2.2.1.1.45", Value: "45"}},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	_, err = service.Ingest(context.Background(), domain.Event{
		SourceIP:      "10.6.8.19",
		EnterpriseOID: ".1.3.6.1.4.1.11.2.14.11.5.1.19",
		VarBinds:      []domain.VarBind{{OID: ".1.3.6.1.4.1.11.2.14.11.5.1.19.1.12.0", Value: "10"}},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	if len(forwarder.forwarded) != 1 {
		t.Fatalf("expected only link trap to be forwarded, got %d", len(forwarder.forwarded))
	}
	if forwarder.forwarded[0].Category != "link-up" {
		t.Fatalf("expected forwarded link-up category, got %q", forwarder.forwarded[0].Category)
	}
	if forwarder.forwarded[0].IfIndex != 45 {
		t.Fatalf("expected forwarded ifIndex 45, got %d", forwarder.forwarded[0].IfIndex)
	}
}
