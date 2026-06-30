package httpserver

import (
	"context"
	"encoding/json"
	"net/http"

	domain "nac/internal/domain/radiusauth"
)

type radiusService interface {
	Authorize(ctx context.Context, req domain.AuthorizeRequest) (domain.AuthorizeResponse, error)
	Accounting(ctx context.Context, req domain.AccountingRequest) (domain.AccountingResponse, error)
}

func registerRadiusRoutes(mux *http.ServeMux, service radiusService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/radius/authorize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req domain.AuthorizeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		resp, err := service.Authorize(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/v1/radius/accounting", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req domain.AccountingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		resp, err := service.Accounting(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}
