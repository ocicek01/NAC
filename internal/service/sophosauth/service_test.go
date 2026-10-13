package sophosauth

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"nac/internal/config"
	domain "nac/internal/domain/sophosauth"
)

type memoryRepository struct {
	events       map[string]domain.Event
	sessions     []domain.Session
	bindings     map[string][]string
	setCalls     []string
	clearCalls   []string
	owner        string
	deviceUser   string
	deviceIP     string
	deviceSeen   time.Time
	deviceMethod string
	auditActions []string
	watermarks   map[string]domain.Event
	failOpenOnce bool
	failOpen     bool
	claimSeq     int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{events: map[string]domain.Event{}, bindings: map[string][]string{}, watermarks: map[string]domain.Event{}, owner: "registered-owner"}
}

func (r *memoryRepository) InsertEvent(_ context.Context, event domain.Event) (domain.Event, bool, error) {
	if existing, ok := r.events[event.Fingerprint]; ok {
		return existing, false, nil
	}
	if event.EventType == "ignored" {
		event.ProcessingStatus = "ignored"
	} else {
		event.ProcessingStatus = "pending"
	}
	r.events[event.Fingerprint] = event
	return event, true, nil
}
func (r *memoryRepository) ClaimEvent(_ context.Context, fingerprint string, now time.Time, lease time.Duration, maxAttempts int) (domain.Claim, bool, error) {
	event, ok := r.events[fingerprint]
	if !ok || !claimable(event, now, maxAttempts) || r.hasNonterminalPredecessor(event) {
		return domain.Claim{}, false, nil
	}
	r.claimSeq++
	event.ProcessingStatus = "processing"
	event.AttemptCount++
	r.events[fingerprint] = event
	return domain.Claim{Event: event, Token: string(rune('a' + r.claimSeq))}, true, nil
}
func (r *memoryRepository) ClaimDueEvents(ctx context.Context, now time.Time, lease time.Duration, maxAttempts, limit int) ([]domain.Claim, error) {
	fingerprints := make([]string, 0, len(r.events))
	for fingerprint := range r.events {
		fingerprints = append(fingerprints, fingerprint)
	}
	sort.Slice(fingerprints, func(i, j int) bool { return causalBefore(r.events[fingerprints[i]], r.events[fingerprints[j]]) })
	for _, fingerprint := range fingerprints {
		if claim, ok, err := r.ClaimEvent(ctx, fingerprint, now, lease, maxAttempts); err != nil {
			return nil, err
		} else if ok {
			return []domain.Claim{claim}, nil
		}
	}
	return nil, nil
}

func (r *memoryRepository) hasNonterminalPredecessor(event domain.Event) bool {
	for _, candidate := range r.events {
		if candidate.ID == event.ID || !sameCausalIdentity(candidate, event) {
			continue
		}
		if candidate.ProcessingStatus != "pending" && candidate.ProcessingStatus != "retry" && candidate.ProcessingStatus != "processing" && candidate.ProcessingStatus != "" {
			continue
		}
		if causalBefore(candidate, event) {
			return true
		}
	}
	return false
}

func sameCausalIdentity(a, b domain.Event) bool {
	return a.Username == b.Username && a.SourceIP == b.SourceIP
}

func causalBefore(a, b domain.Event) bool {
	if !a.EventAt.Equal(b.EventAt) {
		return a.EventAt.Before(b.EventAt)
	}
	if eventRank(a) != eventRank(b) {
		return eventRank(a) < eventRank(b)
	}
	if !a.ReceivedAt.Equal(b.ReceivedAt) {
		return a.ReceivedAt.Before(b.ReceivedAt)
	}
	return a.ID < b.ID
}

