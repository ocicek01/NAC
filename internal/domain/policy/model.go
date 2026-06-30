package policy

import "time"

type Policy struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Type          string    `json:"type"`
	Action        string    `json:"action"`
	MatchField    string    `json:"match_field"`
	MatchOperator string    `json:"match_operator"`
	MatchValue    string    `json:"match_value"`
	Priority      int       `json:"priority"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
