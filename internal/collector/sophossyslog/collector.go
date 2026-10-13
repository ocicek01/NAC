package sophossyslog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"nac/internal/config"
	domain "nac/internal/domain/sophosauth"
)

type processor interface {
	Process(ctx context.Context, event domain.ParsedEvent) error
	ProcessDue(ctx context.Context, limit int) error
	RecordRejectedSource(ctx context.Context, sourceIP string)
	RecordParseFailure(ctx context.Context, sourceIP string, err error)
	RecordQueueDrop(ctx context.Context, sourceIP string, payloadBytes int)
}

type Collector struct {
	cfg       config.SophosAuthConfig
	logger    *slog.Logger
	processor processor
	allowed   []ipRange
}

type ipRange struct {
	ip      net.IP
	network *net.IPNet
}

type datagram struct {
	sourceIP net.IP
	payload  []byte
}

type dropNotice struct {
	sourceIP     string
	payloadBytes int
}

type udpReader interface {
	SetReadDeadline(time.Time) error
	ReadFromUDP([]byte) (int, *net.UDPAddr, error)
}

const (
	defaultWorkerCount = 4
	defaultQueueSize   = 256
)

func New(cfg config.SophosAuthConfig, logger *slog.Logger, processor processor) (*Collector, error) {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = defaultWorkerCount
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	allowed := make([]ipRange, 0, len(cfg.AllowedSources))
	for _, entry := range cfg.AllowedSources {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				return nil, fmt.Errorf("invalid Sophos syslog source CIDR %q: %w", entry, err)
			}
			allowed = append(allowed, ipRange{network: network})
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, fmt.Errorf("invalid Sophos syslog source IP %q", entry)
		}
		allowed = append(allowed, ipRange{ip: ip})
	}
	return &Collector{cfg: cfg, logger: logger, processor: processor, allowed: allowed}, nil
}

func (c *Collector) Start(ctx context.Context) error {
	address := net.JoinHostPort(c.cfg.BindHost, fmt.Sprint(c.cfg.Port))
	resolved, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", resolved)
	if err != nil {
		return err
	}
	defer conn.Close()
	c.logger.Info("Sophos syslog collector started", "addr", address)
	return c.serve(ctx, conn)
}

func (c *Collector) serve(ctx context.Context, conn udpReader) error {
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	jobs := make(chan datagram, c.cfg.QueueSize)
	drops := make(chan dropNotice, c.cfg.QueueSize)
	var workers sync.WaitGroup

	for range c.cfg.WorkerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			c.runWorker(workerCtx, jobs)
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		c.runDropAudit(workerCtx, drops)
	}()
	workers.Add(1)
	go func() {
		defer workers.Done()
		c.runRetries(workerCtx)
	}()

	wakeDone := make(chan struct{})
	wakeReader := context.AfterFunc(workerCtx, func() {
		_ = conn.SetReadDeadline(time.Now())
		close(wakeDone)
	})
	defer func() {
		cancelWorkers()
		if !wakeReader() {
			<-wakeDone
		}
		close(jobs)
		close(drops)
		workers.Wait()
	}()

	buffer := make([]byte, 65535)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return err
		}
		n, remote, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				continue
			}
			return err
		}
		job := datagram{sourceIP: append(net.IP(nil), remote.IP...), payload: append([]byte(nil), buffer[:n]...)}
		select {
		case <-workerCtx.Done():
			return nil
		case jobs <- job:
		default:
			c.logger.Warn("Sophos syslog processing queue full; datagram dropped", "source_ip", remote.IP.String(), "payload_bytes", n, "queue_size", c.cfg.QueueSize)
			select {
			case drops <- dropNotice{sourceIP: remote.IP.String(), payloadBytes: n}:
			default:
				c.logger.Warn("Sophos syslog drop audit queue full", "source_ip", remote.IP.String(), "payload_bytes", n)
			}
		}
	}
}

func (c *Collector) runWorker(ctx context.Context, jobs <-chan datagram) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}
			c.processDatagram(ctx, job.sourceIP, job.payload)
		}
	}
}

func (c *Collector) runDropAudit(ctx context.Context, drops <-chan dropNotice) {
	for {
		select {
		case <-ctx.Done():
			return
		case drop, ok := <-drops:
			if !ok {
				return
			}
			timeout := time.Duration(c.cfg.ProcessingTimeoutSec) * time.Second
			auditCtx, cancel := context.WithTimeout(ctx, timeout)
			c.processor.RecordQueueDrop(auditCtx, drop.sourceIP, drop.payloadBytes)
			cancel()
		}
	}
}

func (c *Collector) runRetries(ctx context.Context) {
	interval := time.Duration(c.cfg.RetryIntervalSec) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			timeout := time.Duration(c.cfg.ProcessingTimeoutSec) * time.Second
			workerCtx, cancel := context.WithTimeout(ctx, timeout*time.Duration(max(1, c.cfg.RetryBatchSize)))
			err := c.processor.ProcessDue(workerCtx, c.cfg.RetryBatchSize)
			cancel()
			if err != nil && ctx.Err() == nil {
				c.logger.Warn("Sophos auth retry worker failed", "error", err)
			}
		}
	}
}

func (c *Collector) processDatagram(parent context.Context, sourceIP net.IP, payload []byte) {
	timeout := time.Duration(c.cfg.ProcessingTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if !c.sourceAllowed(sourceIP) {
		c.logger.Warn("Sophos syslog source rejected", "source_ip", sourceIP.String())
		c.processor.RecordRejectedSource(ctx, sourceIP.String())
		return
	}
	event, err := Parse(payload, time.Now())
	if err != nil {
		if !errors.Is(err, errNotAuthentication) {
			c.processor.RecordParseFailure(ctx, sourceIP.String(), err)
		}
		return
	}
	if err := c.processor.Process(ctx, event); err != nil {
		c.logger.Warn("Sophos auth event processing failed", "source_ip", sourceIP.String(), "error", err)
	}
}

func (c *Collector) sourceAllowed(ip net.IP) bool {
	for _, entry := range c.allowed {
		if entry.ip != nil && entry.ip.Equal(ip) || entry.network != nil && entry.network.Contains(ip) {
			return true
		}
	}
	return false
}
