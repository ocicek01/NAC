package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	auditlogdomain "nac/internal/domain/auditlog"
	porteventdomain "nac/internal/domain/portevent"
)

type portEventService interface {
	Ingest(ctx context.Context, event porteventdomain.Event) (porteventdomain.Event, error)
	ListRecent(ctx context.Context, limit int) ([]porteventdomain.Event, error)
}

type auditLogService interface {
	ListRecent(ctx context.Context, limit int) ([]auditlogdomain.Log, error)
}

func registerPortEventRoutes(mux *http.ServeMux, service portEventService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/port-events", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			items, err := service.ListRecent(r.Context(), limit)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(items)
		case http.MethodPost:
			var event porteventdomain.Event
			if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}
			created, err := service.Ingest(r.Context(), event)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(created)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func registerAuditLogRoutes(mux *http.ServeMux, service auditLogService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		items, err := service.ListRecent(r.Context(), limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	})
}
