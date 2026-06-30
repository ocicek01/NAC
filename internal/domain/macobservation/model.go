package macobservation

import "time"

type Observation struct {
	ID                   string    `json:"id"`
	DHCPEventID          string    `json:"dhcp_event_id"`
	MACAddress           string    `json:"mac_address"`
	SourceType           string    `json:"source_type"`
	Confidence           string    `json:"confidence"`
	SwitchID             string    `json:"switch_id"`
	SwitchName           string    `json:"switch_name"`
	ManagementIP         string    `json:"management_ip"`
	BridgePort           int       `json:"bridge_port"`
	IfIndex              int       `json:"if_index"`
	InterfaceName        string    `json:"interface_name"`
	InterfaceDescription string    `json:"interface_description"`
	CandidateCount       int       `json:"candidate_count"`
	AlternativeSummary   string    `json:"alternative_summary"`
	ObservedAt           time.Time `json:"observed_at"`
	CreatedAt            time.Time `json:"created_at"`
}
