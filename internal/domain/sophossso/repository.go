package sophossso

import (
	"context"
	"time"
)

type Repository interface {
	Enqueue(ctx context.Context, item Item) (Item, bool, error)
	ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]Item, error)
	MarkSent(ctx context.Context, id string, leaseUntil, sentAt time.Time) error
	MarkRetry(ctx context.Context, id string, leaseUntil time.Time, lastError string, nextAttemptAt time.Time, deadLetter bool) error
	List(ctx context.Context, limit, offset int) ([]Item, error)
	Health(ctx context.Context, enabled bool) (Health, error)
}