func eventRank(event domain.Event) int {
	if event.EventType == "logout" {
		return 2
	}
	return 1
}
func claimable(event domain.Event, now time.Time, maxAttempts int) bool {
	if event.AttemptCount >= maxAttempts || event.ProcessingStatus == "completed" || event.ProcessingStatus == "ignored" || event.ProcessingStatus == "dead_letter" {
		return false
	}
	return event.ProcessingStatus == "" || event.ProcessingStatus == "pending" || event.ProcessingStatus == "processing" || (event.ProcessingStatus == "retry" && (event.NextAttemptAt == nil || !event.NextAttemptAt.After(now)))
}
func (r *memoryRepository) BeginEffects(_ context.Context, claim domain.Claim) (domain.Effects, error) {
	return newMemoryEffects(r, claim), nil
}
func (r *memoryRepository) MarkFailed(_ context.Context, claim domain.Claim, message string, next time.Time, maxAttempts int) error {
	event := r.events[claim.Event.Fingerprint]
	event.LastError = message
	if event.AttemptCount >= maxAttempts {
		event.ProcessingStatus = "dead_letter"
		event.NextAttemptAt = nil
	} else {
		event.ProcessingStatus = "retry"
		event.NextAttemptAt = &next
	}
	r.events[claim.Event.Fingerprint] = event
	return nil
}
func (r *memoryRepository) UpdateEventCorrelation(_ context.Context, id, status, mac string, _ map[string]any) error {
	for key, event := range r.events {
		if event.ID == id {
			event.CorrelationStatus, event.CorrelatedMAC = status, mac
			r.events[key] = event
		}
	}
	return nil
}
func (r *memoryRepository) FindFreshMACsByIP(_ context.Context, ip string, _ time.Time) ([]string, error) {
	return r.bindings[ip], nil
}
func (r *memoryRepository) OpenSession(_ context.Context, event domain.Event) (domain.Session, error) {
	if r.failOpen {
		return domain.Session{}, errors.New("injected persistent open session failure")
	}
	if r.failOpenOnce {
		r.failOpenOnce = false
		return domain.Session{}, errors.New("injected open session failure")
	}
	for i := range r.sessions {
		current := &r.sessions[i]
		if current.Status != "active" || current.Username != event.Username || current.IPAddress != event.SourceIP {
			continue
		}
		if !event.EventAt.Before(current.StartedAt) {
			current.UserGroup = event.UserGroup
			current.MACAddress = event.CorrelatedMAC
			current.ClientUsed = event.ClientUsed
			current.AuthMechanism = event.AuthMechanism
			current.CorrelationStatus = event.CorrelationStatus
			current.StartedAt = event.EventAt
		}
		if event.EventAt.After(current.LastSeenAt) {
			current.LastSeenAt = event.EventAt
		}
		return *current, nil
	}
	s := domain.Session{ID: event.ID, Username: event.Username, UserGroup: event.UserGroup, IPAddress: event.SourceIP, MACAddress: event.CorrelatedMAC, ClientUsed: event.ClientUsed, AuthMechanism: event.AuthMechanism, Status: "active", CorrelationStatus: event.CorrelationStatus, StartedAt: event.EventAt, LastSeenAt: event.EventAt}
	r.sessions = append(r.sessions, s)
	return s, nil
}
func (r *memoryRepository) CloseSessions(_ context.Context, username, ip string, endedAt time.Time) ([]domain.Session, error) {
	closed := []domain.Session{}
	for i := range r.sessions {
		if r.sessions[i].Status == "active" && r.sessions[i].Username == username && r.sessions[i].IPAddress == ip && !r.sessions[i].StartedAt.After(endedAt) {
			r.sessions[i].Status = "closed"
			r.sessions[i].EndedAt = &endedAt
			closed = append(closed, r.sessions[i])
		}
	}
	return closed, nil
}
func (r *memoryRepository) SetDeviceIdentity(_ context.Context, mac, username, ip string, seenAt time.Time) error {
	if r.deviceSeen.After(seenAt) {
		return nil
	}
	r.setCalls = append(r.setCalls, mac+"|"+username+"|"+ip)
	r.deviceUser, r.deviceIP, r.deviceMethod = username, ip, "sophos_captive_portal"
	r.deviceSeen = seenAt
	return nil
}
func (r *memoryRepository) ClearDeviceIdentityIfMatch(_ context.Context, mac, username, ip string, logoutAt time.Time) error {
	if r.deviceUser != username || r.deviceIP != ip || r.deviceSeen.After(logoutAt) {
		return nil
	}
	r.clearCalls = append(r.clearCalls, mac+"|"+username+"|"+ip)
	r.deviceUser, r.deviceIP = "", ""
	if r.deviceMethod == "sophos_captive_portal" {
		r.deviceMethod = ""
	}
	return nil
}
func (r *memoryRepository) ListEvents(context.Context, int, int) ([]domain.Event, error) {
	return nil, nil
}
func (r *memoryRepository) ListSessions(context.Context, int, int) ([]domain.Session, error) {
	return r.sessions, nil
}
func (r *memoryRepository) Health(context.Context, bool, string, int) (domain.Health, error) {
	return domain.Health{}, nil
}

