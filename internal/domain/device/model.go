package device

import "time"

type Device struct {
	ID                          string    `json:"id"`
	MACAddress                  string    `json:"mac_address"`
	CurrentIPAddress            string    `json:"current_ip_address"`
	DeviceType                  string    `json:"device_type"`
	RegisteredVendor            string    `json:"registered_vendor"`
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
	ClassificationMethod        string    `json:"classification_method"`
	TrustLevel                  string    `json:"trust_level"`
	AuthenticationMethod        string    `json:"authentication_method"`
	AuthenticationStatus        string    `json:"authentication_status"`
	SophosUsername              string    `json:"sophos_username"`
	SophosLastIP                string    `json:"sophos_last_ip"`
	SophosLastSeenAt            time.Time `json:"sophos_last_seen_at"`
	LastPolicyDecision          string    `json:"last_policy_decision"`
	LastPolicyEvaluatedAt       time.Time `json:"last_policy_evaluated_at"`
	CurrentPolicyDecisionID     string    `json:"current_policy_decision_id"`
	CurrentPolicyID             string    `json:"current_policy_id"`
	CurrentPolicyName           string    `json:"current_policy_name"`
	CurrentDecisionType         string    `json:"current_decision_type"`
	CurrentTargetVLAN           int       `json:"current_target_vlan"`
	CurrentEnforcementAction    string    `json:"current_enforcement_action"`
	CurrentTrustScore           int       `json:"current_trust_score"`
	CurrentDecisionDryRun       bool      `json:"current_decision_dry_run"`
	CurrentReasonCodes          []string  `json:"current_reason_codes"`
	CurrentDecisionExplanation  string    `json:"current_decision_explanation"`
	CurrentPolicyDecisionAt     time.Time `json:"current_policy_decision_at"`
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
	RegisteredOwner             string    `json:"registered_owner"`
	OwnerUsername               string    `json:"owner_username"`
	OwnerDepartment             string    `json:"owner_department"`
	OwnerRole                   string    `json:"owner_role"`
	DefaultVLANID               int       `json:"default_vlan_id"`
	DefaultVLANName             string    `json:"default_vlan_name"`
	AssignedPolicy              string    `json:"assigned_policy"`
	EnrichmentSource            string    `json:"enrichment_source"`
	EnrichmentStatus            string    `json:"enrichment_status"`
	EnrichmentError             string    `json:"enrichment_error"`
	EnrichedAt                  time.Time `json:"enriched_at"`
	LDAPDeviceCN                string    `json:"ldap_device_cn"`
	LDAPOwnerDN                 string    `json:"ldap_owner_dn"`
	LDAPLocationDN              string    `json:"ldap_location_dn"`
	LDAPOwnershipType           string    `json:"ldap_ownership_type"`
	LDAPDepartment              string    `json:"ldap_department"`
	LDAPAssetTag                string    `json:"ldap_asset_tag"`
	LDAPPolicyName              string    `json:"ldap_policy_name"`
	LDAPVendor                  string    `json:"ldap_vendor"`
	LDAPModel                   string    `json:"ldap_model"`
	LDAPDeviceStatus            string    `json:"ldap_device_status"`
	LDAPVLANID                  int       `json:"ldap_vlan_id"`
	LDAPVLANName                string    `json:"ldap_vlan_name"`
	LDAPDefaultVLANID           int       `json:"ldap_default_vlan_id"`
	LDAPDefaultVLANName         string    `json:"ldap_default_vlan_name"`
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

type Observation struct {
	ID          string    `json:"id"`
	DeviceID    string    `json:"device_id"`
	MACAddress  string    `json:"mac_address"`
	IPAddress   string    `json:"ip_address"`
	SwitchID    string    `json:"switch_id"`
	PortIfIndex int       `json:"port_ifindex"`
	VLANID      int       `json:"vlan_id"`
	Source      string    `json:"source"`
	ObservedAt  time.Time `json:"observed_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type EnrichmentUpdate struct {
	MACAddress            string
	DeviceType            string
	RegisteredVendor      string
	Description           string
	RegisteredOwner       string
	OwnerUsername         string
	OwnerDepartment       string
	OwnerRole             string
	DefaultVLANID         int
	DefaultVLANName       string
	AssignedPolicy        string
	TrustLevel            string
	EnrichmentSource      string
	EnrichmentStatus      string
	EnrichmentError       string
	EnrichedAt            time.Time
	PolicyAction          string
	PolicyReason          string
	LastPolicyDecision    string
	LastPolicyEvaluatedAt time.Time
	Status                string
	ClassificationMethod  string
	LDAPDeviceCN          string
	LDAPOwnerDN           string
	LDAPLocationDN        string
	LDAPOwnershipType     string
	LDAPDepartment        string
	LDAPAssetTag          string
	LDAPPolicyName        string
	LDAPVendor            string
	LDAPModel             string
	LDAPDeviceStatus      string
	LDAPVLANID            int
	LDAPVLANName          string
	LDAPDefaultVLANID     int
	LDAPDefaultVLANName   string
}

type AgentlessObservationInput struct {
	MACAddress           string    `json:"mac_address"`
	IPAddress            string    `json:"ip_address"`
	Hostname             string    `json:"hostname"`
	VendorClass          string    `json:"vendor_class"`
	DeviceType           string    `json:"device_type"`
	SwitchID             string    `json:"switch_id"`
	SwitchName           string    `json:"switch_name"`
	ManagementIP         string    `json:"management_ip"`
	IfIndex              int       `json:"if_index"`
	InterfaceName        string    `json:"interface_name"`
	InterfaceDescription string    `json:"interface_description"`
	SourceType           string    `json:"source_type"`
	Confidence           string    `json:"confidence"`
	ObservedAt           time.Time `json:"observed_at"`
}
