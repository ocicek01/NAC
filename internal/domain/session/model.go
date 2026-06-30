package session

import "time"

type Session struct {
	ID               string     `json:"id"`
	ActiveKey        string     `json:"active_key"`
	DeviceID         string     `json:"device_id"`
	SwitchID         string     `json:"switch_id"`
	SwitchName       string     `json:"switch_name"`
	ManagementIP     string     `json:"management_ip"`
	PortID           string     `json:"port_id"`
	NASPort          string     `json:"nas_port"`
	NASPortID        string     `json:"nas_port_id"`
	IfIndex          int        `json:"if_index"`
	InterfaceName    string     `json:"interface_name"`
	IPAddress        string     `json:"ip_address"`
	MACAddress       string     `json:"mac_address"`
	Username         string     `json:"username"`
	Hostname         string     `json:"hostname"`
	VendorClass      string     `json:"vendor_class"`
	CalledStationID  string     `json:"called_station_id"`
	CallingStationID string     `json:"calling_station_id"`
	AcctSessionID    string     `json:"acct_session_id"`
	Authorization    string     `json:"authorization"`
	SessionType      string     `json:"session_type"`
	Status           string     `json:"status"`
	PolicyAction     string     `json:"policy_action"`
	PolicyReason     string     `json:"policy_reason"`
	AssignedVLAN     string     `json:"assigned_vlan"`
	StartedAt        time.Time  `json:"started_at"`
	LastSeenAt       time.Time  `json:"last_seen_at"`
	EndedAt          *time.Time `json:"ended_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
