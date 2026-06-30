package switchport

import "context"

type Repository interface {
	ReplaceBySwitch(ctx context.Context, switchID string, ports []Port) error
	ListBySwitch(ctx context.Context, switchID string) ([]Port, error)
	FindBySwitchIfIndex(ctx context.Context, switchID string, ifIndex int) (*Port, error)
}
