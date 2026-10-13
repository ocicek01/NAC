package sophosauth

import (
	"context"
	"time"
)

type Repository interface {
	InsertEvent(ctx context.Context, event Event) (Event, bool, error)
	ClaimEvent(ctx context.Context, fingerprint string, now time.Time, lease time.Duration, maxAttempts int) (Claim, bool, error)
	ClaimDueEvents(ctx context.Context, now time.Time, lease time.Duration, maxAttempts, limit int) ([]Claim, error)
	BeginEffects(ctx context.Context, claim Claim) (Effects, error)
	MarkFailed(ctx context.Context, claim Claim, message string, nextAttempt time.Time, maxAttempts int) error
	ListEvents(ctx context.Context, limit, offset int) ([]Event, error)
	ListSessions(ctx context.Context, limit, offset int) ([]Session, error)
	Health(ctx context.Context, enabled bool, bindAddress string, allowedSources int) (Health, error)
}

type Effects interface {
	ReserveCausalOrder(ctx context.Context, event Event) (bool, error)
	UpdateEventCorrelation(ctx context.Context, id, status, macAddress string, metadata map[string]any) error
	FindFreshMACsByIP(ctx context.Context, ipAddress string, since time.Time) ([]string, error)
	OpenSession(ctx context.Context, event Event) (Session, error)
	CloseSessions(ctx context.Context, username, ipAddress string, endedAt time.Time) ([]Session, error)
	SetDeviceIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error
	ClearDeviceIdentityIfMatch(ctx context.Context, macAddress, username, ipAddress string, logoutAt time.Time) error
	RecordAudit(ctx context.Context, action, status, eventID, macAddress string, payload map[string]any) error
	Complete(ctx context.Context) error
	Rollback(ctx context.Context) error
}
