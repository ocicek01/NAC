package httpserver

import (
	"context"
	"net/http"

	domain "nac/internal/domain/sophosauth"
)

type sophosAuthService interface {
	ListEvents(ctx context.Context, limit, offset int) ([]domain.Event, error)
	ListSessions(ctx context.Context, limit, offset int) ([]domain.Session, error)
	Health(ctx context.Context) (domain.Health, error)
}

func registerSophosAuthRoutes(mux *http.ServeMux, service sophosAuthService) {
	if service == nil {
		return
	}
	registerSophosAuthList(mux, "/api/v1/sophos-auth/events", func(ctx context.Context, limit, offset int) (any, error) {
		return service.ListEvents(ctx, limit, offset)
	})
	registerSophosAuthList(mux, "/api/v1/sophos-auth/sessions", func(ctx context.Context, limit, offset int) (any, error) {
		return service.ListSessions(ctx, limit, offset)
	})
	mux.HandleFunc("/api/v1/sophos-auth/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		health, err := service.Health(r.Context())
		if err != nil {
			http.Error(w, "failed to read Sophos auth health", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, health)
	})
}

func registerSophosAuthList(mux *http.ServeMux, path string, list func(context.Context, int, int) (any, error)) {
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit, offset, err := parseLimitOffset(r, 50, 200)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items, err := list(r.Context(), limit, offset)
		if err != nil {
			http.Error(w, "failed to list Sophos auth data", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
}
