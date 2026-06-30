package maccorrelation

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	dhcpevent "nac/internal/domain/dhcpevent"
	macobservation "nac/internal/domain/macobservation"
	switchasset "nac/internal/domain/switchasset"
	"nac/internal/service/maclookup"
	"nac/internal/snmp"
)

type ObservationRepository interface {
	Insert(ctx context.Context, observation macobservation.Observation) (macobservation.Observation, error)
	InsertCandidates(ctx context.Context, candidates []macobservation.Candidate) error
}

type DeviceInventoryUpdater interface {
	UpsertFromObservation(ctx context.Context, event dhcpevent.Event, observation macobservation.Observation) error
}

type TopologyLinkChecker interface {
	HasLinkedInterface(ctx context.Context, switchID, interfaceName string) (bool, error)
	FindLinkedSwitchID(ctx context.Context, switchID, interfaceName string) (string, error)
	CountLinkedSwitches(ctx context.Context, switchID, interfaceName string) (int, error)
}

type LookupService interface {
	Lookup(ctx context.Context, req maclookup.Request) ([]maclookup.Result, error)
}

type RelaySwitchRepository interface {
	FindByID(ctx context.Context, id string) (*switchasset.Switch, error)
}

type Service struct {
	logger        *slog.Logger
	lookup        LookupService
	topology      TopologyLinkChecker
	repository    ObservationRepository
	devices       DeviceInventoryUpdater
	relaySwitches RelaySwitchRepository
	snmpClient    snmp.Client
	timeout       time.Duration
	option82      bool
}

const (
	sourceTypeOption82  = "option82"
	sourceTypeSNMPTrace = "snmp_trace"

	confidenceStrong    = "strong"
	confidenceDerived   = "derived"
	confidenceAmbiguous = "ambiguous"
)

func NewService(logger *slog.Logger, lookup LookupService, topology TopologyLinkChecker, repository ObservationRepository, devices DeviceInventoryUpdater, relaySwitches RelaySwitchRepository, snmpClient snmp.Client, option82Enabled bool) *Service {
	return &Service{
		logger:        logger,
		lookup:        lookup,
		topology:      topology,
		repository:    repository,
		devices:       devices,
		relaySwitches: relaySwitches,
		snmpClient:    snmpClient,
		timeout:       8 * time.Second,
		option82:      option82Enabled,
	}
}

func (s *Service) Handle(event dhcpevent.Event) {
	if event.MACAddress == "" {
		return
	}

	go func() {
		if s.option82 {
			if relayObservation, ok := s.resolveRelayObservation(event); ok {
				s.storeObservation(event, relayObservation, nil)
				return
			}
		}

		lookupCtx, cancelLookup := context.WithTimeout(context.Background(), s.timeout)
		defer cancelLookup()

		results, err := s.lookup.Lookup(lookupCtx, maclookup.Request{MACAddress: event.MACAddress})
		if err != nil {
			s.logger.Error("mac correlation lookup failed", "mac_address", event.MACAddress, "error", err)
			return
		}

		scoredResults := s.scoreResults(context.Background(), results)
		best, ok := s.chooseBestResult(scoredResults)
		if !ok {
			s.logger.Info("mac correlation no valid candidate", "mac_address", event.MACAddress)
			return
		}
		best = s.traceTopologyAware(context.Background(), event.MACAddress, best)

		confidence := s.determineSNMPConfidence(context.Background(), best)
		observation := macobservation.Observation{
			ID:                   uuid.NewString(),
			DHCPEventID:          event.ID,
			MACAddress:           event.MACAddress,
			SourceType:           sourceTypeSNMPTrace,
			Confidence:           confidence,
			SwitchID:             best.SwitchID,
			SwitchName:           best.SwitchName,
			ManagementIP:         best.ManagementIP,
			BridgePort:           best.BridgePort,
			IfIndex:              best.IfIndex,
			InterfaceName:        best.InterfaceName,
			InterfaceDescription: best.InterfaceDescription,
			ObservedAt:           event.ObservedAt,
			CreatedAt:            time.Now().UTC(),
		}

		candidates := s.buildCandidates(observation.ID, event, scoredResults, best)
		s.storeObservation(event, observation, candidates)
	}()
}

type scoredResult struct {
	Result maclookup.Result
	Score  int
}

func (s *Service) scoreResults(ctx context.Context, results []maclookup.Result) []scoredResult {
	scored := make([]scoredResult, 0, len(results))
	for _, result := range results {
		if !result.Found {
			continue
		}
		scored = append(scored, scoredResult{
			Result: result,
			Score:  s.scoreResult(ctx, result),
		})
	}
	return scored
}

