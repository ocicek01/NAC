package portevent

import "context"

type Repository interface {
	Insert(ctx context.Context, event Event) (Event, error)
	ListRecent(ctx context.Context, limit int) ([]Event, error)
}
