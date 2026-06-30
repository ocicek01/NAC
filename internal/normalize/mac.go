package normalize

import "strings"

// MACAddress converts supported MAC formats into AA:BB:CC:DD:EE:FF.
// Invalid values return an empty string.
func MACAddress(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		value = strings.NewReplacer("-", "", ":", "", ".", "", " ", "").Replace(value)
		if len(value) != 12 {
			continue
		}

		value = strings.ToUpper(value)
		parts := make([]string, 0, 6)
		for i := 0; i < len(value); i += 2 {
			parts = append(parts, value[i:i+2])
		}
		return strings.Join(parts, ":")
	}

	return ""
}
