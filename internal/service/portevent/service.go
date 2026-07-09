package portevent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	devicedomain "nac/internal/domain/device"
	domain "nac/internal/domain/portevent"
	switchportdomain "nac/internal/domain/switchport"
	"nac/internal/normalize"
	auditlogservice "nac/internal/service/auditlog"
)

type Repository interface {
	Insert(ctx context.Context, event domain.Event) (domain.Event, error)
	ListRecent(ctx context.Context, limit int) ([]domain.Event, error)
}

type SwitchPortRepository interface {
	UpdateStatus(ctx context.Context, switchID string, ifIndex int, interfaceName, interfaceDescription, adminStatus, operStatus string, observedAt time.Time) (*switchportdomain.Port, error)
}

type DeviceObserver interface {
	ObserveAgentlessEvent(ctx context.Context, input devicedomain.AgentlessObservationInput) (devicedomain.Device, error)
}

type Service struct {
	repository  Repository
	switchPorts SwitchPortRepository
	devices     DeviceObserver
	audit       *auditlogservice.Service
}

func NewService(repository Repository, switchPorts SwitchPortRepository, devices DeviceObserver, audit *auditlogservice.Service) *Service {
	return &Service{repository: repository, switchPorts: switchPorts, devices: devices, audit: audit}
}

func (s *Service) Ingest(ctx context.Context, event domain.Event) (domain.Event, error) {
	if s == nil || s.repository == nil {
		return domain.Event{}, fmt.Errorf("port event repository is not configured")
	}

	normalized, err := s.normalizeEvent(event)
	if err != nil {
		return domain.Event{}, err
	}

	if s.switchPorts != nil && normalized.SwitchID != "" && normalized.IfIndex > 0 {
		_, err = s.switchPorts.UpdateStatus(
			ctx,
			normalized.SwitchID,
			normalized.IfIndex,
			normalized.InterfaceName,
			normalized.InterfaceDescription,
			normalized.AdminStatus,
			normalized.OperStatus,
			normalized.ObservedAt,
		)
		if err != nil {
			return domain.Event{}, err
		}
	}

	if s.devices != nil && normalized.MACAddress != "" {
		device, err := s.devices.ObserveAgentlessEvent(ctx, devicedomain.AgentlessObservationInput{
			MACAddress:           normalized.MACAddress,
			IPAddress:            normalized.IPAddress,
			Hostname:             normalized.Hostname,
			VendorClass:          normalized.VendorClass,
			DeviceType:           normalized.DeviceType,
			SwitchID:             normalized.SwitchID,
			SwitchName:           normalized.SwitchName,
			ManagementIP:         normalized.ManagementIP,
			IfIndex:              normalized.IfIndex,
			InterfaceName:        normalized.InterfaceName,
			InterfaceDescription: normalized.InterfaceDescription,
			SourceType:           normalized.EventSource,
			Confidence:           "observed",
			ObservedAt:           normalized.ObservedAt,
		})
		if err != nil {
			return domain.Event{}, err
		}
		normalized.PolicyAction = device.PolicyAction
		normalized.PolicyReason = device.PolicyReason
		normalized.TrustLevel = device.TrustLevel
		normalized.EnforcementAction = device.LastPolicyDecision
	}

	stored, err := s.repository.Insert(ctx, normalized)
	if err != nil {
		return domain.Event{}, err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "port_event_ingested", "success", "port_event", stored.ID, stored.SwitchID, stored.MACAddress, map[string]any{
			"event_type":    stored.EventType,
			"event_source":  stored.EventSource,
			"if_index":      stored.IfIndex,
			"oper_status":   stored.OperStatus,
			"policy_action": stored.PolicyAction,
			"trust_level":   stored.TrustLevel,
		})
	}

	return stored, nil
}

func (s *Service) ListRecent(ctx context.Context, limit int) ([]domain.Event, error) {
	if s == nil || s.repository == nil {
		return []domain.Event{}, nil
	}
	return s.repository.ListRecent(ctx, limit)
}

func (s *Service) normalizeEvent(event domain.Event) (domain.Event, error) {
	event.SwitchID = strings.TrimSpace(event.SwitchID)
	event.SwitchName = strings.TrimSpace(event.SwitchName)
	event.ManagementIP = strings.TrimSpace(event.ManagementIP)
	event.InterfaceName = strings.TrimSpace(event.InterfaceName)
	event.InterfaceDescription = strings.TrimSpace(event.InterfaceDescription)
	event.AdminStatus = normalizeStatus(event.AdminStatus)
	event.OperStatus = normalizeStatus(event.OperStatus)
	event.EventType = strings.TrimSpace(strings.ToLower(event.EventType))
	event.EventSource = strings.TrimSpace(event.EventSource)
	event.MACAddress = normalize.MACAddress(event.MACAddress)
	event.IPAddress = strings.TrimSpace(event.IPAddress)
	event.Hostname = strings.TrimSpace(event.Hostname)
	event.VendorClass = strings.TrimSpace(event.VendorClass)
	event.DeviceType = strings.TrimSpace(event.DeviceType)
	event.PolicyAction = strings.TrimSpace(event.PolicyAction)
	event.PolicyReason = strings.TrimSpace(event.PolicyReason)
	event.EnforcementAction = strings.TrimSpace(event.EnforcementAction)
	event.TrustLevel = strings.TrimSpace(event.TrustLevel)
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now().UTC()
	} else {
		event.ObservedAt = event.ObservedAt.UTC()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = event.ObservedAt
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	if event.SwitchID == "" {
		return domain.Event{}, fmt.Errorf("switch_id is required")
	}
	if event.IfIndex <= 0 {
		return domain.Event{}, fmt.Errorf("if_index must be greater than zero")
	}
	if event.EventType == "" {
		event.EventType = deriveEventType(event.OperStatus, event.MACAddress)
	}
	if event.EventSource == "" {
		event.EventSource = "agentless-port-observation"
	}
	return event, nil
}

func deriveEventType(operStatus, macAddress string) string {
	if strings.TrimSpace(macAddress) != "" {
		return "mac_learned"
	}
	switch strings.TrimSpace(strings.ToLower(operStatus)) {
	case "up":
		return "port_up"
	case "down":
		return "port_down"
	default:
		return "port_observed"
	}
}

func normalizeStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "up", "enabled":
		return "up"
	case "2", "down", "disabled", "admin_down":
		return "down"
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}
