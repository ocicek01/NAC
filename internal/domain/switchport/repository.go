package switchport

import (
	"context"
	"time"
)

type Repository interface {
	ReplaceBySwitch(ctx context.Context, switchID string, ports []Port) error
	ListBySwitch(ctx context.Context, switchID string) ([]Port, error)
	FindBySwitchIfIndex(ctx context.Context, switchID string, ifIndex int) (*Port, error)
	UpdateStatus(ctx context.Context, switchID string, ifIndex int, interfaceName, interfaceDescription, adminStatus, operStatus string, observedAt time.Time) (*Port, error)
}
