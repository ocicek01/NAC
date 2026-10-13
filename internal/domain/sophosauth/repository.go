package sophosauth

import (
	"context"
	"time"
)

type Repository interface {
	InsertEvent(ctx context.Context, event Event) (Event, bool, error)
	UpdateEventCorrelation(ctx context.Context, id, status, macAddress string, metadata map[string]any) error
	FindFreshMACsByIP(ctx context.Context, ipAddress string, since time.Time) ([]string, error)
	OpenSession(ctx context.Context, event Event) (Session, error)
	CloseSessions(ctx context.Context, username, ipAddress string, endedAt time.Time) ([]Session, error)
	SetDeviceIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error
	ClearDeviceIdentityIfMatch(ctx context.Context, macAddress, username, ipAddress string, logoutAt time.Time) error
	ListEvents(ctx context.Context, limit, offset int) ([]Event, error)
	ListSessions(ctx context.Context, limit, offset int) ([]Session, error)
	Health(ctx context.Context, enabled bool, bindAddress string, allowedSources int) (Health, error)
}
