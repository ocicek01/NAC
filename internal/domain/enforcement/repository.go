package enforcement

import (
	"context"
	"time"
)

type Repository interface {
	Insert(ctx context.Context, decision Decision) (Decision, error)
	ListRecent(ctx context.Context, limit int) ([]Decision, error)
	ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]Decision, error)
	FindLatestByKey(ctx context.Context, macAddress, switchID, policyAction string, ifIndex int, interfaceName string) (*Decision, error)
	AcquireState(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName string, lockedUntil time.Time) (bool, error)
	MarkStateExecuted(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string) error
	MarkStateFailed(ctx context.Context, macAddress, switchID, policyAction string, ifIndex, targetVLAN int, interfaceName, decisionID, method string, lockedUntil time.Time) error
	ClearStateForMAC(ctx context.Context, macAddress string) error
	FindByID(ctx context.Context, id string) (*Decision, error)
	Approve(ctx context.Context, id string) error
	Reject(ctx context.Context, id string) error
	Retry(ctx context.Context, id string) error
	MarkExecuted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, lastError string) error
}
