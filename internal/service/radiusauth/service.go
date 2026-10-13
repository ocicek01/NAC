package radiusauth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"nac/internal/config"
	macobservation "nac/internal/domain/macobservation"
	domain "nac/internal/domain/radiusauth"
	radiusevent "nac/internal/domain/radiusevent"
	sessiondomain "nac/internal/domain/session"
	sophosssodomain "nac/internal/domain/sophossso"
	switchasset "nac/internal/domain/switchasset"
	"nac/internal/normalize"
	deviceservice "nac/internal/service/device"
	policyservice "nac/internal/service/policy"
)

const (
	sourceTypeRadius        = "radius"
	confidenceAuthoritative = "authoritative"
	accountingClaimLease    = 2 * time.Minute
)

type PolicyEvaluator interface {
	EnsureDefaults(ctx context.Context) error
	Evaluate(ctx context.Context, input policyservice.EvaluationInput) (policyservice.EvaluationResult, error)
}

type ObservationRecorder interface {
	Insert(ctx context.Context, observation macobservation.Observation) (macobservation.Observation, error)
}

type SwitchResolver interface {
	FindByManagementIP(ctx context.Context, managementIP string) (*switchasset.Switch, error)
	FindByName(ctx context.Context, name string) (*switchasset.Switch, error)
}

type DeviceInventoryUpdater interface {
	UpsertFromRadius(ctx context.Context, input deviceservice.RadiusInventoryInput) error
}

type SessionRecorder interface {
	Upsert(ctx context.Context, session sessiondomain.Session) (sessiondomain.Session, error)
	FindByAcctSession(ctx context.Context, macAddress, switchID, acctSessionID string) (*sessiondomain.Session, error)
	PromoteToAccountingKey(ctx context.Context, oldKey, newKey, acctSessionID string) error
}

type SophosAccountingEnqueuer interface {
	EnqueueAccounting(ctx context.Context, input sophosssodomain.AccountingInput) error
}

type Service struct {
	policies     PolicyEvaluator
	config       config.RadiusConfig
	events       radiusevent.Repository
	observations ObservationRecorder
	switches     SwitchResolver
	devices      DeviceInventoryUpdater
	sessions     SessionRecorder
	sophosSSO    SophosAccountingEnqueuer
	now          func() time.Time
}

func (s *Service) ConfigureSophosSSO(enqueuer SophosAccountingEnqueuer) {
	s.sophosSSO = enqueuer
}

func NewService(policies PolicyEvaluator, cfg config.RadiusConfig, events radiusevent.Repository, observations ObservationRecorder, switches SwitchResolver, devices DeviceInventoryUpdater, sessions SessionRecorder) *Service {
	return &Service{
		policies:     policies,
		config:       cfg,
		events:       events,
		observations: observations,
		switches:     switches,
		devices:      devices,
		sessions:     sessions,
		now:          time.Now,
	}
}

