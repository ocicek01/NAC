package snmp

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

const (
	oidSysName           = ".1.3.6.1.2.1.1.5.0"
	oidDot1dTpFdbPort    = ".1.3.6.1.2.1.17.4.3.1.2"
	oidDot1dBaseAddr     = ".1.3.6.1.2.1.17.1.1.0"
	oidDot1dBasePortIfIx = ".1.3.6.1.2.1.17.1.4.1.2"
	oidQBridgeTpFdbPort  = ".1.3.6.1.2.1.17.7.1.2.2.1.2"
	oidIfName            = ".1.3.6.1.2.1.31.1.1.1.1"
	oidIfAlias           = ".1.3.6.1.2.1.31.1.1.1.18"
	oidIfDescr           = ".1.3.6.1.2.1.2.2.1.2"
	oidIfAdminStatus     = ".1.3.6.1.2.1.2.2.1.7"
	oidIfOperStatus      = ".1.3.6.1.2.1.2.2.1.8"
	oidLldpLocPortDesc   = ".1.0.8802.1.1.2.1.3.7.1.4"
	oidLldpLocPortID     = ".1.0.8802.1.1.2.1.3.7.1.3"
	oidLldpRemSysName    = ".1.0.8802.1.1.2.1.4.1.1.9"
	oidLldpRemPortDesc   = ".1.0.8802.1.1.2.1.4.1.1.8"
	oidLldpRemSysDesc    = ".1.0.8802.1.1.2.1.4.1.1.10"
	oidCdpCacheIfIndex   = ".1.3.6.1.4.1.9.9.23.1.2.1.1.1"
	oidCdpCacheDeviceID  = ".1.3.6.1.4.1.9.9.23.1.2.1.1.6"
	oidCdpCachePortID    = ".1.3.6.1.4.1.9.9.23.1.2.1.1.7"
	oidCdpCachePlatform  = ".1.3.6.1.4.1.9.9.23.1.2.1.1.8"
	oidCdpCacheVersion   = ".1.3.6.1.4.1.9.9.23.1.2.1.1.5"
	oidCiscoTrunkState   = ".1.3.6.1.4.1.9.9.46.1.6.1.1.14"
	oidHuaweiTrunkState  = ".1.3.6.1.4.1.2011.5.25.42.1.1.1.3.1.3"
	oidIpNetToMediaPhys  = ".1.3.6.1.2.1.4.22.1.2"
	oidIpNetToPhysical   = ".1.3.6.1.2.1.4.35.1.4"
)

type SwitchTarget struct {
	Address        string
	Port           uint16
	Community      string
	Timeout        time.Duration
	Retries        int
	MaxRepetitions int
	Vendor         string
	Model          string
}

type LookupResult struct {
	BridgePort           int    `json:"bridge_port"`
	IfIndex              int    `json:"if_index"`
	InterfaceName        string `json:"interface_name"`
	InterfaceDescription string `json:"interface_description"`
}

type LLDPNeighbor struct {
	LocalPortName    string `json:"local_port_name"`
	LocalPortIndex   int    `json:"local_port_index"`
	RemoteSystemName string `json:"remote_system_name"`
	RemotePortName   string `json:"remote_port_name"`
	RemotePortDesc   string `json:"remote_port_description"`
	RemoteSystemDesc string `json:"remote_system_description"`
}

type CDPNeighbor struct {
	LocalIfIndex      int    `json:"local_if_index"`
	LocalPortName     string `json:"local_port_name"`
	RemoteSystemName  string `json:"remote_system_name"`
	RemotePortName    string `json:"remote_port_name"`
	RemotePlatform    string `json:"remote_platform"`
	RemoteDescription string `json:"remote_description"`
}

type TrunkPort struct {
	IfIndex int    `json:"if_index"`
	State   int    `json:"state"`
	Vendor  string `json:"vendor"`
}

type FDBEntry struct {
	MACAddress string `json:"mac_address"`
	BridgePort int    `json:"bridge_port"`
	IfIndex    int    `json:"if_index"`
}

type FDBDiscovery struct {
	Entries           []FDBEntry `json:"entries"`
	Source            string     `json:"source"`
	Dot1DRows         int        `json:"dot1d_rows"`
	QBridgeRows       int        `json:"qbridge_rows"`
	BridgePortMapRows int        `json:"bridge_port_map_rows"`
	MappedEntries     int        `json:"mapped_entries"`
}

