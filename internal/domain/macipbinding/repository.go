package macipbinding

import "context"

type Repository interface {
	UpsertBatch(ctx context.Context, bindings []Binding) error
	FindLatestByMACSwitch(ctx context.Context, macAddress, switchID string) (*Binding, error)
}