func (s *Service) Authorize(ctx context.Context, req domain.AuthorizeRequest) (domain.AuthorizeResponse, error) {
	req.MACAddress = normalizeMACAddress(req.MACAddress, req.CallingStationID, req.Username)

	if s.policies != nil {
		_ = s.policies.EnsureDefaults(ctx)
	}

	result := policyservice.EvaluationResult{
		Status: "unknown",
		Action: "unknown",
		Reason: "No policy matched",
	}
	if s.policies != nil {
		evaluated, err := s.policies.Evaluate(ctx, policyservice.EvaluationInput{
			MACAddress:  strings.TrimSpace(req.MACAddress),
			Hostname:    strings.TrimSpace(req.Hostname),
			VendorClass: strings.TrimSpace(req.VendorClass),
			SwitchName:  strings.TrimSpace(req.NASIdentifier),
			Interface:   strings.TrimSpace(req.NASPortID),
		})
		if err != nil {
			return domain.AuthorizeResponse{}, err
		}
		result = evaluated
	}

	// MAB/RADIUS authorize akisinda hostname cogu zaman bos gelir.
	// Default "Hostname Missing -> observed" kurali quarantine yolunu
	// engellemesin; bu durumda sonucu unknown'a dusur.
	if strings.TrimSpace(req.Hostname) == "" && strings.EqualFold(strings.TrimSpace(result.Action), "observed") {
		result = policyservice.EvaluationResult{
			Status: "unknown",
			Action: "unknown",
			Reason: "RADIUS MAB fallback for missing hostname",
		}
	}

	resp := domain.AuthorizeResponse{
		PolicyAction:      strings.TrimSpace(result.Action),
		PolicyReason:      strings.TrimSpace(result.Reason),
		ReplyAttributes:   map[string]string{},
		ControlAttributes: map[string]string{},
	}

	switch strings.ToLower(strings.TrimSpace(result.Action)) {
	case "blocked":
		resp.Decision = "reject"
		resp.ReplyMessage = "Blocked by NAC policy"
	case "guest":
		resp.Decision = "accept"
		resp.ReplyMessage = "Guest access granted"
		resp.VLANID = strings.TrimSpace(s.config.GuestVLAN)
	case "unknown":
		if strings.TrimSpace(s.config.QuarantineVLAN) != "" {
			resp.Decision = "accept"
			resp.ReplyMessage = "Quarantine access granted"
			resp.VLANID = strings.TrimSpace(s.config.QuarantineVLAN)
		} else {
			resp.Decision = "reject"
			resp.ReplyMessage = "Unknown device rejected"
		}
	case "observed", "active":
		resp.Decision = "accept"
		resp.ReplyMessage = "Access granted"
	default:
		resp.Decision = "accept"
		resp.ReplyMessage = "Access granted"
	}

	if resp.ReplyMessage != "" {
		resp.ReplyAttributes["Reply-Message"] = resp.ReplyMessage
	}
	if resp.VLANID != "" {
		resp.ReplyAttributes["Tunnel-Type"] = "VLAN"
		resp.ReplyAttributes["Tunnel-Medium-Type"] = "IEEE-802"
		resp.ReplyAttributes["Tunnel-Private-Group-Id"] = resp.VLANID
	}
	if resp.Decision == "reject" {
		resp.ControlAttributes["Auth-Type"] = "Reject"
	}

	s.recordAuthorizeEvent(ctx, req, resp)
	s.syncAuthorizeInventory(ctx, req, resp)
	s.syncAuthorizeSession(ctx, req, resp)

	return resp, nil
}

func (s *Service) Accounting(ctx context.Context, req domain.AccountingRequest) (domain.AccountingResponse, error) {
	req.MACAddress = normalizeMACAddress(req.MACAddress, req.CallingStationID, req.Username)
	status, err := validateAccountingRequest(req)
	if err != nil {
		return domain.AccountingResponse{}, err
	}
	req.AcctStatusType = status
	eventAt, err := accountingEventTime(req, s.now().UTC())
	if err != nil {
		return domain.AccountingResponse{}, err
	}

	event, err := buildAccountingEvent(req, eventAt)
	if err != nil {
		return domain.AccountingResponse{}, err
	}
	if s.events == nil {
		return domain.AccountingResponse{}, errors.New("radius accounting event repository is not configured")
	}
	claimedAt := s.now().UTC()
	claim, err := s.events.ClaimAccounting(ctx, event, claimedAt, claimedAt.Add(-accountingClaimLease))
	if err != nil {
		return domain.AccountingResponse{}, fmt.Errorf("claim radius accounting event: %w", err)
	}
	switch claim {
	case radiusevent.Duplicate:
		return domain.AccountingResponse{Status: "ok"}, nil
	case radiusevent.Busy:
		return domain.AccountingResponse{}, domain.ErrAccountingInProgress
	case radiusevent.Claimed:
	default:
		return domain.AccountingResponse{}, fmt.Errorf("unknown radius accounting claim result %q", claim)
	}

	var effectiveSession *sessiondomain.Session
	if status != "Accounting-On" && status != "Accounting-Off" {
		storedSession, applied, err := s.syncAccountingSession(ctx, req, eventAt)
		if err != nil {
			return domain.AccountingResponse{}, s.failAccounting(ctx, event.ID, claimedAt, err)
		}
		effectiveSession = storedSession
		if applied {
			if err := s.syncAccountingInventory(ctx, req, eventAt); err != nil {
				return domain.AccountingResponse{}, s.failAccounting(ctx, event.ID, claimedAt, err)
			}
		}
	}
	if s.sophosSSO != nil && status != "Accounting-On" && status != "Accounting-Off" {
		effectiveIP := strings.TrimSpace(req.FramedIPAddress)
		if effectiveIP == "" && effectiveSession != nil {
			effectiveIP = strings.TrimSpace(effectiveSession.IPAddress)
		}
		enqueueCtx, enqueueCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer enqueueCancel()
		if err := s.sophosSSO.EnqueueAccounting(enqueueCtx, sophosssodomain.AccountingInput{
			SourceEventID:  event.ID,
			Username:       req.Username,
			IPAddress:      effectiveIP,
			MACAddress:     req.MACAddress,
			AcctStatusType: status,
			AcctSessionID:  req.AcctSessionID,
			EventTimestamp: eventAt,
			AcctDelayTime:  req.AcctDelayTime,
		}); err != nil {
			return domain.AccountingResponse{}, s.failAccounting(ctx, event.ID, claimedAt, fmt.Errorf("enqueue Sophos RADIUS SSO accounting: %w", err))
		}
	}
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := s.events.CompleteAccounting(finalizeCtx, event.ID, claimedAt, s.now().UTC()); err != nil {
		return domain.AccountingResponse{}, fmt.Errorf("complete radius accounting event: %w", err)
	}
	return domain.AccountingResponse{Status: "ok"}, nil
}

