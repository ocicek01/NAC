package snmptrap

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"

	"nac/internal/config"
	domain "nac/internal/domain/snmptrap"
)

type EventIngestor interface {
	Ingest(ctx context.Context, event domain.Event) (domain.Event, error)
}

type Collector struct {
	config   config.SNMPTrapConfig
	logger   *slog.Logger
	ingestor EventIngestor
}

func New(config config.SNMPTrapConfig, logger *slog.Logger, ingestor EventIngestor) *Collector {
	return &Collector{
		config:   config,
		logger:   logger,
		ingestor: ingestor,
	}
}

func (c *Collector) Start(ctx context.Context) error {
	if !c.config.Enabled {
		c.logger.Info("snmp trap collector disabled")
		return nil
	}
	if c.ingestor == nil {
		return fmt.Errorf("snmp trap collector ingestor is nil")
	}

	listener := gosnmp.NewTrapListener()
	listener.Params = gosnmp.Default
	listener.OnNewTrap = func(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		if packet == nil || addr == nil {
			return
		}

		event := packetToEvent(packet, addr)
		if _, err := c.ingestor.Ingest(context.Background(), event); err != nil {
			c.logger.Error("failed to ingest snmp trap", "error", err, "source_ip", event.SourceIP)
			return
		}

		c.logger.Info(
			"snmp trap ingested",
			"source_ip", event.SourceIP,
			"trap_oid", event.TrapOID,
			"enterprise_oid", event.EnterpriseOID,
		)
	}

	errCh := make(chan error, 1)
	go func() {
		addr := net.JoinHostPort(c.config.BindHost, strconv.Itoa(c.config.Port))
		c.logger.Info("snmp trap collector started", "addr", addr)
		errCh <- listener.Listen(addr)
	}()

	select {
	case <-ctx.Done():
		c.logger.Info("snmp trap collector stopping", "reason", ctx.Err())
		listener.Close()
		return nil
	case err := <-errCh:
		if err == nil || strings.Contains(strings.ToLower(err.Error()), "use of closed network connection") {
			return nil
		}
		return err
	}
}

func packetToEvent(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) domain.Event {
	varBinds := make([]domain.VarBind, 0, len(packet.Variables))
	trapOID := ""
	for _, variable := range packet.Variables {
		oid := strings.TrimSpace(variable.Name)
		value := formatTrapValue(variable.Value)
		varBinds = append(varBinds, domain.VarBind{
			OID:   oid,
			Type:  variable.Type.String(),
			Value: value,
		})
		if oid == ".1.3.6.1.6.3.1.1.4.1.0" || oid == "1.3.6.1.6.3.1.1.4.1.0" {
			trapOID = value
		}
	}

	return domain.Event{
		SourceIP:      addr.IP.String(),
		SourcePort:    addr.Port,
		SNMPVersion:   packet.Version.String(),
		Community:     strings.TrimSpace(packet.Community),
		TrapOID:       strings.TrimSpace(deriveTrapOID(packet, trapOID)),
		EnterpriseOID: strings.TrimSpace(packet.Enterprise),
		GenericTrap:   int(packet.GenericTrap),
		SpecificTrap:  int(packet.SpecificTrap),
		UptimeTicks:   uint32(packet.Timestamp),
		VarBinds:      varBinds,
		ReceivedAt:    time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
	}
}

func formatTrapValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []byte:
		if len(typed) == 0 {
			return ""
		}
		printable := true
		for _, b := range typed {
			if b < 32 || b > 126 {
				printable = false
				break
			}
		}
		if printable {
			return strings.TrimSpace(string(typed))
		}
		encoded := strings.ToLower(hex.EncodeToString(typed))
		if len(encoded)%2 != 0 {
			return encoded
		}
		parts := make([]string, 0, len(encoded)/2)
		for i := 0; i < len(encoded); i += 2 {
			parts = append(parts, encoded[i:i+2])
		}
		return strings.Join(parts, "_")
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func deriveTrapOID(packet *gosnmp.SnmpPacket, trapOID string) string {
	trapOID = strings.TrimSpace(trapOID)
	if trapOID != "" {
		return trapOID
	}

	switch int(packet.GenericTrap) {
	case 0:
		return ".1.3.6.1.6.3.1.1.5.1"
	case 1:
		return ".1.3.6.1.6.3.1.1.5.2"
	case 2:
		return ".1.3.6.1.6.3.1.1.5.3"
	case 3:
		return ".1.3.6.1.6.3.1.1.5.4"
	case 4:
		return ".1.3.6.1.6.3.1.1.5.5"
	case 5:
		return ".1.3.6.1.6.3.1.1.5.6"
	default:
		return ""
	}
}
