package normalize

import "testing"

func TestMACAddress(t *testing.T) {
	tests := map[string]string{
		"30-9c-23-9b-97-aa": "30:9C:23:9B:97:AA",
		"30:9c:23:9b:97:aa": "30:9C:23:9B:97:AA",
		"309c.239b.97aa":    "30:9C:23:9B:97:AA",
		" 309C239B97AA ":    "30:9C:23:9B:97:AA",
	}

	for input, expected := range tests {
		if got := MACAddress(input); got != expected {
			t.Fatalf("input %q: expected %q, got %q", input, expected, got)
		}
	}
}
