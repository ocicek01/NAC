package switchport

import "time"

type Port struct {
	ID                   string         `json:"id"`
	SwitchID             string         `json:"switch_id"`
	IfIndex              int            `json:"if_index"`
	PortIndex            int            `json:"port_index"`
	InterfaceName        string         `json:"interface_name"`
	InterfaceAlias       string         `json:"interface_alias"`
	InterfaceDescription string         `json:"interface_description"`
	PortLabel            string         `json:"port_label"`
	InterfaceType        string         `json:"interface_type"`
	AdminStatus          string         `json:"admin_status"`
	OperStatus           string         `json:"oper_status"`
	Status               string         `json:"status"`
	PortMode             string         `json:"port_mode"`
	IsPhysical           bool           `json:"is_physical"`
	IsUplink             bool           `json:"is_uplink"`
	IsTrunk              bool           `json:"is_trunk"`
	TrunkSource          string         `json:"trunk_source"`
	EnforcementProtected bool           `json:"enforcement_protected"`
	VLANID               int            `json:"vlan_id"`
	NativeVLAN           int            `json:"native_vlan"`
	AllowedVLANs         []string       `json:"allowed_vlans"`
	VoiceVLAN            int            `json:"voice_vlan"`
	MACCount             int            `json:"mac_count"`
	MACAddresses         []string       `json:"mac_addresses"`
	SpeedBPS             int64          `json:"speed_bps"`
	SpeedLabel           string         `json:"speed_label"`
	Duplex               string         `json:"duplex"`
	PoEEnabled           bool           `json:"poe_enabled"`
	PoEPowerWatts        string         `json:"poe_power_watts"`
	NeighborProtocol     string         `json:"neighbor_protocol"`
	NeighborSwitchID     string         `json:"neighbor_switch_id"`
	NeighborSwitchName   string         `json:"neighbor_switch_name"`
	NeighborPortName     string         `json:"neighbor_port_name"`
	NeighborPlatform     string         `json:"neighbor_platform"`
	NeighborDescription  string         `json:"neighbor_description"`
	NeighborData         map[string]any `json:"neighbor_data"`
	Metadata             map[string]any `json:"metadata"`
	LastChangedAt        time.Time      `json:"last_changed_at,omitempty"`
	LastDiscoveredAt     time.Time      `json:"last_discovered_at"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}
