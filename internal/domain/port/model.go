package port

import "time"

type Port struct {
	ID          string
	SwitchID    string
	Name        string
	IfIndex     int
	Description string
	Type        string
	Status      string
	VLAN        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