func (s *Service) chooseBestResult(results []scoredResult) (maclookup.Result, bool) {
	var (
		best      maclookup.Result
		bestScore int
		found     bool
	)

	for _, result := range results {
		if !found || result.Score > bestScore {
			best = result.Result
			bestScore = result.Score
			found = true
		}
	}

	return best, found
}

func (s *Service) buildCandidates(observationID string, event dhcpevent.Event, results []scoredResult, best maclookup.Result) []macobservation.Candidate {
	now := time.Now().UTC()
	candidates := make([]macobservation.Candidate, 0, len(results))
	for _, item := range results {
		result := item.Result
		candidates = append(candidates, macobservation.Candidate{
			ID:                   uuid.NewString(),
			ObservationID:        observationID,
			DHCPEventID:          event.ID,
			MACAddress:           event.MACAddress,
			SourceType:           sourceTypeSNMPTrace,
			Confidence:           s.determineSNMPConfidence(context.Background(), result),
			SwitchID:             result.SwitchID,
			SwitchName:           result.SwitchName,
			ManagementIP:         result.ManagementIP,
			BridgePort:           result.BridgePort,
			IfIndex:              result.IfIndex,
			InterfaceName:        result.InterfaceName,
			InterfaceDescription: result.InterfaceDescription,
			Score:                item.Score,
			IsSelected:           result.SwitchID == best.SwitchID && result.IfIndex == best.IfIndex && result.InterfaceName == best.InterfaceName,
			ObservedAt:           event.ObservedAt,
			CreatedAt:            now,
		})
	}
	return candidates
}

func (s *Service) scoreResult(ctx context.Context, result maclookup.Result) int {
	score := 0

	switchName := strings.ToLower(strings.TrimSpace(result.SwitchName))
	iface := strings.ToLower(strings.TrimSpace(result.InterfaceName))
	descr := strings.ToLower(strings.TrimSpace(result.InterfaceDescription))

	if result.BridgePort > 0 {
		score += 20
	}
	if result.IfIndex > 0 {
		score += 20
	}

	if !strings.Contains(switchName, "core") {
		score += 40
	} else {
		score -= 20
	}

	if isAccessLikeInterface(iface, descr) {
		score += 60
	}

	if isLikelyUplinkInterface(iface, descr) {
		score -= 50
	}

	if s.topology != nil && result.SwitchID != "" && result.InterfaceName != "" {
		linked, err := s.topology.HasLinkedInterface(ctx, result.SwitchID, result.InterfaceName)
		if err == nil && linked {
			score -= 120
		}
	}

	return score
}

func isAccessLikeInterface(iface, descr string) bool {
	combined := iface + " " + descr
	return strings.Contains(combined, "gigabitethernet") ||
		strings.Contains(combined, "fastethernet") ||
		strings.Contains(combined, "ethernet") ||
		strings.Contains(combined, "ge0/0/")
}

func isLikelyUplinkInterface(iface, descr string) bool {
	for _, candidate := range []string{iface, descr} {
		candidate = strings.TrimSpace(strings.ToLower(candidate))
		if candidate == "" {
			continue
		}

		if strings.Contains(candidate, "uplink") || strings.Contains(candidate, "trunk") || strings.Contains(candidate, "core") {
			return true
		}

		if portNumber, err := strconv.Atoi(candidate); err == nil {
			if portNumber >= 48 {
				return true
			}
		}
	}

	return false
}

