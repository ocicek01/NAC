package discoveryjob

import "time"

type Job struct {
	ID              string         `json:"id"`
	SwitchID        string         `json:"switch_id,omitempty"`
	Scope           string         `json:"scope"`
	Status          string         `json:"status"`
	RequestedSource string         `json:"requested_source"`
	RequestedBy     string         `json:"requested_by"`
	WorkerID        string         `json:"worker_id"`
	CurrentStep     string         `json:"current_step"`
	ProgressPercent int            `json:"progress_percent"`
	AttemptCount    int            `json:"attempt_count"`
	MaxAttempts     int            `json:"max_attempts"`
	ErrorMessage    string         `json:"error_message"`
	Summary         map[string]any `json:"summary"`
	StartedAt       time.Time      `json:"started_at,omitempty"`
	CompletedAt     time.Time      `json:"completed_at,omitempty"`
	LockedAt        time.Time      `json:"locked_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}
