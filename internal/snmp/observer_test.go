package snmp

import (
	"context"
	"errors"
	"github.com/gosnmp/gosnmp"
	"testing"
)

type fakeWalk struct {
	bulkErr, walkErr error
	bulk, walk       []gosnmp.SnmpPDU
	fallback         int
}

func (f *fakeWalk) BulkWalkAll(string) ([]gosnmp.SnmpPDU, error) { return f.bulk, f.bulkErr }
func (f *fakeWalk) WalkAll(string) ([]gosnmp.SnmpPDU, error)     { f.fallback++; return f.walk, f.walkErr }
func TestObservationWalkFallbackDiscardsPartialBulk(t *testing.T) {
	w := &fakeWalk{bulkErr: errors.New("too big"), bulk: []gosnmp.SnmpPDU{{Name: ".1.2.1", Value: 1}}, walk: []gosnmp.SnmpPDU{{Name: ".1.2.2", Type: gosnmp.Integer, Value: 2}}}
	rows, err := observationTable(context.Background(), w, ".1.2")
	if err != nil || w.fallback != 1 || len(rows) != 1 || observationInt(rows["2"]) != 2 {
		t.Fatalf("bad fallback: %v %v", rows, err)
	}
}
func TestObservationWalkDoesNotReturnFailedPartialData(t *testing.T) {
	w := &fakeWalk{bulkErr: errors.New("timeout"), walkErr: errors.New("timeout"), walk: []gosnmp.SnmpPDU{{Name: ".1.2.1", Value: 1}}}
	rows, err := observationTable(context.Background(), w, ".1.2")
	if err == nil || rows != nil {
		t.Fatal("failed walk became a snapshot")
	}
}
func TestObservationWalkCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := &fakeWalk{}
	if _, err := observationTable(ctx, w, ".1.2"); !errors.Is(err, context.Canceled) || w.fallback != 0 {
		t.Fatal("cancel ignored")
	}
}

func TestPhysicalEthernetIfType(t *testing.T) {
	for _, value := range []int{6, 26, 62, 69, 117} {
		if !physicalEthernetIfType(value) {
			t.Fatalf("ifType %d must be treated as physical ethernet", value)
		}
	}
	for _, value := range []int{1, 24, 53, 131, 135, 136, 161} {
		if physicalEthernetIfType(value) {
			t.Fatalf("ifType %d must not be treated as a physical ethernet port", value)
		}
	}
}

func TestMergeFDBTablesCombinesSourcesAndSkipsUnmappedRows(t *testing.T) {
	dot1d := map[string]gosnmp.SnmpPDU{
		"0.17.34.51.68.85": {Type: gosnmp.Integer, Value: 10},
	}
	qbridge := map[string]gosnmp.SnmpPDU{
		"106.170.187.204.221.238.255": {Type: gosnmp.Integer, Value: 11},
		"106.1.2.3.4.5.6":             {Type: gosnmp.Integer, Value: 99},
	}
	entries, unmapped := mergeFDBTables(map[string]int{"10": 1010, "11": 1011}, dot1d, qbridge)
	if len(entries) != 2 || unmapped != 1 {
		t.Fatalf("merged=%v unmapped=%d", entries, unmapped)
	}
}
