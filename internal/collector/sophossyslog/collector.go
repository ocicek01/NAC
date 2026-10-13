package sophossyslog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"nac/internal/config"
	domain "nac/internal/domain/sophosauth"
)

type processor interface {
	Process(ctx context.Context, event domain.ParsedEvent) error
	RecordRejectedSource(ctx context.Context, sourceIP string)
	RecordParseFailure(ctx context.Context, sourceIP string, err error)
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

func New(cfg config.SophosAuthConfig, logger *slog.Logger, processor processor) (*Collector, error) {
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
		c.processDatagram(ctx, remote.IP, append([]byte(nil), buffer[:n]...))
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
