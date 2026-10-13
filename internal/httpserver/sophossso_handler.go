package httpserver

import (
	"context"
	"net/http"

	domain "nac/internal/domain/sophossso"
)

type sophosSSOService interface {
	List(ctx context.Context, limit, offset int) ([]domain.Item, error)
	Health(ctx context.Context) (domain.Health, error)
}

func registerSophosSSORoutes(mux *http.ServeMux, service sophosSSOService) {
	if service == nil {
		return
	}
	mux.HandleFunc("/api/v1/sophos-sso/outbox", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit, offset, err := parseLimitOffset(r, 50, 200)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items, err := service.List(r.Context(), limit, offset)
		if err != nil {
			http.Error(w, "failed to list Sophos SSO outbox", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
	mux.HandleFunc("/api/v1/sophos-sso/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		health, err := service.Health(r.Context())
		if err != nil {
			http.Error(w, "failed to read Sophos SSO health", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, health)
	})
}
