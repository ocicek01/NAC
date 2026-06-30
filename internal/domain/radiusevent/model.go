package radiusevent

import "time"

type Event struct {
	ID                string    `json:"id"`
	EventType         string    `json:"event_type"`
	Username          string    `json:"username"`
	MACAddress        string    `json:"mac_address"`
	Hostname          string    `json:"hostname"`
	VendorClass       string    `json:"vendor_class"`
	NASIPAddress      string    `json:"nas_ip_address"`
	NASIdentifier     string    `json:"nas_identifier"`
	NASPort           string    `json:"nas_port"`
	NASPortID         string    `json:"nas_port_id"`
	NASPortType       string    `json:"nas_port_type"`
	CalledStationID   string    `json:"called_station_id"`
	CallingStationID  string    `json:"calling_station_id"`
	AcctStatusType    string    `json:"acct_status_type"`
	AcctSessionID     string    `json:"acct_session_id"`
	FramedIPAddress   string    `json:"framed_ip_address"`
	SessionTime       string    `json:"session_time"`
	TerminateCause    string    `json:"terminate_cause"`
	Decision          string    `json:"decision"`
	PolicyAction      string    `json:"policy_action"`
	PolicyReason      string    `json:"policy_reason"`
	ReplyMessage      string    `json:"reply_message"`
	VLANID            string    `json:"vlan_id"`
	ReplyAttributes   string    `json:"reply_attributes"`
	ControlAttributes string    `json:"control_attributes"`
	CreatedAt         time.Time `json:"created_at"`
}
