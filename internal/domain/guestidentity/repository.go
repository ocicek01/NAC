package guestidentity

import "context"

type Repository interface {
	List(ctx context.Context) ([]Identity, error)
	Insert(ctx context.Context, identity Identity) (Identity, error)
	FindActiveByIdentifier(ctx context.Context, identifier string) (*Identity, error)
}