func (s *Service) recordAuthorizeEvent(ctx context.Context, req domain.AuthorizeRequest, resp domain.AuthorizeResponse) {
	if s.events == nil {
		return
	}

	replyAttributes, _ := json.Marshal(resp.ReplyAttributes)
	controlAttributes, _ := json.Marshal(resp.ControlAttributes)

	_, _ = s.events.Insert(ctx, radiusevent.Event{
		ID:                uuid.NewString(),
		EventType:         "authorize",
		Username:          strings.TrimSpace(req.Username),
		MACAddress:        strings.TrimSpace(req.MACAddress),
		Hostname:          strings.TrimSpace(req.Hostname),
		VendorClass:       strings.TrimSpace(req.VendorClass),
		NASIPAddress:      strings.TrimSpace(req.NASIPAddress),
		NASIdentifier:     strings.TrimSpace(req.NASIdentifier),
		NASPort:           strings.TrimSpace(req.NASPort),
		NASPortID:         strings.TrimSpace(req.NASPortID),
		NASPortType:       "",
		CalledStationID:   strings.TrimSpace(req.CalledStationID),
		CallingStationID:  strings.TrimSpace(req.CallingStationID),
		AcctStatusType:    "",
		AcctSessionID:     "",
		FramedIPAddress:   "",
		SessionTime:       "",
		TerminateCause:    "",
		Decision:          strings.TrimSpace(resp.Decision),
		PolicyAction:      strings.TrimSpace(resp.PolicyAction),
		PolicyReason:      strings.TrimSpace(resp.PolicyReason),
		ReplyMessage:      strings.TrimSpace(resp.ReplyMessage),
		VLANID:            strings.TrimSpace(resp.VLANID),
		ReplyAttributes:   string(replyAttributes),
		ControlAttributes: string(controlAttributes),
		CreatedAt:         time.Now().UTC(),
	})
}

func buildAccountingEvent(req domain.AccountingRequest, eventAt time.Time) (radiusevent.Event, error) {
	accountingMetadata, err := json.Marshal(map[string]any{
		"acct_delay_time": req.AcctDelayTime,
		"event_timestamp": strings.TrimSpace(req.EventTimestamp),
	})
	if err != nil {
		return radiusevent.Event{}, fmt.Errorf("encode radius accounting metadata: %w", err)
	}

	return radiusevent.Event{
		ID:                accountingEventID(req, eventAt),
		EventType:         "accounting",
		Username:          strings.TrimSpace(req.Username),
		MACAddress:        strings.TrimSpace(req.MACAddress),
		Hostname:          strings.TrimSpace(req.Hostname),
		VendorClass:       strings.TrimSpace(req.VendorClass),
		NASIPAddress:      strings.TrimSpace(req.NASIPAddress),
		NASIdentifier:     strings.TrimSpace(req.NASIdentifier),
		NASPort:           strings.TrimSpace(req.NASPort),
		NASPortID:         strings.TrimSpace(req.NASPortID),
		NASPortType:       strings.TrimSpace(req.NASPortType),
		CalledStationID:   strings.TrimSpace(req.CalledStationID),
		CallingStationID:  strings.TrimSpace(req.CallingStationID),
		AcctStatusType:    strings.TrimSpace(req.AcctStatusType),
		AcctSessionID:     strings.TrimSpace(req.AcctSessionID),
		FramedIPAddress:   strings.TrimSpace(req.FramedIPAddress),
		SessionTime:       strings.TrimSpace(req.SessionTime),
		TerminateCause:    strings.TrimSpace(req.TerminateCause),
		Decision:          "",
		PolicyAction:      "",
		PolicyReason:      "",
		ReplyMessage:      "",
		VLANID:            "",
		ReplyAttributes:   "{}",
		ControlAttributes: string(accountingMetadata),
		CreatedAt:         eventAt,
	}, nil
}