type ARPEntry struct {
	IfIndex    int    `json:"if_index"`
	IPAddress  string `json:"ip_address"`
	MACAddress string `json:"mac_address"`
	Source     string `json:"source"`
}

type ARPDiscovery struct {
	Entries             []ARPEntry `json:"entries"`
	Source              string     `json:"source"`
	IPNetToMediaRows    int        `json:"ip_net_to_media_rows"`
	IPNetToPhysicalRows int        `json:"ip_net_to_physical_rows"`
	MappedEntries       int        `json:"mapped_entries"`
}

type InterfaceState struct {
	IfIndex     int
	Name        string
	Description string
	Alias       string
	AdminStatus string
	OperStatus  string
}

type Client interface {
	LookupMAC(ctx context.Context, target SwitchTarget, macAddress string) (LookupResult, error)
	GetSystemName(ctx context.Context, target SwitchTarget) (string, error)
	GetBaseMAC(ctx context.Context, target SwitchTarget) (string, error)
	ResolveBridgePort(ctx context.Context, target SwitchTarget, bridgePort int) (LookupResult, error)
	DiscoverLLDPNeighbors(ctx context.Context, target SwitchTarget) ([]LLDPNeighbor, error)
	DiscoverCDPNeighbors(ctx context.Context, target SwitchTarget) ([]CDPNeighbor, error)
	DiscoverTrunkPorts(ctx context.Context, target SwitchTarget) ([]TrunkPort, error)
	DiscoverFDB(ctx context.Context, target SwitchTarget) (FDBDiscovery, error)
	DiscoverFDBEntries(ctx context.Context, target SwitchTarget) ([]FDBEntry, error)
	DiscoverARP(ctx context.Context, target SwitchTarget) (ARPDiscovery, error)
	DiscoverARPEntries(ctx context.Context, target SwitchTarget) ([]ARPEntry, error)
	WalkInterfaces(ctx context.Context, target SwitchTarget) ([]InterfaceState, error)
	SetInt(ctx context.Context, target SwitchTarget, oid string, value int) error
	SetUint(ctx context.Context, target SwitchTarget, oid string, value uint32) error
}

type GoSNMPClient struct{}

func NewClient() *GoSNMPClient {
	return &GoSNMPClient{}
}

func (c *GoSNMPClient) LookupMAC(ctx context.Context, target SwitchTarget, macAddress string) (LookupResult, error) {
	params, err := c.connect(target)
	if err != nil {
		return LookupResult{}, err
	}
	defer params.Conn.Close()

	macOID, err := macToOIDSuffix(macAddress)
	if err != nil {
		return LookupResult{}, err
	}

	bridgePort, err := getInt(ctx, params, oidDot1dTpFdbPort+"."+macOID)
	if err != nil {
		return LookupResult{}, err
	}

	ifIndex, err := getInt(ctx, params, fmt.Sprintf("%s.%d", oidDot1dBasePortIfIx, bridgePort))
	if err != nil {
		return LookupResult{}, err
	}

	ifName, _ := getString(ctx, params, fmt.Sprintf("%s.%d", oidIfName, ifIndex))
	ifDescr, _ := getString(ctx, params, fmt.Sprintf("%s.%d", oidIfDescr, ifIndex))
	ifName, ifDescr = normalizeInterfaceLabels(bridgePort, ifName, ifDescr)

	return LookupResult{
		BridgePort:           bridgePort,
		IfIndex:              ifIndex,
		InterfaceName:        ifName,
		InterfaceDescription: ifDescr,
	}, nil
}

func (c *GoSNMPClient) GetSystemName(ctx context.Context, target SwitchTarget) (string, error) {
	params, err := c.connect(target)
	if err != nil {
		return "", err
	}
	defer params.Conn.Close()

	return getString(ctx, params, oidSysName)
}

func (c *GoSNMPClient) GetBaseMAC(ctx context.Context, target SwitchTarget) (string, error) {
	params, err := c.connect(target)
	if err != nil {
		return "", err
	}
	defer params.Conn.Close()

	return getMAC(ctx, params, oidDot1dBaseAddr)
}

