package portendpoint

import "time"

type Endpoint struct {
	ID               string    `json:"id"`
	SwitchID         string    `json:"switch_id"`
	PortIfIndex      int       `json:"port_ifindex"`
	MACAddress       string    `json:"mac_address"`
	IPAddress        string    `json:"ip_address"`
	Hostname         string    `json:"hostname"`
	SourceConfidence string    `json:"source_confidence"`
	LastSeenAt       time.Time `json:"last_seen_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
