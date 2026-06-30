package session

import (
	"context"
	"strings"

	domain "nac/internal/domain/session"
)

type Service struct {
	repository domain.Repository
}

func NewService(repository domain.Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) ListRecent(ctx context.Context, limit int) ([]domain.Session, error) {
	if s == nil || s.repository == nil {
		return nil, nil
	}
	return s.repository.ListRecent(ctx, limit)
}

func (s *Service) ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]domain.Session, error) {
	if s == nil || s.repository == nil {
		return nil, nil
	}
	return s.repository.ListRecentByMAC(ctx, strings.TrimSpace(macAddress), limit)
}
