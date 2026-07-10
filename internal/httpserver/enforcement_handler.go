package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	domain "nac/internal/domain/enforcement"
)

type enforcementService interface {
	ListRequests(ctx context.Context, filters domain.RequestFilters) ([]domain.Request, error)
	FindRequestByID(ctx context.Context, id string) (*domain.Request, error)
	ListResultsByRequest(ctx context.Context, requestID string) ([]domain.Result, error)
	ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]domain.Request, error)
	EnforcePolicyDecision(ctx context.Context, decisionID string, input domain.RequestInput) (domain.Request, error)
	RetryRequestByID(ctx context.Context, id string) (*domain.Request, error)
	CancelRequestByID(ctx context.Context, id string) (*domain.Request, error)
	RollbackRequestByID(ctx context.Context, id string, input domain.RollbackInput) (domain.Request, error)
	WorkerStats(ctx context.Context) (domain.WorkerStats, error)
}

func registerEnforcementRoutes(mux *http.ServeMux, service enforcementService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/enforcement-worker/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		stats, err := service.WorkerStats(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"running": stats.Running, "queue_depth": stats.QueueDepth, "oldest_pending_age_seconds": stats.OldestPendingAgeSec, "running_request_count": stats.RunningRequestCount, "failed_request_count": stats.FailedRequestCount, "retry_scheduled_count": stats.RetryScheduledCount, "last_successful_at": nullableTime(stats.LastSuccessfulAt), "last_worker_error": stats.LastWorkerError, "last_worker_error_at": nullableTime(stats.LastWorkerErrorAt), "last_worker_heartbeat_at": nullableTime(stats.LastWorkerHeartbeatAt)})
	})

	mux.HandleFunc("/api/v1/enforcement-requests", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit, offset, err := parseLimitOffset(r, 50, 200)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		filters := domain.RequestFilters{Limit: limit, Offset: offset, DeviceID: strings.TrimSpace(r.URL.Query().Get("device_id")), SwitchID: strings.TrimSpace(r.URL.Query().Get("switch_id")), Status: strings.TrimSpace(r.URL.Query().Get("status")), Mode: strings.TrimSpace(r.URL.Query().Get("mode")), Action: strings.TrimSpace(r.URL.Query().Get("action"))}
		if filters.DateFrom, err = parseOptionalRFC3339(r.URL.Query().Get("date_from")); err != nil {
			http.Error(w, "invalid date_from", http.StatusBadRequest)
			return
		}
		if filters.DateTo, err = parseOptionalRFC3339(r.URL.Query().Get("date_to")); err != nil {
			http.Error(w, "invalid date_to", http.StatusBadRequest)
			return
		}
		items, err := service.ListRequests(r.Context(), filters)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, toEnforcementRequestResponses(items))
	})

	mux.HandleFunc("/api/v1/enforcement-requests/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/enforcement-requests/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			http.NotFound(w, r)
			return
		}
		id := strings.TrimSpace(parts[0])
		if len(parts) == 1 {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			item, err := service.FindRequestByID(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if item == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, http.StatusOK, toEnforcementRequestResponse(*item))
			return
		}
		switch {
		case r.Method == http.MethodGet && parts[1] == "results":
			items, err := service.ListResultsByRequest(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			limit, offset, err := parseLimitOffset(r, 50, 500)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if offset > len(items) {
				items = []domain.Result{}
			} else {
				end := offset + limit
				if end > len(items) {
					end = len(items)
				}
				items = items[offset:end]
			}
			writeJSON(w, http.StatusOK, toEnforcementResultResponses(items))
			return
		case r.Method == http.MethodPost && parts[1] == "retry":
			item, err := service.RetryRequestByID(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusAccepted, toEnforcementRequestResponse(*item))
			return
		case r.Method == http.MethodPost && parts[1] == "cancel":
			item, err := service.CancelRequestByID(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusAccepted, toEnforcementRequestResponse(*item))
			return
		case r.Method == http.MethodPost && parts[1] == "rollback":
			payload, err := decodeJSONMap(r)
			if err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}
			item, err := service.RollbackRequestByID(r.Context(), id, domain.RollbackInput{RequestedBy: readString(payload, "requested_by"), RequestSource: readString(payload, "request_source"), Reason: readString(payload, "reason"), ForceExecution: readBool(payload, "force_execution")})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusAccepted, toEnforcementRequestResponse(item))
			return
		default:
			http.NotFound(w, r)
			return
		}
	})

	mux.HandleFunc("/api/v1/policy-decisions/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/policy-decisions/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[1] != "enforce" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		decisionID := strings.TrimSpace(parts[0])
		payload, err := decodeJSONMap(r)
		if err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		item, err := service.EnforcePolicyDecision(r.Context(), decisionID, domain.RequestInput{RequestedBy: readString(payload, "requested_by"), RequestSource: readString(payload, "request_source"), Reason: readString(payload, "reason"), TargetVLAN: readInt(payload, "target_vlan"), ActionOverride: readString(payload, "action_override"), ForceExecution: readBool(payload, "force_execution")})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		status := http.StatusAccepted
		if item.Status == domain.RequestStatusSkipped || item.Status == domain.RequestStatusFailed || item.Status == domain.RequestStatusCancelled {
			status = http.StatusOK
		}
		writeJSON(w, status, toEnforcementRequestResponse(item))
	})
}

func parsePositiveInt(raw string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseOptionalRFC3339(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func decodeJSONMap(r *http.Request) (map[string]any, error) {
	payload := map[string]any{}
	if r.Body == nil {
		return payload, nil
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return payload, nil
}

func readString(payload map[string]any, key string) string {
	if value, ok := payload[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func readBool(payload map[string]any, key string) bool {
	if value, ok := payload[key].(bool); ok {
		return value
	}
	return false
}

func readInt(payload map[string]any, key string) int {
	if value, ok := payload[key].(float64); ok {
		return int(value)
	}
	if value, ok := payload[key].(int); ok {
		return value
	}
	return 0
}
