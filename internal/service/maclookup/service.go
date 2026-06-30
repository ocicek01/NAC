package maclookup

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"nac/internal/domain/switchasset"
	"nac/internal/snmp"
)

type Request struct {
	MACAddress string `json:"mac_address"`
}

type Result struct {
	SwitchID             string `json:"switch_id"`
	SwitchName           string `json:"switch_name"`
	ManagementIP         string `json:"management_ip"`
	Vendor               string `json:"vendor"`
	Model                string `json:"model"`
	MACAddress           string `json:"mac_address"`
	Found                bool   `json:"found"`
	BridgePort           int    `json:"bridge_port"`
	IfIndex              int    `json:"if_index"`
	InterfaceName        string `json:"interface_name"`
	InterfaceDescription string `json:"interface_description"`
	Error                string `json:"error,omitempty"`
}

type SwitchRepository interface {
	ListEnabledSNMP(ctx context.Context) ([]switchasset.Switch, error)
}

type Service struct {
	switches SwitchRepository
	client   snmp.Client
}

func NewService(switches SwitchRepository, client snmp.Client) *Service {
	return &Service{
		switches: switches,
		client:   client,
	}
}

func (s *Service) Lookup(ctx context.Context, req Request) ([]Result, error) {
	macAddress := strings.ToUpper(strings.TrimSpace(req.MACAddress))
	if macAddress == "" {
		return nil, fmt.Errorf("mac_address is required")
	}

	assets, err := s.switches.ListEnabledSNMP(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(assets))
	for _, asset := range assets {
		lookupCtx, cancel := context.WithTimeout(ctx, time.Duration(asset.SNMPTimeoutMS+1000)*time.Millisecond)
		targetAddress := normalizeTargetAddress(asset.ManagementIP)
		response, lookupErr := s.client.LookupMAC(lookupCtx, snmp.SwitchTarget{
			Address:   targetAddress,
			Port:      uint16(asset.SNMPPort),
			Community: asset.SNMPCommunity,
			Timeout:   time.Duration(asset.SNMPTimeoutMS) * time.Millisecond,
			Retries:   asset.SNMPRetries,
		}, macAddress)
		cancel()

		result := Result{
			SwitchID:     asset.ID,
			SwitchName:   asset.Name,
			ManagementIP: asset.ManagementIP,
			Vendor:       asset.Vendor,
			Model:        asset.Model,
			MACAddress:   macAddress,
		}

		if lookupErr != nil {
			result.Error = lookupErr.Error()
			results = append(results, result)
			continue
		}

		result.BridgePort = response.BridgePort
		result.IfIndex = response.IfIndex
		result.InterfaceName = response.InterfaceName
		result.InterfaceDescription = response.InterfaceDescription
		result.Found = response.BridgePort > 0 && response.IfIndex > 0
		results = append(results, result)
	}

	return results, nil
}

func normalizeTargetAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}

	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.String()
	}

	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Addr().String()
	}

	if idx := strings.IndexByte(value, '/'); idx > 0 {
		return value[:idx]
	}

	return value
}