func (s *Service) failAccounting(ctx context.Context, eventID string, claimedAt time.Time, processingErr error) error {
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := s.events.FailAccounting(failCtx, eventID, processingErr.Error(), claimedAt, s.now().UTC()); err != nil {
		return errors.Join(processingErr, fmt.Errorf("mark radius accounting event failed: %w", err))
	}
	return processingErr
}

func (s *Service) syncAuthorizeInventory(ctx context.Context, req domain.AuthorizeRequest, resp domain.AuthorizeResponse) {
	switchAsset, ok := s.resolveSwitch(ctx, req.NASIPAddress, req.NASIdentifier)
	if !ok {
		log.Printf("radius authorize inventory sync skipped: switch not resolved nas_ip=%q nas_identifier=%q mac=%q", req.NASIPAddress, req.NASIdentifier, req.MACAddress)
		return
	}

	observedAt := time.Now().UTC()
	observation := macobservation.Observation{
		ID:                   uuid.NewString(),
		MACAddress:           strings.ToUpper(strings.TrimSpace(req.MACAddress)),
		SourceType:           sourceTypeRadius,
		Confidence:           confidenceAuthoritative,
		SwitchID:             switchAsset.ID,
		SwitchName:           switchAsset.Name,
		ManagementIP:         switchAsset.ManagementIP,
		BridgePort:           0,
		IfIndex:              parseIfIndex(req.NASPort),
		InterfaceName:        firstNonEmpty(req.NASPortID, req.NASPort),
		InterfaceDescription: firstNonEmpty(req.NASPortID, req.NASPort),
		ObservedAt:           observedAt,
		CreatedAt:            observedAt,
	}

	if s.observations != nil {
		if _, err := s.observations.Insert(ctx, observation); err != nil {
			log.Printf("radius authorize observation insert failed: mac=%q switch=%q err=%v", observation.MACAddress, observation.SwitchName, err)
		}
	}

	if s.devices != nil {
		if err := s.devices.UpsertFromRadius(ctx, deviceservice.RadiusInventoryInput{
			MACAddress:           req.MACAddress,
			Hostname:             req.Hostname,
			VendorClass:          req.VendorClass,
			SwitchID:             switchAsset.ID,
			SwitchName:           switchAsset.Name,
			ManagementIP:         switchAsset.ManagementIP,
			NASPort:              req.NASPort,
			NASPortID:            req.NASPortID,
			ObservedAt:           observedAt,
			PolicyActionOverride: resp.PolicyAction,
			PolicyReasonOverride: resp.PolicyReason,
		}); err != nil {
			log.Printf("radius authorize device upsert failed: mac=%q switch=%q err=%v", req.MACAddress, switchAsset.Name, err)
		}
	}
}

func (s *Service) syncAccountingInventory(ctx context.Context, req domain.AccountingRequest, observedAt time.Time) error {
	switchAsset, err := s.resolveAccountingSwitch(ctx, req.NASIPAddress, req.NASIdentifier)
	if err != nil {
		return err
	}

	observation := macobservation.Observation{
		ID:                   uuid.NewString(),
		MACAddress:           strings.ToUpper(strings.TrimSpace(req.MACAddress)),
		SourceType:           sourceTypeRadius,
		Confidence:           confidenceAuthoritative,
		SwitchID:             switchAsset.ID,
		SwitchName:           switchAsset.Name,
		ManagementIP:         switchAsset.ManagementIP,
		BridgePort:           0,
		IfIndex:              parseIfIndex(req.NASPort),
		InterfaceName:        firstNonEmpty(req.NASPortID, req.NASPort),
		InterfaceDescription: firstNonEmpty(req.NASPortID, req.NASPort),
		ObservedAt:           observedAt,
		CreatedAt:            observedAt,
	}

	if s.devices != nil {
		if err := s.devices.UpsertFromRadius(ctx, deviceservice.RadiusInventoryInput{
			MACAddress:   req.MACAddress,
			Hostname:     req.Hostname,
			VendorClass:  req.VendorClass,
			SwitchID:     switchAsset.ID,
			SwitchName:   switchAsset.Name,
			ManagementIP: switchAsset.ManagementIP,
			NASPort:      req.NASPort,
			NASPortID:    req.NASPortID,
			ObservedAt:   observedAt,
		}); err != nil {
			return fmt.Errorf("upsert radius accounting device: %w", err)
		}
	}

	if s.observations != nil {
		if _, err := s.observations.Insert(ctx, observation); err != nil {
			return fmt.Errorf("insert radius accounting observation: %w", err)
		}
	}
	return nil
}

