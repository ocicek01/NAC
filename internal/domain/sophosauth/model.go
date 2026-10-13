package sophosauth

import "time"

type Event struct {
	ID                string         `json:"id"`
	Fingerprint       string         `json:"fingerprint"`
	EventType         string         `json:"event_type"`
	MessageID         string         `json:"message_id"`
	Username          string         `json:"username"`
	UserGroup         string         `json:"user_group"`
	ClientUsed        string         `json:"client_used"`
	AuthMechanism     string         `json:"auth_mechanism"`
	AuthStatus        string         `json:"auth_status"`
	SourceIP          string         `json:"source_ip"`
	SourceMAC         string         `json:"source_mac"`
	CorrelatedMAC     string         `json:"correlated_mac"`
	CorrelationStatus string         `json:"correlation_status"`
	Metadata          map[string]any `json:"metadata"`
	EventAt           time.Time      `json:"event_at"`
	ReceivedAt        time.Time      `json:"received_at"`
}

type Session struct {
	ID                string     `json:"id"`
	Username          string     `json:"username"`
	UserGroup         string     `json:"user_group"`
	IPAddress         string     `json:"ip_address"`
	MACAddress        string     `json:"mac_address"`
	ClientUsed        string     `json:"client_used"`
	AuthMechanism     string     `json:"auth_mechanism"`
	Status            string     `json:"status"`
	CorrelationStatus string     `json:"correlation_status"`
	StartedAt         time.Time  `json:"started_at"`
	LastSeenAt        time.Time  `json:"last_seen_at"`
	EndedAt           *time.Time `json:"ended_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type ParsedEvent struct {
	Fingerprint   string
	EventType     string
	MessageID     string
	Username      string
	UserGroup     string
	ClientUsed    string
	AuthMechanism string
	AuthStatus    string
	SourceIP      string
	SourceMAC     string
	Message       string
	EventAt       time.Time
	Metadata      map[string]any
}

type Health struct {
	Enabled          bool       `json:"enabled"`
	BindAddress      string     `json:"bind_address"`
	AllowedSources   int        `json:"allowed_source_count"`
	ActiveSessions   int        `json:"active_sessions"`
	Uncorrelated     int        `json:"uncorrelated_events"`
	Ambiguous        int        `json:"ambiguous_events"`
	LastEventAt      *time.Time `json:"last_event_at"`
	LastCorrelatedAt *time.Time `json:"last_correlated_at"`
}
