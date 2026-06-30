package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	sessiondomain "nac/internal/domain/session"
)

type sessionService interface {
	ListRecent(ctx context.Context, limit int) ([]sessiondomain.Session, error)
	ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]sessiondomain.Session, error)
}

func registerSessionRoutes(mux *http.ServeMux, service sessionService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/radius-sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 50
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
				limit = parsed
			}
		}

		macAddress := strings.TrimSpace(r.URL.Query().Get("mac"))
		var (
			items []sessiondomain.Session
			err   error
		)
		if macAddress != "" {
			items, err = service.ListRecentByMAC(r.Context(), macAddress, limit)
		} else {
			items, err = service.ListRecent(r.Context(), limit)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	})
}