type memoryEffects struct {
	repo                               *memoryRepository
	claim                              domain.Claim
	events                             map[string]domain.Event
	sessions                           []domain.Session
	setCalls                           []string
	clearCalls                         []string
	deviceUser, deviceIP, deviceMethod string
	deviceSeen                         time.Time
	auditActions                       []string
	watermarks                         map[string]domain.Event
}

func newMemoryEffects(r *memoryRepository, claim domain.Claim) *memoryEffects {
	events := make(map[string]domain.Event, len(r.events))
	for key, event := range r.events {
		events[key] = event
	}
	watermarks := make(map[string]domain.Event, len(r.watermarks))
	for key, event := range r.watermarks {
		watermarks[key] = event
	}
	return &memoryEffects{repo: r, claim: claim, events: events, sessions: append([]domain.Session(nil), r.sessions...), setCalls: append([]string(nil), r.setCalls...), clearCalls: append([]string(nil), r.clearCalls...), deviceUser: r.deviceUser, deviceIP: r.deviceIP, deviceMethod: r.deviceMethod, deviceSeen: r.deviceSeen, auditActions: append([]string(nil), r.auditActions...), watermarks: watermarks}
}
func (e *memoryEffects) ReserveCausalOrder(_ context.Context, event domain.Event) (bool, error) {
	key := event.Username + "|" + event.SourceIP
	if current, ok := e.repo.watermarks[key]; ok && !causalBefore(current, event) {
		return false, nil
	}
	e.repo.watermarks[key] = event
	return true, nil
}
func (e *memoryEffects) UpdateEventCorrelation(ctx context.Context, id, status, mac string, metadata map[string]any) error {
	return e.repo.UpdateEventCorrelation(ctx, id, status, mac, metadata)
}
func (e *memoryEffects) FindFreshMACsByIP(ctx context.Context, ip string, since time.Time) ([]string, error) {
	return e.repo.FindFreshMACsByIP(ctx, ip, since)
}
func (e *memoryEffects) OpenSession(ctx context.Context, event domain.Event) (domain.Session, error) {
	return e.repo.OpenSession(ctx, event)
}
func (e *memoryEffects) CloseSessions(ctx context.Context, username, ip string, ended time.Time) ([]domain.Session, error) {
	return e.repo.CloseSessions(ctx, username, ip, ended)
}
func (e *memoryEffects) SetDeviceIdentity(ctx context.Context, mac, username, ip string, seen time.Time) error {
	return e.repo.SetDeviceIdentity(ctx, mac, username, ip, seen)
}
func (e *memoryEffects) ClearDeviceIdentityIfMatch(ctx context.Context, mac, username, ip string, at time.Time) error {
	return e.repo.ClearDeviceIdentityIfMatch(ctx, mac, username, ip, at)
}
func (e *memoryEffects) RecordAudit(_ context.Context, action, _, _, _ string, _ map[string]any) error {
	e.repo.auditActions = append(e.repo.auditActions, action)
	return nil
}
func (e *memoryEffects) Complete(_ context.Context) error {
	event := e.repo.events[e.claim.Event.Fingerprint]
	event.ProcessingStatus = "completed"
	event.NextAttemptAt = nil
	now := time.Now()
	event.ProcessedAt = &now
	e.repo.events[e.claim.Event.Fingerprint] = event
	return nil
}
func (e *memoryEffects) Rollback(_ context.Context) error {
	e.repo.events = e.events
	e.repo.sessions = e.sessions
	e.repo.setCalls = e.setCalls
	e.repo.clearCalls = e.clearCalls
	e.repo.deviceUser = e.deviceUser
	e.repo.deviceIP = e.deviceIP
	e.repo.deviceMethod = e.deviceMethod
	e.repo.deviceSeen = e.deviceSeen
	e.repo.auditActions = e.auditActions
	e.repo.watermarks = e.watermarks
	return nil
}

