package portendpoint

import (
	"context"
	"fmt"
	"sort"
	"strings"

	portendpointdomain "nac/internal/domain/portendpoint"
	switchasset "nac/internal/domain/switchasset"
	switchport "nac/internal/domain/switchport"
)

type Service struct {
	switches  switchasset.Repository
	ports     switchport.Repository
	endpoints portendpointdomain.Repository
}

type PortView struct {
	SwitchID      string                        `json:"switch_id"`
	IfIndex       int                           `json:"if_index"`
	PortIndex     int                           `json:"port_index"`
	InterfaceName string                        `json:"interface_name"`
	PortLabel     string                        `json:"port_label"`
	IsUplink      bool                          `json:"is_uplink"`
	MACCount      int                           `json:"mac_count"`
	MACAddresses  []string                      `json:"mac_addresses"`
	Endpoints     []portendpointdomain.Endpoint `json:"endpoints"`
}

func NewService(switches switchasset.Repository, ports switchport.Repository, endpoints portendpointdomain.Repository) *Service {
	return &Service{
		switches:  switches,
		ports:     ports,
		endpoints: endpoints,
	}
}

func (s *Service) ListBySwitch(ctx context.Context, switchID string) ([]PortView, error) {
	switchID = strings.TrimSpace(switchID)
	if switchID == "" {
		return nil, fmt.Errorf("switch id is required")
	}
	if s.switches == nil || s.ports == nil || s.endpoints == nil {
		return nil, fmt.Errorf("port endpoint dependencies are not configured")
	}

	asset, err := s.switches.FindByID(ctx, switchID)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("switch not found")
	}

	ports, err := s.ports.ListBySwitch(ctx, switchID)
	if err != nil {
		return nil, err
	}
	items, err := s.endpoints.ListBySwitch(ctx, switchID)
	if err != nil {
		return nil, err
	}

	byIfIndex := make(map[int][]portendpointdomain.Endpoint, len(items))
	for _, item := range items {
		byIfIndex[item.PortIfIndex] = append(byIfIndex[item.PortIfIndex], item)
	}
	for ifIndex := range byIfIndex {
		sort.Slice(byIfIndex[ifIndex], func(i, j int) bool {
			if byIfIndex[ifIndex][i].MACAddress == byIfIndex[ifIndex][j].MACAddress {
				return byIfIndex[ifIndex][i].IPAddress < byIfIndex[ifIndex][j].IPAddress
			}
			return byIfIndex[ifIndex][i].MACAddress < byIfIndex[ifIndex][j].MACAddress
		})
	}

	result := make([]PortView, 0, len(ports))
	for _, port := range ports {
		result = append(result, PortView{
			SwitchID:      port.SwitchID,
			IfIndex:       port.IfIndex,
			PortIndex:     port.PortIndex,
			InterfaceName: port.InterfaceName,
			PortLabel:     port.PortLabel,
			IsUplink:      port.IsUplink,
			MACCount:      port.MACCount,
			MACAddresses:  append([]string{}, port.MACAddresses...),
			Endpoints:     append([]portendpointdomain.Endpoint{}, byIfIndex[port.IfIndex]...),
		})
	}

	return result, nil
}
