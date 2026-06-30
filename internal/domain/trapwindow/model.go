package trapwindow

import "time"

type Window struct {
	ID            string         `json:"id"`
	DedupeKey     string         `json:"dedupe_key"`
	SwitchID      string         `json:"switch_id"`
	Scope         string         `json:"scope"`
	Category      string         `json:"category"`
	Status        string         `json:"status"`
	PortIfIndex   int            `json:"port_ifindex"`
	MACAddress    string         `json:"mac_address"`
	VLANID        int            `json:"vlan_id"`
	EventCount    int            `json:"event_count"`
	FirstSeenAt   time.Time      `json:"first_seen_at"`
	LastSeenAt    time.Time      `json:"last_seen_at"`
	AvailableAt   time.Time      `json:"available_at"`
	DispatchedAt  time.Time      `json:"dispatched_at,omitempty"`
	TrapOID       string         `json:"trap_oid"`
	EnterpriseOID string         `json:"enterprise_oid"`
	SourceIP      string         `json:"source_ip"`
	Summary       map[string]any `json:"summary"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
