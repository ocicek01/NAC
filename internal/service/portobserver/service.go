package portobserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"nac/internal/domain/switchasset"
	"nac/internal/snmp"
)

type Switches interface {
	ListEnabledSNMP(context.Context) ([]switchasset.Switch, error)
	UpdatePollStatus(context.Context, string, time.Time, string) (switchasset.Switch, error)
}
type Store interface {
	Observe(context.Context, switchasset.Switch, snmp.PortSnapshot, time.Time) (int, error)
}
type Collector interface {
	ObservePorts(context.Context, snmp.SwitchTarget) (snmp.PortSnapshot, error)
}
type Config struct {
	Workers           int
	Interval, Timeout time.Duration
}
type Health struct {
	Heartbeat time.Time `json:"heartbeat"`
	LastCycle time.Time `json:"last_cycle"`
	Active    int       `json:"active"`
	Succeeded uint64    `json:"succeeded"`
	Failed    uint64    `json:"failed"`
	Skipped   uint64    `json:"skipped"`
	Changes   uint64    `json:"changes"`
	LastError string    `json:"last_error"`
}
type Service struct {
	switches  Switches
	store     Store
	collector Collector
	cfg       Config
	mu        sync.Mutex
	health    Health
	active    map[string]struct{}
	nextDue   map[string]time.Time
	cursor    int
	outbox    Outbox
	deliverer Deliverer
}

func (s *Service) ConfigureDelivery(outbox Outbox, deliverer Deliverer) {
	s.outbox, s.deliverer = outbox, deliverer
}

func New(s Switches, store Store, c Collector, cfg Config) (*Service, error) {
	if cfg.Workers < 1 || cfg.Workers > 64 || cfg.Interval < time.Second || cfg.Timeout < time.Second {
		return nil, fmt.Errorf("invalid port observer configuration")
	}
	return &Service{switches: s, store: store, collector: c, cfg: cfg, active: map[string]struct{}{}, nextDue: map[string]time.Time{}}, nil
}
func (s *Service) Health() Health { s.mu.Lock(); defer s.mu.Unlock(); return s.health }
func (s *Service) Run(ctx context.Context) {
	// Exactly Workers collection goroutines and a bounded queue. Every switch has
	// its own due time, so one slow inventory cycle cannot delay all other switches.
	jobs := make(chan switchasset.Switch, s.cfg.Workers)
	var workers sync.WaitGroup
	for i := 0; i < s.cfg.Workers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for asset := range jobs {
				s.poll(ctx, asset)
			}
		}()
	}
	defer func() { close(jobs); workers.Wait() }()
	heartbeatCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.runDeliveryLoop(heartbeatCtx)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case now := <-ticker.C:
				s.mu.Lock()
				s.health.Heartbeat = now.UTC()
				s.mu.Unlock()
			}
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		listCtx, done := context.WithTimeout(ctx, s.cfg.Timeout)
		assets, err := s.switches.ListEnabledSNMP(listCtx)
		done()
		if err != nil {
			s.mu.Lock()
			s.health.LastError = err.Error()
			s.mu.Unlock()
			slog.Error("observer inventory", "error", err)
		}
		now := time.Now()
		seen := map[string]bool{}
		unique := make([]switchasset.Switch, 0, len(assets))
		for _, asset := range assets {
			if seen[asset.ID] {
				continue
			}
			seen[asset.ID] = true
			unique = append(unique, asset)
		}
		s.mu.Lock()
		start := 0
		if len(unique) > 0 {
			start = s.cursor % len(unique)
			s.cursor = (s.cursor + 1) % len(unique)
		}
		s.mu.Unlock()
		for offset := 0; offset < len(unique); offset++ {
			asset := unique[(start+offset)%len(unique)]
			s.mu.Lock()
			_, busy := s.active[asset.ID]
			due := s.nextDue[asset.ID]
			if !busy && (due.IsZero() || !now.Before(due)) {
				s.active[asset.ID] = struct{}{}
				s.nextDue[asset.ID] = now.Add(s.cfg.Interval)
			} else {
				busy = true
			}
			s.mu.Unlock()
			if busy {
				continue
			}
			select {
			case jobs <- asset:
			case <-ctx.Done():
				s.mu.Lock()
				delete(s.active, asset.ID)
				s.mu.Unlock()
			default:
				s.mu.Lock()
				delete(s.active, asset.ID)
				s.nextDue[asset.ID] = now.Add(time.Second)
				s.health.Skipped++
				s.mu.Unlock()
			}
			if ctx.Err() != nil {
				break
			}
		}
		s.mu.Lock()
		s.health.LastCycle = time.Now().UTC()
		if err == nil {
			s.health.LastError = ""
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) runDeliveryLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatchPending(ctx)
		}
	}
}

