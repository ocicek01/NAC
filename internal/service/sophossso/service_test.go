package sophossso

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"nac/internal/config"
	domain "nac/internal/domain/sophossso"
)

type memoryRepository struct {
	items     map[string]domain.Item
	retryDead bool
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{items: map[string]domain.Item{}}
}
func (r *memoryRepository) Enqueue(_ context.Context, item domain.Item) (domain.Item, bool, error) {
	if existing, ok := r.items[item.SourceKey]; ok {
		return existing, false, nil
	}
	r.items[item.SourceKey] = item
	return item, true, nil
}
func (r *memoryRepository) ClaimDue(_ context.Context, now, leaseUntil time.Time, _ int) ([]domain.Item, error) {
	for key, item := range r.items {
		if item.Status == "sent" || item.Status == "dead_letter" {
			continue
		}
		if item.Status == "processing" && item.LeaseUntil != nil && !item.LeaseUntil.Before(now) {
			continue
		}
		if (item.Status == "pending" || item.Status == "retry") && item.NextAttemptAt.After(now) {
			continue
		}
		item.AttemptCount++
		item.Status = "processing"
		item.LeaseUntil = &leaseUntil
		r.items[key] = item
		return []domain.Item{item}, nil
	}
	return []domain.Item{}, nil
}
func (r *memoryRepository) MarkSent(_ context.Context, id string, leaseUntil, sentAt time.Time) error {
	for key, item := range r.items {
		if item.ID == id {
			if item.Status != "processing" || item.LeaseUntil == nil || !item.LeaseUntil.Equal(leaseUntil) {
				return errors.New("lease lost")
			}
			item.Status = "sent"
			item.SentAt = &sentAt
			r.items[key] = item
			return nil
		}
	}
	return errors.New("item not found")
}
func (r *memoryRepository) MarkRetry(_ context.Context, id string, leaseUntil time.Time, lastError string, next time.Time, dead bool) error {
	r.retryDead = dead
	for key, item := range r.items {
		if item.ID == id {
			if item.Status != "processing" || item.LeaseUntil == nil || !item.LeaseUntil.Equal(leaseUntil) {
				return errors.New("lease lost")
			}
			if dead {
				item.Status = "dead_letter"
			} else {
				item.Status = "retry"
			}
			item.LastError = lastError
			item.NextAttemptAt = next
			r.items[key] = item
			return nil
		}
	}
	return errors.New("item not found")
}
func (r *memoryRepository) List(_ context.Context, _, _ int) ([]domain.Item, error) {
	result := []domain.Item{}
	for _, item := range r.items {
		result = append(result, item)
	}
	return result, nil
}
func (r *memoryRepository) Health(_ context.Context, enabled bool) (domain.Health, error) {
	return domain.Health{Enabled: enabled}, nil
}

type fakeUDP struct {
	invalid  bool
	requests [][]byte
}

func (c *fakeUDP) Exchange(_ string, packet []byte, _ time.Duration) ([]byte, error) {
	c.requests = append(c.requests, append([]byte{}, packet...))
	response := make([]byte, 20)
	response[0] = 5
	response[1] = packet[1]
	binary.BigEndian.PutUint16(response[2:4], 20)
	input := append([]byte{}, response[:4]...)
	input = append(input, packet[4:20]...)
	input = append(input, []byte("secret")...)
	h := md5.Sum(input)
	copy(response[4:20], h[:])
	if c.invalid {
		response[4]++
	}
	return response, nil
}

func testConfig() config.SophosSSOConfig {
	return config.SophosSSOConfig{Enabled: true, Host: "192.0.2.1", Port: 1813, Secret: "secret", NASIdentifier: "nac", TimeoutSeconds: 1, WorkerBatchSize: 10, MaxRetries: 2, RetryBaseSeconds: 1, ClaimLeaseSeconds: 30}
}
func validInput(status, id, ip string) domain.AccountingInput {
	return domain.AccountingInput{SourceEventID: id, Username: "alice", IPAddress: ip, MACAddress: "AA:BB:CC:DD:EE:FF", AcctStatusType: status, AcctSessionID: "session-1", EventTimestamp: time.Unix(1700000000, 0).UTC()}
}

func TestEnqueueSkipsMABAndDisabled(t *testing.T) {
	repo := newMemoryRepository()
	cfg := testConfig()
	service := NewService(cfg, repo, nil)
	mab := validInput("Start", "mab", "10.0.0.2")
	mab.Username = "aa-bb-cc-dd-ee-ff"
	if err := service.EnqueueAccounting(context.Background(), mab); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 0 {
		t.Fatal("MAB identity was enqueued")
	}
	cfg.Enabled = false
	service = NewService(cfg, repo, nil)
	if err := service.EnqueueAccounting(context.Background(), validInput("Start", "disabled", "10.0.0.2")); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 0 {
		t.Fatal("disabled service enqueued an item")
	}
}

