package httpserver

import (
	"context"
	"encoding/json"
	"net/http"

	portalservice "nac/internal/service/portal"
)

type portalService interface {
	Register(ctx context.Context, input portalservice.RegistrationInput) (portalservice.RegistrationResult, error)
}

type portalRegisterRequest struct {
	MACAddress string `json:"mac_address"`
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
	ApprovedBy string `json:"approved_by"`
}

func registerPortalRoutes(mux *http.ServeMux, service portalService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/portal/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req portalRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		result, err := service.Register(r.Context(), portalservice.RegistrationInput{
			MACAddress: req.MACAddress,
			Identifier: req.Identifier,
			Password:   req.Password,
			ApprovedBy: req.ApprovedBy,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
}
