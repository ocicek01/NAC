package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	guestdomain "nac/internal/domain/guestidentity"
	guestservice "nac/internal/service/guestidentity"
)

type guestService interface {
	List(ctx context.Context) ([]guestdomain.Identity, error)
	Create(ctx context.Context, input guestservice.CreateInput) (guestdomain.Identity, error)
}

type guestCreateRequest struct {
	ExternalID string `json:"external_id"`
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Status     string `json:"status"`
	TargetVLAN int    `json:"target_vlan"`
	ExpiresAt  string `json:"expires_at"`
}

func registerGuestRoutes(mux *http.ServeMux, service guestService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/guests", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := service.List(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(items)
		case http.MethodPost:
			var req guestCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}
			var expiresAt time.Time
			if strings.TrimSpace(req.ExpiresAt) != "" {
				parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
				if err != nil {
					http.Error(w, "expires_at must be RFC3339", http.StatusBadRequest)
					return
				}
				expiresAt = parsed.UTC()
			}
			item, err := service.Create(r.Context(), guestservice.CreateInput{
				ExternalID: req.ExternalID,
				Username:   req.Username,
				FullName:   req.FullName,
				Email:      req.Email,
				Phone:      req.Phone,
				Status:     req.Status,
				TargetVLAN: req.TargetVLAN,
				ExpiresAt:  expiresAt,
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(item)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