func (c *GoSNMPClient) ResolveBridgePort(ctx context.Context, target SwitchTarget, bridgePort int) (LookupResult, error) {
	params, err := c.connect(target)
	if err != nil {
		return LookupResult{}, err
	}
	defer params.Conn.Close()

	ifIndex, err := getInt(ctx, params, fmt.Sprintf("%s.%d", oidDot1dBasePortIfIx, bridgePort))
	if err != nil {
		return LookupResult{}, err
	}

	ifName, _ := getString(ctx, params, fmt.Sprintf("%s.%d", oidIfName, ifIndex))
	ifDescr, _ := getString(ctx, params, fmt.Sprintf("%s.%d", oidIfDescr, ifIndex))
	ifName, ifDescr = normalizeInterfaceLabels(bridgePort, ifName, ifDescr)

	return LookupResult{
		BridgePort:           bridgePort,
		IfIndex:              ifIndex,
		InterfaceName:        ifName,
		InterfaceDescription: ifDescr,
	}, nil
}

func (c *GoSNMPClient) DiscoverLLDPNeighbors(ctx context.Context, target SwitchTarget) ([]LLDPNeighbor, error) {
	params, err := c.connect(target)
	if err != nil {
		return nil, err
	}
	defer params.Conn.Close()

	localPorts, err := walkStringMap(ctx, params, oidLldpLocPortDesc)
	if err != nil {
		return nil, err
	}
	localPortIDs, err := walkStringMap(ctx, params, oidLldpLocPortID)
	if err != nil {
		return nil, err
	}

	remoteSystems, err := walkStringMap(ctx, params, oidLldpRemSysName)
	if err != nil {
		return nil, err
	}

	remotePorts, err := walkStringMap(ctx, params, oidLldpRemPortDesc)
	if err != nil {
		return nil, err
	}
	remoteSystemDescs, err := walkStringMap(ctx, params, oidLldpRemSysDesc)
	if err != nil {
		return nil, err
	}

	var neighbors []LLDPNeighbor
	for suffix, remoteSystemName := range remoteSystems {
		parsed := parseLLDPRemoteSuffix(suffix)
		if parsed == nil {
			continue
		}

		localPortName := normalizeLocalLLDPPortName(localPorts[parsed.LocalPortIndex], localPortIDs[parsed.LocalPortIndex], parsed.LocalPort)
		remotePortName := strings.TrimSpace(remotePorts[suffix])
		remoteSystemName = strings.TrimSpace(remoteSystemName)
		if localPortName == "" || remoteSystemName == "" {
			continue
		}

		neighbors = append(neighbors, LLDPNeighbor{
			LocalPortName:    localPortName,
			LocalPortIndex:   parsed.LocalPort,
			RemoteSystemName: remoteSystemName,
			RemotePortName:   remotePortName,
			RemotePortDesc:   remotePortName,
			RemoteSystemDesc: strings.TrimSpace(remoteSystemDescs[suffix]),
		})
	}

	return neighbors, nil
}

func (c *GoSNMPClient) DiscoverCDPNeighbors(ctx context.Context, target SwitchTarget) ([]CDPNeighbor, error) {
	params, err := c.connect(target)
	if err != nil {
		return nil, err
	}
	defer params.Conn.Close()

	ifNames, err := walkStringMap(ctx, params, oidIfName)
	if err != nil {
		return nil, err
	}
	ifDescrs, err := walkStringMap(ctx, params, oidIfDescr)
	if err != nil {
		return nil, err
	}

	localIfIndexes, err := walkIntMap(ctx, params, oidCdpCacheIfIndex)
	if err != nil {
		return nil, err
	}
	remoteSystems, err := walkStringMap(ctx, params, oidCdpCacheDeviceID)
	if err != nil {
		return nil, err
	}
	remotePorts, err := walkStringMap(ctx, params, oidCdpCachePortID)
	if err != nil {
		return nil, err
	}
	remotePlatforms, err := walkStringMap(ctx, params, oidCdpCachePlatform)
	if err != nil {
		return nil, err
	}
	remoteVersions, err := walkStringMap(ctx, params, oidCdpCacheVersion)
	if err != nil {
		return nil, err
	}

	neighbors := make([]CDPNeighbor, 0, len(remoteSystems))
	for suffix, remoteSystemName := range remoteSystems {
		parsed := parseCDPSuffix(suffix)
		if parsed == nil {
			continue
		}

		localIfIndex := localIfIndexes[suffix]
		if localIfIndex <= 0 {
			localIfIndex = parsed.IfIndex
		}
		if localIfIndex <= 0 {
			continue
		}

		localPortName, _ := normalizeInterfaceLabels(localIfIndex, ifNames[strconv.Itoa(localIfIndex)], ifDescrs[strconv.Itoa(localIfIndex)])
		remoteSystemName = strings.TrimSpace(remoteSystemName)
		if remoteSystemName == "" {
			continue
		}

		neighbors = append(neighbors, CDPNeighbor{
			LocalIfIndex:      localIfIndex,
			LocalPortName:     localPortName,
			RemoteSystemName:  remoteSystemName,
			RemotePortName:    strings.TrimSpace(remotePorts[suffix]),
			RemotePlatform:    strings.TrimSpace(remotePlatforms[suffix]),
			RemoteDescription: strings.TrimSpace(remoteVersions[suffix]),
		})
	}

	return neighbors, nil
}

