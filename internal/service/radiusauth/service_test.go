package radiusauth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"nac/internal/config"
	"nac/internal/domain/macobservation"
	domain "nac/internal/domain/radiusauth"
	"nac/internal/domain/radiusevent"
	"nac/internal/domain/session"
	"nac/internal/domain/sophossso"
	"nac/internal/domain/switchasset"
	deviceservice "nac/internal/service/device"
)

type accountingEventStore struct {
	items  map[string]radiusevent.Event
	states map[string]string
	err    error
}

func (s *accountingEventStore) Insert(_ context.Context, event radiusevent.Event) (radiusevent.Event, error) {
	if s.err != nil {
		return radiusevent.Event{}, s.err
	}
	if s.items == nil {
		s.items = map[string]radiusevent.Event{}
	}
	if _, exists := s.items[event.ID]; exists {
		return radiusevent.Event{}, nil
	}
	s.items[event.ID] = event
	return event, nil
}

func (s *accountingEventStore) ClaimAccounting(_ context.Context, event radiusevent.Event, _, _ time.Time) (radiusevent.ClaimResult, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.items == nil {
		s.items = map[string]radiusevent.Event{}
		s.states = map[string]string{}
	}
	switch s.states[event.ID] {
	case "processed":
		return radiusevent.Duplicate, nil
	case "processing":
		return radiusevent.Busy, nil
	}
	s.items[event.ID] = event
	s.states[event.ID] = "processing"
	return radiusevent.Claimed, nil
}

func (s *accountingEventStore) CompleteAccounting(_ context.Context, eventID string, _, _ time.Time) error {
	if s.err != nil {
		return s.err
	}
	s.states[eventID] = "processed"
	return nil
}

func (s *accountingEventStore) FailAccounting(_ context.Context, eventID, _ string, _, _ time.Time) error {
	s.states[eventID] = "failed"
	return nil
}

type accountingSessionStore struct {
	items     map[string]session.Session
	upsertErr error
	findErr   error
	upserts   int
}

func (s *accountingSessionStore) Upsert(_ context.Context, item session.Session) (session.Session, error) {
	if s.upsertErr != nil {
		return session.Session{}, s.upsertErr
	}
	s.upserts++
	if s.items == nil {
		s.items = map[string]session.Session{}
	}
	if current, exists := s.items[item.ActiveKey]; exists {
		item.ID = current.ID
		if item.IPAddress == "" {
			item.IPAddress = current.IPAddress
		}
		if item.StartedAt.IsZero() {
			item.StartedAt = current.StartedAt
		}
	}
	s.items[item.ActiveKey] = item
	return item, nil
}