func (s *Service) resolveRelayObservation(event dhcpevent.Event) (macobservation.Observation, bool) {
	if s.relaySwitches == nil || s.snmpClient == nil {
		return macobservation.Observation{}, false
	}
	if strings.TrimSpace(event.RelaySwitchID) == "" {
		return macobservation.Observation{}, false
	}

	bridgePort, err := strconv.Atoi(strings.TrimSpace(event.Option82CircuitID))
	if err != nil || bridgePort <= 0 {
		return macobservation.Observation{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	relaySwitch, err := s.relaySwitches.FindByID(ctx, event.RelaySwitchID)
	if err != nil || relaySwitch == nil {
		return macobservation.Observation{}, false
	}

	result, err := s.snmpClient.ResolveBridgePort(ctx, snmp.SwitchTarget{
		Address:   relaySwitch.ManagementIP,
		Port:      uint16(relaySwitch.SNMPPort),
		Community: relaySwitch.SNMPCommunity,
		Timeout:   time.Duration(relaySwitch.SNMPTimeoutMS) * time.Millisecond,
		Retries:   relaySwitch.SNMPRetries,
	}, bridgePort)
	if err != nil || result.IfIndex <= 0 {
		return macobservation.Observation{}, false
	}

	return macobservation.Observation{
		ID:                   uuid.NewString(),
		DHCPEventID:          event.ID,
		MACAddress:           event.MACAddress,
		SourceType:           sourceTypeOption82,
		Confidence:           confidenceStrong,
		SwitchID:             relaySwitch.ID,
		SwitchName:           relaySwitch.Name,
		ManagementIP:         relaySwitch.ManagementIP,
		BridgePort:           bridgePort,
		IfIndex:              result.IfIndex,
		InterfaceName:        result.InterfaceName,
		InterfaceDescription: result.InterfaceDescription,
		ObservedAt:           event.ObservedAt,
		CreatedAt:            time.Now().UTC(),
	}, true
}

func (s *Service) traceTopologyAware(ctx context.Context, macAddress string, start maclookup.Result) maclookup.Result {
	if s.topology == nil || s.relaySwitches == nil || s.snmpClient == nil {
		return start
	}

	visited := map[string]struct{}{}
	current := start
	for {
		switchID := strings.TrimSpace(current.SwitchID)
		interfaceName := strings.TrimSpace(current.InterfaceName)
		if switchID == "" || interfaceName == "" {
			return current
		}
		if _, seen := visited[switchID]; seen {
			return current
		}
		visited[switchID] = struct{}{}

		nextSwitchID, err := s.topology.FindLinkedSwitchID(ctx, switchID, interfaceName)
		if err != nil || strings.TrimSpace(nextSwitchID) == "" || nextSwitchID == switchID {
			return current
		}

		nextSwitch, err := s.relaySwitches.FindByID(ctx, nextSwitchID)
		if err != nil || nextSwitch == nil {
			return current
		}

		nextResult, ok := s.lookupMACOnSwitch(ctx, macAddress, *nextSwitch)
		if !ok {
			return current
		}

		current = nextResult
	}
}

func (s *Service) determineSNMPConfidence(ctx context.Context, result maclookup.Result) string {
	if s.topology == nil || strings.TrimSpace(result.SwitchID) == "" || strings.TrimSpace(result.InterfaceName) == "" {
		return confidenceDerived
	}

	count, err := s.topology.CountLinkedSwitches(ctx, result.SwitchID, result.InterfaceName)
	if err != nil {
		return confidenceDerived
	}
	if count > 1 {
		return confidenceAmbiguous
	}

	return confidenceDerived
}

func (s *Service) lookupMACOnSwitch(ctx context.Context, macAddress string, asset switchasset.Switch) (maclookup.Result, bool) {
	result := maclookup.Result{
		SwitchID:     asset.ID,
		SwitchName:   asset.Name,
		ManagementIP: asset.ManagementIP,
		Vendor:       asset.Vendor,
		Model:        asset.Model,
		MACAddress:   strings.ToUpper(strings.TrimSpace(macAddress)),
	}

	response, err := s.snmpClient.LookupMAC(ctx, snmp.SwitchTarget{
		Address:   normalizeSNMPTargetAddress(asset.ManagementIP),
		Port:      uint16(asset.SNMPPort),
		Community: asset.SNMPCommunity,
		Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
		Retries:   asset.SNMPRetries,
	}, macAddress)
	if err != nil {
		return result, false
	}

	result.BridgePort = response.BridgePort
	result.IfIndex = response.IfIndex
	result.InterfaceName = response.InterfaceName
	result.InterfaceDescription = response.InterfaceDescription
	result.Found = response.BridgePort > 0 && response.IfIndex > 0

	return result, result.Found
}

func normalizeSNMPTargetAddress(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexByte(value, '/'); idx > 0 {
		return value[:idx]
	}
	return value
}

func (s *Service) storeObservation(event dhcpevent.Event, observation macobservation.Observation, candidates []macobservation.Candidate) {
	insertCtx, cancelInsert := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelInsert()

	if _, err := s.repository.Insert(insertCtx, observation); err != nil {
		s.logger.Error("mac observation insert failed", "mac_address", event.MACAddress, "switch_id", observation.SwitchID, "error", err)
		return
	}

	if len(candidates) > 0 {
		candidateCtx, cancelCandidates := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelCandidates()

		if err := s.repository.InsertCandidates(candidateCtx, candidates); err != nil {
			s.logger.Error("mac observation candidates insert failed", "mac_address", event.MACAddress, "error", err)
		}
	}

	if s.devices != nil {
		deviceCtx, cancelDevice := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelDevice()

		if err := s.devices.UpsertFromObservation(deviceCtx, event, observation); err != nil {
			s.logger.Error("device inventory upsert failed", "mac_address", event.MACAddress, "switch_id", observation.SwitchID, "error", err)
			return
		}
	}

	s.logger.Info(
		"mac correlation stored",
		"mac_address", event.MACAddress,
		"switch_name", observation.SwitchName,
		"interface_name", observation.InterfaceName,
		"source_type", observation.SourceType,
		"confidence", observation.Confidence,
	)
}
