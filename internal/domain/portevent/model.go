package portevent

import "time"

type Event struct {
	ID                   string         `json:"id"`
	SwitchID             string         `json:"switch_id"`
	SwitchName           string         `json:"switch_name"`
	ManagementIP         string         `json:"management_ip"`
	IfIndex              int            `json:"if_index"`
	InterfaceName        string         `json:"interface_name"`
	InterfaceDescription string         `json:"interface_description"`
	AdminStatus          string         `json:"admin_status"`
	OperStatus           string         `json:"oper_status"`
	EventType            string         `json:"event_type"`
	EventSource          string         `json:"event_source"`
	MACAddress           string         `json:"mac_address"`
	IPAddress            string         `json:"ip_address"`
	Hostname             string         `json:"hostname"`
	VendorClass          string         `json:"vendor_class"`
	DeviceType           string         `json:"device_type"`
	PolicyAction         string         `json:"policy_action"`
	PolicyReason         string         `json:"policy_reason"`
	EnforcementAction    string         `json:"enforcement_action"`
	TrustLevel           string         `json:"trust_level"`
	Metadata             map[string]any `json:"metadata"`
	ObservedAt           time.Time      `json:"observed_at"`
	CreatedAt            time.Time      `json:"created_at"`
}