func (c *GoSNMPClient) DiscoverTrunkPorts(ctx context.Context, target SwitchTarget) ([]TrunkPort, error) {
	params, err := c.connect(target)
	if err != nil {
		return nil, err
	}
	defer params.Conn.Close()

	oid := trunkStateOID(target.Vendor, target.Model)
	if oid == "" {
		return nil, nil
	}

	stateMap, err := walkIntMap(ctx, params, oid)
	if err != nil {
		return nil, err
	}

	ports := make([]TrunkPort, 0, len(stateMap))
	for suffix, state := range stateMap {
		ifIndex, ok := parsePositiveInt(suffix)
		if !ok || state != 1 {
			continue
		}

		ports = append(ports, TrunkPort{
			IfIndex: ifIndex,
			State:   state,
			Vendor:  strings.TrimSpace(target.Vendor),
		})
	}

	sort.Slice(ports, func(i, j int) bool {
		return ports[i].IfIndex < ports[j].IfIndex
	})

	return ports, nil
}

func (c *GoSNMPClient) DiscoverFDB(ctx context.Context, target SwitchTarget) (FDBDiscovery, error) {
	params, err := c.connect(target)
	if err != nil {
		return FDBDiscovery{}, err
	}
	defer params.Conn.Close()

	ifIndexByBridgePort, err := walkIntMap(ctx, params, oidDot1dBasePortIfIx)
	if err != nil {
		return FDBDiscovery{}, err
	}

	dot1dRows, err := walkIntMap(ctx, params, oidDot1dTpFdbPort)
	if err != nil {
		return FDBDiscovery{}, err
	}

	qBridgeRows := map[string]int{}
	if len(dot1dRows) == 0 || vendorPrefersQBridge(target) {
		qBridgeRows, err = walkIntMap(ctx, params, oidQBridgeTpFdbPort)
		if err != nil {
			return FDBDiscovery{}, err
		}
	}

	source := "dot1d"
	entryMap := make(map[string]FDBEntry, len(dot1dRows)+len(qBridgeRows))
	appendFDBEntries(entryMap, dot1dRows, ifIndexByBridgePort)
	if len(entryMap) == 0 && len(qBridgeRows) > 0 {
		source = "qbridge"
	}
	if len(qBridgeRows) > 0 {
		if len(entryMap) > 0 {
			source = "merged"
		} else {
			source = "qbridge"
		}
		appendFDBEntries(entryMap, qBridgeRows, ifIndexByBridgePort)
	}
	if len(dot1dRows) == 0 && len(qBridgeRows) == 0 {
		source = "none"
	}

	entries := make([]FDBEntry, 0, len(entryMap))
	for _, entry := range entryMap {
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IfIndex == entries[j].IfIndex {
			return entries[i].MACAddress < entries[j].MACAddress
		}
		return entries[i].IfIndex < entries[j].IfIndex
	})

	return FDBDiscovery{
		Entries:           entries,
		Source:            source,
		Dot1DRows:         len(dot1dRows),
		QBridgeRows:       len(qBridgeRows),
		BridgePortMapRows: len(ifIndexByBridgePort),
		MappedEntries:     len(entries),
	}, nil
}

func (c *GoSNMPClient) DiscoverFDBEntries(ctx context.Context, target SwitchTarget) ([]FDBEntry, error) {
	discovery, err := c.DiscoverFDB(ctx, target)
	if err != nil {
		return nil, err
	}
	return discovery.Entries, nil
}