func (s *Service) syncAuthorizeSession(ctx context.Context, req domain.AuthorizeRequest, resp domain.AuthorizeResponse) {
	if s.sessions == nil {
		return
	}

	switchAsset, ok := s.resolveSwitch(ctx, req.NASIPAddress, req.NASIdentifier)
	if !ok {
		return
	}

	now := time.Now().UTC()
	session := sessiondomain.Session{
		ID:               uuid.NewString(),
		ActiveKey:        buildActiveSessionKey(req.MACAddress, switchAsset.ID, req.NASPortID, req.NASPort),
		SwitchID:         switchAsset.ID,
		SwitchName:       switchAsset.Name,
		ManagementIP:     switchAsset.ManagementIP,
		NASPort:          strings.TrimSpace(req.NASPort),
		NASPortID:        strings.TrimSpace(req.NASPortID),
		IfIndex:          parseIfIndex(req.NASPort),
		InterfaceName:    firstNonEmpty(req.NASPortID, req.NASPort),
		MACAddress:       strings.ToUpper(strings.TrimSpace(req.MACAddress)),
		Username:         strings.TrimSpace(req.Username),
		Hostname:         strings.TrimSpace(req.Hostname),
		VendorClass:      strings.TrimSpace(req.VendorClass),
		CalledStationID:  strings.TrimSpace(req.CalledStationID),
		CallingStationID: strings.TrimSpace(req.CallingStationID),
		Authorization:    strings.TrimSpace(resp.Decision),
		SessionType:      inferSessionType(req.Username, req.MACAddress, req.CallingStationID),
		Status:           deriveAuthorizeSessionStatus(resp.Decision),
		PolicyAction:     strings.TrimSpace(resp.PolicyAction),
		PolicyReason:     strings.TrimSpace(resp.PolicyReason),
		AssignedVLAN:     strings.TrimSpace(resp.VLANID),
		StartedAt:        time.Time{},
		LastSeenAt:       time.Time{},
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if session.Status == "rejected" {
		session.EndedAt = &now
	}

	if _, err := s.sessions.Upsert(ctx, session); err != nil {
		log.Printf("radius authorize session upsert failed: mac=%q switch=%q err=%v", req.MACAddress, switchAsset.Name, err)
	}
}

func (s *Service) syncAccountingSession(ctx context.Context, req domain.AccountingRequest, eventAt time.Time) (*sessiondomain.Session, bool, error) {
	if s.sessions == nil {
		return nil, false, errors.New("radius session repository is not configured")
	}

	switchAsset, err := s.resolveAccountingSwitch(ctx, req.NASIPAddress, req.NASIdentifier)
	if err != nil {
		return nil, false, err
	}

	now := eventAt
	portKey := buildActiveSessionKey(req.MACAddress, switchAsset.ID, req.NASPortID, req.NASPort)
	activeKey := portKey
	acctSessionID := strings.TrimSpace(req.AcctSessionID)
	if acctSessionID != "" {
		acctKey := buildAccountingSessionKey(req.MACAddress, switchAsset.ID, acctSessionID)
		existingByAcct, err := s.sessions.FindByAcctSession(ctx, req.MACAddress, switchAsset.ID, acctSessionID)
		if err != nil {
			return nil, false, fmt.Errorf("find radius accounting session: %w", err)
		} else if existingByAcct != nil {
			activeKey = existingByAcct.ActiveKey
			if !existingByAcct.LastSeenAt.IsZero() && eventAt.Before(existingByAcct.LastSeenAt) {
				return existingByAcct, false, nil
			}
		} else if err := s.sessions.PromoteToAccountingKey(ctx, portKey, acctKey, acctSessionID); err != nil {
			return nil, false, fmt.Errorf("promote radius accounting session key: %w", err)
		} else {
			activeKey = acctKey
		}
	}

	session := sessiondomain.Session{
		ID:               uuid.NewString(),
		ActiveKey:        activeKey,
		SwitchID:         switchAsset.ID,
		SwitchName:       switchAsset.Name,
		ManagementIP:     switchAsset.ManagementIP,
		NASPort:          strings.TrimSpace(req.NASPort),
		NASPortID:        strings.TrimSpace(req.NASPortID),
		IfIndex:          parseIfIndex(req.NASPort),
		InterfaceName:    firstNonEmpty(req.NASPortID, req.NASPort),
		IPAddress:        strings.TrimSpace(req.FramedIPAddress),
		MACAddress:       strings.ToUpper(strings.TrimSpace(req.MACAddress)),
		Username:         strings.TrimSpace(req.Username),
		Hostname:         strings.TrimSpace(req.Hostname),
		VendorClass:      strings.TrimSpace(req.VendorClass),
		CalledStationID:  strings.TrimSpace(req.CalledStationID),
		CallingStationID: strings.TrimSpace(req.CallingStationID),
		AcctSessionID:    acctSessionID,
		SessionType:      inferSessionType(req.Username, req.MACAddress, req.CallingStationID),
		Status:           deriveAccountingSessionStatus(req.AcctStatusType),
		StartedAt:        deriveAccountingStartedAt(req.AcctStatusType, now),
		LastSeenAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if strings.EqualFold(strings.TrimSpace(req.AcctStatusType), "Stop") {
		session.EndedAt = &now
	}

	stored, err := s.sessions.Upsert(ctx, session)
	if err != nil {
		return nil, false, fmt.Errorf("upsert radius accounting session: %w", err)
	}
	return &stored, true, nil
}

func (s *Service) resolveAccountingSwitch(ctx context.Context, nasIPAddress, nasIdentifier string) (*switchasset.Switch, error) {
	if s.switches == nil {
		return nil, errors.New("radius switch repository is not configured")
	}
	if value := strings.TrimSpace(nasIPAddress); value != "" {
		candidate, err := s.switches.FindByManagementIP(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("resolve radius switch by management IP: %w", err)
		}
		if candidate != nil {
			return candidate, nil
		}
	}
	if value := strings.TrimSpace(nasIdentifier); value != "" {
		candidate, err := s.switches.FindByName(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("resolve radius switch by name: %w", err)
		}
		if candidate != nil {
			return candidate, nil
		}
	}
	return nil, errors.New("radius accounting switch was not found")
}

func (s *Service) resolveSwitch(ctx context.Context, nasIPAddress, nasIdentifier string) (*switchasset.Switch, bool) {
	if s.switches == nil {
		return nil, false
	}

	if candidate, err := s.switches.FindByManagementIP(ctx, nasIPAddress); err == nil && candidate != nil {
		return candidate, true
	}

	identifier := strings.TrimSpace(nasIdentifier)
	if identifier == "" {
		return nil, false
	}

	candidate, err := s.switches.FindByName(ctx, identifier)
	if err != nil || candidate == nil {
		return nil, false
	}

	return candidate, true
}

func parseIfIndex(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	var parsed int
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0
		}
		parsed = parsed*10 + int(ch-'0')
	}

	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeMACAddress(values ...string) string {
	return normalize.MACAddress(values...)
}

func buildActiveSessionKey(macAddress, switchID, nasPortID, nasPort string) string {
	return strings.ToUpper(strings.TrimSpace(firstNonEmpty(macAddress))) + "|" +
		strings.TrimSpace(switchID) + "|" +
		firstNonEmpty(strings.TrimSpace(nasPortID), strings.TrimSpace(nasPort))
}

func buildAccountingSessionKey(macAddress, switchID, acctSessionID string) string {
	return strings.ToUpper(strings.TrimSpace(firstNonEmpty(macAddress))) + "|" +
		strings.TrimSpace(switchID) + "|acct:" + strings.TrimSpace(acctSessionID)
}

func inferSessionType(username, macAddress, callingStationID string) string {
	normalizedMAC := normalizeMACAddress(macAddress, callingStationID)
	normalizedUser := normalizeMACAddress(username)
	if normalizedMAC != "" && normalizedUser != "" && normalizedMAC == normalizedUser {
		return "mab"
	}
	if strings.TrimSpace(username) != "" {
		return "802.1x"
	}
	return "unknown"
}

func deriveAuthorizeSessionStatus(decision string) string {
	if strings.EqualFold(strings.TrimSpace(decision), "reject") {
		return "rejected"
	}
	return "authorization-pending"
}

func deriveAccountingSessionStatus(acctStatusType string) string {
	switch strings.ToLower(strings.TrimSpace(acctStatusType)) {
	case "start":
		return "started"
	case "interim-update":
		return "interim-update"
	case "stop":
		return "stopped"
	default:
		return "accounting"
	}
}

func deriveAccountingStartedAt(acctStatusType string, now time.Time) time.Time {
	switch strings.ToLower(strings.TrimSpace(acctStatusType)) {
	case "start", "interim-update":
		return now
	default:
		return time.Time{}
	}
}

func validateAccountingRequest(req domain.AccountingRequest) (string, error) {
	status := canonicalAccountingStatus(req.AcctStatusType)
	if status == "" {
		return "", domain.ValidationError{Field: "acct_status_type", Message: "must be Start, Interim-Update, Stop, Accounting-On, or Accounting-Off"}
	}
	if strings.TrimSpace(req.NASIPAddress) == "" && strings.TrimSpace(req.NASIdentifier) == "" {
		return "", domain.ValidationError{Field: "nas_ip_address", Message: "or nas_identifier is required"}
	}
	if value := strings.TrimSpace(req.NASIPAddress); value != "" && net.ParseIP(value) == nil {
		return "", domain.ValidationError{Field: "nas_ip_address", Message: "must be a valid IP address"}
	}
	if req.AcctDelayTime < 0 || req.AcctDelayTime > 604800 {
		return "", domain.ValidationError{Field: "acct_delay_time", Message: "must be between 0 and 604800 seconds"}
	}
	if status == "Accounting-On" || status == "Accounting-Off" {
		return status, nil
	}
	if strings.TrimSpace(req.MACAddress) == "" {
		return "", domain.ValidationError{Field: "mac_address", Message: "is required"}
	}
	if strings.TrimSpace(req.AcctSessionID) == "" {
		return "", domain.ValidationError{Field: "acct_session_id", Message: "is required"}
	}
	if value := strings.TrimSpace(req.FramedIPAddress); value != "" && net.ParseIP(value) == nil {
		return "", domain.ValidationError{Field: "framed_ip_address", Message: "must be a valid IP address"}
	}
	return status, nil
}

func canonicalAccountingStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "start":
		return "Start"
	case "interim-update", "interim_update", "interim":
		return "Interim-Update"
	case "stop":
		return "Stop"
	case "accounting-on", "accounting_on":
		return "Accounting-On"
	case "accounting-off", "accounting_off":
		return "Accounting-Off"
	default:
		return ""
	}
}

