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

	InsertRequest(ctx context.Context, request Request) (Request, error)
	InsertResult(ctx context.Context, result Result) (Result, error)
	ListRequests(ctx context.Context, limit, offset int) ([]Request, error)
	ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]Request, error)
	ListResultsByRequest(ctx context.Context, requestID string) ([]Result, error)
	FindRequestByID(ctx context.Context, id string) (*Request, error)
	FindActiveRequest(ctx context.Context, policyDecisionID, action string, targetVLAN int) (*Request, error)
	ClaimNextRequest(ctx context.Context, now time.Time) (*Request, error)
	MarkRequestStarted(ctx context.Context, id string, adapter string, startedAt time.Time) error
	MarkRequestCompleted(ctx context.Context, id, status, errorCode, errorMessage, verificationStatus string, completedAt, verifiedAt time.Time) error
	MarkRequestRetry(ctx context.Context, id, errorCode, errorMessage string, nextAttemptAt time.Time) error
	UpdateRequestPolicyBinding(ctx context.Context, id, policyDecisionID string) error
	UpdatePolicyDecisionEnforcement(ctx context.Context, policyDecisionID, requestID, status, errorMessage string, startedAt, completedAt, enforcedAt time.Time, requested bool) error
	UpdateDeviceEnforcementSnapshot(ctx context.Context, deviceID, action string, vlanID int, status, errorMessage string, observedAt time.Time) error
}
