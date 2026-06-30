package switchport

import (
	"context"
	"fmt"
	"strings"

	switchasset "nac/internal/domain/switchasset"
	domain "nac/internal/domain/switchport"
)

type Service struct {
	switches switchasset.Repository
	ports    domain.Repository
}

type Summary struct {
	SwitchID         string `json:"switch_id"`
	TotalPorts       int    `json:"total_ports"`
	UplinkPorts      int    `json:"uplink_ports"`
	TrunkPorts       int    `json:"trunk_ports"`
	NeighborPorts    int    `json:"neighbor_ports"`
	PortsWithMACs    int    `json:"ports_with_macs"`
	TotalLearnedMACs int    `json:"total_learned_macs"`
	TopMACPortIndex  int    `json:"top_mac_port_index"`
	TopMACPortName   string `json:"top_mac_port_name"`
	TopMACCount      int    `json:"top_mac_count"`
}

func NewService(switches switchasset.Repository, ports domain.Repository) *Service {
	return &Service{switches: switches, ports: ports}
}

func (s *Service) ListBySwitch(ctx context.Context, switchID string) ([]domain.Port, error) {
	switchID = strings.TrimSpace(switchID)
	if switchID == "" {
		return nil, fmt.Errorf("switch id is required")
	}
	if s.switches != nil {
		asset, err := s.switches.FindByID(ctx, switchID)
		if err != nil {
			return nil, err
		}
		if asset == nil {
			return nil, fmt.Errorf("switch not found")
		}
	}
	return s.ports.ListBySwitch(ctx, switchID)
}

func (s *Service) SummaryBySwitch(ctx context.Context, switchID string) (*Summary, error) {
	ports, err := s.ListBySwitch(ctx, switchID)
	if err != nil {
		return nil, err
	}

	summary := &Summary{SwitchID: strings.TrimSpace(switchID), TotalPorts: len(ports)}
	for _, port := range ports {
		if port.IsUplink {
			summary.UplinkPorts++
		}
		if port.IsTrunk {
			summary.TrunkPorts++
		}
		if strings.TrimSpace(port.NeighborProtocol) != "" {
			summary.NeighborPorts++
		}
		if port.MACCount > 0 {
			summary.PortsWithMACs++
		}
		summary.TotalLearnedMACs += port.MACCount
		if port.MACCount > summary.TopMACCount {
			summary.TopMACCount = port.MACCount
			summary.TopMACPortIndex = port.PortIndex
			summary.TopMACPortName = strings.TrimSpace(port.InterfaceName)
		}
	}

	return summary, nil
}
