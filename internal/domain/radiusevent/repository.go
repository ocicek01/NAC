package radiusevent

import "context"

type Repository interface {
	Insert(ctx context.Context, event Event) (Event, error)
}
