package topology

import (
	"context"
	"time"
)

type Repository interface {
	Upsert(ctx context.Context, link Link) (Link, error)
	PruneDiscovered(ctx context.Context, sourceSwitchID string, methods []string, observedAt time.Time) error
	List(ctx context.Context) ([]Link, error)
	HasLinkedInterface(ctx context.Context, switchID, interfaceName string) (bool, error)
	FindLinkedSwitchID(ctx context.Context, switchID, interfaceName string) (string, error)
	CountLinkedSwitches(ctx context.Context, switchID, interfaceName string) (int, error)
}
