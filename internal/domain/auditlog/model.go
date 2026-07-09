package auditlog

import "time"

type Log struct {
	ID         string         `json:"id"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	Status     string         `json:"status"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	SwitchID   string         `json:"switch_id"`
	MACAddress string         `json:"mac_address"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  time.Time      `json:"created_at"`
}
