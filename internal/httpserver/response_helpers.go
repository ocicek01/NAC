package httpserver

import (
	"time"

	enforcementdomain "nac/internal/domain/enforcement"
	policydomain "nac/internal/domain/policy"
)

func nullableTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value
	return &copyValue
}

func toEnforcementRequestResponse(item enforcementdomain.Request) map[string]any {
	return map[string]any{
		"id":                     item.ID,
		"device_id":              item.DeviceID,
		"policy_decision_id":     item.PolicyDecisionID,
		"switch_id":              item.SwitchID,
		"port_id":                item.PortID,
		"requested_action":       item.RequestedAction,
		"target_vlan":            item.TargetVLAN,
		"previous_vlan":          item.PreviousVLAN,
		"requested_by":           item.RequestedBy,
		"request_source":         item.RequestSource,
		"mode":                   item.Mode,
		"status":                 item.Status,
		"attempt_count":          item.AttemptCount,
		"adapter":                item.Adapter,
		"command_summary":        item.CommandSummary,
		"error_code":             item.ErrorCode,
		"error_message":          item.ErrorMessage,
		"requested_at":           item.RequestedAt,
		"started_at":             nullableTime(item.StartedAt),
		"completed_at":           nullableTime(item.CompletedAt),
		"verified_at":            nullableTime(item.VerifiedAt),
		"rollback_of_request_id": item.RollbackOfRequestID,
		"verification_status":    item.VerificationStatus,
		"current_switch_id":      item.CurrentSwitchID,
		"current_if_index":       item.CurrentIfIndex,
		"current_interface_name": item.CurrentInterfaceName,
		"target_device_mac":      item.TargetDeviceMAC,
		"metadata":               item.Metadata,
		"created_at":             item.CreatedAt,
		"updated_at":             item.UpdatedAt,
	}
}

func toEnforcementRequestResponses(items []enforcementdomain.Request) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, toEnforcementRequestResponse(item))
	}
	return out
}

func toEnforcementResultResponse(item enforcementdomain.Result) map[string]any {
	return map[string]any{
		"id":                     item.ID,
		"enforcement_request_id": item.EnforcementRequestID,
		"attempt_number":         item.AttemptNumber,
		"adapter":                item.Adapter,
		"transport":              item.Transport,
		"action":                 item.Action,
		"success":                item.Success,
		"changed":                item.Changed,
		"execution_status":       item.ExecutionStatus,
		"verification_status":    item.VerificationStatus,
		"previous_state":         item.PreviousState,
		"expected_state":         item.ExpectedState,
		"observed_state":         item.ObservedState,
		"command_summary":        item.CommandSummary,
		"adapter_response":       item.AdapterResponse,
		"duration_ms":            item.DurationMS,
		"error_code":             item.ErrorCode,
		"error_message":          item.ErrorMessage,
		"started_at":             nullableTime(item.StartedAt),
		"completed_at":           nullableTime(item.CompletedAt),
		"verified_at":            nullableTime(item.VerifiedAt),
		"created_at":             item.CreatedAt,
	}
}

func toEnforcementResultResponses(items []enforcementdomain.Result) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, toEnforcementResultResponse(item))
	}
	return out
}

func toPolicyDecisionResponse(item policydomain.Decision) map[string]any {
	return map[string]any{
		"id":                       item.ID,
		"device_id":                item.DeviceID,
		"port_event_id":            item.PortEventID,
		"policy_id":                item.PolicyID,
		"policy_name":              item.PolicyName,
		"decision_type":            item.DecisionType,
		"target_vlan":              item.TargetVLAN,
		"enforcement_action":       item.EnforcementAction,
		"trust_score":              item.TrustScore,
		"trust_signals":            item.TrustSignals,
		"reason_codes":             item.ReasonCodes,
		"explanation":              item.Explanation,
		"dry_run":                  item.DryRun,
		"enforcement_status":       item.EnforcementStatus,
		"enforcement_requested":    item.EnforcementRequested,
		"enforcement_request_id":   item.EnforcementRequestID,
		"enforcement_started_at":   nullableTime(item.EnforcementStartedAt),
		"enforcement_completed_at": nullableTime(item.EnforcementCompletedAt),
		"enforcement_error":        item.EnforcementError,
		"enforced_at":              nullableTime(item.EnforcedAt),
		"evaluation_duration_ms":   item.EvaluationDurationMS,
		"created_at":               item.CreatedAt,
	}
}

func toPolicyDecisionResponses(items []policydomain.Decision) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, toPolicyDecisionResponse(item))
	}
	return out
}