func (c *GoSNMPClient) DiscoverARP(ctx context.Context, target SwitchTarget) (ARPDiscovery, error) {
	params, err := c.connect(target)
	if err != nil {
		return ARPDiscovery{}, err
	}
	defer params.Conn.Close()

	ipNetToMediaRows, mediaErr := walkARPValueMap(ctx, params, oidIpNetToMediaPhys)
	ipNetToPhysicalRows, physicalErr := walkARPValueMap(ctx, params, oidIpNetToPhysical)
	if mediaErr != nil && physicalErr != nil {
		return ARPDiscovery{}, physicalErr
	}

	entryMap := make(map[string]ARPEntry, len(ipNetToMediaRows)+len(ipNetToPhysicalRows))
	appendARPEntries(entryMap, ipNetToMediaRows, "ipNetToMedia")
	appendARPEntries(entryMap, ipNetToPhysicalRows, "ipNetToPhysical")

	entries := make([]ARPEntry, 0, len(entryMap))
	for _, entry := range entryMap {
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IfIndex == entries[j].IfIndex {
			if entries[i].IPAddress == entries[j].IPAddress {
				return entries[i].MACAddress < entries[j].MACAddress
			}
			return entries[i].IPAddress < entries[j].IPAddress
		}
		return entries[i].IfIndex < entries[j].IfIndex
	})

	source := "none"
	switch {
	case len(ipNetToMediaRows) > 0 && len(ipNetToPhysicalRows) > 0:
		source = "merged"
	case len(ipNetToPhysicalRows) > 0:
		source = "ipNetToPhysical"
	case len(ipNetToMediaRows) > 0:
		source = "ipNetToMedia"
	}

	return ARPDiscovery{
		Entries:             entries,
		Source:              source,
		IPNetToMediaRows:    len(ipNetToMediaRows),
		IPNetToPhysicalRows: len(ipNetToPhysicalRows),
		MappedEntries:       len(entries),
	}, nil
}

func (c *GoSNMPClient) DiscoverARPEntries(ctx context.Context, target SwitchTarget) ([]ARPEntry, error) {
	discovery, err := c.DiscoverARP(ctx, target)
	if err != nil {
		return nil, err
	}
	return discovery.Entries, nil
}

func (c *GoSNMPClient) WalkInterfaces(ctx context.Context, target SwitchTarget) ([]InterfaceState, error) {
	params, err := c.connect(target)
	if err != nil {
		return nil, err
	}
	defer params.Conn.Close()

	ifNames, err := walkStringMap(ctx, params, oidIfName)
	if err != nil {
		return nil, err
	}
	ifDescrs, err := walkStringMap(ctx, params, oidIfDescr)
	if err != nil {
		return nil, err
	}
	ifAliases, err := walkStringMap(ctx, params, oidIfAlias)
	if err != nil {
		return nil, err
	}
	adminStatuses, err := walkIntMap(ctx, params, oidIfAdminStatus)
	if err != nil {
		return nil, err
	}
	operStatuses, err := walkIntMap(ctx, params, oidIfOperStatus)
	if err != nil {
		return nil, err
	}

	indexes := make(map[int]struct{}, len(ifNames))
	for suffix := range ifNames {
		if index, ok := parsePositiveInt(suffix); ok {
			indexes[index] = struct{}{}
		}
	}
	for suffix := range ifDescrs {
		if index, ok := parsePositiveInt(suffix); ok {
			indexes[index] = struct{}{}
		}
	}
	for suffix := range adminStatuses {
		if index, ok := parsePositiveInt(suffix); ok {
			indexes[index] = struct{}{}
		}
	}

	sortedIndexes := make([]int, 0, len(indexes))
	for index := range indexes {
		sortedIndexes = append(sortedIndexes, index)
	}
	sort.Ints(sortedIndexes)

	states := make([]InterfaceState, 0, len(sortedIndexes))
	for _, ifIndex := range sortedIndexes {
		name, description := normalizeInterfaceLabels(ifIndex, ifNames[strconv.Itoa(ifIndex)], ifDescrs[strconv.Itoa(ifIndex)])
		states = append(states, InterfaceState{
			IfIndex:     ifIndex,
			Name:        name,
			Description: description,
			Alias:       strings.TrimSpace(ifAliases[strconv.Itoa(ifIndex)]),
			AdminStatus: normalizeIfStatus(adminStatuses[strconv.Itoa(ifIndex)]),
			OperStatus:  normalizeIfStatus(operStatuses[strconv.Itoa(ifIndex)]),
		})
	}

	return states, nil
}

func (c *GoSNMPClient) SetInt(ctx context.Context, target SwitchTarget, oid string, value int) error {
	return c.set(ctx, target, oid, gosnmp.Integer, value)
}

func (c *GoSNMPClient) SetUint(ctx context.Context, target SwitchTarget, oid string, value uint32) error {
	return c.set(ctx, target, oid, gosnmp.Gauge32, value)
}

