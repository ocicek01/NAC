package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	policy "nac/internal/domain/policy"
	policyservice "nac/internal/service/policy"
)

type policyService interface {
	List(ctx context.Context, limit, offset int) ([]policy.Policy, error)
	ListActive(ctx context.Context) ([]policy.Policy, error)
	Create(ctx context.Context, input policyservice.CreateInput) (policy.Policy, error)
	FindByID(ctx context.Context, id string) (*policy.Policy, error)
	Update(ctx context.Context, id string, input policyservice.UpdateInput) (policy.Policy, error)
	Disable(ctx context.Context, id string) error
	ListDecisions(ctx context.Context, limit, offset int) ([]policy.Decision, error)
}

func registerPolicyRoutes(mux *http.ServeMux, service policyService) {
	if service == nil {
		return
	}
	mux.HandleFunc("/api/v1/policies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			limit, offset, err := parseLimitOffset(r, 50, 200)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			items, err := service.List(r.Context(), limit, offset)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload struct {
				Name, Description, DecisionType, EnforcementAction string
				Priority, TargetVLAN                               int
				Enabled, DryRun                                    bool
				MatchConditions                                    []policy.Condition `json:"match_conditions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}
			item, err := service.Create(r.Context(), policyservice.CreateInput{Name: payload.Name, Description: payload.Description, Priority: payload.Priority, Enabled: payload.Enabled, MatchConditions: payload.MatchConditions, DecisionType: payload.DecisionType, TargetVLAN: payload.TargetVLAN, EnforcementAction: payload.EnforcementAction, DryRun: payload.DryRun})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/policies/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/policies/"), "/")
		if path == "" {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(path, "/disable") {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			id := strings.Trim(strings.TrimSuffix(path, "/disable"), "/")
			if err := service.Disable(r.Context(), id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch r.Method {
		case http.MethodGet:
			item, err := service.FindByID(r.Context(), path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if item == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case http.MethodPatch:
			var payload struct {
				Name, Description, DecisionType, EnforcementAction *string
				Priority, TargetVLAN                               *int
				Enabled, DryRun                                    *bool
				MatchConditions                                    []policy.Condition `json:"match_conditions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}
			item, err := service.Update(r.Context(), path, policyservice.UpdateInput{Name: payload.Name, Description: payload.Description, Priority: payload.Priority, Enabled: payload.Enabled, MatchConditions: payload.MatchConditions, DecisionType: payload.DecisionType, TargetVLAN: payload.TargetVLAN, EnforcementAction: payload.EnforcementAction, DryRun: payload.DryRun})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/policy-decisions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit, offset, err := parseLimitOffset(r, 50, 200)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items, err := service.ListDecisions(r.Context(), limit, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
}

func parseLimitOffset(r *http.Request, defaultLimit, maxLimit int) (int, int, error) {
	limit, offset := defaultLimit, 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return 0, 0, fmt.Errorf("invalid limit")
		}
		if parsed > maxLimit {
			parsed = maxLimit
		}
		limit = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return 0, 0, fmt.Errorf("invalid offset")
		}
		offset = parsed
	}
	return limit, offset, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
