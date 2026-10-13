package switchport

import (
	"nac/internal/snmp"
	"reflect"
	"testing"
	"time"
)

func TestMergeObservationPreservesUnreadableTables(t *testing.T) {
	previous := observationState{Admin: "up", Oper: "up", VLAN: 106, MACs: []string{"AA:BB:CC:DD:EE:FF"}}
	next := mergeObservation(previous, snmp.PortObservation{InterfaceState: snmp.InterfaceState{AdminStatus: "down", OperStatus: "down"}})
	if next.VLAN != 106 || !reflect.DeepEqual(next.MACs, previous.MACs) || next.Admin != "down" {
		t.Fatalf("unexpected partial snapshot: %+v", next)
	}
}

func TestSnapshotConflictSkipsAllStalePortFields(t *testing.T) {
	started := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	if !snapshotConflicts(started.Add(time.Millisecond), started) {
		t.Fatal("port updated after collection start must be skipped")
	}
	if snapshotConflicts(started, started) {
		t.Fatal("same-version snapshot should be accepted")
	}
}
func TestMergeObservationClearsMACsAndTracksVLAN(t *testing.T) {
	vlan := 300
	next := mergeObservation(observationState{VLAN: 106, MACs: []string{"AA:BB:CC:DD:EE:FF"}}, snmp.PortObservation{MACsKnown: true, VLANID: &vlan})
	if next.VLAN != 300 || next.MACs == nil || len(next.MACs) != 0 {
		t.Fatalf("successful empty FDB must clear MACs: %+v", next)
	}
}
func TestMergeObservationCanonicalMACSet(t *testing.T) {
	next := mergeObservation(observationState{}, snmp.PortObservation{MACsKnown: true, MACAddresses: []string{"BB:00:00:00:00:02", "aa:00:00:00:00:01", "AA:00:00:00:00:01"}})
	if !reflect.DeepEqual(next.MACs, []string{"AA:00:00:00:00:01", "BB:00:00:00:00:02"}) {
		t.Fatalf("noncanonical snapshot: %+v", next)
	}
}

func TestMACDeltaDoesNotRepeatStableEntries(t *testing.T) {
	learned, removed := macDelta(
		[]string{"AA:00:00:00:00:01", "BB:00:00:00:00:02"},
		[]string{"BB:00:00:00:00:02", "CC:00:00:00:00:03"},
	)
	if !reflect.DeepEqual(learned, []string{"CC:00:00:00:00:03"}) || !reflect.DeepEqual(removed, []string{"AA:00:00:00:00:01"}) {
		t.Fatalf("unexpected delta learned=%v removed=%v", learned, removed)
	}
}

func TestShouldPublishMACDeltasOnlyForEstablishedAccessPorts(t *testing.T) {
	tests := []struct {
		name           string
		missing        bool
		classification portClassification
		want           bool
	}{
		{name: "access", classification: portClassification{Mode: "access"}, want: true},
		{name: "initial baseline", missing: true, classification: portClassification{Mode: "access"}},
		{name: "uplink flag", classification: portClassification{Uplink: true, Mode: "access"}},
		{name: "trunk flag", classification: portClassification{Trunk: true}},
		{name: "trunk mode", classification: portClassification{Mode: "TrUnK"}},
		{name: "unknown mode", classification: portClassification{Mode: "unknown"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldPublishMACDeltas(test.missing, test.classification); got != test.want {
				t.Fatalf("shouldPublishMACDeltas()=%v want=%v", got, test.want)
			}
		})
	}
}
