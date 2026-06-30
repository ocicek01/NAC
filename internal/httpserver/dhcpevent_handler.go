package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	domain "nac/internal/domain/dhcpevent"
)

type dhcpEventIngestor interface {
	Ingest(ctx context.Context, event domain.Event) (domain.Event, error)
	IngestSample(ctx context.Context) (domain.Event, error)
}

func registerDHCPEventRoutes(mux *http.ServeMux, ingestor dhcpEventIngestor) {
	if ingestor == nil {
		return
	}

	mux.HandleFunc("/api/v1/dhcp-events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var event domain.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		created, err := ingestor.Ingest(r.Context(), event)
		if err != nil {
			if errors.Is(err, domain.ErrDuplicateSuppressed) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status": "suppressed",
					"event":  created,
				})
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	})

	mux.HandleFunc("/internal/dev/dhcp-events/sample", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		created, err := ingestor.IngestSample(r.Context())
		if err != nil {
			if errors.Is(err, domain.ErrDuplicateSuppressed) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status": "suppressed",
					"event":  created,
				})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	})
}
