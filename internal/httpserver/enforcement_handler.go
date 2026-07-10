package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	domain "nac/internal/domain/enforcement"
)

type enforcementService interface {
	ListRequests(ctx context.Context, limit, offset int) ([]domain.Request, error)
	FindRequestByID(ctx context.Context, id string) (*domain.Request, error)
	ListResultsByRequest(ctx context.Context, requestID string) ([]domain.Result, error)
	ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]domain.Request, error)
	EnforcePolicyDecision(ctx context.Context, decisionID string, input domain.RequestInput) (domain.Request, error)
	RetryRequestByID(ctx context.Context, id string) (*domain.Request, error)
	RollbackRequestByID(ctx context.Context, id string, input domain.RollbackInput) (domain.Request, error)
}

type enforceDecisionRequest struct {
	ForceExecution bool   `json:"force_execution"`
	RequestedBy    string `json:"requested_by"`
	RequestSource  string `json:"request_source"`
	Reason         string `json:"reason"`
	TargetVLAN     int    `json:"target_vlan"`
	ActionOverride string `json:"action_override"`
}

type rollbackRequest struct {
	ForceExecution bool   `json:"force_execution"`
	RequestedBy    string `json:"requested_by"`
	RequestSource  string `json:"request_source"`
	Reason         string `json:"reason"`
}

func registerEnforcementRoutes(mux *http.ServeMux, service enforcementService) {
	if service == nil {
		return
	}

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
		items, err := service.ListRequests(r.Context(), limit, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, items)
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
			writeJSON(w, http.StatusOK, item)
			return
		}
		switch {
		case r.Method == http.MethodGet && parts[1] == "results":
			items, err := service.ListResultsByRequest(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, items)
			return
		case r.Method == http.MethodPost && parts[1] == "retry":
			item, err := service.RetryRequestByID(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusAccepted, item)
			return
		case r.Method == http.MethodPost && parts[1] == "rollback":
			var payload rollbackRequest
			if r.Body != nil {
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err.Error() != "EOF" {
					http.Error(w, "invalid json body", http.StatusBadRequest)
					return
				}
			}
			item, err := service.RollbackRequestByID(r.Context(), id, domain.RollbackInput{RequestedBy: payload.RequestedBy, RequestSource: payload.RequestSource, Reason: payload.Reason, ForceExecution: payload.ForceExecution})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusAccepted, item)
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
		var payload enforceDecisionRequest
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err.Error() != "EOF" {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}
		}
		item, err := service.EnforcePolicyDecision(r.Context(), decisionID, domain.RequestInput{RequestedBy: payload.RequestedBy, RequestSource: payload.RequestSource, Reason: payload.Reason, TargetVLAN: payload.TargetVLAN, ActionOverride: payload.ActionOverride, ForceExecution: payload.ForceExecution})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		status := http.StatusAccepted
		if item.Status == domain.RequestStatusSkipped || item.Status == domain.RequestStatusFailed {
			status = http.StatusOK
		}
		writeJSON(w, status, item)
	})
}

func parsePositiveInt(raw string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
