package snmp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// PortObservation distinguishes an unreadable optional table from an empty one.
// In particular, an FDB timeout must never be interpreted as all MACs removed.
type PortObservation struct {
	InterfaceState
	IfType       int
	IsPhysical   bool
	VLANID       *int
	MACAddresses []string
	MACsKnown    bool
}

type PortSnapshot struct {
	Ports    []PortObservation
	Warnings []string
}

func (c *GoSNMPClient) ObservePorts(ctx context.Context, target SwitchTarget) (PortSnapshot, error) {
	client := &gosnmp.GoSNMP{Target: target.Address, Port: target.Port, Community: target.Community,
		Version: gosnmp.Version2c, Timeout: target.Timeout, Retries: target.Retries,
		MaxRepetitions: 25, Context: ctx}
	if err := client.Connect(); err != nil {
		return PortSnapshot{}, err
	}
	defer client.Conn.Close()
	walk := func(oid string) (map[string]gosnmp.SnmpPDU, error) {
		return observationTable(ctx, client, oid)
	}
	ifTypes, err := walk(oidIfType)
	if err != nil || len(ifTypes) == 0 {
		return PortSnapshot{}, fmt.Errorf("interface types unavailable: %v", err)
	}
	admin, err := walk(oidIfAdminStatus)
	if err != nil || len(admin) == 0 {
		return PortSnapshot{}, fmt.Errorf("admin status unavailable: %v", err)
	}
	oper, err := walk(oidIfOperStatus)
	if err != nil || len(oper) == 0 {
		return PortSnapshot{}, fmt.Errorf("oper status unavailable: %v", err)
	}
	// Labels are optional; retained database labels survive unsupported ifName.
	names, _ := walk(oidIfName)
	descriptions, _ := walk(oidIfDescr)
	result := PortSnapshot{}
	bridge, bridgeErr := walk(oidDot1dBasePortIfIx)
	pvid, vlanErr := walk(".1.3.6.1.2.1.17.7.1.4.5.1.1")
	vlans := map[int]int{}
	if vlanErr == nil && bridgeErr == nil {
		for suffix, pdu := range pvid {
			if ix := observationInt(bridge[suffix]); ix > 0 {
				vlans[ix] = observationInt(pdu)
			}
		}
	}
	if strings.Contains(strings.ToLower(target.Vendor), "cisco") {
		access, _ := walk(".1.3.6.1.4.1.9.9.68.1.2.2.1.2")
		for suffix, pdu := range access {
			if ix, e := strconv.Atoi(suffix); e == nil {
				vlans[ix] = observationInt(pdu)
			}
		}
	}
	if len(vlans) == 0 {
		result.Warnings = append(result.Warnings, "VLAN table unavailable; retaining previous VLANs")
	}
	dot1d, dot1dErr := walk(oidDot1dTpFdbPort)
	qbridge, qbridgeErr := walk(oidQBridgeTpFdbPort)
	macsKnown := bridgeErr == nil && len(bridge) > 0 && (dot1dErr == nil || qbridgeErr == nil)
	entries := map[string]FDBEntry{}
	unmapped := 0
	if macsKnown {
		mapping := map[string]int{}
		for key, pdu := range bridge {
			mapping[key] = observationInt(pdu)
		}
		tables := make([]map[string]gosnmp.SnmpPDU, 0, 2)
		if dot1dErr == nil {
			tables = append(tables, dot1d)
		}
		if qbridgeErr == nil {
			tables = append(tables, qbridge)
		}
		entries, unmapped = mergeFDBTables(mapping, tables...)
		if unmapped > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("ignored %d unmapped FDB rows", unmapped))
		}
	}
	if !macsKnown {
		result.Warnings = append(result.Warnings, "FDB table incomplete; retaining previous MACs")
	}
	macs := map[int][]string{}
	for _, entry := range entries {
		macs[entry.IfIndex] = append(macs[entry.IfIndex], entry.MACAddress)
	}
	for suffix, typePDU := range ifTypes {
		if physicalEthernetIfType(observationInt(typePDU)) {
			if _, ok := admin[suffix]; !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("ifIndex %s skipped: admin status missing", suffix))
			}
		}
	}
	for suffix, pdu := range admin {
		ix, e := strconv.Atoi(suffix)
		if e != nil || ix <= 0 {
			continue
		}
		typePDU, typeOK := ifTypes[suffix]
		if !typeOK {
			result.Warnings = append(result.Warnings, fmt.Sprintf("ifIndex %d skipped: ifType missing", ix))
			continue
		}
		if !physicalEthernetIfType(observationInt(typePDU)) {
			continue
		}
		operPDU, ok := oper[suffix]
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("ifIndex %d skipped: oper status missing", ix))
			continue
		}
		adminValue, operValue := observationInt(pdu), observationInt(operPDU)
		if adminValue < 1 || adminValue > 3 || operValue < 1 || operValue > 7 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("ifIndex %d skipped: invalid status", ix))
			continue
		}
		ifType := observationInt(typePDU)
		item := PortObservation{InterfaceState: InterfaceState{IfIndex: ix, Name: observationString(names[suffix]),
			Description: observationString(descriptions[suffix]), AdminStatus: normalizeIfStatus(adminValue), OperStatus: normalizeIfStatus(operValue)},
			IfType: ifType, IsPhysical: true, MACsKnown: macsKnown, MACAddresses: macs[ix]}
		if vlan := vlans[ix]; vlan > 0 && vlan < 4095 {
			item.VLANID = &vlan
		}
		sort.Strings(item.MACAddresses)
		result.Ports = append(result.Ports, item)
	}
	if err := ctx.Err(); err != nil {
		return PortSnapshot{}, err
	}
	sort.Slice(result.Ports, func(i, j int) bool { return result.Ports[i].IfIndex < result.Ports[j].IfIndex })
	return result, nil
}

