package policy

import "time"

type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type Policy struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	Type              string      `json:"type"`
	Action            string      `json:"action"`
	MatchField        string      `json:"match_field"`
	MatchOperator     string      `json:"match_operator"`
	MatchValue        string      `json:"match_value"`
	Priority          int         `json:"priority"`
	Status            string      `json:"status"`
	Enabled           bool        `json:"enabled"`
	MatchConditions   []Condition `json:"match_conditions"`
	DecisionType      string      `json:"decision_type"`
	TargetVLAN        int         `json:"target_vlan"`
	EnforcementAction string      `json:"enforcement_action"`
	DryRun            bool        `json:"dry_run"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type TrustSignal struct {
	Code   string `json:"code"`
	Effect int    `json:"effect"`
}

type TrustScoreResult struct {
	ID                 string        `json:"id"`
	DeviceID           string        `json:"device_id"`
	Score              int           `json:"score"`
	Signals            []TrustSignal `json:"signals"`
	CalculatedAt       time.Time     `json:"calculated_at"`
	CalculationVersion string        `json:"calculation_version"`
}

type Decision struct {
	ID                   string        `json:"id"`
	DeviceID             string        `json:"device_id"`
	PortEventID          string        `json:"port_event_id"`
	PolicyID             string        `json:"policy_id"`
	PolicyName           string        `json:"policy_name"`
	DecisionType         string        `json:"decision_type"`
	TargetVLAN           int           `json:"target_vlan"`
	EnforcementAction    string        `json:"enforcement_action"`
	TrustScore           int           `json:"trust_score"`
	TrustSignals         []TrustSignal `json:"trust_signals"`
	ReasonCodes          []string      `json:"reason_codes"`
	Explanation          string        `json:"explanation"`
	DryRun               bool          `json:"dry_run"`
	EnforcementStatus    string        `json:"enforcement_status"`
	EvaluationDurationMS int64         `json:"evaluation_duration_ms"`
	CreatedAt            time.Time     `json:"created_at"`
}
