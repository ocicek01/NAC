package auditlog

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/auditlog"
)

type Service struct {
	repository domain.Repository
}

func NewService(repository domain.Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Record(ctx context.Context, action, status, targetType, targetID, switchID, macAddress string, payload map[string]any) error {
	if s == nil || s.repository == nil {
		return nil
	}
	_, err := s.repository.Insert(ctx, domain.Log{
		ID:         uuid.NewString(),
		Actor:      "system",
		Action:     strings.TrimSpace(action),
		Status:     strings.TrimSpace(status),
		TargetType: strings.TrimSpace(targetType),
		TargetID:   strings.TrimSpace(targetID),
		SwitchID:   strings.TrimSpace(switchID),
		MACAddress: strings.ToUpper(strings.TrimSpace(macAddress)),
		Payload:    payload,
		CreatedAt:  time.Now().UTC(),
	})
	return err
}

func (s *Service) ListRecent(ctx context.Context, limit int) ([]domain.Log, error) {
	if s == nil || s.repository == nil {
		return []domain.Log{}, nil
	}
	return s.repository.ListRecent(ctx, limit)
}
