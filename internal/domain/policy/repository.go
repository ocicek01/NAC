package policy

import (
	"context"
	"time"
)

type Repository interface {
	Insert(ctx context.Context, policy Policy) (Policy, error)
	Update(ctx context.Context, policy Policy) (Policy, error)
	FindByID(ctx context.Context, id string) (*Policy, error)
	List(ctx context.Context, limit, offset int) ([]Policy, error)
	ListActive(ctx context.Context) ([]Policy, error)
	Disable(ctx context.Context, id string) error
	InsertDecision(ctx context.Context, decision Decision) (Decision, error)
	FindDecisionByID(ctx context.Context, id string) (*Decision, error)
	ListDecisions(ctx context.Context, limit, offset int) ([]Decision, error)
	ListDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]Decision, error)
	UpdateDecisionEnforcement(ctx context.Context, decisionID, requestID, status, errorMessage string, startedAt, completedAt, enforcedAt time.Time, requested bool) error
	InsertTrustScoreResult(ctx context.Context, result TrustScoreResult) (TrustScoreResult, error)
}
