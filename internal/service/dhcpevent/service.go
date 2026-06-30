package dhcpevent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/dhcpevent"
	switchasset "nac/internal/domain/switchasset"
)

type Service struct {
	repository        domain.Repository
	relaySwitches     RelaySwitchResolver
	postIngestHook    func(domain.Event)
	dedupWindow       time.Duration
	transactionWindow time.Duration
}

type RelaySwitchResolver interface {
	FindByBaseMAC(ctx context.Context, macAddress string) (*switchasset.Switch, error)
}

func NewService(repository domain.Repository, relaySwitches RelaySwitchResolver, postIngestHook func(domain.Event)) *Service {
	return &Service{
		repository:        repository,
		relaySwitches:     relaySwitches,
		postIngestHook:    postIngestHook,
		dedupWindow:       2 * time.Second,
		transactionWindow: 15 * time.Second,
	}
}

func (s *Service) Ingest(ctx context.Context, event domain.Event) (domain.Event, error) {
	now := time.Now().UTC()

	if event.ID == "" {
		event.ID = uuid.NewString()
	}

	if event.MessageType == "" {
		return domain.Event{}, fmt.Errorf("message_type is required")
	}

	if event.MACAddress == "" {
		return domain.Event{}, fmt.Errorf("mac_address is required")
	}

	event.MACAddress = strings.ToUpper(event.MACAddress)
	event.TransactionID = strings.ToLower(strings.TrimSpace(event.TransactionID))

	if event.ObservedAt.IsZero() {
		event.ObservedAt = now
	}

	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}

	event = s.resolveRelaySwitch(ctx, event)

	recent, err := s.findRecent(ctx, event, now)
	if err != nil {
		return domain.Event{}, err
	}

	if recent != nil {
		ingested, err := s.mergeOrSuppress(ctx, *recent, event)
		if err != nil {
			return domain.Event{}, err
		}
		s.afterIngest(ingested)
		return ingested, nil
	}

	created, err := s.repository.Insert(ctx, event)
	if err != nil {
		return domain.Event{}, err
	}

	s.afterIngest(created)
	return created, nil
}

func (s *Service) findRecent(ctx context.Context, event domain.Event, now time.Time) (*domain.Event, error) {
	if event.TransactionID != "" {
		recent, err := s.repository.FindRecentByTransaction(
			ctx,
			event.MACAddress,
			event.MessageType,
			event.TransactionID,
			now.Add(-s.transactionWindow),
		)
		if err != nil {
			return nil, err
		}
		if recent != nil {
			return recent, nil
		}
	}

	return s.repository.FindRecent(ctx, event.MACAddress, event.MessageType, now.Add(-s.dedupWindow))
}

func (s *Service) mergeOrSuppress(ctx context.Context, recent, event domain.Event) (domain.Event, error) {
	if recent.SourceIP == event.SourceIP {
		if isRicherDHCPEvent(event, recent) {
			return s.repository.UpdateByID(ctx, recent.ID, mergeDHCPEvent(recent, event))
		}
		return event, domain.ErrDuplicateSuppressed
	}

	if recent.SourceIP == "0.0.0.0" && event.SourceIP != "" && event.SourceIP != "0.0.0.0" {
		return s.repository.UpdateByID(ctx, recent.ID, mergeDHCPEvent(recent, event))
	}

	if recent.SourceIP != "" && recent.SourceIP != "0.0.0.0" && (event.SourceIP == "" || event.SourceIP == "0.0.0.0") {
		if isRicherDHCPEvent(event, recent) {
			return s.repository.UpdateByID(ctx, recent.ID, mergeDHCPEvent(recent, event))
		}
		return event, domain.ErrDuplicateSuppressed
	}

	if recent.TransactionID != "" && recent.TransactionID == event.TransactionID {
		if isRicherDHCPEvent(event, recent) {
			return s.repository.UpdateByID(ctx, recent.ID, mergeDHCPEvent(recent, event))
		}
		return event, domain.ErrDuplicateSuppressed
	}

	return s.repository.Insert(ctx, event)
}

func isRicherDHCPEvent(candidate, current domain.Event) bool {
	return dhcpEventPriority(candidate) > dhcpEventPriority(current)
}