func (c *GoSNMPClient) set(ctx context.Context, target SwitchTarget, oid string, typ gosnmp.Asn1BER, value any) error {
	params, err := c.connect(target)
	if err != nil {
		return err
	}
	defer params.Conn.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	packet, err := params.Set([]gosnmp.SnmpPDU{
		{
			Name:  oid,
			Type:  typ,
			Value: value,
		},
	})
	if err != nil {
		return err
	}
	if packet == nil || packet.Error != gosnmp.NoError {
		if packet == nil {
			return fmt.Errorf("snmp set returned nil packet for oid %s", oid)
		}
		return fmt.Errorf("snmp set failed for oid %s: %s", oid, packet.Error.String())
	}

	return nil
}

func (c *GoSNMPClient) connect(target SwitchTarget) (*gosnmp.GoSNMP, error) {
	params := &gosnmp.GoSNMP{
		Target:    target.Address,
		Port:      target.Port,
		Community: target.Community,
		Version:   gosnmp.Version2c,
		Timeout:   target.Timeout,
		Retries:   target.Retries,
	}
	if target.MaxRepetitions > 0 {
		params.MaxRepetitions = uint32(target.MaxRepetitions)
	}

	if err := params.Connect(); err != nil {
		return nil, err
	}
	return params, nil
}

func getInt(ctx context.Context, client *gosnmp.GoSNMP, oid string) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	packet, err := client.Get([]string{oid})
	if err != nil {
		return 0, err
	}

	if len(packet.Variables) == 0 {
		return 0, fmt.Errorf("snmp empty response for oid %s", oid)
	}

	variable := packet.Variables[0]
	value := gosnmp.ToBigInt(variable.Value)
	if value == nil {
		return 0, fmt.Errorf("snmp integer conversion failed for oid %s", oid)
	}

	return int(value.Int64()), nil
}

func getString(ctx context.Context, client *gosnmp.GoSNMP, oid string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	packet, err := client.Get([]string{oid})
	if err != nil {
		return "", err
	}

	if len(packet.Variables) == 0 {
		return "", fmt.Errorf("snmp empty response for oid %s", oid)
	}

	value := packet.Variables[0].Value
	if value == nil {
		return "", nil
	}

	switch typed := value.(type) {
	case []byte:
		return sanitizeSNMPStringBytes(typed), nil
	case string:
		return sanitizeSNMPString(typed), nil
	default:
		return sanitizeSNMPString(fmt.Sprintf("%v", typed)), nil
	}
}

func getMAC(ctx context.Context, client *gosnmp.GoSNMP, oid string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	packet, err := client.Get([]string{oid})
	if err != nil {
		return "", err
	}

	if len(packet.Variables) == 0 {
		return "", fmt.Errorf("snmp empty response for oid %s", oid)
	}

	value := packet.Variables[0].Value
	switch typed := value.(type) {
	case []byte:
		if len(typed) == 0 {
			return "", nil
		}
		parts := make([]string, 0, len(typed))
		for _, b := range typed {
			parts = append(parts, fmt.Sprintf("%02X", b))
		}
		return strings.Join(parts, ":"), nil
	case string:
		return strings.TrimSpace(typed), nil
	default:
		return "", fmt.Errorf("snmp mac conversion failed for oid %s", oid)
	}
}

func walkStringMap(ctx context.Context, client *gosnmp.GoSNMP, oid string) (map[string]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	pdus, err := client.WalkAll(oid)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(pdus))
	for _, pdu := range pdus {
		suffix := strings.TrimPrefix(pdu.Name, oid+".")
		if suffix == pdu.Name {
			continue
		}

		if pdu.Value == nil {
			result[suffix] = ""
			continue
		}

		switch typed := pdu.Value.(type) {
		case []byte:
			result[suffix] = sanitizeSNMPStringBytes(typed)
		case string:
			result[suffix] = sanitizeSNMPString(typed)
		default:
			result[suffix] = sanitizeSNMPString(fmt.Sprintf("%v", typed))
		}
	}

	return result, nil
}

func walkIntMap(ctx context.Context, client *gosnmp.GoSNMP, oid string) (map[string]int, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	pdus, err := client.WalkAll(oid)
	if err != nil {
		return nil, err
	}

	result := make(map[string]int, len(pdus))
	for _, pdu := range pdus {
		suffix := strings.TrimPrefix(pdu.Name, oid+".")
		if suffix == pdu.Name {
			continue
		}
		value := gosnmp.ToBigInt(pdu.Value)
		if value == nil {
			continue
		}
		result[suffix] = int(value.Int64())
	}

	return result, nil
}

