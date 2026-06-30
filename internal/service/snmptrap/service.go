package snmptrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/snmptrap"
	switchasset "nac/internal/domain/switchasset"
	switchport "nac/internal/domain/switchport"
	trapwindowdomain "nac/internal/domain/trapwindow"
)

type Service struct {
	repository domain.Repository
	switches   switchResolver
	ports      portResolver
	windows    trapWindowRecorder
	logger     *slog.Logger
}

type switchResolver interface {
	FindByManagementIP(ctx context.Context, managementIP string) (*switchasset.Switch, error)
	FindByID(ctx context.Context, id string) (*switchasset.Switch, error)
}

type portResolver interface {
	ListBySwitch(ctx context.Context, switchID string) ([]switchport.Port, error)
}

type trapWindowRecorder interface {
	Record(ctx context.Context, window trapwindowdomain.Window) (trapwindowdomain.Window, error)
}

func NewService(logger *slog.Logger, repository domain.Repository, switches switchResolver, ports portResolver, windows trapWindowRecorder) *Service {
	return &Service{
		repository: repository,
		switches:   switches,
		ports:      ports,
		windows:    windows,
		logger:     logger,
	}
}

func (s *Service) Ingest(ctx context.Context, event domain.Event) (domain.Event, error) {
	if s.repository == nil {
		return domain.Event{}, fmt.Errorf("snmp trap repository is not configured")
	}

	now := time.Now().UTC()
	if event.ID == "" {
		event.ID = uuid.NewString()
	}

	event.SourceIP = strings.TrimSpace(event.SourceIP)
	if event.SourceIP == "" {
		return domain.Event{}, fmt.Errorf("source_ip is required")
	}
	if addr, err := netip.ParseAddr(event.SourceIP); err == nil {
		event.SourceIP = addr.String()
	}
	event.SNMPVersion = strings.TrimSpace(event.SNMPVersion)
	event.Community = strings.TrimSpace(event.Community)
	event.TrapOID = strings.TrimSpace(event.TrapOID)
	event.EnterpriseOID = strings.TrimSpace(event.EnterpriseOID)
	event.SwitchID = strings.TrimSpace(event.SwitchID)
	event.SwitchName = strings.TrimSpace(event.SwitchName)

	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = now
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}

	if s.switches != nil && event.SwitchID == "" {
		if asset, err := s.switches.FindByManagementIP(ctx, event.SourceIP); err == nil && asset != nil {
			event.SwitchID = asset.ID
			event.SwitchName = asset.Name
		}
	}

	s.classifyTrap(&event)

	stored, err := s.repository.Insert(ctx, event)
	if err != nil {
		return domain.Event{}, err
	}

	if err := s.recordActionableWindows(ctx, stored); err != nil && s.logger != nil {
		s.logger.Warn("snmp trap window record skipped", "error", err, "source_ip", stored.SourceIP, "trap_oid", stored.TrapOID)
	}

	return stored, nil
}

func (s *Service) classifyTrap(event *domain.Event) {
	if event == nil {
		return
	}

	for _, item := range event.VarBinds {
		oid := normalizeOID(item.OID)
		switch {
		case strings.HasPrefix(oid, normalizeOID(oidIfIndex)):
			if value, err := strconv.Atoi(strings.TrimSpace(item.Value)); err == nil && value > 0 {
				event.IfIndex = value
			}
		case oid == normalizeOID(oidHPPortAccessMAC):
			event.MACAddress = normalizeMAC(item.Value)
		case oid == normalizeOID(oidHPPortAccessIfIndex):
			if value, err := strconv.Atoi(strings.TrimSpace(item.Value)); err == nil && value > 0 {
				event.IfIndex = value
			}
		case oid == normalizeOID(oidHPPortAccessVLAN):
			if value, err := strconv.Atoi(strings.TrimSpace(item.Value)); err == nil && value > 0 {
				event.VLANID = value
			}
		}
	}

	trapOID := normalizeOID(event.TrapOID)
	switch {
	case trapOID == normalizeOID(oidTrapLinkDown):
		event.Category = "link-down"
		event.Actionable = true
	case trapOID == normalizeOID(oidTrapLinkUp):
		event.Category = "link-up"
		event.Actionable = true
	case trapOID == normalizeOID(oidTrapColdStart):
		event.Category = "cold-start"
		event.Actionable = true
	case trapOID == normalizeOID(oidTrapWarmStart):
		event.Category = "warm-start"
		event.Actionable = true
	case normalizeOID(event.EnterpriseOID) == normalizeOID(oidHPPortAccessTrap):
		event.Category = "hp-port-access"
		event.Actionable = event.IfIndex > 0 || event.MACAddress != ""
	case normalizeOID(event.EnterpriseOID) == normalizeOID(oidHPAuthSessionTrap):
		event.Category = "hp-auth-session"
		event.Actionable = true
	default:
		event.Category = "other"
		event.Actionable = false
	}
}

func (s *Service) recordActionableWindows(ctx context.Context, event domain.Event) error {
	if !event.Actionable || s.windows == nil || strings.TrimSpace(event.SwitchID) == "" {
		return nil
	}

	if err := s.recordWindow(ctx, event.SwitchID, primaryScopeForCategory(event.Category), event, trapDebounceWindow); err != nil {
		return err
	}

	if event.Category == "hp-port-access" || event.Category == "hp-auth-session" || strings.HasPrefix(event.Category, "link-") {
		relatedSwitchID := s.resolveARPSourceSwitch(ctx, event)
		if relatedSwitchID != "" && relatedSwitchID != event.SwitchID {
			return s.recordWindow(ctx, relatedSwitchID, "arp", event, arpTrapDebounceWindow)
		}
	}

	return nil
}

