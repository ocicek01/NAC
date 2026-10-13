package radiusevent

import (
	"context"
	"time"
)

type Repository interface {
	Insert(ctx context.Context, event Event) (Event, error)
	ClaimAccounting(ctx context.Context, event Event, claimedAt, staleBefore time.Time) (ClaimResult, error)
	CompleteAccounting(ctx context.Context, eventID string, claimedAt, completedAt time.Time) error
	FailAccounting(ctx context.Context, eventID, processingError string, claimedAt, failedAt time.Time) error
}
