package switchasset

import "time"

type Switch struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	SystemName             string    `json:"system_name"`
	Aliases                []string  `json:"aliases"`
	BaseMAC                string    `json:"base_mac"`
	ManagementIP           string    `json:"management_ip"`
	RoutingSwitchID        string    `json:"routing_switch_id"`
	Vendor                 string    `json:"vendor"`
	Model                  string    `json:"model"`
	Status                 string    `json:"status"`
	RadiusSecret           string    `json:"radius_secret,omitempty"`
	SSHUsername            string    `json:"ssh_username,omitempty"`
	SSHPassword            string    `json:"ssh_password,omitempty"`
	SSHPort                int       `json:"ssh_port"`
	SNMPVersion            string    `json:"snmp_version"`
	SNMPCommunity          string    `json:"snmp_community,omitempty"`
	SNMPPort               int       `json:"snmp_port"`
	SNMPTimeoutMS          int       `json:"snmp_timeout_ms"`
	SNMPRetries            int       `json:"snmp_retries"`
	SupportsRadiusVLAN     bool      `json:"supports_radius_vlan"`
	SupportsCoA            bool      `json:"supports_coa"`
	SupportsSSHEnforcement bool      `json:"supports_ssh_enforcement"`
	SupportsSNMPWrite      bool      `json:"supports_snmp_write"`
	LastPolledAt           time.Time `json:"last_polled_at,omitempty"`
	LastError              string    `json:"last_error"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type ConnectedDevice struct {
	MACAddress    string    `json:"mac_address"`
	Hostname      string    `json:"hostname"`
	Status        string    `json:"status"`
	PolicyAction  string    `json:"policy_action"`
	PolicyReason  string    `json:"policy_reason"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	SourceType    string    `json:"source_type"`
	Confidence    string    `json:"confidence"`
	InterfaceName string    `json:"interface_name"`
	ManagementIP  string    `json:"management_ip"`
}

type InterfaceState struct {
	IfIndex              int               `json:"if_index"`
	BridgePort           int               `json:"bridge_port"`
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	Alias                string            `json:"alias"`
	AdminStatus          string            `json:"admin_status"`
	OperStatus           string            `json:"oper_status"`
	OperationallyUp      bool              `json:"operationally_up"`
	OperationallyDown    bool              `json:"operationally_down"`
	ConnectedDeviceCount int               `json:"connected_device_count"`
	ConnectedDevices     []ConnectedDevice `json:"connected_devices"`
}

type LiveDetail struct {
	Switch               Switch           `json:"switch"`
	Interfaces           []InterfaceState `json:"interfaces"`
	TotalInterfaces      int              `json:"total_interfaces"`
	UpInterfaces         int              `json:"up_interfaces"`
	DownInterfaces       int              `json:"down_interfaces"`
	ConnectedDeviceCount int              `json:"connected_device_count"`
	ObservedAt           time.Time        `json:"observed_at"`
}