func (s *Service) resolveARPSourceSwitch(ctx context.Context, event domain.Event) string {
	if s.switches != nil && strings.TrimSpace(event.SwitchID) != "" {
		if asset, err := s.switches.FindByID(ctx, event.SwitchID); err == nil && asset != nil && strings.TrimSpace(asset.RoutingSwitchID) != "" {
			return strings.TrimSpace(asset.RoutingSwitchID)
		}
	}

	if s.ports == nil || strings.TrimSpace(event.SwitchID) == "" {
		return ""
	}

	ports, err := s.ports.ListBySwitch(ctx, event.SwitchID)
	if err != nil {
		return ""
	}

	var matched *switchport.Port
	for i := range ports {
		if event.IfIndex > 0 && ports[i].IfIndex == event.IfIndex {
			matched = &ports[i]
			break
		}
	}
	if matched != nil && matched.IsUplink && strings.TrimSpace(matched.NeighborSwitchID) != "" {
		return matched.NeighborSwitchID
	}

	for i := range ports {
		if ports[i].IsUplink && strings.TrimSpace(ports[i].NeighborSwitchID) != "" {
			return ports[i].NeighborSwitchID
		}
	}

	return ""
}

func (s *Service) recordWindow(ctx context.Context, switchID, scope string, event domain.Event, cooldown time.Duration) error {
	switchID = strings.TrimSpace(switchID)
	scope = strings.TrimSpace(scope)
	if switchID == "" || scope == "" {
		return nil
	}

	now := time.Now().UTC()
	window := trapwindowdomain.Window{
		ID:            uuid.NewString(),
		DedupeKey:     buildWindowKey(switchID, scope, event),
		SwitchID:      switchID,
		Scope:         scope,
		Category:      strings.TrimSpace(event.Category),
		Status:        "pending",
		PortIfIndex:   event.IfIndex,
		MACAddress:    event.MACAddress,
		VLANID:        event.VLANID,
		EventCount:    1,
		FirstSeenAt:   now,
		LastSeenAt:    now,
		AvailableAt:   now.Add(cooldown),
		TrapOID:       event.TrapOID,
		EnterpriseOID: event.EnterpriseOID,
		SourceIP:      event.SourceIP,
		Summary: map[string]any{
			"category":       event.Category,
			"if_index":       event.IfIndex,
			"mac_address":    event.MACAddress,
			"vlan_id":        event.VLANID,
			"trap_oid":       event.TrapOID,
			"enterprise_oid": event.EnterpriseOID,
			"source_ip":      event.SourceIP,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.windows.Record(ctx, window); err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Info("snmp trap recorded window", "switch_id", switchID, "scope", scope, "category", event.Category, "if_index", event.IfIndex, "mac_address", event.MACAddress)
	}
	return nil
}

func primaryScopeForCategory(category string) string {
	switch strings.TrimSpace(category) {
	case "cold-start", "warm-start":
		return "full"
	default:
		return "ports"
	}
}

func normalizeOID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, ".") {
		return value
	}
	return "." + value
}

func normalizeMAC(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("_", ":", "-", ":", ".", ":", " ", "")
	value = replacer.Replace(value)
	if strings.Count(value, ":") == 5 {
		return value
	}
	compact := strings.ReplaceAll(value, ":", "")
	if len(compact) != 12 {
		return value
	}
	parts := make([]string, 0, 6)
	for i := 0; i < len(compact); i += 2 {
		parts = append(parts, compact[i:i+2])
	}
	return strings.Join(parts, ":")
}

const (
	oidIfIndex             = ".1.3.6.1.2.1.2.2.1.1"
	oidTrapColdStart       = ".1.3.6.1.6.3.1.1.5.1"
	oidTrapWarmStart       = ".1.3.6.1.6.3.1.1.5.2"
	oidTrapLinkDown        = ".1.3.6.1.6.3.1.1.5.3"
	oidTrapLinkUp          = ".1.3.6.1.6.3.1.1.5.4"
	oidHPPortAccessTrap    = ".1.3.6.1.4.1.11.2.14.11.5.1.19"
	oidHPAuthSessionTrap   = ".1.3.6.1.4.1.11.2.14.11.5.1.32"
	oidHPPortAccessState   = ".1.3.6.1.4.1.11.2.14.11.5.1.19.1.10.0"
	oidHPPortAccessMAC     = ".1.3.6.1.4.1.11.2.14.11.5.1.19.1.11.0"
	oidHPPortAccessIfIndex = ".1.3.6.1.4.1.11.2.14.11.5.1.19.1.12.0"
	oidHPPortAccessVLAN    = ".1.3.6.1.4.1.11.2.14.11.5.1.19.1.13.0"
)

const (
	trapDebounceWindow    = 60 * time.Second
	arpTrapDebounceWindow = 90 * time.Second
)

func buildWindowKey(switchID, scope string, event domain.Event) string {
	if event.IfIndex > 0 && strings.TrimSpace(event.MACAddress) != "" {
		return switchID + "|" + scope + "|" + event.Category + "|" + strconv.Itoa(event.IfIndex) + "|" + event.MACAddress
	}
	if event.IfIndex > 0 {
		return switchID + "|" + scope + "|" + event.Category + "|" + strconv.Itoa(event.IfIndex)
	}
	return switchID + "|" + scope + "|" + event.Category
}
