package portevent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	devicedomain "nac/internal/domain/device"
	domain "nac/internal/domain/portevent"
)

type inboxRepository struct {
	mu      sync.Mutex
	status  map[string]string
	inserts int
}

func (r *inboxRepository) BeginIngest(_ context.Context, id string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == nil {
		r.status = map[string]string{}
	}
	status := r.status[id]
	if status == "" || status == "failed" {
		r.status[id] = "processing"
		return "claimed", nil
	}
	return status, nil
}
func (r *inboxRepository) CompleteIngest(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status[id] = "completed"
	return nil
}
func (r *inboxRepository) FailIngest(_ context.Context, id, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status[id] = "failed"
	return nil
}
func (r *inboxRepository) Insert(_ context.Context, event domain.Event) (domain.Event, error) {
	r.mu.Lock()
	r.inserts++
	r.mu.Unlock()
	return event, nil
}
func (r *inboxRepository) ListRecent(context.Context, int) ([]domain.Event, error) { return nil, nil }

type blockingDeviceObserver struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
	fail    bool
}

func (d *blockingDeviceObserver) ObserveAgentlessEvent(ctx context.Context, _ devicedomain.AgentlessObservationInput) (devicedomain.Device, error) {
	d.mu.Lock()
	d.calls++
	fail := d.fail
	d.fail = false
	d.mu.Unlock()
	if d.entered != nil {
		select {
		case d.entered <- struct{}{}:
		default:
		}
	}
	if d.release != nil {
		select {
		case <-ctx.Done():
			return devicedomain.Device{}, ctx.Err()
		case <-d.release:
		}
	}
	if fail {
		return devicedomain.Device{}, context.DeadlineExceeded
	}
	return devicedomain.Device{}, nil
}
func (d *blockingDeviceObserver) DetachAgentlessEvent(context.Context, string, string, int, time.Time) (bool, error) {
	return false, nil
}

func observerEvent() domain.Event {
	return domain.Event{ID: "9aa0ad97-1496-4df5-9f2d-0bb7dd943e6d", SwitchID: "91b47420-527e-409d-bc0e-d00983e9ccf6", IfIndex: 32, MACAddress: "AA:BB:CC:DD:EE:FF", EventType: "mac_learned", EventSource: "port-observer", ObservedAt: time.Now().UTC()}
}

func TestConcurrentDuplicateIsClaimedBeforeSideEffects(t *testing.T) {
	repo := &inboxRepository{}
	device := &blockingDeviceObserver{entered: make(chan struct{}, 1), release: make(chan struct{})}
	service := NewService(repo, nil, device, nil)
	done := make(chan error, 1)
	go func() { _, err := service.Ingest(context.Background(), observerEvent()); done <- err }()
	<-device.entered
	if _, err := service.Ingest(context.Background(), observerEvent()); !errors.Is(err, ErrEventProcessing) {
		t.Fatalf("duplicate should observe processing claim, got %v", err)
	}
	close(device.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	device.mu.Lock()
	defer device.mu.Unlock()
	if device.calls != 1 || repo.inserts != 1 {
		t.Fatalf("side effects repeated: device=%d inserts=%d", device.calls, repo.inserts)
	}
}

func TestFailedTimeoutCanBeRetriedOnce(t *testing.T) {
	repo := &inboxRepository{}
	device := &blockingDeviceObserver{fail: true}
	service := NewService(repo, nil, device, nil)
	if _, err := service.Ingest(context.Background(), observerEvent()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout, got %v", err)
	}
	if _, err := service.Ingest(context.Background(), observerEvent()); err != nil {
		t.Fatalf("failed claim did not retry: %v", err)
	}
	device.mu.Lock()
	defer device.mu.Unlock()
	if device.calls != 2 || repo.inserts != 1 {
		t.Fatalf("unexpected retry side effects: device=%d inserts=%d", device.calls, repo.inserts)
	}
}
