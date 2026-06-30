package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	policy "nac/internal/domain/policy"
	policyservice "nac/internal/service/policy"
)

type policyService interface {
	ListActive(ctx context.Context) ([]policy.Policy, error)
	Create(ctx context.Context, input policyservice.CreateInput) (policy.Policy, error)
	Disable(ctx context.Context, id string) error
}

func registerPolicyRoutes(mux *http.ServeMux, service policyService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/policies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := service.ListActive(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(items)
		case http.MethodPost:
			var payload struct {
				Name          string `json:"name"`
				Description   string `json:"description"`
				Action        string `json:"action"`
				MatchField    string `json:"match_field"`
				MatchOperator string `json:"match_operator"`
				MatchValue    string `json:"match_value"`
				Priority      int    `json:"priority"`
			}

			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			item, err := service.Create(r.Context(), policyservice.CreateInput{
				Name:          payload.Name,
				Description:   payload.Description,
				Action:        payload.Action,
				MatchField:    payload.MatchField,
				MatchOperator: payload.MatchOperator,
				MatchValue:    payload.MatchValue,
				Priority:      payload.Priority,
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(item)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/policies/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/policies/")
		if !strings.HasSuffix(path, "/disable") {
			http.NotFound(w, r)
			return
		}

		id := strings.TrimSuffix(path, "/disable")
		id = strings.Trim(id, "/")
		if id == "" {
			http.Error(w, "policy id is required", http.StatusBadRequest)
			return
		}

		if err := service.Disable(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
