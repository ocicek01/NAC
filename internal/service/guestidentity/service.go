package guestidentity

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/guestidentity"
)

type Service struct {
	repository domain.Repository
}

func NewService(repository domain.Repository) *Service {
	return &Service{repository: repository}
}

type CreateInput struct {
	ExternalID string
	Username   string
	FullName   string
	Email      string
	Phone      string
	Status     string
	TargetVLAN int
	ExpiresAt  time.Time
}

func (s *Service) List(ctx context.Context) ([]domain.Identity, error) {
	return s.repository.List(ctx)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domain.Identity, error) {
	now := time.Now().UTC()
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "active"
	}
	return s.repository.Insert(ctx, domain.Identity{
		ID:         uuid.NewString(),
		ExternalID: strings.TrimSpace(input.ExternalID),
		Username:   strings.TrimSpace(input.Username),
		FullName:   strings.TrimSpace(input.FullName),
		Email:      strings.TrimSpace(input.Email),
		Phone:      strings.TrimSpace(input.Phone),
		Status:     status,
		TargetVLAN: input.TargetVLAN,
		ExpiresAt:  input.ExpiresAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
}

func (s *Service) FindActiveByIdentifier(ctx context.Context, identifier string) (*domain.Identity, error) {
	return s.repository.FindActiveByIdentifier(ctx, identifier)
}
