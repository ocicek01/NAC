package dhcpevent

import (
	"context"
	"time"
)

type Repository interface {
	FindRecentByTransaction(ctx context.Context, macAddress, messageType, transactionID string, since time.Time) (*Event, error)
	FindRecent(ctx context.Context, macAddress, messageType string, since time.Time) (*Event, error)
	Insert(ctx context.Context, event Event) (Event, error)
	UpdateByID(ctx context.Context, id string, event Event) (Event, error)
}
