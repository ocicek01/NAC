package sophosauth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"nac/internal/config"
	domain "nac/internal/domain/sophosauth"
	"nac/internal/normalize"
)

type auditRecorder interface {
	Record(ctx context.Context, action, status, targetType, targetID, switchID, macAddress string, payload map[string]any) error
}

type Service struct {
	cfg        config.SophosAuthConfig
	repository domain.Repository
	audit      auditRecorder
	now        func() time.Time
}

func NewService(cfg config.SophosAuthConfig, repository domain.Repository, audit auditRecorder) *Service {
	return &Service{cfg: cfg, repository: repository, audit: audit, now: time.Now}
}

func (s *Service) Process(ctx context.Context, parsed domain.ParsedEvent) error {
	if !s.cfg.Enabled {
		return nil
	}
	now := s.now().UTC()
	if parsed.EventAt.IsZero() {
		parsed.EventAt = now
	}
	event := domain.Event{
		ID:                uuid.NewString(),
		Fingerprint:       parsed.Fingerprint,
		EventType:         parsed.EventType,
		MessageID:         strings.TrimSpace(parsed.MessageID),
		Username:          strings.TrimSpace(parsed.Username),
		UserGroup:         strings.TrimSpace(parsed.UserGroup),
		ClientUsed:        strings.TrimSpace(parsed.ClientUsed),
		AuthMechanism:     strings.TrimSpace(parsed.AuthMechanism),
		AuthStatus:        strings.TrimSpace(parsed.AuthStatus),
		SourceIP:          strings.TrimSpace(parsed.SourceIP),
		SourceMAC:         normalize.MACAddress(parsed.SourceMAC),
		CorrelationStatus: "pending",
		Metadata:          selectedMetadata(parsed),
		EventAt:           parsed.EventAt.UTC(),
		ReceivedAt:        now,
	}
	if event.Fingerprint == "" {
		return errors.New("Sophos auth event fingerprint is required")
	}
	if !s.allowedClient(event.ClientUsed) || excludedClient(event.ClientUsed) || event.EventType == "ignored" || event.Username == "" || net.ParseIP(event.SourceIP) == nil {
		event.EventType = "ignored"
		event.CorrelationStatus = "ignored"
		_, _, err := s.repository.InsertEvent(ctx, event)
		return err
	}
	stored, inserted, err := s.repository.InsertEvent(ctx, event)
	if err != nil || !inserted {
		return err
	}
	if event.EventType == "logout" {
		return s.processLogout(ctx, stored)
	}
	return s.processLogin(ctx, stored)
}

func (s *Service) processLogin(ctx context.Context, event domain.Event) error {
	macs, err := s.repository.FindFreshMACsByIP(ctx, event.SourceIP, event.EventAt.Add(-time.Duration(s.cfg.BindingFreshnessSec)*time.Second))
	if err != nil {
		return err
	}
	correlation, mac, detail := correlate(event.SourceMAC, macs)
	event.CorrelationStatus, event.CorrelatedMAC = correlation, mac
	if err := s.repository.UpdateEventCorrelation(ctx, event.ID, correlation, mac, detail); err != nil {
		return err
	}
	session, err := s.repository.OpenSession(ctx, event)
	if err != nil {
		return err
	}
	payload := map[string]any{"username": event.Username, "ip_address": event.SourceIP, "client_used": event.ClientUsed, "correlation_status": correlation}
	for key, value := range detail {
		payload[key] = value
	}
	switch correlation {
	case "correlated":
		appliesCurrentIdentity := !session.StartedAt.After(event.EventAt) && normalize.MACAddress(session.MACAddress) == mac
		payload["applied_current_identity"] = appliesCurrentIdentity
		if appliesCurrentIdentity {
			if err := s.repository.SetDeviceIdentity(ctx, mac, event.Username, event.SourceIP, event.EventAt); err != nil {
				return err
			}
		}
		return s.record(ctx, "sophos_auth_login_correlated", "success", event.ID, mac, payload)
	case "ambiguous":
		return s.record(ctx, "sophos_auth_ambiguous", "warning", event.ID, event.SourceMAC, payload)
	default:
		return s.record(ctx, "sophos_auth_uncorrelated", "warning", event.ID, event.SourceMAC, payload)
	}
}

