package httpserver

import (
	"context"
	"encoding/json"
	"net/http"

	domain "nac/internal/domain/macobservation"
)

type macObservationService interface {
	ListRecent(ctx context.Context, limit int) ([]domain.Observation, error)
}

func registerMACObservationRoutes(mux *http.ServeMux, service macObservationService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/mac-observations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		items, err := service.ListRecent(r.Context(), 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	})
}