func walkARPValueMap(ctx context.Context, client *gosnmp.GoSNMP, oid string) (map[string]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := make(map[string]string)
	err := client.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		suffix := strings.TrimPrefix(pdu.Name, oid+".")
		if suffix == pdu.Name {
			return nil
		}

		macAddress := pduValueToMAC(pdu.Value)
		if macAddress == "" {
			return nil
		}
		result[suffix] = macAddress
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

type lldpRemoteSuffix struct {
	TimeMark       int
	LocalPort      int
	RemoteIndex    int
	LocalPortIndex string
}

func parseLLDPRemoteSuffix(suffix string) *lldpRemoteSuffix {
	parts := strings.Split(strings.TrimSpace(suffix), ".")
	if len(parts) < 3 {
		return nil
	}

	timeMark, err := strconv.Atoi(strings.TrimSpace(parts[len(parts)-3]))
	if err != nil || timeMark < 0 {
		return nil
	}
	localPort, ok := parsePositiveInt(parts[len(parts)-2])
	if !ok {
		return nil
	}
	remoteIndex, ok := parsePositiveInt(parts[len(parts)-1])
	if !ok {
		return nil
	}

	return &lldpRemoteSuffix{
		TimeMark:       timeMark,
		LocalPort:      localPort,
		RemoteIndex:    remoteIndex,
		LocalPortIndex: strconv.Itoa(localPort),
	}
}

type cdpSuffix struct {
	IfIndex     int
	DeviceIndex int
}

func parseCDPSuffix(suffix string) *cdpSuffix {
	parts := strings.Split(strings.TrimSpace(suffix), ".")
	if len(parts) < 2 {
		return nil
	}

	ifIndex, ok := parsePositiveInt(parts[len(parts)-2])
	if !ok {
		return nil
	}
	deviceIndex, ok := parsePositiveInt(parts[len(parts)-1])
	if !ok {
		return nil
	}

	return &cdpSuffix{
		IfIndex:     ifIndex,
		DeviceIndex: deviceIndex,
	}
}

func normalizeLocalLLDPPortName(portDesc, portID string, fallbackPort int) string {
	portDesc = strings.TrimSpace(portDesc)
	portID = strings.TrimSpace(portID)

	switch {
	case portDesc != "":
		return portDesc
	case portID != "":
		return portID
	case fallbackPort > 0:
		return fmt.Sprintf("Port %d", fallbackPort)
	default:
		return ""
	}
}

func trunkStateOID(vendor, model string) string {
	normalizedVendor := strings.ToLower(strings.TrimSpace(vendor))
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	combined := normalizedVendor + " " + normalizedModel

	switch {
	case strings.Contains(combined, "cisco"):
		return oidCiscoTrunkState
	case strings.Contains(combined, "huawei"):
		return oidHuaweiTrunkState
	default:
		return ""
	}
}

func macToOIDSuffix(macAddress string) (string, error) {
	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(macAddress)), ":", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	if len(normalized) != 12 {
		return "", fmt.Errorf("invalid mac address")
	}

	raw, err := hex.DecodeString(normalized)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, len(raw))
	for _, b := range raw {
		parts = append(parts, fmt.Sprintf("%d", b))
	}

	return strings.Join(parts, "."), nil
}

func oidSuffixToMAC(suffix string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(suffix), ".")
	if len(parts) < 6 {
		return "", false
	}

	parts = parts[len(parts)-6:]
	octets := make([]string, 0, 6)
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 || value > 255 {
			return "", false
		}
		octets = append(octets, fmt.Sprintf("%02X", value))
	}

	return strings.Join(octets, ":"), true
}

func appendFDBEntries(target map[string]FDBEntry, rows map[string]int, ifIndexByBridgePort map[string]int) {
	for suffix, bridgePort := range rows {
		if bridgePort <= 0 {
			continue
		}

		macAddress, ok := oidSuffixToMAC(suffix)
		if !ok {
			continue
		}

		key := strconv.Itoa(bridgePort) + "|" + macAddress
		target[key] = FDBEntry{
			MACAddress: macAddress,
			BridgePort: bridgePort,
			IfIndex:    ifIndexByBridgePort[strconv.Itoa(bridgePort)],
		}
	}
}

func appendARPEntries(target map[string]ARPEntry, rows map[string]string, source string) {
	for suffix, macAddress := range rows {
		entry, ok := parseARPEntry(suffix, macAddress, source)
		if !ok {
			continue
		}

		key := strconv.Itoa(entry.IfIndex) + "|" + entry.IPAddress + "|" + entry.MACAddress
		if existing, exists := target[key]; exists && existing.Source == "ipNetToPhysical" {
			continue
		}
		target[key] = entry
	}
}

