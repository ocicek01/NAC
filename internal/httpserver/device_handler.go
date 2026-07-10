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
	enforcementdomain "nac/internal/domain/enforcement"
	policydomain "nac/internal/domain/policy"
	policyservice "nac/internal/service/policy"
)

type deviceService interface {
	List(ctx context.Context, limit, offset int) ([]device.Device, error)
	FindByID(ctx context.Context, id string) (*device.Device, error)
	ListByMAC(ctx context.Context, macAddress string) ([]device.Device, error)
	ListBySwitch(ctx context.Context, switchID string) ([]device.Device, error)
	ListBySwitchAndIfIndex(ctx context.Context, switchID string, ifIndex int) ([]device.Device, error)
	UpdateStatus(ctx context.Context, macAddress, status, approvedBy string, expiresAt time.Time, targetVLAN int) (device.Device, error)
	AddIdentitySnapshot(ctx context.Context, snapshot device.IdentitySnapshot) (device.IdentitySnapshot, error)
	RecordSophosIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error
	EvaluatePolicyByID(ctx context.Context, deviceID string) (policyservice.EvaluationResult, error)
	ListPolicyDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]policydomain.Decision, error)
	ListDeviceRequests(ctx context.Context, deviceID string, limit, offset int) ([]enforcementdomain.Request, error)
}

type deviceStatusUpdateRequest struct {
	ApprovedBy, ExpiresAt string
	TargetVLAN            int
}
type identitySnapshotRequest struct {
	IdentityType, IdentitySource, ExternalID, Username, FullName, VerifiedAt, ExpiresAt string
	Attributes                                                                          map[string]any
}
type sophosIdentityRequest struct{ Username, IPAddress, SeenAt string }

func registerDeviceRoutes(mux *http.ServeMux, service deviceService) {
	if service == nil {
		return
	}
	mux.HandleFunc("/api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		macAddress, switchID := strings.TrimSpace(r.URL.Query().Get("mac")), strings.TrimSpace(r.URL.Query().Get("switch_id"))
		ifIndex, limit, offset := 0, 50, 0
		if raw := strings.TrimSpace(r.URL.Query().Get("if_index")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				http.Error(w, "if_index must be a positive integer", http.StatusBadRequest)
				return
			}
			ifIndex = parsed
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed <= 0 {
				http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
				return
			}
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				http.Error(w, "offset must be zero or a positive integer", http.StatusBadRequest)
				return
			}
			offset = parsed
		}
		var items []device.Device
		var err error
		switch {
		case macAddress != "":
			items, err = service.ListByMAC(r.Context(), macAddress)
		case switchID != "" && ifIndex > 0:
			items, err = service.ListBySwitchAndIfIndex(r.Context(), switchID, ifIndex)
		case switchID != "":
			items, err = service.ListBySwitch(r.Context(), switchID)
		default:
			items, err = service.List(r.Context(), limit, offset)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
	mux.HandleFunc("/api/v1/devices/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/devices/"), "/")
		if path == "" {
			http.NotFound(w, r)
			return
		}
		parts := strings.Split(path, "/")
		identifier := strings.TrimSpace(parts[0])
		if identifier == "" {
			http.Error(w, "device identifier is required", http.StatusBadRequest)
			return
		}
		if len(parts) == 1 {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			items, err := service.ListByMAC(r.Context(), identifier)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(items) == 0 {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, http.StatusOK, items[0])
			return
		}
		if len(parts) == 2 {
			switch {
			case r.Method == http.MethodPost && parts[1] == "approve":
				handleDeviceStatusUpdate(w, r, service, identifier, "allowed")
				return
			case r.Method == http.MethodPost && parts[1] == "block":
				handleDeviceStatusUpdate(w, r, service, identifier, "blocked")
				return
			case r.Method == http.MethodPost && parts[1] == "retire":
				handleDeviceStatusUpdate(w, r, service, identifier, "retired")
				return
			case r.Method == http.MethodPost && parts[1] == "identity-snapshots":
				handleIdentitySnapshotCreate(w, r, service, identifier)
				return
			case r.Method == http.MethodPost && parts[1] == "sophos-identity":
				handleSophosIdentityUpdate(w, r, service, identifier)
				return
			case r.Method == http.MethodPost && parts[1] == "evaluate-policy":
				result, err := service.EvaluatePolicyByID(r.Context(), identifier)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, http.StatusOK, result)
				return
			case r.Method == http.MethodGet && parts[1] == "policy-decisions":
				limit, offset, err := parseLimitOffset(r, 50, 200)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				items, err := service.ListPolicyDecisionsByDevice(r.Context(), identifier, limit, offset)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				writeJSON(w, http.StatusOK, toPolicyDecisionResponses(items))
				return
			}
			if r.Method == http.MethodGet && parts[1] == "enforcement-history" {
				limit, offset, err := parseLimitOffset(r, 50, 200)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				items, err := service.ListDeviceRequests(r.Context(), identifier, limit, offset)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				writeJSON(w, http.StatusOK, toEnforcementRequestResponses(items))
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
	writeJSON(w, http.StatusOK, updated)
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
	var verifiedAt, expiresAt time.Time
	if strings.TrimSpace(req.VerifiedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.VerifiedAt))
		if err != nil {
			http.Error(w, "verified_at must be RFC3339", http.StatusBadRequest)
			return
		}
		verifiedAt = parsed.UTC()
	}
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
		if err != nil {
			http.Error(w, "expires_at must be RFC3339", http.StatusBadRequest)
			return
		}
		expiresAt = parsed.UTC()
	}
	snapshot, err := service.AddIdentitySnapshot(r.Context(), device.IdentitySnapshot{ID: newUUID(), DeviceID: devices[0].ID, IdentityType: req.IdentityType, IdentitySource: req.IdentitySource, ExternalID: req.ExternalID, Username: req.Username, FullName: req.FullName, Attributes: req.Attributes, VerifiedAt: verifiedAt, ExpiresAt: expiresAt})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, snapshot)
}
func handleSophosIdentityUpdate(w http.ResponseWriter, r *http.Request, service deviceService, macAddress string) {
	var req sophosIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	seenAt := time.Now().UTC()
	if strings.TrimSpace(req.SeenAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.SeenAt))
		if err != nil {
			http.Error(w, "seen_at must be RFC3339", http.StatusBadRequest)
			return
		}
		seenAt = parsed.UTC()
	}
	if err := service.RecordSophosIdentity(r.Context(), macAddress, req.Username, req.IPAddress, seenAt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func newUUID() string { return uuid.NewString() }
