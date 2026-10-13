package portobserver

import (
	"context"
	"fmt"
	"nac/internal/domain/switchasset"
	"nac/internal/snmp"
	"sync"
	"testing"
	"time"
)

type fakeSwitches struct {
	assets   []switchasset.Switch
	mu       sync.Mutex
	statuses []string
}

func (f *fakeSwitches) ListEnabledSNMP(context.Context) ([]switchasset.Switch, error) {
	return f.assets, nil
}
func (f *fakeSwitches) UpdatePollStatus(_ context.Context, _ string, _ time.Time, message string) (switchasset.Switch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, message)
	return switchasset.Switch{}, nil
}

type fakeStore struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeStore) Observe(_ context.Context, _ switchasset.Switch, _ snmp.PortSnapshot, _ time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return 1, nil
}

type fakeCollector struct {
	mu                 sync.Mutex
	active, max, calls int
	perSwitch          map[string]int
	observed           map[string]int
	overlap            bool
	release            chan struct{}
}

func (f *fakeCollector) ObservePorts(ctx context.Context, target snmp.SwitchTarget) (snmp.PortSnapshot, error) {
	f.mu.Lock()
	f.active++
	f.calls++
	if f.active > f.max {
		f.max = f.active
	}
	f.perSwitch[target.Address]++
	f.observed[target.Address]++
	if f.perSwitch[target.Address] > 1 {
		f.overlap = true
	}
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.active--; f.perSwitch[target.Address]--; f.mu.Unlock() }()
	select {
	case <-ctx.Done():
		return snmp.PortSnapshot{}, ctx.Err()
	case <-f.release:
		return snmp.PortSnapshot{}, nil
	}
}
func TestPoolIsBoundedDeduplicatesAndStops(t *testing.T) {
	switches := &fakeSwitches{}
	for i := 0; i < 7; i++ {
		switches.assets = append(switches.assets, switchasset.Switch{ID: fmt.Sprint(i), ManagementIP: fmt.Sprint(i), SNMPVersion: "2c"})
	}
	switches.assets = append(switches.assets, switches.assets[0])
	collector := &fakeCollector{perSwitch: map[string]int{}, observed: map[string]int{}, release: make(chan struct{})}
	service, _ := New(switches, &fakeStore{}, collector, Config{Workers: 2, Interval: time.Second, Timeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { service.Run(ctx); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		collector.mu.Lock()
		active := collector.active
		collector.mu.Unlock()
		if active == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("workers did not start")
		}
		time.Sleep(time.Millisecond)
	}
	close(collector.release)
	for {
		collector.mu.Lock()
		observed := len(collector.observed)
		collector.mu.Unlock()
		if observed == 7 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("all independently scheduled switches were not observed: count=%d", observed)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker shutdown hung")
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	if collector.max != 2 || len(collector.observed) != 7 || collector.overlap {
		t.Fatalf("max=%d calls=%d overlap=%v", collector.max, collector.calls, collector.overlap)
	}
}
func TestTimeoutRecordsFailureAndReleasesWorker(t *testing.T) {
	switches := &fakeSwitches{}
	collector := &fakeCollector{perSwitch: map[string]int{}, observed: map[string]int{}, release: make(chan struct{})}
	service, _ := New(switches, &fakeStore{}, collector, Config{Workers: 1, Interval: time.Second, Timeout: time.Second})
	service.cfg.Timeout = 10 * time.Millisecond
	service.poll(context.Background(), switchasset.Switch{ID: "one", SNMPVersion: "2c"})
	if service.Health().Failed != 1 || service.Health().Active != 0 || len(switches.statuses) != 1 || switches.statuses[0] == "" {
		t.Fatalf("missing failure health: %+v", service.Health())
	}
}
func TestPollCollectsBeforePersisting(t *testing.T) {
	release := make(chan struct{})
	close(release)
	collector := &fakeCollector{perSwitch: map[string]int{}, observed: map[string]int{}, release: release}
	store := &fakeStore{}
	switches := &fakeSwitches{}
	service, _ := New(switches, store, collector, Config{Workers: 1, Interval: time.Second, Timeout: time.Second})
	service.poll(context.Background(), switchasset.Switch{ID: "one", SNMPVersion: "2c"})
	store.mu.Lock()
	defer store.mu.Unlock()
	if collector.calls != 1 || store.calls != 1 || service.Health().Succeeded != 1 {
		t.Fatalf("collector=%d store=%d health=%+v", collector.calls, store.calls, service.Health())
	}
}
