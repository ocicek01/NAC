package radiusauth

import (
	"errors"
	"fmt"
)

var ErrAccountingInProgress = errors.New("radius accounting packet is already being processed")

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s %s", e.Field, e.Message)
}

type AuthorizeRequest struct {
	Username         string `json:"username"`
	MACAddress       string `json:"mac_address"`
	Hostname         string `json:"hostname"`
	VendorClass      string `json:"vendor_class"`
	NASIPAddress     string `json:"nas_ip_address"`
	NASIdentifier    string `json:"nas_identifier"`
	NASPort          string `json:"nas_port"`
	NASPortID        string `json:"nas_port_id"`
	CalledStationID  string `json:"called_station_id"`
	CallingStationID string `json:"calling_station_id"`
}

type AccountingRequest struct {
	Username         string `json:"username"`
	MACAddress       string `json:"mac_address"`
	Hostname         string `json:"hostname"`
	VendorClass      string `json:"vendor_class"`
	NASIPAddress     string `json:"nas_ip_address"`
	NASIdentifier    string `json:"nas_identifier"`
	NASPort          string `json:"nas_port"`
	NASPortID        string `json:"nas_port_id"`
	NASPortType      string `json:"nas_port_type"`
	CalledStationID  string `json:"called_station_id"`
	CallingStationID string `json:"calling_station_id"`
	AcctStatusType   string `json:"acct_status_type"`
	AcctSessionID    string `json:"acct_session_id"`
	FramedIPAddress  string `json:"framed_ip_address"`
	SessionTime      string `json:"session_time"`
	TerminateCause   string `json:"terminate_cause"`
	EventTimestamp   string `json:"event_timestamp"`
	AcctDelayTime    int    `json:"acct_delay_time"`
}

type AuthorizeResponse struct {
	Decision          string            `json:"decision"`
	PolicyAction      string            `json:"policy_action"`
	PolicyReason      string            `json:"policy_reason"`
	ReplyMessage      string            `json:"reply_message"`
	VLANID            string            `json:"vlan_id"`
	ReplyAttributes   map[string]string `json:"reply_attributes"`
	ControlAttributes map[string]string `json:"control_attributes"`
}

type AccountingResponse struct {
	Status string `json:"status"`
}
