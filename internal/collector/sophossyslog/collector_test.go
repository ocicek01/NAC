package sophossyslog

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nac/internal/config"
	domain "nac/internal/domain/sophosauth"
)

type fakePacket struct {
	payload []byte
	remote  *net.UDPAddr
}

type fakeUDPReader struct {
	packets chan fakePacket
	wake    chan struct{}
	once    sync.Once
	reads   atomic.Int32
}

func newFakeUDPReader() *fakeUDPReader {
	return &fakeUDPReader{packets: make(chan fakePacket, 8), wake: make(chan struct{})}
}

func (f *fakeUDPReader) SetReadDeadline(deadline time.Time) error {
	if !deadline.After(time.Now()) {
		f.once.Do(func() { close(f.wake) })
	}
	return nil
}

func (f *fakeUDPReader) ReadFromUDP(buffer []byte) (int, *net.UDPAddr, error) {
	select {
	case packet := <-f.packets:
		f.reads.Add(1)
		return copy(buffer, packet.payload), packet.remote, nil
	case <-f.wake:
		return 0, nil, deadlineError{}
	}
}

type deadlineError struct{}

func (deadlineError) Error() string   { return "read deadline exceeded" }
func (deadlineError) Timeout() bool   { return true }
func (deadlineError) Temporary() bool { return true }

type failingUDPReader struct{}

func (failingUDPReader) SetReadDeadline(time.Time) error { return nil }
func (failingUDPReader) ReadFromUDP([]byte) (int, *net.UDPAddr, error) {
	return 0, nil, errors.New("injected read failure")
}

type blockingProcessor struct {
	started   chan struct{}
	startOnce sync.Once
	active    atomic.Int32
	drops     atomic.Int32
	dropSeen  chan struct{}
}

func (p *blockingProcessor) Process(ctx context.Context, _ domain.ParsedEvent) error {
	p.active.Add(1)
	defer p.active.Add(-1)
	p.startOnce.Do(func() { close(p.started) })
	<-ctx.Done()
	return ctx.Err()
}

func (p *blockingProcessor) ProcessDue(context.Context, int) error             { return nil }
func (p *blockingProcessor) RecordRejectedSource(context.Context, string)      {}
func (p *blockingProcessor) RecordParseFailure(context.Context, string, error) {}
func (p *blockingProcessor) RecordQueueDrop(context.Context, string, int) {
	if p.drops.Add(1) == 2 {
		close(p.dropSeen)
	}
}

func TestServeContinuesReadingUntilBoundedQueueFillsAndShutsDown(t *testing.T) {
	processor := &blockingProcessor{started: make(chan struct{}), dropSeen: make(chan struct{})}
	collector, err := New(config.SophosAuthConfig{
		AllowedSources:       []string{"10.6.0.1"},
		WorkerCount:          1,
		QueueSize:            2,
		ProcessingTimeoutSec: 30,
		RetryIntervalSec:     3600,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), processor)
	if err != nil {
		t.Fatal(err)
	}

	reader := newFakeUDPReader()
	payload := []byte(`component=Authentication log_id=17701 user_name=test client_used="Captive Portal" status=Success src_ip=10.6.9.93`)
	packet := fakePacket{payload: payload, remote: &net.UDPAddr{IP: net.ParseIP("10.6.0.1"), Port: 5514}}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- collector.serve(ctx, reader) }()

	reader.packets <- packet
	waitFor(t, processor.started, "first processor call")
	for range 4 {
		reader.packets <- packet
	}
	waitUntil(t, func() bool { return reader.reads.Load() >= 5 }, "reader to consume all datagrams")
	waitFor(t, processor.dropSeen, "queue drop audit")

	if got := processor.drops.Load(); got != 2 {
		t.Fatalf("expected 2 queue drops, got %d", got)
	}
	if got := processor.active.Load(); got != 1 {
		t.Fatalf("expected exactly one bounded worker, got %d active", got)
	}

	cancel()
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve returned an error during shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("collector workers did not shut down")
	}
	if got := processor.active.Load(); got != 0 {
		t.Fatalf("processor leaked after shutdown: %d active", got)
	}
}

func TestServeStopsWorkersWhenReaderFails(t *testing.T) {
	processor := &blockingProcessor{started: make(chan struct{}), dropSeen: make(chan struct{})}
	collector, err := New(config.SophosAuthConfig{
		AllowedSources:       []string{"10.6.0.1"},
		WorkerCount:          2,
		QueueSize:            2,
		ProcessingTimeoutSec: 1,
		RetryIntervalSec:     3600,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), processor)
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	err = collector.serve(context.Background(), failingUDPReader{})
	if err == nil || err.Error() != "injected read failure" {
		t.Fatalf("expected injected read failure, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("worker shutdown took too long after reader failure: %s", elapsed)
	}
	if got := processor.active.Load(); got != 0 {
		t.Fatalf("processor leaked after reader failure: %d active", got)
	}
}

func waitFor(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitUntil(t *testing.T, condition func() bool, description string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", description)
		}
		time.Sleep(time.Millisecond)
	}
}
