package httpserver

import (
	"context"
	"encoding/json"
	"net/http"

	"nac/internal/service/maclookup"
)

type macLookupService interface {
	Lookup(ctx context.Context, req maclookup.Request) ([]maclookup.Result, error)
}

func registerMACLookupRoutes(mux *http.ServeMux, service macLookupService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/mac-lookups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req maclookup.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		results, err := service.Lookup(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(results)
	})
}
