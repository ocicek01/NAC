package httpserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"

	domain "nac/internal/domain/radiusauth"
)

type radiusService interface {
	Authorize(ctx context.Context, req domain.AuthorizeRequest) (domain.AuthorizeResponse, error)
	Accounting(ctx context.Context, req domain.AccountingRequest) (domain.AccountingResponse, error)
}

func registerRadiusRoutes(mux *http.ServeMux, service radiusService, apiToken string) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/radius/authorize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorizeRadiusRequest(w, r, apiToken) {
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
		if !authorizeRadiusRequest(w, r, apiToken) {
			return
		}

		var req domain.AccountingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		resp, err := service.Accounting(r.Context(), req)
		if err != nil {
			var validationErr domain.ValidationError
			if errors.As(err, &validationErr) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if errors.Is(err, domain.ErrAccountingInProgress) {
				http.Error(w, "radius accounting packet is being processed", http.StatusServiceUnavailable)
				return
			}
			http.Error(w, "radius accounting persistence failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func authorizeRadiusRequest(w http.ResponseWriter, r *http.Request, apiToken string) bool {
	if apiToken == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "RADIUS API authentication is not configured", http.StatusServiceUnavailable)
		return false
	}

	expected := sha256.Sum256([]byte("Bearer " + apiToken))
	provided := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	if subtle.ConstantTimeCompare(expected[:], provided[:]) == 1 {
		return true
	}

	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return false
}
