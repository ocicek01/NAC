package trapwindow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	discoveryjobdomain "nac/internal/domain/discoveryjob"
	domain "nac/internal/domain/trapwindow"
)

type Repository interface {
	UpsertPending(ctx context.Context, window domain.Window) (domain.Window, error)
	ListDue(ctx context.Context, now time.Time, limit int) ([]domain.Window, error)
	MarkDispatched(ctx context.Context, id string, dispatchedAt time.Time) error
	PruneDispatchedOlderThan(ctx context.Context, cutoff time.Time) error
}

type JobRepository interface {
	Insert(ctx context.Context, job discoveryjobdomain.Job) (discoveryjobdomain.Job, error)
	ListBySwitch(ctx context.Context, switchID string, limit int) ([]discoveryjobdomain.Job, error)
}

type Service struct {
	logger *slog.Logger
	repo   Repository
	jobs   JobRepository
}

func NewService(logger *slog.Logger, repo Repository, jobs JobRepository) *Service {
	return &Service{logger: logger, repo: repo, jobs: jobs}
}

func (s *Service) Record(ctx context.Context, window domain.Window) (domain.Window, error) {
	if s.repo == nil {
		return domain.Window{}, fmt.Errorf("trap window repository is not configured")
	}
	if strings.TrimSpace(window.ID) == "" {
		window.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if window.EventCount <= 0 {
		window.EventCount = 1
	}
	if window.Status == "" {
		window.Status = "pending"
	}
	if window.FirstSeenAt.IsZero() {
		window.FirstSeenAt = now
	}
	if window.LastSeenAt.IsZero() {
		window.LastSeenAt = now
	}
	if window.AvailableAt.IsZero() {
		window.AvailableAt = now
	}
	if window.CreatedAt.IsZero() {
		window.CreatedAt = now
	}
	window.UpdatedAt = now
	return s.repo.UpsertPending(ctx, window)
}

func (s *Service) ProcessDue(ctx context.Context, limit int) error {
	if s.repo == nil || s.jobs == nil {
		return nil
	}
	now := time.Now().UTC()
	items, err := s.repo.ListDue(ctx, now, limit)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := s.enqueueWindowJob(ctx, item, now); err != nil {
			return err
		}
		if err := s.repo.MarkDispatched(ctx, item.ID, now); err != nil {
			return err
		}
	}
	return s.repo.PruneDispatchedOlderThan(ctx, now.Add(-24*time.Hour))
}

func (s *Service) enqueueWindowJob(ctx context.Context, item domain.Window, now time.Time) error {
	recent, err := s.jobs.ListBySwitch(ctx, item.SwitchID, 10)
	if err != nil {
		return err
	}
	for _, job := range recent {
		if job.Scope != item.Scope || job.RequestedSource != "snmp-trap-window" {
			continue
		}
		if job.Status == "queued" || job.Status == "running" {
			return nil
		}
		if now.Sub(job.CreatedAt) < 30*time.Second {
			return nil
		}
	}

	job := discoveryjobdomain.Job{
		ID:              uuid.NewString(),
		SwitchID:        item.SwitchID,
		Scope:           item.Scope,
		Status:          "queued",
		RequestedSource: "snmp-trap-window",
		RequestedBy:     item.Category,
		CurrentStep:     "queued",
		MaxAttempts:     3,
		Summary: map[string]any{
			"category":       item.Category,
			"if_index":       item.PortIfIndex,
			"mac_address":    item.MACAddress,
			"vlan_id":        item.VLANID,
			"event_count":    item.EventCount,
			"trap_oid":       item.TrapOID,
			"enterprise_oid": item.EnterpriseOID,
			"source_ip":      item.SourceIP,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.jobs.Insert(ctx, job); err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Info("trap window queued discovery job", "switch_id", item.SwitchID, "scope", item.Scope, "category", item.Category, "event_count", item.EventCount)
	}
	return nil
}
