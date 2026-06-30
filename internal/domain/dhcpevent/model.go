package dhcpevent

import "time"

type Event struct {
	ID                string    `json:"id"`
	MACAddress        string    `json:"mac_address"`
	TransactionID     string    `json:"transaction_id"`
	SourceIP          string    `json:"source_ip"`
	ClientIP          string    `json:"client_ip"`
	YourIP            string    `json:"your_ip"`
	RequestedIP       string    `json:"requested_ip"`
	MessageType       string    `json:"message_type"`
	Hostname          string    `json:"hostname"`
	VendorClass       string    `json:"vendor_class"`
	Option82Raw       string    `json:"option82_raw"`
	Option82CircuitID string    `json:"option82_circuit_id"`
	Option82RemoteID  string    `json:"option82_remote_id"`
	Option82VLAN      string    `json:"option82_vlan"`
	RelayIP           string    `json:"relay_ip"`
	RelaySwitchID     string    `json:"relay_switch_id"`
	RelaySwitchName   string    `json:"relay_switch_name"`
	ObservedAt        time.Time `json:"observed_at"`
	CreatedAt         time.Time `json:"created_at"`
}