func (s *Service) dispatchPending(parent context.Context) {
	if s.outbox == nil || s.deliverer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	events, err := s.outbox.PendingPortEvents(ctx, 25)
	if err != nil {
		slog.Warn("observer outbox read failed", "error", err)
		return
	}
	for _, event := range events {
		if err = s.deliverer.Deliver(ctx, event); err != nil {
			var deliveryErr *DeliveryError
			permanent := errors.As(err, &deliveryErr) && deliveryErr.Permanent
			retryCtx, retryCancel := context.WithTimeout(context.WithoutCancel(parent), 3*time.Second)
			_ = s.outbox.RetryPortEvent(retryCtx, event.ID, err.Error(), permanent)
			retryCancel()
			continue
		}
		if err = s.outbox.MarkPortEventDelivered(ctx, event.ID); err != nil {
			slog.Warn("observer outbox acknowledgement failed", "event_id", event.ID, "error", err)
		}
	}
}
func (s *Service) poll(parent context.Context, asset switchasset.Switch) {
	if parent.Err() != nil {
		s.mu.Lock()
		delete(s.active, asset.ID)
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.health.Active++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.health.Active--
		delete(s.active, asset.ID)
		s.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(parent, s.cfg.Timeout)
	defer cancel()
	var warnings []string
	started := time.Now().UTC()
	if asset.SNMPVersion != "" && asset.SNMPVersion != "2c" && asset.SNMPVersion != "v2c" && asset.SNMPVersion != "2" {
		s.recordResult(parent, asset, 0, nil, fmt.Errorf("observer supports SNMP v2c; configured version %s", asset.SNMPVersion))
		return
	}
	timeout := time.Duration(asset.SNMPTimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > 3*time.Second {
		timeout = 2 * time.Second
	}
	port := asset.SNMPPort
	if port <= 0 || port > 65535 {
		port = 161
	}
	retries := asset.SNMPRetries
	if retries < 0 {
		retries = 0
	}
	if retries > 1 {
		retries = 1
	}
	snapshot, err := s.collector.ObservePorts(ctx, snmp.SwitchTarget{Address: asset.ManagementIP, Port: uint16(port), Community: asset.SNMPCommunity, Timeout: timeout, Retries: retries, Vendor: asset.Vendor, Model: asset.Model})
	warnings = snapshot.Warnings
	changes := 0
	if err == nil {
		changes, err = s.store.Observe(ctx, asset, snapshot, started)
	}
	s.recordResult(parent, asset, changes, warnings, err)
}

func (s *Service) recordResult(parent context.Context, asset switchasset.Switch, changes int, warnings []string, err error) {
	message := strings.Join(warnings, "; ")
	if err != nil {
		message = err.Error()
	}
	// A timed-out scan still records failure, using a separate bounded context.
	statusCtx, done := context.WithTimeout(parent, 3*time.Second)
	_, statusErr := s.switches.UpdatePollStatus(statusCtx, asset.ID, time.Now().UTC(), message)
	done()
	if statusErr != nil {
		err = fmt.Errorf("poll health write: %w", statusErr)
	}
	s.mu.Lock()
	if err != nil {
		s.health.Failed++
	} else {
		s.health.Succeeded++
	}
	s.health.Changes += uint64(changes)
	s.mu.Unlock()
	if err != nil {
		slog.Warn("observer scan failed", "switch_id", asset.ID, "error", err)
	}
}