func mergeFDBTables(mapping map[string]int, tables ...map[string]gosnmp.SnmpPDU) (map[string]FDBEntry, int) {
	entries := map[string]FDBEntry{}
	unmapped := 0
	for _, table := range tables {
		rows := map[string]int{}
		for key, pdu := range table {
			port := observationInt(pdu)
			if port <= 0 || mapping[strconv.Itoa(port)] <= 0 {
				unmapped++
				continue
			}
			rows[key] = port
		}
		appendFDBEntries(entries, rows, mapping)
	}
	return entries, unmapped
}

func physicalEthernetIfType(value int) bool {
	switch value {
	case 6, 26, 62, 69, 117:
		return true
	default:
		return false
	}
}

type observationWalker interface {
	BulkWalkAll(string) ([]gosnmp.SnmpPDU, error)
	WalkAll(string) ([]gosnmp.SnmpPDU, error)
}

func observationTable(ctx context.Context, client observationWalker, oid string) (map[string]gosnmp.SnmpPDU, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pdus, err := client.BulkWalkAll(oid)
	if err != nil && ctx.Err() == nil {
		pdus, err = client.WalkAll(oid)
	}
	if err != nil {
		return nil, err
	}
	result := map[string]gosnmp.SnmpPDU{}
	for _, pdu := range pdus {
		if pdu.Type == gosnmp.NoSuchObject || pdu.Type == gosnmp.NoSuchInstance || pdu.Type == gosnmp.EndOfMibView {
			continue
		}
		if strings.HasPrefix(pdu.Name, oid+".") {
			result[strings.TrimPrefix(pdu.Name, oid+".")] = pdu
		}
	}
	return result, nil
}

func observationInt(pdu gosnmp.SnmpPDU) int { return int(gosnmp.ToBigInt(pdu.Value).Int64()) }
func observationString(pdu gosnmp.SnmpPDU) string {
	switch v := pdu.Value.(type) {
	case []byte:
		return sanitizeSNMPStringBytes(v)
	case string:
		return sanitizeSNMPString(v)
	}
	return ""
}
