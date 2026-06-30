package trapwindow

import (
	"context"
	"time"
)

type Repository interface {
	UpsertPending(ctx context.Context, window Window) (Window, error)
	ListDue(ctx context.Context, now time.Time, limit int) ([]Window, error)
	MarkDispatched(ctx context.Context, id string, dispatchedAt time.Time) error
	PruneDispatchedOlderThan(ctx context.Context, cutoff time.Time) error
}