func (s *Service) processLogout(ctx context.Context, event domain.Event) error {
	sessions, err := s.repository.CloseSessions(ctx, event.Username, event.SourceIP, event.EventAt)
	if err != nil {
		return err
	}
	correlation, mac := "uncorrelated", ""
	for _, session := range sessions {
		if session.CorrelationStatus != "correlated" || normalize.MACAddress(session.MACAddress) == "" {
			continue
		}
		correlation, mac = "correlated", normalize.MACAddress(session.MACAddress)
		if err := s.repository.ClearDeviceIdentityIfMatch(ctx, mac, event.Username, event.SourceIP, event.EventAt); err != nil {
			return err
		}
	}
	if err := s.repository.UpdateEventCorrelation(ctx, event.ID, correlation, mac, map[string]any{"closed_session_count": len(sessions)}); err != nil {
		return err
	}
	if err := s.record(ctx, "sophos_auth_logout", "success", event.ID, mac, map[string]any{"username": event.Username, "ip_address": event.SourceIP, "closed_session_count": len(sessions), "correlation_status": correlation}); err != nil {
		return err
	}
	if correlation == "uncorrelated" {
		return s.record(ctx, "sophos_auth_uncorrelated", "warning", event.ID, "", map[string]any{"reason": "active_session_not_found", "username": event.Username, "ip_address": event.SourceIP})
	}
	return nil
}

func correlate(sourceMAC string, bindings []string) (string, string, map[string]any) {
	sourceMAC = normalize.MACAddress(sourceMAC)
	unique := make(map[string]struct{}, len(bindings))
	for _, candidate := range bindings {
		if normalized := normalize.MACAddress(candidate); normalized != "" {
			unique[normalized] = struct{}{}
		}
	}
	candidates := make([]string, 0, len(unique))
	for candidate := range unique {
		candidates = append(candidates, candidate)
	}
	detail := map[string]any{"binding_count": len(candidates), "source_mac": sourceMAC, "binding_macs": candidates}
	if len(candidates) > 1 {
		detail["reason"] = "multiple_fresh_bindings"
		return "ambiguous", "", detail
	}
	if len(candidates) == 1 {
		if sourceMAC != "" && sourceMAC != candidates[0] {
			detail["reason"] = "source_mac_binding_conflict"
			return "ambiguous", "", detail
		}
		return "correlated", candidates[0], detail
	}
	if sourceMAC != "" {
		detail["reason"] = "source_mac_only"
		return "correlated", sourceMAC, detail
	}
	detail["reason"] = "fresh_binding_not_found"
	return "uncorrelated", "", detail
}

func (s *Service) ListEvents(ctx context.Context, limit, offset int) ([]domain.Event, error) {
	return s.repository.ListEvents(ctx, limit, offset)
}

func (s *Service) ListSessions(ctx context.Context, limit, offset int) ([]domain.Session, error) {
	return s.repository.ListSessions(ctx, limit, offset)
}

func (s *Service) Health(ctx context.Context) (domain.Health, error) {
	return s.repository.Health(ctx, s.cfg.Enabled, fmt.Sprintf("%s:%d", s.cfg.BindHost, s.cfg.Port), len(s.cfg.AllowedSources))
}

func (s *Service) RecordRejectedSource(ctx context.Context, sourceIP string) {
	_ = s.record(ctx, "sophos_auth_rejected_source", "warning", "", "", map[string]any{"source_ip": sourceIP})
}

func (s *Service) RecordParseFailure(ctx context.Context, sourceIP string, err error) {
	_ = s.record(ctx, "sophos_auth_parse_failed", "warning", "", "", map[string]any{"source_ip": sourceIP, "error": err.Error()})
}

func (s *Service) record(ctx context.Context, action, status, targetID, mac string, payload map[string]any) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, action, status, "sophos_captive_session", targetID, "", mac, payload)
}

func (s *Service) allowedClient(client string) bool {
	normalized := compact(client)
	for _, allowed := range s.cfg.AllowedClients {
		candidate := compact(allowed)
		if candidate != "" && (normalized == candidate || strings.Contains(normalized, candidate)) {
			return true
		}
	}
	return false
}

func excludedClient(client string) bool {
	value := compact(client)
	return strings.Contains(value, "radius") || strings.Contains(value, "wifisso") || strings.Contains(value, "wirelesssso")
}

func compact(value string) string {
	var out strings.Builder
	for _, char := range strings.ToLower(value) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			out.WriteRune(char)
		}
	}
	return out.String()
}

func selectedMetadata(parsed domain.ParsedEvent) map[string]any {
	out := make(map[string]any, len(parsed.Metadata))
	for key, value := range parsed.Metadata {
		out[key] = value
	}
	return out
}
