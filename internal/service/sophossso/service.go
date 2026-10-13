package sophossso

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"nac/internal/config"
	domain "nac/internal/domain/sophossso"
	"nac/internal/normalize"
)

type auditRecorder interface {
	Record(ctx context.Context, action, status, targetType, targetID, switchID, macAddress string, payload map[string]any) error
}

type Service struct {
	cfg        config.SophosSSOConfig
	repository domain.Repository
	audit      auditRecorder
	client     UDPClient
	now        func() time.Time
}

func NewService(cfg config.SophosSSOConfig, repository domain.Repository, audit auditRecorder) *Service {
	return &Service{cfg: cfg, repository: repository, audit: audit, client: netUDPClient{}, now: time.Now}
}

func (s *Service) EnqueueAccounting(ctx context.Context, input domain.AccountingInput) error {
	if !s.cfg.Enabled {
		return nil
	}
	reason := eligibilityFailure(input)
	if reason != "" {
		s.record(ctx, "sophos_sso_skipped", "warning", input.SourceEventID, input.MACAddress, map[string]any{"reason": reason, "acct_status_type": input.AcctStatusType})
		return nil
	}
	now := s.now().UTC()
	item := domain.Item{ID: uuid.NewString(), SourceKey: strings.TrimSpace(input.SourceEventID), Username: strings.TrimSpace(input.Username), IPAddress: strings.TrimSpace(input.IPAddress), MACAddress: strings.ToUpper(strings.TrimSpace(input.MACAddress)), AcctStatusType: input.AcctStatusType, AcctSessionID: strings.TrimSpace(input.AcctSessionID), EventTimestamp: input.EventTimestamp.UTC(), AcctDelayTime: input.AcctDelayTime, Status: "pending", NextAttemptAt: now, CreatedAt: now, UpdatedAt: now}
	stored, inserted, err := s.repository.Enqueue(ctx, item)
	if err != nil {
		s.record(ctx, "sophos_sso_queue_failed", "error", item.SourceKey, item.MACAddress, map[string]any{"error": err.Error()})
		return err
	}
	if inserted {
		s.record(ctx, "sophos_sso_queued", "success", stored.ID, stored.MACAddress, map[string]any{"acct_status_type": stored.AcctStatusType, "acct_session_id": stored.AcctSessionID, "username": stored.Username, "ip_address": stored.IPAddress})
	}
	return nil
}

func eligibilityFailure(input domain.AccountingInput) string {
	status := strings.TrimSpace(input.AcctStatusType)
	if status != "Start" && status != "Interim-Update" && status != "Stop" {
		return "unsupported_status"
	}
	username := strings.TrimSpace(input.Username)
	if username == "" {
		return "username_missing"
	}
	if normalize.MACAddress(username) != "" {
		return "mab_identity"
	}
	if normalize.MACAddress(input.MACAddress) != "" && normalize.MACAddress(username) == normalize.MACAddress(input.MACAddress) {
		return "mab_identity"
	}
	if net.ParseIP(strings.TrimSpace(input.IPAddress)) == nil {
		return "framed_ip_missing_or_invalid"
	}
	if strings.TrimSpace(input.AcctSessionID) == "" {
		return "acct_session_id_missing"
	}
	if strings.TrimSpace(input.SourceEventID) == "" {
		return "source_event_id_missing"
	}
	return ""
}

func (s *Service) ProcessDue(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}
	now := s.now().UTC()
	items, err := s.repository.ClaimDue(ctx, now, now.Add(time.Duration(s.cfg.ClaimLeaseSeconds)*time.Second), s.cfg.WorkerBatchSize)
	if err != nil {
		return err
	}
	var result error
	for _, item := range items {
		if err := s.send(ctx, item); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (s *Service) send(ctx context.Context, item domain.Item) error {
	if item.LeaseUntil == nil {
		return errors.New("Sophos SSO claimed item has no lease token")
	}
	now := s.now().UTC()
	effectiveItem := item
	if delay := now.Sub(item.CreatedAt); delay > 0 {
		effectiveItem.AcctDelayTime += int(delay / time.Second)
	}
	packet, identifier, authenticator, err := buildAccountingPacket(effectiveItem, s.cfg.NASIdentifier, s.cfg.Secret)
	if err == nil {
		response, exchangeErr := s.client.Exchange(net.JoinHostPort(s.cfg.Host, fmt.Sprint(s.cfg.Port)), packet, time.Duration(s.cfg.TimeoutSeconds)*time.Second)
		if exchangeErr != nil {
			err = exchangeErr
		} else {
			err = validateAccountingResponse(response, identifier, authenticator, s.cfg.Secret)
		}
	}
	if err == nil {
		if markErr := s.repository.MarkSent(ctx, item.ID, *item.LeaseUntil, now); markErr != nil {
			return markErr
		}
		s.record(ctx, "sophos_sso_sent", "success", item.ID, item.MACAddress, map[string]any{"acct_status_type": item.AcctStatusType, "attempt_count": item.AttemptCount, "username": item.Username, "ip_address": item.IPAddress})
		return nil
	}
	dead := item.AttemptCount >= s.cfg.MaxRetries
	backoff := time.Duration(s.cfg.RetryBaseSeconds) * time.Second
	for i := 1; i < item.AttemptCount && i < 10; i++ {
		backoff *= 2
	}
	if markErr := s.repository.MarkRetry(ctx, item.ID, *item.LeaseUntil, err.Error(), now.Add(backoff), dead); markErr != nil {
		return errors.Join(err, markErr)
	}
	action, status := "sophos_sso_retry_scheduled", "warning"
	if dead {
		action, status = "sophos_sso_dead_letter", "error"
	}
	s.record(ctx, action, status, item.ID, item.MACAddress, map[string]any{"error": err.Error(), "attempt_count": item.AttemptCount, "next_attempt_at": now.Add(backoff)})
	return err
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]domain.Item, error) {
	return s.repository.List(ctx, limit, offset)
}

func (s *Service) Health(ctx context.Context) (domain.Health, error) {
	return s.repository.Health(ctx, s.cfg.Enabled)
}

func (s *Service) Enabled() bool { return s.cfg.Enabled }
func (s *Service) WorkerInterval() time.Duration {
	return time.Duration(s.cfg.WorkerIntervalSeconds) * time.Second
}

func (s *Service) record(ctx context.Context, action, status, targetID, mac string, payload map[string]any) {
	if s.audit != nil {
		_ = s.audit.Record(ctx, action, status, "sophos_radius_sso", targetID, "", mac, payload)
	}
}
