package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	device "nac/internal/domain/device"
)

type deviceService interface {
	List(ctx context.Context) ([]device.Device, error)
	ListByMAC(ctx context.Context, macAddress string) ([]device.Device, error)
	ListBySwitch(ctx context.Context, switchID string) ([]device.Device, error)
	ListBySwitchAndIfIndex(ctx context.Context, switchID string, ifIndex int) ([]device.Device, error)
	UpdateStatus(ctx context.Context, macAddress, status, approvedBy string, expiresAt time.Time, targetVLAN int) (device.Device, error)
	AddIdentitySnapshot(ctx context.Context, snapshot device.IdentitySnapshot) (device.IdentitySnapshot, error)
}

type deviceStatusUpdateRequest struct {
	ApprovedBy string `json:"approved_by"`
	ExpiresAt  string `json:"expires_at"`
	TargetVLAN int    `json:"target_vlan"`
}

type identitySnapshotRequest struct {
	IdentityType   string         `json:"identity_type"`
	IdentitySource string         `json:"identity_source"`
	ExternalID     string         `json:"external_id"`
	Username       string         `json:"username"`
	FullName       string         `json:"full_name"`
	Attributes     map[string]any `json:"attributes"`
	VerifiedAt     string         `json:"verified_at"`
	ExpiresAt      string         `json:"expires_at"`
}

func registerDeviceRoutes(mux *http.ServeMux, service deviceService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		macAddress := strings.TrimSpace(r.URL.Query().Get("mac"))
		switchID := strings.TrimSpace(r.URL.Query().Get("switch_id"))
		ifIndex := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("if_index")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				http.Error(w, "if_index must be a positive integer", http.StatusBadRequest)
				return
			}
			ifIndex = parsed
		}
		var (
			devices []device.Device
			err     error
		)
		switch {
		case macAddress != "":
			devices, err = service.ListByMAC(r.Context(), macAddress)
		case switchID != "" && ifIndex > 0:
			devices, err = service.ListBySwitchAndIfIndex(r.Context(), switchID, ifIndex)
		case switchID != "":
			devices, err = service.ListBySwitch(r.Context(), switchID)
		default:
			devices, err = service.List(r.Context())
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(devices)
	})

	mux.HandleFunc("/api/v1/devices/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
		path = strings.Trim(path, "/")
		if path == "" {
			http.NotFound(w, r)
			return
		}

		parts := strings.Split(path, "/")
		macAddress := strings.TrimSpace(parts[0])
		if macAddress == "" {
			http.Error(w, "mac is required", http.StatusBadRequest)
			return
		}

		if len(parts) == 1 {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			devices, err := service.ListByMAC(r.Context(), macAddress)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(devices) == 0 {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(devices[0])
			return
		}

		if len(parts) == 2 && r.Method == http.MethodPost {
			switch parts[1] {
			case "approve":
				handleDeviceStatusUpdate(w, r, service, macAddress, "allowed")
				return
			case "block":
				handleDeviceStatusUpdate(w, r, service, macAddress, "blocked")
				return
			case "retire":
				handleDeviceStatusUpdate(w, r, service, macAddress, "retired")
				return
			case "identity-snapshots":
				handleIdentitySnapshotCreate(w, r, service, macAddress)
				return
			}
		}

		http.NotFound(w, r)
	})
}

func handleDeviceStatusUpdate(w http.ResponseWriter, r *http.Request, service deviceService, macAddress, status string) {
	var req deviceStatusUpdateRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
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

	updated, err := service.UpdateStatus(r.Context(), macAddress, status, req.ApprovedBy, expiresAt, req.TargetVLAN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

func handleIdentitySnapshotCreate(w http.ResponseWriter, r *http.Request, service deviceService, macAddress string) {
	var req identitySnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	devices, err := service.ListByMAC(r.Context(), macAddress)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(devices) == 0 {
		http.NotFound(w, r)
		return
	}

	var verifiedAt time.Time
	if strings.TrimSpace(req.VerifiedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.VerifiedAt))
		if err != nil {
			http.Error(w, "verified_at must be RFC3339", http.StatusBadRequest)
			return
		}
		verifiedAt = parsed.UTC()
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

	snapshot, err := service.AddIdentitySnapshot(r.Context(), device.IdentitySnapshot{
		ID:             newUUID(),
		DeviceID:       devices[0].ID,
		IdentityType:   req.IdentityType,
		IdentitySource: req.IdentitySource,
		ExternalID:     req.ExternalID,
		Username:       req.Username,
		FullName:       req.FullName,
		Attributes:     req.Attributes,
		VerifiedAt:     verifiedAt,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(snapshot)
}

func newUUID() string {
	return uuid.NewString()
}