func (s *accountingSessionStore) FindByAcctSession(_ context.Context, macAddress, switchID, acctSessionID string) (*session.Session, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	for _, item := range s.items {
		if item.MACAddress == macAddress && item.SwitchID == switchID && item.AcctSessionID == acctSessionID {
			copy := item
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *accountingSessionStore) PromoteToAccountingKey(_ context.Context, oldKey, newKey, acctSessionID string) error {
	if item, exists := s.items[oldKey]; exists {
		delete(s.items, oldKey)
		item.ActiveKey = newKey
		item.AcctSessionID = acctSessionID
		s.items[newKey] = item
	}
	return nil
}

type accountingSwitchStore struct {
	item *switchasset.Switch
	err  error
}

type countingObservationStore struct {
	calls int
}

func (s *countingObservationStore) Insert(_ context.Context, item macobservation.Observation) (macobservation.Observation, error) {
	s.calls++
	return item, nil
}

type countingDeviceUpdater struct {
	calls    int
	failOnce bool
}

type accountingEnqueuer struct {
	inputs   []sophossso.AccountingInput
	err      error
	failOnce bool
}

func (e *accountingEnqueuer) EnqueueAccounting(_ context.Context, input sophossso.AccountingInput) error {
	e.inputs = append(e.inputs, input)
	if e.failOnce {
		e.failOnce = false
		return e.err
	}
	return e.err
}

func (s *countingDeviceUpdater) UpsertFromRadius(context.Context, deviceservice.RadiusInventoryInput) error {
	s.calls++
	if s.failOnce {
		s.failOnce = false
		return errors.New("device repository unavailable")
	}
	return nil
}

func (s accountingSwitchStore) FindByManagementIP(context.Context, string) (*switchasset.Switch, error) {
	return s.item, s.err
}

func (s accountingSwitchStore) FindByName(context.Context, string) (*switchasset.Switch, error) {
	return s.item, s.err
}

func newAccountingService(events *accountingEventStore, sessions *accountingSessionStore) *Service {
	service := NewService(nil, config.RadiusConfig{}, events, nil, accountingSwitchStore{item: &switchasset.Switch{
		ID: "91b47420-527e-409d-bc0e-d00983e9ccf6", Name: "sw-1", ManagementIP: "10.6.8.19",
	}}, nil, sessions)
	service.now = func() time.Time { return time.Date(2026, 10, 13, 12, 0, 30, 0, time.UTC) }
	return service
}

func validAccountingRequest(status string) domain.AccountingRequest {
	return domain.AccountingRequest{
		Username:        "omer.cicek",
		MACAddress:      "30:9c:23:9b:97:aa",
		NASIPAddress:    "10.6.8.19",
		NASPort:         "5",
		NASPortID:       "5",
		AcctStatusType:  status,
		AcctSessionID:   "session-1",
		FramedIPAddress: "10.6.9.93",
		EventTimestamp:  "2026-10-13T12:00:00Z",
	}
}

func TestAccountingStartInterimStopUpdatesOneSession(t *testing.T) {
	events := &accountingEventStore{}
	sessions := &accountingSessionStore{}
	service := newAccountingService(events, sessions)

	for index, status := range []string{"Start", "Interim-Update", "Stop"} {
		req := validAccountingRequest(status)
		req.EventTimestamp = time.Date(2026, 10, 13, 12, index, 0, 0, time.UTC).Format(time.RFC3339)
		req.SessionTime = string(rune('0' + index))
		if _, err := service.Accounting(context.Background(), req); err != nil {
			t.Fatalf("Accounting(%s) error = %v", status, err)
		}
	}

	if len(sessions.items) != 1 {
		t.Fatalf("session count = %d, want 1", len(sessions.items))
	}
	for _, item := range sessions.items {
		if item.Status != "stopped" || item.EndedAt == nil {
			t.Fatalf("final session = status %q ended_at %v, want stopped with ended_at", item.Status, item.EndedAt)
		}
		if item.AcctSessionID != "session-1" {
			t.Fatalf("acct_session_id = %q", item.AcctSessionID)
		}
	}
	if len(events.items) != 3 {
		t.Fatalf("event count = %d, want 3", len(events.items))
	}
}

func TestAccountingRetriesWhenSophosEnqueueFails(t *testing.T) {
	events := &accountingEventStore{}
	service := newAccountingService(events, &accountingSessionStore{})
	enqueuer := &accountingEnqueuer{err: errors.New("outbox unavailable"), failOnce: true}
	service.ConfigureSophosSSO(enqueuer)
	if _, err := service.Accounting(context.Background(), validAccountingRequest("Start")); err == nil {
		t.Fatal("Accounting() error = nil, want outbox error")
	}
	enqueuer.err = nil
	response, err := service.Accounting(context.Background(), validAccountingRequest("Start"))
	if err != nil {
		t.Fatalf("retry Accounting() error = %v", err)
	}
	if response.Status != "ok" || len(enqueuer.inputs) != 2 {
		t.Fatalf("response=%+v enqueue calls=%d", response, len(enqueuer.inputs))
	}
	if events.states[accountingEventID(validAccountingRequest("Start"), time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))] != "processed" {
		t.Fatal("accounting event was not completed after successful retry")
	}
}

func TestAccountingStopWithoutIPUsesPreservedSessionIP(t *testing.T) {
	service := newAccountingService(&accountingEventStore{}, &accountingSessionStore{})
	enqueuer := &accountingEnqueuer{}
	service.ConfigureSophosSSO(enqueuer)
	if _, err := service.Accounting(context.Background(), validAccountingRequest("Start")); err != nil {
		t.Fatal(err)
	}
	stop := validAccountingRequest("Stop")
	stop.FramedIPAddress = ""
	stop.EventTimestamp = "2026-10-13T12:01:00Z"
	stop.SessionTime = "60"
	if _, err := service.Accounting(context.Background(), stop); err != nil {
		t.Fatal(err)
	}
	if len(enqueuer.inputs) != 2 || enqueuer.inputs[1].IPAddress != "10.6.9.93" {
		t.Fatalf("Stop enqueue inputs = %+v", enqueuer.inputs)
	}
}

func TestAccountingDuplicateDoesNotCreateEventOrSession(t *testing.T) {
	events := &accountingEventStore{}
	sessions := &accountingSessionStore{}
	service := newAccountingService(events, sessions)
	observations := &countingObservationStore{}
	devices := &countingDeviceUpdater{}
	service.observations = observations
	service.devices = devices
	req := validAccountingRequest("Start")

	for range 2 {
		if _, err := service.Accounting(context.Background(), req); err != nil {
			t.Fatalf("Accounting() error = %v", err)
		}
	}

	if len(events.items) != 1 {
		t.Fatalf("event count = %d, want 1", len(events.items))
	}
	if len(sessions.items) != 1 {
		t.Fatalf("session count = %d, want 1", len(sessions.items))
	}
	if sessions.upserts != 1 {
		t.Fatalf("session upserts = %d, want 1", sessions.upserts)
	}
	if observations.calls != 1 || devices.calls != 1 {
		t.Fatalf("duplicate ran inventory side effects: observations=%d devices=%d", observations.calls, devices.calls)
	}
}

func TestAccountingBusyClaimDoesNotRunSideEffects(t *testing.T) {
	req := validAccountingRequest("Start")
	eventAt, err := accountingEventTime(req, time.Date(2026, 10, 13, 12, 0, 30, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	event, err := buildAccountingEvent(req, eventAt)
	if err != nil {
		t.Fatal(err)
	}
	events := &accountingEventStore{
		items:  map[string]radiusevent.Event{event.ID: event},
		states: map[string]string{event.ID: "processing"},
	}
	sessions := &accountingSessionStore{}
	service := newAccountingService(events, sessions)

	if _, err := service.Accounting(context.Background(), req); !errors.Is(err, domain.ErrAccountingInProgress) {
		t.Fatalf("Accounting() error = %v, want ErrAccountingInProgress", err)
	}
	if sessions.upserts != 0 || len(sessions.items) != 0 {
		t.Fatalf("busy claim ran session side effects: upserts=%d items=%d", sessions.upserts, len(sessions.items))
	}
}

func TestAccountingFailureCanBeRetriedWithoutNewEvent(t *testing.T) {
	events := &accountingEventStore{}
	sessions := &accountingSessionStore{upsertErr: errors.New("database unavailable")}
	service := newAccountingService(events, sessions)
	req := validAccountingRequest("Start")

	if _, err := service.Accounting(context.Background(), req); err == nil {
		t.Fatal("first Accounting() error = nil")
	}
	sessions.upsertErr = nil
	if _, err := service.Accounting(context.Background(), req); err != nil {
		t.Fatalf("retry Accounting() error = %v", err)
	}

	if len(events.items) != 1 {
		t.Fatalf("event count = %d, want 1", len(events.items))
	}
	for eventID := range events.items {
		if events.states[eventID] != "processed" {
			t.Fatalf("event state = %q, want processed", events.states[eventID])
		}
	}
	if len(sessions.items) != 1 || sessions.upserts != 1 {
		t.Fatalf("session count/upserts = %d/%d, want 1/1", len(sessions.items), sessions.upserts)
	}
}

func TestAccountingInventoryFailureRetriesWithoutDuplicateObservation(t *testing.T) {
	events := &accountingEventStore{}
	sessions := &accountingSessionStore{}
	service := newAccountingService(events, sessions)
	observations := &countingObservationStore{}
	devices := &countingDeviceUpdater{failOnce: true}
	service.observations = observations
	service.devices = devices
	req := validAccountingRequest("Start")

	if _, err := service.Accounting(context.Background(), req); err == nil {
		t.Fatal("first Accounting() error = nil")
	}
	if _, err := service.Accounting(context.Background(), req); err != nil {
		t.Fatalf("retry Accounting() error = %v", err)
	}

	if len(events.items) != 1 || len(sessions.items) != 1 {
		t.Fatalf("event/session count = %d/%d, want 1/1", len(events.items), len(sessions.items))
	}
	if devices.calls != 2 || observations.calls != 1 {
		t.Fatalf("inventory calls = devices %d observations %d, want 2/1", devices.calls, observations.calls)
	}
}

func TestLateStartCannotOverwriteStoppedSession(t *testing.T) {
	events := &accountingEventStore{}
	key := buildAccountingSessionKey("30:9C:23:9B:97:AA", "91b47420-527e-409d-bc0e-d00983e9ccf6", "session-1")
	stopTime := time.Date(2026, 10, 13, 12, 10, 0, 0, time.UTC)
	sessions := &accountingSessionStore{items: map[string]session.Session{
		key: {
			ID: "session-row", ActiveKey: key, SwitchID: "91b47420-527e-409d-bc0e-d00983e9ccf6",
			MACAddress: "30:9C:23:9B:97:AA", AcctSessionID: "session-1", Status: "stopped",
			IPAddress: "10.6.9.93", NASPort: "5", NASPortID: "5", Username: "omer.cicek",
			LastSeenAt: stopTime, EndedAt: &stopTime,
		},
	}}
	service := newAccountingService(events, sessions)
	req := validAccountingRequest("Start")
	req.EventTimestamp = "2026-10-13T12:00:00Z"
	req.FramedIPAddress = "10.6.9.205"
	req.NASPort = "47"
	req.NASPortID = "47"
	req.Username = "stale.user"

	if _, err := service.Accounting(context.Background(), req); err != nil {
		t.Fatalf("Accounting() error = %v", err)
	}
	item := sessions.items[key]
	if sessions.upserts != 0 {
		t.Fatalf("session upserts = %d, want 0", sessions.upserts)
	}
	if item.Status != "stopped" || item.EndedAt == nil || item.IPAddress != "10.6.9.93" || item.NASPort != "5" || item.Username != "omer.cicek" {
		t.Fatalf("late Start changed stopped session: %+v", item)
	}
}

func TestAccountingUsesEventTimestampAndDelay(t *testing.T) {
	events := &accountingEventStore{}
	service := newAccountingService(events, &accountingSessionStore{})
	req := validAccountingRequest("Start")
	req.EventTimestamp = ""
	req.AcctDelayTime = 30

	if _, err := service.Accounting(context.Background(), req); err != nil {
		t.Fatalf("Accounting() error = %v", err)
	}
	for _, event := range events.items {
		want := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
		if !event.CreatedAt.Equal(want) {
			t.Fatalf("event time = %v, want %v", event.CreatedAt, want)
		}
		var metadata map[string]any
		if err := json.Unmarshal([]byte(event.ControlAttributes), &metadata); err != nil {
			t.Fatalf("accounting metadata is invalid: %v", err)
		}
		if metadata["acct_delay_time"] != float64(30) {
			t.Fatalf("acct_delay_time metadata = %v, want 30", metadata["acct_delay_time"])
		}
	}
}

func TestAccountingReturnsRepositoryErrors(t *testing.T) {
	t.Run("session", func(t *testing.T) {
		service := newAccountingService(&accountingEventStore{}, &accountingSessionStore{upsertErr: errors.New("database unavailable")})
		if _, err := service.Accounting(context.Background(), validAccountingRequest("Start")); err == nil {
			t.Fatal("Accounting() error = nil, want repository error")
		}
	})

	t.Run("event", func(t *testing.T) {
		service := newAccountingService(&accountingEventStore{err: errors.New("database unavailable")}, &accountingSessionStore{})
		if _, err := service.Accounting(context.Background(), validAccountingRequest("Start")); err == nil {
			t.Fatal("Accounting() error = nil, want repository error")
		}
	})
}

func TestAccountingRejectsInvalidPayload(t *testing.T) {
	tests := []struct {
		name string
		req  domain.AccountingRequest
	}{
		{name: "empty", req: domain.AccountingRequest{}},
		{name: "invalid status", req: validAccountingRequest("Alive")},
		{name: "missing session", req: func() domain.AccountingRequest {
			req := validAccountingRequest("Start")
			req.AcctSessionID = ""
			return req
		}()},
		{name: "invalid IP", req: func() domain.AccountingRequest {
			req := validAccountingRequest("Start")
			req.FramedIPAddress = "not-an-ip"
			return req
		}()},
		{name: "invalid timestamp", req: func() domain.AccountingRequest {
			req := validAccountingRequest("Start")
			req.EventTimestamp = "yesterday"
			return req
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newAccountingService(&accountingEventStore{}, &accountingSessionStore{})
			if _, err := service.Accounting(context.Background(), test.req); err == nil {
				t.Fatal("Accounting() error = nil, want validation error")
			}
		})
	}
}

func TestAccountingOnRecordsOnlyNASLifecycleEvent(t *testing.T) {
	events := &accountingEventStore{}
	service := newAccountingService(events, nil)
	req := domain.AccountingRequest{NASIPAddress: "10.6.8.19", AcctStatusType: "Accounting-On", EventTimestamp: "1760356800"}

	if _, err := service.Accounting(context.Background(), req); err != nil {
		t.Fatalf("Accounting() error = %v", err)
	}
	if len(events.items) != 1 {
		t.Fatalf("event count = %d, want 1", len(events.items))
	}
}

func TestAuthorizeStatusIsNotAuthenticated(t *testing.T) {
	if got := deriveAuthorizeSessionStatus("accept"); got != "authorization-pending" {
		t.Fatalf("deriveAuthorizeSessionStatus(accept) = %q", got)
	}
	if got := deriveAuthorizeSessionStatus("reject"); got != "rejected" {
		t.Fatalf("deriveAuthorizeSessionStatus(reject) = %q", got)
	}
}

func TestAuthorizeSessionRemainsPendingUntilAccountingStarts(t *testing.T) {
	sessions := &accountingSessionStore{}
	service := NewService(nil, config.RadiusConfig{QuarantineVLAN: "999"}, nil, nil, accountingSwitchStore{item: &switchasset.Switch{
		ID: "91b47420-527e-409d-bc0e-d00983e9ccf6", Name: "sw-1", ManagementIP: "10.6.8.19",
	}}, nil, sessions)

	if _, err := service.Authorize(context.Background(), domain.AuthorizeRequest{
		Username: "omer.cicek", MACAddress: "30:9C:23:9B:97:AA", NASIPAddress: "10.6.8.19", NASPort: "5",
	}); err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if len(sessions.items) != 1 {
		t.Fatalf("session count = %d, want 1", len(sessions.items))
	}
	for _, item := range sessions.items {
		if item.Status != "authorization-pending" {
			t.Fatalf("status = %q, want authorization-pending", item.Status)
		}
		if !item.StartedAt.IsZero() {
			t.Fatalf("started_at = %v, want zero", item.StartedAt)
		}
	}
}