func parseARPEntry(suffix, macAddress, source string) (ARPEntry, bool) {
	ifIndex, ipAddress, ok := parseIPNetToPhysicalSuffix(suffix)
	if !ok {
		ifIndex, ipAddress, ok = parseIPNetToMediaSuffix(suffix)
	}
	if !ok || ifIndex <= 0 || strings.TrimSpace(ipAddress) == "" || strings.TrimSpace(macAddress) == "" {
		return ARPEntry{}, false
	}

	return ARPEntry{
		IfIndex:    ifIndex,
		IPAddress:  ipAddress,
		MACAddress: strings.TrimSpace(strings.ToUpper(macAddress)),
		Source:     source,
	}, true
}

func parseIPNetToMediaSuffix(suffix string) (int, string, bool) {
	parts := strings.Split(strings.TrimSpace(suffix), ".")
	if len(parts) != 5 {
		return 0, "", false
	}

	ifIndex, ok := parsePositiveInt(parts[0])
	if !ok {
		return 0, "", false
	}

	ip, ok := parseIPv4Parts(parts[1:])
	if !ok {
		return 0, "", false
	}

	return ifIndex, ip, true
}

func parseIPNetToPhysicalSuffix(suffix string) (int, string, bool) {
	parts := strings.Split(strings.TrimSpace(suffix), ".")
	if len(parts) != 6 {
		return 0, "", false
	}

	ifIndex, ok := parsePositiveInt(parts[0])
	if !ok {
		return 0, "", false
	}
	if parts[1] != "1" {
		return 0, "", false
	}

	ip, ok := parseIPv4Parts(parts[2:])
	if !ok {
		return 0, "", false
	}

	return ifIndex, ip, true
}

func parseIPv4Parts(parts []string) (string, bool) {
	if len(parts) != 4 {
		return "", false
	}

	addr := [4]byte{}
	for i, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 || value > 255 {
			return "", false
		}
		addr[i] = byte(value)
	}

	ip, ok := netip.AddrFromSlice(addr[:])
	if !ok {
		return "", false
	}
	return ip.String(), true
}

func pduValueToMAC(value any) string {
	switch typed := value.(type) {
	case []byte:
		if len(typed) == 0 {
			return ""
		}
		parts := make([]string, 0, len(typed))
		for _, b := range typed {
			parts = append(parts, fmt.Sprintf("%02X", b))
		}
		return strings.Join(parts, ":")
	case string:
		return strings.TrimSpace(strings.ToUpper(typed))
	default:
		return ""
	}
}

func vendorPrefersQBridge(target SwitchTarget) bool {
	value := strings.ToLower(strings.TrimSpace(target.Vendor + " " + target.Model))
	return strings.Contains(value, "hp") ||
		strings.Contains(value, "hpe") ||
		strings.Contains(value, "aruba") ||
		strings.Contains(value, "huawei") ||
		strings.Contains(value, "v1910") ||
		strings.Contains(value, "comware")
}

func normalizeInterfaceLabels(bridgePort int, ifName, ifDescr string) (string, string) {
	ifName = strings.TrimSpace(ifName)
	ifDescr = strings.TrimSpace(ifDescr)

	if ifName == "" && ifDescr == "" && bridgePort > 0 {
		label := fmt.Sprintf("Port %d", bridgePort)
		return label, label
	}

	if ifName == "" {
		ifName = ifDescr
	}
	if ifDescr == "" {
		ifDescr = ifName
	}

	if isNumericLabel(ifName) && (ifDescr == "" || ifDescr == ifName || isNumericLabel(ifDescr)) {
		label := fmt.Sprintf("Port %s", ifName)
		return label, label
	}

	return ifName, ifDescr
}

func isNumericLabel(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parsePositiveInt(value string) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func normalizeIfStatus(value int) string {
	switch value {
	case 1:
		return "up"
	case 2:
		return "down"
	case 3:
		return "testing"
	case 4:
		return "unknown"
	case 5:
		return "dormant"
	case 6:
		return "not-present"
	case 7:
		return "lower-layer-down"
	default:
		return "unknown"
	}
}

func sanitizeSNMPStringBytes(value []byte) string {
	return sanitizeSNMPString(string(value))
}

func sanitizeSNMPString(value string) string {
	return strings.TrimSpace(strings.ToValidUTF8(value, ""))
}
