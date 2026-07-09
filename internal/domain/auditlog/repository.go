package auditlog

import "context"

type Repository interface {
	Insert(ctx context.Context, log Log) (Log, error)
	ListRecent(ctx context.Context, limit int) ([]Log, error)
}
