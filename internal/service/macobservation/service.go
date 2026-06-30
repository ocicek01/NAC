package macobservation

import (
	"context"

	domain "nac/internal/domain/macobservation"
)

type Repository interface {
	ListRecent(ctx context.Context, limit int) ([]domain.Observation, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) ListRecent(ctx context.Context, limit int) ([]domain.Observation, error) {
	return s.repository.ListRecent(ctx, limit)
}
