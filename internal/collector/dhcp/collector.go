package dhcp

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"nac/internal/config"
	domain "nac/internal/domain/dhcpevent"
)

type EventIngestor interface {
	Ingest(ctx context.Context, event domain.Event) (domain.Event, error)
}

type Collector struct {
	config   config.DHCPCollectorConfig
	logger   *slog.Logger
	ingestor EventIngestor
}

func New(config config.DHCPCollectorConfig, logger *slog.Logger, ingestor EventIngestor) *Collector {
	return &Collector{
		config:   config,
		logger:   logger,
		ingestor: ingestor,
	}
}

func (c *Collector) Start(ctx context.Context) error {
	if !c.config.Enabled {
		c.logger.Info("dhcp collector disabled")
		return nil
	}

	if c.ingestor == nil {
		return fmt.Errorf("dhcp collector ingestor is nil")
	}

	handle, err := pcap.OpenLive(
		c.config.Interface,
		c.config.SnapshotLen,
		c.config.Promiscuous,
		pcap.BlockForever,
	)
	if err != nil {
		return err
	}
	defer handle.Close()

	if err := handle.SetBPFFilter("udp and (port 67 or port 68)"); err != nil {
		return err
	}

	c.logger.Info(
		"dhcp collector started",
		"interface", c.config.Interface,
		"promiscuous", c.config.Promiscuous,
		"snapshot_len", c.config.SnapshotLen,
	)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetChan := packetSource.Packets()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("dhcp collector stopping", "reason", ctx.Err())
			return nil
		case packet, ok := <-packetChan:
			if !ok {
				return nil
			}

			event, ok := c.packetToEvent(packet)
			if !ok {
				continue
			}

			if _, err := c.ingestor.Ingest(ctx, event); err != nil {
				if errors.Is(err, domain.ErrDuplicateSuppressed) {
					c.logger.Info(
						"dhcp duplicate suppressed",
						"mac_address", event.MACAddress,
						"message_type", event.MessageType,
						"source_ip", event.SourceIP,
					)
					continue
				}
				c.logger.Error("failed to ingest dhcp event", "error", err)
				continue
			}

			c.logger.Info(
				"dhcp event ingested",
				"mac_address", event.MACAddress,
				"message_type", event.MessageType,
				"source_ip", event.SourceIP,
			)
		}
	}
}

func (c *Collector) packetToEvent(packet gopacket.Packet) (domain.Event, bool) {
	dhcpLayer := packet.Layer(layers.LayerTypeDHCPv4)
	if dhcpLayer == nil {
		return domain.Event{}, false
	}

	dhcpPacket, ok := dhcpLayer.(*layers.DHCPv4)
	if !ok {
		return domain.Event{}, false
	}

	messageType := ""
	hostname := ""
	vendorClass := ""
	option82Raw := ""
	option82CircuitID := ""
	option82RemoteID := ""
	option82VLAN := ""
	requestedIP := ""

	for _, option := range dhcpPacket.Options {
		switch option.Type {
		case layers.DHCPOptMessageType:
			if len(option.Data) > 0 {
				messageType = dhcpMessageType(option.Data[0])
			}
		case layers.DHCPOptHostname:
			hostname = string(option.Data)
		case layers.DHCPOptClassID:
			vendorClass = string(option.Data)
		case layers.DHCPOptRequestIP:
			if len(option.Data) == 4 {
				requestedIP = net.IP(option.Data).String()
			}
		case layers.DHCPOpt(82):
			option82Raw = hex.EncodeToString(option.Data)
			option82CircuitID, option82RemoteID, option82VLAN = parseOption82(option.Data)
		}
	}

	if messageType != "DISCOVER" && messageType != "REQUEST" {
		return domain.Event{}, false
	}

	sourceIP := ""
	if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
		if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
			sourceIP = ipv4Packet.SrcIP.String()
		}
	}

	clientIP := ""
	if ip := dhcpPacket.ClientIP; ip != nil {
		if normalized := ip.String(); normalized != "<nil>" && normalized != "0.0.0.0" {
			clientIP = normalized
		}
	}

	yourIP := ""
	if ip := dhcpPacket.YourClientIP; ip != nil {
		if normalized := ip.String(); normalized != "<nil>" && normalized != "0.0.0.0" {
			yourIP = normalized
		}
	}

	macAddress := normalizeMAC(dhcpPacket.ClientHWAddr)
	if macAddress == "" {
		return domain.Event{}, false
	}

	return domain.Event{
		MACAddress:        macAddress,
		TransactionID:     fmt.Sprintf("%08x", dhcpPacket.Xid),
		SourceIP:          sourceIP,
		ClientIP:          clientIP,
		YourIP:            yourIP,
		RequestedIP:       strings.TrimSpace(requestedIP),
		MessageType:       messageType,
		Hostname:          hostname,
		VendorClass:       vendorClass,
		Option82Raw:       option82Raw,
		Option82CircuitID: option82CircuitID,
		Option82RemoteID:  option82RemoteID,
		Option82VLAN:      option82VLAN,
		RelayIP:           normalizeRelayIP(sourceIP),
		ObservedAt:        time.Now().UTC(),
	}, true
}

