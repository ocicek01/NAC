package httpserver

import (
	"context"
	"encoding/json"
	"net/http"

	"nac/internal/domain/topology"
)

type topologyService interface {
	Create(ctx context.Context, link topology.Link) (topology.Link, error)
	List(ctx context.Context) ([]topology.Link, error)
	DiscoverLLDP(ctx context.Context) ([]topology.Link, error)
}

func registerTopologyRoutes(mux *http.ServeMux, service topologyService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/topology-links", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			links, err := service.List(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(links)
		case http.MethodPost:
			var link topology.Link
			if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			created, err := service.Create(r.Context(), link)
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

	mux.HandleFunc("/api/v1/topology-links/discover", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		links, err := service.DiscoverLLDP(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(links)
	})
}