func accountingEventTime(req domain.AccountingRequest, receivedAt time.Time) (time.Time, error) {
	if value := strings.TrimSpace(req.EventTimestamp); value != "" {
		if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
			return time.Unix(unixSeconds, 0).UTC(), nil
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return time.Time{}, domain.ValidationError{Field: "event_timestamp", Message: "must be RFC3339 or Unix seconds"}
		}
		return parsed.UTC(), nil
	}
	return receivedAt.Add(-time.Duration(req.AcctDelayTime) * time.Second), nil
}

func accountingEventID(req domain.AccountingRequest, eventAt time.Time) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(req.AcctStatusType)),
		strings.TrimSpace(req.AcctSessionID),
		strings.ToLower(strings.TrimSpace(req.NASIPAddress)),
		strings.ToLower(strings.TrimSpace(req.NASIdentifier)),
		strings.ToUpper(strings.TrimSpace(req.MACAddress)),
		strings.TrimSpace(req.EventTimestamp),
		strings.TrimSpace(req.SessionTime),
		strings.TrimSpace(req.FramedIPAddress),
		strings.TrimSpace(req.TerminateCause),
	}
	if (req.AcctStatusType == "Accounting-On" || req.AcctStatusType == "Accounting-Off") && strings.TrimSpace(req.EventTimestamp) == "" {
		parts = append(parts, eventAt.Truncate(time.Minute).Format(time.RFC3339))
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	id := uuid.UUID{}
	copy(id[:], digest[:16])
	id[6] = (id[6] & 0x0f) | 0x80
	id[8] = (id[8] & 0x3f) | 0x80
	return id.String()
}
