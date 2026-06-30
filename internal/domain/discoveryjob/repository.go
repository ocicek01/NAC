package discoveryjob

import "context"

type Repository interface {
	Insert(ctx context.Context, job Job) (Job, error)
	FindByID(ctx context.Context, id string) (*Job, error)
	Update(ctx context.Context, job Job) (Job, error)
	ListBySwitch(ctx context.Context, switchID string, limit int) ([]Job, error)
	ClaimNextQueued(ctx context.Context, workerID string) (*Job, error)
	ClaimQueuedByID(ctx context.Context, id, workerID string) (*Job, error)
}
