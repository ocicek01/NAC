package device

import (
	"context"
	"time"

	devicedomain "nac/internal/domain/device"
	policydomain "nac/internal/domain/policy"
)

func (s *stubDeviceRepository) FindByID(ctx context.Context, id string) (*devicedomain.Device, error) {
	for _, item := range s.list {
		if item.ID == id {
			copyItem := item
			return &copyItem, nil
		}
	}
	for _, items := range s.byMAC {
		for _, item := range items {
			if item.ID == id {
				copyItem := item
				return &copyItem, nil
			}
		}
	}
	return nil, nil
}

func (s *stubDeviceRepository) UpdatePolicyEvaluationByID(ctx context.Context, deviceID, status, policyAction, policyReason, trustLevel, lastPolicyDecision string, evaluatedAt time.Time) error {
	return nil
}

func (s stubPolicyEvaluator) ListDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]policydomain.Decision, error) {
	return nil, nil
}
