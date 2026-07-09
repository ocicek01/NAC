package device

import (
	"context"
	"time"
)

type Repository interface {
	Upsert(ctx context.Context, device Device) (Device, error)
	List(ctx context.Context, limit, offset int) ([]Device, error)
	ListEnrichmentBackfillCandidates(ctx context.Context, limit int) ([]Device, error)
	ListByMAC(ctx context.Context, macAddress string) ([]Device, error)
	ListBySwitch(ctx context.Context, switchID string) ([]Device, error)
	ListBySwitchAndIfIndex(ctx context.Context, switchID string, ifIndex int) ([]Device, error)
	UpdateStatus(ctx context.Context, macAddress, status, approvedBy, policyAction, policyReason string, approvedAt, expiresAt time.Time) (Device, error)
	AddIdentitySnapshot(ctx context.Context, snapshot IdentitySnapshot) (IdentitySnapshot, error)
	AddObservation(ctx context.Context, observation Observation) (Observation, error)
	UpdateEnrichment(ctx context.Context, update EnrichmentUpdate) (Device, error)
	UpdateEnrichmentStatusByID(ctx context.Context, deviceID, source, status, enrichmentError string, enrichedAt time.Time) error
	UpdateSophosIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error
	UpdateEnforcementState(ctx context.Context, macAddress, action string, vlanID int, status, switchID string, ifIndex int, method string, enforcedAt time.Time) error
	UpdateIPLearningState(ctx context.Context, macAddress, switchID string, ifIndex int, state string, startedAt, learnedAt, lastBounceAt time.Time) error
}
