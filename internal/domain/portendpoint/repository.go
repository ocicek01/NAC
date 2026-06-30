package portendpoint

import "context"

type Repository interface {
	ListBySwitch(ctx context.Context, switchID string) ([]Endpoint, error)
}
