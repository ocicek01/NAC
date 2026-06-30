package policy

import "context"

type Repository interface {
	Insert(ctx context.Context, policy Policy) (Policy, error)
	ListActive(ctx context.Context) ([]Policy, error)
	Disable(ctx context.Context, id string) error
}
