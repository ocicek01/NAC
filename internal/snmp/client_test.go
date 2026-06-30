package snmp

import "testing"

func TestParseLLDPRemoteSuffix(t *testing.T) {
	t.Parallel()

	parsed := parseLLDPRemoteSuffix("0.37.2")
	if parsed == nil {
		t.Fatal("expected parsed suffix")
	}
	if parsed.TimeMark != 0 {
		t.Fatalf("expected time mark 0, got %d", parsed.TimeMark)
	}
	if parsed.LocalPort != 37 {
		t.Fatalf("expected local port 37, got %d", parsed.LocalPort)
	}
	if parsed.RemoteIndex != 2 {
		t.Fatalf("expected remote index 2, got %d", parsed.RemoteIndex)
	}
	if parsed.LocalPortIndex != "37" {
		t.Fatalf("expected local port index 37, got %q", parsed.LocalPortIndex)
	}
}

func TestParseLLDPRemoteSuffixRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"37",
		"abc.1.2",
		"1.two.3",
	}

	for _, value := range cases {
		if parsed := parseLLDPRemoteSuffix(value); parsed != nil {
			t.Fatalf("expected nil for %q, got %#v", value, parsed)
		}
	}
}

func TestParseCDPSuffix(t *testing.T) {
	t.Parallel()

	parsed := parseCDPSuffix("10101.7")
	if parsed == nil {
		t.Fatal("expected parsed suffix")
	}
	if parsed.IfIndex != 10101 {
		t.Fatalf("expected ifIndex 10101, got %d", parsed.IfIndex)
	}
	if parsed.DeviceIndex != 7 {
		t.Fatalf("expected device index 7, got %d", parsed.DeviceIndex)
	}
}

func TestNormalizeLocalLLDPPortName(t *testing.T) {
	t.Parallel()

	if got := normalizeLocalLLDPPortName("GigabitEthernet1/0/24", "24", 24); got != "GigabitEthernet1/0/24" {
		t.Fatalf("expected port description priority, got %q", got)
	}
	if got := normalizeLocalLLDPPortName("", "1/0/24", 24); got != "1/0/24" {
		t.Fatalf("expected port id fallback, got %q", got)
	}
	if got := normalizeLocalLLDPPortName("", "", 24); got != "Port 24" {
		t.Fatalf("expected numeric fallback, got %q", got)
	}
}

func TestTrunkStateOID(t *testing.T) {
	t.Parallel()

	if got := trunkStateOID("Cisco", "C9300"); got != oidCiscoTrunkState {
		t.Fatalf("expected cisco trunk oid, got %q", got)
	}
	if got := trunkStateOID("Huawei", "S5735"); got != oidHuaweiTrunkState {
		t.Fatalf("expected huawei trunk oid, got %q", got)
	}
	if got := trunkStateOID("HP", "2530"); got != "" {
		t.Fatalf("expected unsupported vendor to return empty oid, got %q", got)
	}
}

func TestOIDSuffixToMACSupportsQBridgeSuffix(t *testing.T) {
	t.Parallel()

	got, ok := oidSuffixToMAC("106.56.157.146.225.245.112")
	if !ok {
		t.Fatal("expected qbridge suffix to parse")
	}
	if got != "38:9D:92:E1:F5:70" {
		t.Fatalf("unexpected mac %q", got)
	}
}

func TestVendorPrefersQBridge(t *testing.T) {
	t.Parallel()

	if !vendorPrefersQBridge(SwitchTarget{Vendor: "HP", Model: "V1910-24G"}) {
		t.Fatal("expected HP V1910 to prefer qbridge fallback")
	}
	if vendorPrefersQBridge(SwitchTarget{Vendor: "Cisco", Model: "C9300"}) {
		t.Fatal("expected Cisco C9300 not to prefer qbridge fallback")
	}
}

func TestSanitizeSNMPStringRemovesInvalidUTF8(t *testing.T) {
	t.Parallel()

	got := sanitizeSNMPStringBytes([]byte{'G', 'i', '1', '/', 0xcd, '0', '/', '1'})
	if got != "Gi1/0/1" {
		t.Fatalf("expected invalid utf8 byte to be removed, got %q", got)
	}
}
