package arpsnmpwalk

import "testing"

func TestParseSNMPWalkOutputParsesHexStringARPEntries(t *testing.T) {
	t.Parallel()

	output := []byte(".1.3.6.1.2.1.4.22.1.2.678.10.1.9.36 = Hex-STRING: 00 08 D1 04 E7 0A\n")
	rows, err := parseSNMPWalkOutput(output, oidIPNetToMedia)
	if err != nil {
		t.Fatalf("parseSNMPWalkOutput returned error: %v", err)
	}
	if got := rows["678.10.1.9.36"]; got != "00:08:D1:04:E7:0A" {
		t.Fatalf("expected parsed mac, got %q", got)
	}
}

func TestParseWalkValueToMACSkipsZeroMAC(t *testing.T) {
	t.Parallel()

	got := parseWalkValueToMAC("Hex-STRING", "00 00 00 00 00 00")
	if got != "00:00:00:00:00:00" {
		t.Fatalf("expected zero mac formatting, got %q", got)
	}
}

func TestParseSNMPWalkOutputParsesIsoPrefixedOID(t *testing.T) {
	t.Parallel()

	output := []byte("iso.3.6.1.2.1.4.22.1.2.678.10.1.9.36 = Hex-STRING: 00 08 D1 04 E7 0A\n")
	rows, err := parseSNMPWalkOutput(output, oidIPNetToMedia)
	if err != nil {
		t.Fatalf("parseSNMPWalkOutput returned error: %v", err)
	}
	if got := rows["678.10.1.9.36"]; got != "00:08:D1:04:E7:0A" {
		t.Fatalf("expected parsed mac, got %q", got)
	}
}

func TestParseSNMPWalkOutputParsesCompactOutput(t *testing.T) {
	t.Parallel()

	output := []byte(".1.3.6.1.2.1.4.22.1.2.678.10.1.9.36 Hex-STRING: 00 08 D1 04 E7 0A\n")
	rows, err := parseSNMPWalkOutput(output, oidIPNetToMedia)
	if err != nil {
		t.Fatalf("parseSNMPWalkOutput returned error: %v", err)
	}
	if got := rows["678.10.1.9.36"]; got != "00:08:D1:04:E7:0A" {
		t.Fatalf("expected parsed mac, got %q", got)
	}
}