func TestStartInterimStopAndIdempotency(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo, nil)
	startNoIP := validInput("Start", "start-no-ip", "")
	if err := service.EnqueueAccounting(context.Background(), startNoIP); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 0 {
		t.Fatal("IP-less Start must not be queued")
	}
	for _, tc := range []struct{ status, id string }{{"Start", "start"}, {"Interim-Update", "interim"}, {"Stop", "stop"}} {
		if err := service.EnqueueAccounting(context.Background(), validInput(tc.status, tc.id, "10.0.0.2")); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.EnqueueAccounting(context.Background(), validInput("Start", "start", "10.0.0.2")); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 3 {
		t.Fatalf("items=%d, want 3", len(repo.items))
	}
}

func TestProcessDueValidatesResponseAndDeadLetters(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo, nil)
	client := &fakeUDP{}
	service.client = client
	_ = service.EnqueueAccounting(context.Background(), validInput("Start", "ok", "10.0.0.2"))
	if err := service.ProcessDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 || repo.items["ok"].Status != "sent" {
		t.Fatal("valid response was not marked sent")
	}
	if string(client.requests[0]) == "secret" {
		t.Fatal("secret leaked as packet payload")
	}

	repo = newMemoryRepository()
	cfg := testConfig()
	cfg.MaxRetries = 2
	service = NewService(cfg, repo, nil)
	service.client = &fakeUDP{invalid: true}
	_ = service.EnqueueAccounting(context.Background(), validInput("Stop", "bad", "10.0.0.2"))
	if err := service.ProcessDue(context.Background()); err == nil {
		t.Fatal("invalid authenticator accepted")
	}
	if repo.retryDead || repo.items["bad"].Status != "retry" {
		t.Fatal("first failure was not scheduled for retry")
	}
	item := repo.items["bad"]
	item.NextAttemptAt = time.Time{}
	repo.items["bad"] = item
	if err := service.ProcessDue(context.Background()); err == nil {
		t.Fatal("second invalid authenticator accepted")
	}
	if !repo.retryDead || repo.items["bad"].Status != "dead_letter" {
		t.Fatal("failed item was not dead-lettered")
	}
	if len(service.client.(*fakeUDP).requests) != cfg.MaxRetries {
		t.Fatalf("attempts=%d, want exactly %d", len(service.client.(*fakeUDP).requests), cfg.MaxRetries)
	}
	if err := service.ProcessDue(context.Background()); err != nil {
		t.Fatalf("dead-letter item was processed again: %v", err)
	}
}

func TestLeaseFencingRejectsStaleWorker(t *testing.T) {
	repo := newMemoryRepository()
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	item := domain.Item{ID: "item-1", SourceKey: "source-1", Status: "pending", NextAttemptAt: now, CreatedAt: now}
	_, _, _ = repo.Enqueue(context.Background(), item)
	first, _ := repo.ClaimDue(context.Background(), now, now.Add(time.Second), 1)
	second, _ := repo.ClaimDue(context.Background(), now.Add(2*time.Second), now.Add(3*time.Second), 1)
	if len(first) != 1 || len(second) != 1 {
		t.Fatal("item was not reclaimed after lease expiry")
	}
	if err := repo.MarkSent(context.Background(), item.ID, *first[0].LeaseUntil, now.Add(2*time.Second)); err == nil {
		t.Fatal("stale worker overwrote reclaimed item")
	}
	if err := repo.MarkSent(context.Background(), item.ID, *second[0].LeaseUntil, now.Add(3*time.Second)); err != nil {
		t.Fatalf("current lease could not complete item: %v", err)
	}
}

func TestAccountingPacketContainsNoPasswordAttribute(t *testing.T) {
	packet, _, _, err := buildAccountingPacket(domain.Item{Username: "alice", IPAddress: "10.0.0.2", MACAddress: "AA:BB:CC:DD:EE:FF", AcctStatusType: "Start", AcctSessionID: "s1", EventTimestamp: time.Now()}, "nac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	for offset := 20; offset < len(packet); {
		length := int(packet[offset+1])
		if packet[offset] == 2 {
			t.Fatal("User-Password attribute present")
		}
		if length < 2 {
			t.Fatal("invalid attribute")
		}
		offset += length
	}
}

func TestValidateAccountingResponseRejectsMalformed(t *testing.T) {
	if err := validateAccountingResponse([]byte{5}, 1, [16]byte{}, "secret"); err == nil {
		t.Fatal("short response accepted")
	}
}
