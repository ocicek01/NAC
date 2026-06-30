package arpsnmpwalk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"nac/internal/config"
	"nac/internal/snmp"
)

const (
	oidIPNetToMedia = ".1.3.6.1.2.1.4.22.1.2"
	oidIPNetToPhys  = ".1.3.6.1.2.1.4.35.1.4"
)

type Collector struct {
	walkPath string
	enabled  bool
}

func New(cfg config.SNMPConfig) *Collector {
	return &Collector{
		walkPath: strings.TrimSpace(cfg.WalkPath),
		enabled:  cfg.ARPExternalEnabled,
	}
}

func (c *Collector) DiscoverARP(ctx context.Context, target snmp.SwitchTarget) (snmp.ARPDiscovery, error) {
	if c == nil || !c.enabled {
		return snmp.ARPDiscovery{}, fmt.Errorf("external arp collector is disabled")
	}
	if strings.TrimSpace(c.walkPath) == "" {
		return snmp.ARPDiscovery{}, fmt.Errorf("snmp walk path is empty")
	}

	mediaRows, mediaErr := c.walkOID(ctx, target, oidIPNetToMedia)
	physicalRows, physicalErr := c.walkOID(ctx, target, oidIPNetToPhys)
	if mediaErr != nil && physicalErr != nil {
		return snmp.ARPDiscovery{}, fmt.Errorf("ipNetToMedia: %v; ipNetToPhysical: %v", mediaErr, physicalErr)
	}

	entryMap := make(map[string]snmp.ARPEntry, len(mediaRows)+len(physicalRows))
	appendParsedRows(entryMap, mediaRows, "ipNetToMedia")
	appendParsedRows(entryMap, physicalRows, "ipNetToPhysical")

	entries := make([]snmp.ARPEntry, 0, len(entryMap))
	for _, entry := range entryMap {
		entries = append(entries, entry)
	}

	source := "none"
	switch {
	case len(mediaRows) > 0 && len(physicalRows) > 0:
		source = "merged"
	case len(physicalRows) > 0:
		source = "ipNetToPhysical"
	case len(mediaRows) > 0:
		source = "ipNetToMedia"
	}

	return snmp.ARPDiscovery{
		Entries:             entries,
		Source:              source,
		IPNetToMediaRows:    len(mediaRows),
		IPNetToPhysicalRows: len(physicalRows),
		MappedEntries:       len(entries),
	}, nil
}

func (c *Collector) walkOID(ctx context.Context, target snmp.SwitchTarget, oid string) (map[string]string, error) {
	timeoutSeconds := int(target.Timeout / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 5
	}
	args := []string{
		"-v2c",
		"-c", target.Community,
		"-On",
		"-t", strconv.Itoa(timeoutSeconds),
		"-r", strconv.Itoa(max(0, target.Retries)),
		target.Address + ":" + strconv.Itoa(int(target.Port)),
		strings.TrimPrefix(oid, "."),
	}

	cmd := exec.CommandContext(ctx, c.walkPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return parseSNMPWalkOutput(stdout.Bytes(), oid)
}

func parseSNMPWalkOutput(output []byte, oid string) (map[string]string, error) {
	result := make(map[string]string)
	normalizedOID := normalizeOIDName(oid)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		name, valueType, value, ok := splitSNMPWalkLine(line)
		if !ok {
			continue
		}
		normalizedName := normalizeOIDName(name)
		suffix := strings.TrimPrefix(normalizedName, normalizedOID+".")
		if suffix == normalizedName {
			continue
		}

		macAddress := parseWalkValueToMAC(valueType, value)
		if macAddress == "" || macAddress == "00:00:00:00:00:00" {
			continue
		}
		result[suffix] = macAddress
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func splitSNMPWalkLine(line string) (name, valueType, value string, ok bool) {
	parts := strings.SplitN(line, " = ", 2)
	if len(parts) != 2 {
		return splitCompactSNMPWalkLine(line)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	typeParts := strings.SplitN(right, ": ", 2)
	if len(typeParts) != 2 {
		return left, "", strings.TrimSpace(right), true
	}
	return left, strings.TrimSpace(typeParts[0]), strings.TrimSpace(typeParts[1]), true
}

func splitCompactSNMPWalkLine(line string) (name, valueType, value string, ok bool) {
	line = strings.TrimSpace(line)
	for _, marker := range []string{" Hex-STRING: ", " STRING: ", " INTEGER: ", " IpAddress: "} {
		if idx := strings.Index(line, marker); idx >= 0 {
			return strings.TrimSpace(line[:idx]), strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(marker, " "), ": ")), strings.TrimSpace(line[idx+len(marker):]), true
		}
	}
	return "", "", "", false
}

func normalizeOIDName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "iso.") {
		value = ".1." + strings.TrimPrefix(value, "iso.")
	}
	if !strings.HasPrefix(value, ".") {
		value = "." + value
	}
	return value
}

func parseWalkValueToMAC(valueType, value string) string {
	switch strings.ToUpper(strings.TrimSpace(valueType)) {
	case "HEX-STRING":
		hexBytes := strings.Fields(strings.TrimSpace(value))
		raw := make([]byte, 0, len(hexBytes))
		for _, item := range hexBytes {
			decoded, err := hex.DecodeString(item)
			if err != nil || len(decoded) != 1 {
				return ""
			}
			raw = append(raw, decoded[0])
		}
		if len(raw) == 0 {
			return ""
		}
		return bytesToMAC(raw)
	case "STRING":
		unquoted := strings.Trim(strings.TrimSpace(value), "\"")
		if len(unquoted) == 6 {
			return bytesToMAC([]byte(unquoted))
		}
		return ""
	default:
		return ""
	}
}

func bytesToMAC(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	parts := make([]string, 0, len(raw))
	for _, b := range raw {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	return strings.Join(parts, ":")
}

func appendParsedRows(target map[string]snmp.ARPEntry, rows map[string]string, source string) {
	for suffix, macAddress := range rows {
		entry, ok := parseWalkARPEntry(suffix, macAddress, source)
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

func parseWalkARPEntry(suffix, macAddress, source string) (snmp.ARPEntry, bool) {
	ifIndex, ipAddress, ok := parseIPNetToPhysicalSuffix(suffix)
	if !ok {
		ifIndex, ipAddress, ok = parseIPNetToMediaSuffix(suffix)
	}
	if !ok || ifIndex <= 0 || strings.TrimSpace(ipAddress) == "" || strings.TrimSpace(macAddress) == "" {
		return snmp.ARPEntry{}, false
	}
	return snmp.ARPEntry{
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
	ifIndex, err := strconv.Atoi(parts[0])
	if err != nil || ifIndex <= 0 {
		return 0, "", false
	}
	ip := strings.Join(parts[1:], ".")
	return ifIndex, ip, true
}

func parseIPNetToPhysicalSuffix(suffix string) (int, string, bool) {
	parts := strings.Split(strings.TrimSpace(suffix), ".")
	if len(parts) != 6 {
		return 0, "", false
	}
	ifIndex, err := strconv.Atoi(parts[0])
	if err != nil || ifIndex <= 0 || parts[1] != "1" {
		return 0, "", false
	}
	ip := strings.Join(parts[2:], ".")
	return ifIndex, ip, true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
