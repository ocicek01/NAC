package sophossso

import "time"

type AccountingInput struct {
	SourceEventID  string
	Username       string
	IPAddress      string
	MACAddress     string
	AcctStatusType string
	AcctSessionID  string
	EventTimestamp time.Time
	AcctDelayTime  int
}

type Item struct {
	ID             string     `json:"id"`
	SourceKey      string     `json:"source_key"`
	Username       string     `json:"username"`
	IPAddress      string     `json:"ip_address"`
	MACAddress     string     `json:"mac_address"`
	AcctStatusType string     `json:"acct_status_type"`
	AcctSessionID  string     `json:"acct_session_id"`
	EventTimestamp time.Time  `json:"event_timestamp"`
	AcctDelayTime  int        `json:"acct_delay_time"`
	Status         string     `json:"status"`
	AttemptCount   int        `json:"attempt_count"`
	NextAttemptAt  time.Time  `json:"next_attempt_at"`
	LeaseUntil     *time.Time `json:"lease_until"`
	LastError      string     `json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	SentAt         *time.Time `json:"sent_at"`
}

type Health struct {
	Enabled         bool       `json:"enabled"`
	QueueDepth      int        `json:"queue_depth"`
	RetryCount      int        `json:"retry_count"`
	ProcessingCount int        `json:"processing_count"`
	DeadLetterCount int        `json:"dead_letter_count"`
	LastSentAt      *time.Time `json:"last_sent_at"`
}
