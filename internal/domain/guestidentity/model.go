package guestidentity

import "time"

type Identity struct {
	ID         string    `json:"id"`
	ExternalID string    `json:"external_id"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Status     string    `json:"status"`
	TargetVLAN int       `json:"target_vlan"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
