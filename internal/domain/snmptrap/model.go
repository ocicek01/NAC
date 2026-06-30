package snmptrap

import "time"

type VarBind struct {
	OID   string `json:"oid"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Event struct {
	ID            string    `json:"id"`
	SourceIP      string    `json:"source_ip"`
	SourcePort    int       `json:"source_port"`
	SwitchID      string    `json:"switch_id"`
	SwitchName    string    `json:"switch_name"`
	SNMPVersion   string    `json:"snmp_version"`
	Community     string    `json:"community"`
	TrapOID       string    `json:"trap_oid"`
	EnterpriseOID string    `json:"enterprise_oid"`
	GenericTrap   int       `json:"generic_trap"`
	SpecificTrap  int       `json:"specific_trap"`
	UptimeTicks   uint32    `json:"uptime_ticks"`
	VarBinds      []VarBind `json:"varbinds"`
	Category      string    `json:"category"`
	Actionable    bool      `json:"actionable"`
	IfIndex       int       `json:"if_index"`
	MACAddress    string    `json:"mac_address"`
	VLANID        int       `json:"vlan_id"`
	ReceivedAt    time.Time `json:"received_at"`
	CreatedAt     time.Time `json:"created_at"`
}
