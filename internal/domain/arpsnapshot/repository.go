package arpsnapshot

import "context"

type Repository interface {
	UpsertBatch(ctx context.Context, snapshots []Snapshot) error
}
