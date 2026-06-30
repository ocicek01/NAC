package topology

import "time"

type Link struct {
	ID               string    `json:"id"`
	SourceSwitchID   string    `json:"source_switch_id"`
	SourceSwitchName string    `json:"source_switch_name"`
	SourcePortName   string    `json:"source_port_name"`
	TargetSwitchID   string    `json:"target_switch_id"`
	TargetSwitchName string    `json:"target_switch_name"`
	TargetPortName   string    `json:"target_port_name"`
	DiscoveryMethod  string    `json:"discovery_method"`
	Status           string    `json:"status"`
	LastObservedAt   time.Time `json:"last_observed_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
