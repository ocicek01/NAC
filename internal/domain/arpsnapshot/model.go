package arpsnapshot

import "time"

type Snapshot struct {
	ID          string    `json:"id"`
	SwitchID    string    `json:"switch_id"`
	IfIndex     int       `json:"if_index"`
	MACAddress  string    `json:"mac_address"`
	IPAddress   string    `json:"ip_address"`
	Source      string    `json:"source"`
	VLANID      int       `json:"vlan_id"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