func dhcpEventPriority(event domain.Event) int {
	score := 0

	if event.SourceIP != "" && event.SourceIP != "0.0.0.0" {
		score += 10
	}
	if strings.TrimSpace(event.ClientIP) != "" && event.ClientIP != "0.0.0.0" {
		score += 15
	}
	if strings.TrimSpace(event.YourIP) != "" && event.YourIP != "0.0.0.0" {
		score += 25
	}
	if strings.TrimSpace(event.RequestedIP) != "" && event.RequestedIP != "0.0.0.0" {
		score += 20
	}
	if strings.TrimSpace(event.Option82Raw) != "" {
		score += 20
	}
	if strings.TrimSpace(event.Hostname) != "" {
		score += 2
	}
	if strings.TrimSpace(event.VendorClass) != "" {
		score += 1
	}

	return score
}

func mergeDHCPEvent(base, incoming domain.Event) domain.Event {
	merged := base

	if strings.TrimSpace(incoming.TransactionID) != "" {
		merged.TransactionID = incoming.TransactionID
	}
	if strings.TrimSpace(incoming.SourceIP) != "" {
		merged.SourceIP = incoming.SourceIP
	}
	if strings.TrimSpace(incoming.ClientIP) != "" {
		merged.ClientIP = incoming.ClientIP
	}
	if strings.TrimSpace(incoming.YourIP) != "" {
		merged.YourIP = incoming.YourIP
	}
	if strings.TrimSpace(incoming.RequestedIP) != "" {
		merged.RequestedIP = incoming.RequestedIP
	}
	if strings.TrimSpace(incoming.Hostname) != "" {
		merged.Hostname = incoming.Hostname
	}
	if strings.TrimSpace(incoming.VendorClass) != "" {
		merged.VendorClass = incoming.VendorClass
	}
	if strings.TrimSpace(incoming.Option82Raw) != "" {
		merged.Option82Raw = incoming.Option82Raw
	}
	if strings.TrimSpace(incoming.Option82CircuitID) != "" {
		merged.Option82CircuitID = incoming.Option82CircuitID
	}
	if strings.TrimSpace(incoming.Option82RemoteID) != "" {
		merged.Option82RemoteID = incoming.Option82RemoteID
	}
	if strings.TrimSpace(incoming.Option82VLAN) != "" {
		merged.Option82VLAN = incoming.Option82VLAN
	}
	if strings.TrimSpace(incoming.RelayIP) != "" {
		merged.RelayIP = incoming.RelayIP
	}
	if strings.TrimSpace(incoming.RelaySwitchID) != "" {
		merged.RelaySwitchID = incoming.RelaySwitchID
	}
	if strings.TrimSpace(incoming.RelaySwitchName) != "" {
		merged.RelaySwitchName = incoming.RelaySwitchName
	}
	if !incoming.ObservedAt.IsZero() {
		merged.ObservedAt = incoming.ObservedAt
	}
	if !incoming.CreatedAt.IsZero() {
		merged.CreatedAt = incoming.CreatedAt
	}

	return merged
}

func (s *Service) resolveRelaySwitch(ctx context.Context, event domain.Event) domain.Event {
	if s.relaySwitches == nil {
		return event
	}
	if strings.TrimSpace(event.RelaySwitchID) != "" || strings.TrimSpace(event.Option82RemoteID) == "" {
		return event
	}

	relaySwitch, err := s.relaySwitches.FindByBaseMAC(ctx, event.Option82RemoteID)
	if err != nil || relaySwitch == nil {
		return event
	}

	event.RelaySwitchID = relaySwitch.ID
	event.RelaySwitchName = relaySwitch.Name
	return event
}

func (s *Service) IngestSample(ctx context.Context) (domain.Event, error) {
	return s.Ingest(ctx, domain.Event{
		MACAddress:    "4C:A6:4D:AC:EB:10",
		TransactionID: "0000beef",
		SourceIP:      "10.1.8.2",
		RequestedIP:   "10.1.8.100",
		MessageType:   "REQUEST",
		Hostname:      "sample-device",
		VendorClass:   "sample-vendor",
		Option82Raw:   "",
	})
}

func (s *Service) afterIngest(event domain.Event) {
	if s.postIngestHook == nil {
		return
	}

	s.postIngestHook(event)
}
