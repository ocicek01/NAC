package sophosauth

import (
	"context"
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
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{events: map[string]domain.Event{}, bindings: map[string][]string{}, owner: "registered-owner"}
}

func (r *memoryRepository) InsertEvent(_ context.Context, event domain.Event) (domain.Event, bool, error) {
	if existing, ok := r.events[event.Fingerprint]; ok {
		return existing, false, nil
	}
	r.events[event.Fingerprint] = event
	return event, true, nil
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

type auditCapture struct{ actions []string }

func (a *auditCapture) Record(_ context.Context, action, _, _, _, _, _ string, _ map[string]any) error {
	a.actions = append(a.actions, action)
	return nil
}

func newService(repo *memoryRepository) *Service {
	return NewService(config.SophosAuthConfig{Enabled: true, AllowedClients: []string{"Captive Portal", "Web Client"}, BindingFreshnessSec: 600}, repo, &auditCapture{})
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
