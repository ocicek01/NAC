package enforcement

import "time"

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
	Executed bool    `json:"executed"`
	Output   string  `json:"output"`
}
