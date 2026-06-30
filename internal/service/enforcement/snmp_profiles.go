package enforcement

import (
	"strings"

	switchasset "nac/internal/domain/switchasset"
)

const (
	snmpStrategyCiscoAccess = "cisco-access-vlan"
	snmpStrategyQBridgePVID = "qbridge-pvid"
	snmpStrategyExtreme     = "extreme-bitmap"
	snmpStrategyJuniperAPI  = "juniper-api"
)

type snmpProfile struct {
	name                 string
	strategy             string
	vlanOIDPrefix        string
	vlanIndexMode        string
	requiresPortBounce   bool
	allowIfIndexFallback bool
}

type snmpProfileMatcher struct {
	vendors []string
	models  []string
	names   []string
	profile snmpProfile
}

var snmpProfileRegistry = []snmpProfileMatcher{
	{
		vendors: []string{"cisco"},
		profile: snmpProfile{
			name:                 "cisco-access-vlan",
			strategy:             snmpStrategyCiscoAccess,
			vlanOIDPrefix:        oidCiscoVLANPrefix,
			vlanIndexMode:        "ifindex",
			requiresPortBounce:   true,
			allowIfIndexFallback: false,
		},
	},
	{
		vendors: []string{"hp", "hpe", "aruba", "procurve"},
		models:  []string{"2530", "2610", "v1910", "e8206", "procurve"},
		profile: snmpProfile{
			name:                 "hp-aruba-qbridge",
			strategy:             snmpStrategyQBridgePVID,
			vlanOIDPrefix:        oidQBridgePVIDPrefix,
			vlanIndexMode:        "bridgeport",
			requiresPortBounce:   true,
			allowIfIndexFallback: true,
		},
	},
	{
		vendors: []string{"huawei", "h3c"},
		profile: snmpProfile{
			name:                 "huawei-qbridge",
			strategy:             snmpStrategyQBridgePVID,
			vlanOIDPrefix:        oidQBridgePVIDPrefix,
			vlanIndexMode:        "bridgeport",
			requiresPortBounce:   true,
			allowIfIndexFallback: true,
		},
	},
	{
		vendors: []string{"d-link", "dlink", "netgear", "ruijie", "dell", "brocade", "ruckus", "3com", "pnetworks", "ip-com"},
		profile: snmpProfile{
			name:                 "generic-qbridge",
			strategy:             snmpStrategyQBridgePVID,
			vlanOIDPrefix:        oidQBridgePVIDPrefix,
			vlanIndexMode:        "bridgeport",
			requiresPortBounce:   false,
			allowIfIndexFallback: true,
		},
	},
	{
		vendors: []string{"extreme", "extremenetworks"},
		profile: snmpProfile{
			name:                 "extreme-bitmap",
			strategy:             snmpStrategyExtreme,
			vlanOIDPrefix:        oidQBridgePVIDPrefix,
			vlanIndexMode:        "bridgeport",
			requiresPortBounce:   false,
			allowIfIndexFallback: false,
		},
	},
	{
		vendors: []string{"juniper"},
		profile: snmpProfile{
			name:                 "juniper-api",
			strategy:             snmpStrategyJuniperAPI,
			vlanOIDPrefix:        "",
			vlanIndexMode:        "bridgeport",
			requiresPortBounce:   false,
			allowIfIndexFallback: false,
		},
	},
}

func selectSNMPProfile(asset switchasset.Switch) snmpProfile {
	vendor := strings.ToLower(strings.TrimSpace(asset.Vendor))
	model := strings.ToLower(strings.TrimSpace(asset.Model))
	name := strings.ToLower(strings.TrimSpace(asset.Name))
	systemName := strings.ToLower(strings.TrimSpace(asset.SystemName))

	for _, matcher := range snmpProfileRegistry {
		if containsAny(vendor, matcher.vendors) ||
			containsAny(model, matcher.models) ||
			containsAny(name, matcher.names) ||
			containsAny(systemName, matcher.names) {
			return matcher.profile
		}
	}

	return snmpProfile{
		name:                 "generic-qbridge",
		strategy:             snmpStrategyQBridgePVID,
		vlanOIDPrefix:        oidQBridgePVIDPrefix,
		vlanIndexMode:        "bridgeport",
		requiresPortBounce:   false,
		allowIfIndexFallback: true,
	}
}

func supportsSNMPWriteStrategy(asset switchasset.Switch) bool {
	switch selectSNMPProfile(asset).strategy {
	case snmpStrategyCiscoAccess, snmpStrategyQBridgePVID:
		return true
	default:
		return false
	}
}

func preferSNMPWrite(asset switchasset.Switch) bool {
	profile := selectSNMPProfile(asset)
	return profile.strategy == snmpStrategyCiscoAccess || profile.strategy == snmpStrategyQBridgePVID
}

func containsAny(value string, patterns []string) bool {
	if strings.TrimSpace(value) == "" || len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		if strings.Contains(value, strings.ToLower(strings.TrimSpace(pattern))) {
			return true
		}
	}
	return false
}