func normalizeRelayIP(sourceIP string) string {
	sourceIP = strings.TrimSpace(sourceIP)
	if sourceIP == "" || sourceIP == "0.0.0.0" {
		return ""
	}

	return sourceIP
}

func parseOption82(data []byte) (string, string, string) {
	var circuitID, remoteID, vlan string

	for i := 0; i+1 < len(data); {
		subOption := data[i]
		length := int(data[i+1])
		i += 2

		if i+length > len(data) {
			break
		}

		payload := data[i : i+length]
		i += length

		switch subOption {
		case 1:
			circuitID = decodeOption82CircuitID(payload)
			if detectedVLAN := extractVLANFromOption82(circuitID); detectedVLAN != "" {
				vlan = detectedVLAN
			}
		case 2:
			if len(payload) == 6 {
				remoteID = normalizeMAC(net.HardwareAddr(payload))
			} else {
				remoteID = sanitizeOption82String(payload)
			}
		}
	}

	return circuitID, remoteID, vlan
}

func decodeOption82CircuitID(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	if isPrintableOption82(data) {
		return sanitizeOption82String(data)
	}

	switch len(data) {
	case 1:
		return strconv.Itoa(int(data[0]))
	case 2:
		return strconv.Itoa(int(binary.BigEndian.Uint16(data)))
	case 4:
		return strconv.Itoa(int(binary.BigEndian.Uint32(data)))
	default:
		return strings.ToUpper(hex.EncodeToString(data))
	}
}

func sanitizeOption82String(data []byte) string {
	var builder strings.Builder
	for _, b := range data {
		if b >= 32 && b <= 126 {
			builder.WriteByte(b)
		}
	}

	return strings.TrimSpace(builder.String())
}

func isPrintableOption82(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	for _, b := range data {
		if b < 32 || b > 126 {
			return false
		}
	}

	return true
}

func extractVLANFromOption82(circuitID string) string {
	lower := strings.ToLower(strings.TrimSpace(circuitID))
	if lower == "" || !strings.Contains(lower, "vlan") {
		return ""
	}

	for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= '0' && r <= '9')
	}) {
		if token == "" {
			continue
		}

		if _, err := strconv.Atoi(token); err == nil {
			return token
		}
	}

	return ""
}

func dhcpMessageType(value byte) string {
	switch value {
	case 1:
		return "DISCOVER"
	case 2:
		return "OFFER"
	case 3:
		return "REQUEST"
	case 4:
		return "DECLINE"
	case 5:
		return "ACK"
	case 6:
		return "NAK"
	case 7:
		return "RELEASE"
	case 8:
		return "INFORM"
	default:
		return "UNKNOWN"
	}
}

func normalizeMAC(hwAddr net.HardwareAddr) string {
	if len(hwAddr) < 6 {
		return ""
	}

	return strings.ToUpper(hwAddr.String())
}
