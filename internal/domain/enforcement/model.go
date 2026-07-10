package enforcement

import "time"

const (
	ModeDisabled = "disabled"
	ModeDryRun   = "dry_run"
	ModePilot    = "pilot"
	ModeEnabled  = "enabled"
)

const (
	ActionNone                 = "none"
	ActionMonitorOnly          = "monitor_only"
	ActionAssignVLAN           = "assign_vlan"
	ActionAssignRestrictedVLAN = "assign_restricted_vlan"
	ActionAssignQuarantineVLAN = "assign_quarantine_vlan"
	ActionShutdownPort         = "shutdown_port"
	ActionEnablePort           = "enable_port"
	ActionBouncePort           = "bounce_port"
	ActionRestorePreviousState = "restore_previous_state"
)

const (
	RequestStatusPending            = "pending"
	RequestStatusQueued             = "queued"
	RequestStatusRunning            = "running"
	RequestStatusVerifying          = "verifying"
	RequestStatusSucceeded          = "succeeded"
	RequestStatusFailed             = "failed"
	RequestStatusVerificationFailed = "verification_failed"
	RequestStatusSkipped            = "skipped"
	RequestStatusRetryScheduled     = "retry_scheduled"
	RequestStatusCancelled          = "cancelled"
	RequestStatusRollbackPending    = "rollback_pending"
	RequestStatusRolledBack         = "rolled_back"
	RequestStatusRollbackFailed     = "rollback_failed"
)

type Decision struct {
	ID                   string    `json:"id"`
	DeviceMACAddress     string    `json:"device_mac_address"`
	DeviceHostname       string    `json:"device_hostname"`
	PolicyAction         string    `json:"policy_action"`
	PolicyReason         string    `json:"policy_reason"`
	DecisionAction       string    `json:"decision_action"`
	DecisionMode         string    `json:"decision_mode"`
	SelectedMethod       string    `json:"selected_method"`
	FallbackMethods      []string  `json:"fallback_methods"`
	RequiresApproval     bool      `json:"requires_approval"`
	ApprovalStatus       string    `json:"approval_status"`
	AttemptCount         int       `json:"attempt_count"`
	MaxAttempts          int       `json:"max_attempts"`
	NextAttemptAt        time.Time `json:"next_attempt_at"`
	LastError            string    `json:"last_error"`
	SwitchID             string    `json:"switch_id"`
	SwitchName           string    `json:"switch_name"`
	ManagementIP         string    `json:"management_ip"`
	BridgePort           int       `json:"bridge_port"`
	IfIndex              int       `json:"if_index"`
	InterfaceName        string    `json:"interface_name"`
	InterfaceDescription string    `json:"interface_description"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
}

type Request struct {
	ID                   string         `json:"id"`
	DeviceID             string         `json:"device_id"`
	PolicyDecisionID     string         `json:"policy_decision_id"`
	SwitchID             string         `json:"switch_id"`
	PortID               string         `json:"port_id"`
	RequestedAction      string         `json:"requested_action"`
	TargetVLAN           int            `json:"target_vlan"`
	PreviousVLAN         int            `json:"previous_vlan"`
	RequestedBy          string         `json:"requested_by"`
	RequestSource        string         `json:"request_source"`
	Mode                 string         `json:"mode"`
	Status               string         `json:"status"`
	AttemptCount         int            `json:"attempt_count"`
	Adapter              string         `json:"adapter"`
	CommandSummary       string         `json:"command_summary"`
	ErrorCode            string         `json:"error_code"`
	ErrorMessage         string         `json:"error_message"`
	RequestedAt          time.Time      `json:"requested_at"`
	StartedAt            time.Time      `json:"started_at"`
	CompletedAt          time.Time      `json:"completed_at"`
	VerifiedAt           time.Time      `json:"verified_at"`
	RollbackOfRequestID  string         `json:"rollback_of_request_id"`
	VerificationStatus   string         `json:"verification_status"`
	CurrentSwitchID      string         `json:"current_switch_id"`
	CurrentIfIndex       int            `json:"current_if_index"`
	CurrentInterfaceName string         `json:"current_interface_name"`
	TargetDeviceMAC      string         `json:"target_device_mac"`
	Metadata             map[string]any `json:"metadata"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type Result struct {
	ID                   string         `json:"id"`
	EnforcementRequestID string         `json:"enforcement_request_id"`
	AttemptNumber        int            `json:"attempt_number"`
	Adapter              string         `json:"adapter"`
	Transport            string         `json:"transport"`
	Action               string         `json:"action"`
	Success              bool           `json:"success"`
	Changed              bool           `json:"changed"`
	ExecutionStatus      string         `json:"execution_status"`
	VerificationStatus   string         `json:"verification_status"`
	PreviousState        map[string]any `json:"previous_state"`
	ExpectedState        map[string]any `json:"expected_state"`
	ObservedState        map[string]any `json:"observed_state"`
	CommandSummary       string         `json:"command_summary"`
	AdapterResponse      map[string]any `json:"adapter_response"`
	DurationMS           int64          `json:"duration_ms"`
	ErrorCode            string         `json:"error_code"`
	ErrorMessage         string         `json:"error_message"`
	StartedAt            time.Time      `json:"started_at"`
	CompletedAt          time.Time      `json:"completed_at"`
	VerifiedAt           time.Time      `json:"verified_at"`
	CreatedAt            time.Time      `json:"created_at"`
}

type PortState struct {
	VLANID        int            `json:"vlan_id"`
	AdminStatus   string         `json:"admin_status"`
	OperStatus    string         `json:"oper_status"`
	PortMode      string         `json:"port_mode"`
	Protected     bool           `json:"protected"`
	InterfaceName string         `json:"interface_name"`
	Metadata      map[string]any `json:"metadata"`
}

type RequestInput struct {
	PolicyDecisionID string
	RequestedBy      string
	RequestSource    string
	ForceExecution   bool
	Reason           string
	TargetVLAN       int
	ActionOverride   string
}

type RollbackInput struct {
	RequestedBy    string
	RequestSource  string
	Reason         string
	ForceExecution bool
}

type WorkerOutcome struct {
	Request Request `json:"request"`
	Result  Result  `json:"result"`
}

type WorkerStats struct {
	Running                bool      `json:"running"`
	QueueDepth             int       `json:"queue_depth"`
	OldestPendingAgeSec    int64     `json:"oldest_pending_age_seconds"`
	RunningRequestCount    int       `json:"running_request_count"`
	FailedRequestCount     int       `json:"failed_request_count"`
	RetryScheduledCount    int       `json:"retry_scheduled_count"`
	LastSuccessfulAt       time.Time `json:"last_successful_at"`
	LastWorkerError        string    `json:"last_worker_error"`
	LastWorkerErrorAt      time.Time `json:"last_worker_error_at"`
	LastWorkerHeartbeatAt  time.Time `json:"last_worker_heartbeat_at"`
}

type VLANPlan struct {
	SwitchID       string   `json:"switch_id"`
	SwitchName     string   `json:"switch_name"`
	ManagementIP   string   `json:"management_ip"`
	BridgePort     int      `json:"bridge_port"`
	IfIndex        int      `json:"if_index"`
	InterfaceName  string   `json:"interface_name"`
	VLANID         int      `json:"vlan_id"`
	SelectedMethod string   `json:"selected_method"`
	Commands       []string `json:"commands"`
	OIDs           []string `json:"oids"`
}

type VLANExecutionResult struct {
	Plan     VLANPlan `json:"plan"`
	Executed bool     `json:"executed"`
	Output   string   `json:"output"`
}