type auditCapture struct{ actions []string }

func (a *auditCapture) Record(_ context.Context, action, _, _, _, _, _ string, _ map[string]any) error {
	a.actions = append(a.actions, action)
	return nil
}

func newService(repo *memoryRepository) *Service {
	return NewService(config.SophosAuthConfig{Enabled: true, AllowedClients: []string{"Captive Portal", "Web Client"}, BindingFreshnessSec: 600, MaxAttempts: 3, RetryBaseSec: 1, ClaimLeaseSec: 10, RetryBatchSize: 10}, repo, &auditCapture{})
}

func parsed(fingerprint string) domain.ParsedEvent {
	return domain.ParsedEvent{Fingerprint: fingerprint, EventType: "login", MessageID: "17701", Username: "omer", ClientUsed: "Captive Portal", AuthStatus: "Success", SourceIP: "10.6.9.93", EventAt: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
}

func TestLoginCorrelationOutcomesAndOwnerUntouched(t *testing.T) {
	tests := []struct {
		name       string
		bindings   []string
		sourceMAC  string
		wantStatus string
		wantSet    int
	}{
		{"unique", []string{"30:9C:23:9B:97:AA"}, "", "correlated", 1},
		{"none", nil, "", "uncorrelated", 0},
		{"source mac only", nil, "30:9C:23:9B:97:AA", "correlated", 1},
		{"ambiguous", []string{"30:9C:23:9B:97:AA", "00:11:22:33:44:55"}, "", "ambiguous", 0},
		{"source conflict", []string{"30:9C:23:9B:97:AA"}, "00:11:22:33:44:55", "ambiguous", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMemoryRepository()
			repo.bindings["10.6.9.93"] = tt.bindings
			event := parsed(tt.name)
			event.SourceMAC = tt.sourceMAC
			if err := newService(repo).Process(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			if len(repo.sessions) != 1 || repo.sessions[0].CorrelationStatus != tt.wantStatus || len(repo.setCalls) != tt.wantSet {
				t.Fatalf("unexpected repo state: %#v set=%v", repo.sessions, repo.setCalls)
			}
			if repo.owner != "registered-owner" {
				t.Fatal("registered owner was modified")
			}
		})
	}
}

func TestDuplicateFailedAndRadiusEventsDoNotUpdateDevice(t *testing.T) {
	repo := newMemoryRepository()
	repo.bindings["10.6.9.93"] = []string{"30:9C:23:9B:97:AA"}
	service := newService(repo)
	event := parsed("same")
	if err := service.Process(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := service.Process(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	failed := parsed("failed")
	failed.EventType = "ignored"
	failed.AuthStatus = "Failed"
	radius := parsed("radius")
	radius.ClientUsed = "RADIUS SSO"
	if err := service.Process(context.Background(), failed); err != nil {
		t.Fatal(err)
	}
	if err := service.Process(context.Background(), radius); err != nil {
		t.Fatal(err)
	}
	if len(repo.setCalls) != 1 || len(repo.sessions) != 1 {
		t.Fatalf("duplicate/excluded event had side effect: set=%v sessions=%v", repo.setCalls, repo.sessions)
	}
}

func TestLogoutClosesAndClearsOnlyCorrelatedSession(t *testing.T) {
	repo := newMemoryRepository()
	repo.bindings["10.6.9.93"] = []string{"30:9C:23:9B:97:AA"}
	service := newService(repo)
	if err := service.Process(context.Background(), parsed("login")); err != nil {
		t.Fatal(err)
	}
	logout := parsed("logout")
	logout.EventType = "logout"
	logout.MessageID = "17703"
	logout.EventAt = logout.EventAt.Add(time.Minute)
	if err := service.Process(context.Background(), logout); err != nil {
		t.Fatal(err)
	}
	if repo.sessions[0].Status != "closed" || len(repo.clearCalls) != 1 {
		t.Fatalf("logout did not close/clear: %#v %v", repo.sessions, repo.clearCalls)
	}
	if repo.deviceMethod != "" {
		t.Fatalf("matching logout did not reset captive authentication method: %q", repo.deviceMethod)
	}
}

func TestDelayedLogoutDoesNotCloseNewerSessionOrDeviceIdentity(t *testing.T) {
	repo := newMemoryRepository()
	repo.bindings["10.6.9.93"] = []string{"30:9C:23:9B:97:AA"}
	service := newService(repo)

	login := parsed("new-login")
	login.EventAt = time.Date(2026, 10, 13, 12, 5, 0, 0, time.UTC)
	if err := service.Process(context.Background(), login); err != nil {
		t.Fatal(err)
	}
	if repo.deviceMethod != "sophos_captive_portal" {
		t.Fatalf("login did not set authentication method: %q", repo.deviceMethod)
	}

	delayedLogout := parsed("old-logout")
	delayedLogout.EventType = "logout"
	delayedLogout.MessageID = "17703"
	delayedLogout.EventAt = login.EventAt.Add(-2 * time.Minute)
	if err := service.Process(context.Background(), delayedLogout); err != nil {
		t.Fatal(err)
	}

	if repo.sessions[0].Status != "active" {
		t.Fatalf("delayed logout closed newer session: %#v", repo.sessions[0])
	}
	if len(repo.clearCalls) != 0 || repo.deviceUser != "omer" || repo.deviceIP != "10.6.9.93" || repo.deviceMethod != "sophos_captive_portal" {
		t.Fatalf("delayed logout cleared newer device identity: calls=%v user=%q ip=%q method=%q", repo.clearCalls, repo.deviceUser, repo.deviceIP, repo.deviceMethod)
	}
}

func TestDelayedOlderLoginPreservesNewerSessionAndDeviceIdentity(t *testing.T) {
	repo := newMemoryRepository()
	service := newService(repo)

	newer := parsed("newer-login")
	newer.EventAt = time.Date(2026, 10, 13, 12, 10, 0, 0, time.UTC)
	newer.SourceMAC = "30:9C:23:9B:97:AA"
	newer.UserGroup = "new-group"
	newer.ClientUsed = "Web Client"
	newer.AuthMechanism = "LDAP"
	if err := service.Process(context.Background(), newer); err != nil {
		t.Fatal(err)
	}

	older := parsed("older-login")
	older.EventAt = newer.EventAt.Add(-5 * time.Minute)
	older.SourceMAC = "00:11:22:33:44:55"
	older.UserGroup = "old-group"
	older.ClientUsed = "Captive Portal"
	older.AuthMechanism = "Local"
	if err := service.Process(context.Background(), older); err != nil {
		t.Fatal(err)
	}

	if len(repo.sessions) != 1 {
		t.Fatalf("expected one active session, got %#v", repo.sessions)
	}
	session := repo.sessions[0]
	if session.MACAddress != "30:9C:23:9B:97:AA" || session.UserGroup != "new-group" || session.ClientUsed != "Web Client" || session.AuthMechanism != "LDAP" || !session.StartedAt.Equal(newer.EventAt) || !session.LastSeenAt.Equal(newer.EventAt) {
		t.Fatalf("delayed login overwrote newer session: %#v", session)
	}
	if repo.deviceUser != "omer" || repo.deviceIP != "10.6.9.93" || !repo.deviceSeen.Equal(newer.EventAt) || repo.deviceMethod != "sophos_captive_portal" {
		t.Fatalf("delayed login overwrote newer device identity: user=%q ip=%q seen=%s method=%q", repo.deviceUser, repo.deviceIP, repo.deviceSeen, repo.deviceMethod)
	}
	if len(repo.setCalls) != 1 {
		t.Fatalf("delayed login attempted a second current identity update: %v", repo.setCalls)
	}
}

func TestPersistedEventRetriesAfterDownstreamFailureWithoutDuplicateEffects(t *testing.T) {
	repo := newMemoryRepository()
	repo.bindings["10.6.9.93"] = []string{"30:9C:23:9B:97:AA"}
	repo.failOpenOnce = true
	service := newService(repo)
	clock := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	event := parsed("retry-after-insert")

	if err := service.Process(context.Background(), event); err == nil {
		t.Fatal("expected injected downstream failure")
	}
	stored := repo.events[event.Fingerprint]
	if stored.ProcessingStatus != "retry" || stored.AttemptCount != 1 || stored.LastError == "" {
		t.Fatalf("failure was not persisted for retry: %#v", stored)
	}
	if len(repo.sessions) != 0 || len(repo.setCalls) != 0 || len(repo.auditActions) != 0 {
		t.Fatalf("failed transaction leaked side effects: sessions=%v set=%v audit=%v", repo.sessions, repo.setCalls, repo.auditActions)
	}

	clock = clock.Add(2 * time.Second)
	if err := service.Process(context.Background(), event); err != nil {
		t.Fatalf("duplicate datagram did not retry persisted event: %v", err)
	}
	stored = repo.events[event.Fingerprint]
	if stored.ProcessingStatus != "completed" || stored.AttemptCount != 2 || stored.ProcessedAt == nil {
		t.Fatalf("retry did not complete event: %#v", stored)
	}
	if len(repo.sessions) != 1 || len(repo.setCalls) != 1 || len(repo.auditActions) != 1 {
		t.Fatalf("retry produced missing or duplicate side effects: sessions=%v set=%v audit=%v", repo.sessions, repo.setCalls, repo.auditActions)
	}

	if err := service.Process(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(repo.sessions) != 1 || len(repo.setCalls) != 1 || len(repo.auditActions) != 1 {
		t.Fatalf("completed duplicate produced side effects: sessions=%v set=%v audit=%v", repo.sessions, repo.setCalls, repo.auditActions)
	}
}

func TestBackgroundRetryCompletesDueEvent(t *testing.T) {
	repo := newMemoryRepository()
	repo.bindings["10.6.9.93"] = []string{"30:9C:23:9B:97:AA"}
	repo.failOpenOnce = true
	service := newService(repo)
	clock := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	if err := service.Process(context.Background(), parsed("background-retry")); err == nil {
		t.Fatal("expected injected downstream failure")
	}
	clock = clock.Add(2 * time.Second)
	if err := service.ProcessDue(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if repo.events["background-retry"].ProcessingStatus != "completed" || len(repo.auditActions) != 1 {
		t.Fatalf("background retry did not complete exactly once: %#v audit=%v", repo.events["background-retry"], repo.auditActions)
	}
}

func TestFailedLoginRetryBlocksLaterLogoutUntilLoginCompletes(t *testing.T) {
	repo := newMemoryRepository()
	repo.bindings["10.6.9.93"] = []string{"30:9C:23:9B:97:AA"}
	repo.failOpenOnce = true
	service := newService(repo)
	clock := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }

	login := parsed("ordered-login")
	if err := service.Process(context.Background(), login); err == nil {
		t.Fatal("expected login processing failure")
	}
	logout := parsed("ordered-logout")
	logout.EventType = "logout"
	logout.MessageID = "17703"
	logout.EventAt = login.EventAt.Add(time.Minute)
	if err := service.Process(context.Background(), logout); err != nil {
		t.Fatal(err)
	}
	if repo.events[logout.Fingerprint].ProcessingStatus != "pending" {
		t.Fatalf("logout bypassed retrying login: %#v", repo.events[logout.Fingerprint])
	}
	if len(repo.sessions) != 0 || len(repo.clearCalls) != 0 {
		t.Fatalf("blocked logout produced side effects: sessions=%v clear=%v", repo.sessions, repo.clearCalls)
	}

	clock = clock.Add(2 * time.Second)
	if err := service.ProcessDue(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if repo.events[login.Fingerprint].ProcessingStatus != "completed" || repo.events[logout.Fingerprint].ProcessingStatus != "completed" {
		t.Fatalf("ordered events did not complete: login=%#v logout=%#v", repo.events[login.Fingerprint], repo.events[logout.Fingerprint])
	}
	if len(repo.sessions) != 1 || repo.sessions[0].Status != "closed" || len(repo.clearCalls) != 1 || repo.deviceUser != "" {
		t.Fatalf("causal login/logout result is wrong: sessions=%#v clear=%v user=%q", repo.sessions, repo.clearCalls, repo.deviceUser)
	}
}

func TestLateOlderLoginAfterCompletedLogoutCannotReactivateIdentity(t *testing.T) {
	repo := newMemoryRepository()
	repo.bindings["10.6.9.93"] = []string{"30:9C:23:9B:97:AA"}
	service := newService(repo)
	clock := time.Date(2026, 10, 13, 12, 20, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }

	logout := parsed("completed-logout")
	logout.EventType = "logout"
	logout.MessageID = "17703"
	logout.EventAt = time.Date(2026, 10, 13, 12, 10, 0, 0, time.UTC)
	if err := service.Process(context.Background(), logout); err != nil {
		t.Fatal(err)
	}

	lateLogin := parsed("late-old-login")
	lateLogin.EventAt = logout.EventAt.Add(-time.Minute)
	lateLogin.SourceMAC = "30:9C:23:9B:97:AA"
	clock = clock.Add(time.Second)
	if err := service.Process(context.Background(), lateLogin); err != nil {
		t.Fatal(err)
	}
	stored := repo.events[lateLogin.Fingerprint]
	if stored.ProcessingStatus != "completed" || stored.CorrelationStatus != "ignored" {
		t.Fatalf("late login was not durably suppressed: %#v", stored)
	}
	if len(repo.sessions) != 0 || len(repo.setCalls) != 0 || repo.deviceUser != "" {
		t.Fatalf("late login reactivated identity: sessions=%#v set=%v user=%q", repo.sessions, repo.setCalls, repo.deviceUser)
	}
	if len(repo.auditActions) == 0 || repo.auditActions[len(repo.auditActions)-1] != "sophos_auth_stale_event" {
		t.Fatalf("stale login audit missing: %v", repo.auditActions)
	}
}

func TestPoisonEventStopsAtDeadLetterWithoutSideEffects(t *testing.T) {
	repo := newMemoryRepository()
	repo.bindings["10.6.9.93"] = []string{"30:9C:23:9B:97:AA"}
	repo.failOpen = true
	service := newService(repo)
	clock := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	event := parsed("poison")

	for attempt := 1; attempt <= 3; attempt++ {
		if err := service.Process(context.Background(), event); err == nil {
			t.Fatalf("attempt %d unexpectedly succeeded", attempt)
		}
		clock = clock.Add(time.Duration(1<<attempt) * time.Second)
	}
	stored := repo.events[event.Fingerprint]
	if stored.ProcessingStatus != "dead_letter" || stored.AttemptCount != 3 || stored.NextAttemptAt != nil {
		t.Fatalf("poison event was not bounded/dead-lettered: %#v", stored)
	}
	if err := service.Process(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(repo.sessions) != 0 || len(repo.setCalls) != 0 || len(repo.auditActions) != 0 {
		t.Fatalf("poison event leaked side effects: sessions=%v set=%v audit=%v", repo.sessions, repo.setCalls, repo.auditActions)
	}
}
