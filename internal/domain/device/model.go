package device

import "time"

type Device struct {
	ID                          string    `json:"id"`
	MACAddress                  string    `json:"mac_address"`
	CurrentIPAddress            string    `json:"current_ip_address"`
	DeviceType                  string    `json:"device_type"`
	Label                       string    `json:"label"`
	Description                 string    `json:"description"`
	Hostname                    string    `json:"hostname"`
	VendorClass                 string    `json:"vendor_class"`
	Status                      string    `json:"status"`
	ApprovedAt                  time.Time `json:"approved_at"`
	ApprovedBy                  string    `json:"approved_by"`
	ExpiresAt                   time.Time `json:"expires_at"`
	PolicyAction                string    `json:"policy_action"`
	PolicyReason                string    `json:"policy_reason"`
	CurrentSwitchID             string    `json:"current_switch_id"`
	CurrentSwitchName           string    `json:"current_switch_name"`
	CurrentManagementIP         string    `json:"current_management_ip"`
	CurrentPortID               string    `json:"current_port_id"`
	CurrentBridgePort           int       `json:"current_bridge_port"`
	CurrentIfIndex              int       `json:"current_if_index"`
	CurrentInterfaceName        string    `json:"current_interface_name"`
	CurrentInterfaceDescription string    `json:"current_interface_description"`
	CurrentSourceType           string    `json:"current_source_type"`
	CurrentConfidence           string    `json:"current_confidence"`
	IdentityType                string    `json:"identity_type"`
	IdentitySource              string    `json:"identity_source"`
	IdentityUsername            string    `json:"identity_username"`
	IdentityFullName            string    `json:"identity_full_name"`
	LastEnforcementAction       string    `json:"last_enforcement_action"`
	LastEnforcementVLAN         int       `json:"last_enforcement_vlan"`
	LastEnforcementStatus       string    `json:"last_enforcement_status"`
	LastEnforcementSwitchID     string    `json:"last_enforcement_switch_id"`
	LastEnforcementIfIndex      int       `json:"last_enforcement_if_index"`
	LastEnforcementAt           time.Time `json:"last_enforcement_at"`
	DesiredEnforcementState     string    `json:"desired_enforcement_state"`
	AppliedEnforcementState     string    `json:"applied_enforcement_state"`
	AppliedEnforcementVLAN      int       `json:"applied_enforcement_vlan"`
	LastEnforcementMethod       string    `json:"last_enforcement_method"`
	LastEnforcementAttemptAt    time.Time `json:"last_enforcement_attempt_at"`
	LastEnforcementSuccessAt    time.Time `json:"last_enforcement_success_at"`
	LastEnforcementError        string    `json:"last_enforcement_error"`
	LastEnforcementRetryCount   int       `json:"last_enforcement_retry_count"`
	IPLearningStatus            string    `json:"ip_learning_status"`
	IPLearningStartedAt         time.Time `json:"ip_learning_started_at"`
	IPLearnedAt                 time.Time `json:"ip_learned_at"`
	LastPortBounceAt            time.Time `json:"last_port_bounce_at"`
	FirstSeenAt                 time.Time `json:"first_seen_at"`
	LastSeenAt                  time.Time `json:"last_seen_at"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

type IdentitySnapshot struct {
	ID             string         `json:"id"`
	DeviceID       string         `json:"device_id"`
	IdentityType   string         `json:"identity_type"`
	IdentitySource string         `json:"identity_source"`
	ExternalID     string         `json:"external_id"`
	Username       string         `json:"username"`
	FullName       string         `json:"full_name"`
	Attributes     map[string]any `json:"attributes"`
	VerifiedAt     time.Time      `json:"verified_at"`
	ExpiresAt      time.Time      `json:"expires_at"`
	CreatedAt      time.Time      `json:"created_at"`
}
