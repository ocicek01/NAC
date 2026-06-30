package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	domain "nac/internal/domain/enforcement"
)

type enforcementService interface {
	ListRecent(ctx context.Context, limit int) ([]domain.Decision, error)
	ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]domain.Decision, error)
	Approve(ctx context.Context, id string) error
	Reject(ctx context.Context, id string) error
	Retry(ctx context.Context, id string) error
	ExecuteDecision(ctx context.Context, id string, vlanID int, dryRun bool) (domain.VLANExecutionResult, error)
	PreviewSSHPortVLAN(ctx context.Context, switchID, interfaceName string, vlanID int) (domain.VLANPlan, error)
	ExecuteSSHPortVLAN(ctx context.Context, switchID, interfaceName string, vlanID int) (domain.VLANExecutionResult, error)
	PreviewSNMPPortVLAN(ctx context.Context, switchID string, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANPlan, error)
	ExecuteSNMPPortVLAN(ctx context.Context, switchID string, bridgePort, ifIndex int, interfaceName string, vlanID int, skipPortBounce bool) (domain.VLANExecutionResult, error)
}

type sshPortVLANRequest struct {
	SwitchID      string `json:"switch_id"`
	BridgePort    int    `json:"bridge_port"`
	IfIndex       int    `json:"if_index"`
	InterfaceName string `json:"interface_name"`
	VLANID        int    `json:"vlan_id"`
	DryRun        bool   `json:"dry_run"`
	SkipPortBounce bool  `json:"skip_port_bounce"`
}

type executeDecisionRequest struct {
	VLANID int  `json:"vlan_id"`
	DryRun bool `json:"dry_run"`
}

func registerEnforcementRoutes(mux *http.ServeMux, service enforcementService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/enforcement-decisions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		macAddress := strings.TrimSpace(r.URL.Query().Get("mac"))
		limit := 20
		if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
			if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		var (
			items []domain.Decision
			err   error
		)
		if macAddress != "" {
			items, err = service.ListRecentByMAC(r.Context(), macAddress, limit)
		} else {
			items, err = service.ListRecent(r.Context(), limit)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	})

	mux.HandleFunc("/api/v1/enforcement/snmp-port-vlan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req sshPortVLANRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if req.DryRun {
			plan, err := service.PreviewSNMPPortVLAN(r.Context(), req.SwitchID, req.BridgePort, req.IfIndex, req.InterfaceName, req.VLANID, req.SkipPortBounce)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(plan)
			return
		}

		result, err := service.ExecuteSNMPPortVLAN(r.Context(), req.SwitchID, req.BridgePort, req.IfIndex, req.InterfaceName, req.VLANID, req.SkipPortBounce)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("/api/v1/enforcement/ssh-port-vlan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req sshPortVLANRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if req.DryRun {
			plan, err := service.PreviewSSHPortVLAN(r.Context(), req.SwitchID, req.InterfaceName, req.VLANID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(plan)
			return
		}

		result, err := service.ExecuteSSHPortVLAN(r.Context(), req.SwitchID, req.InterfaceName, req.VLANID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("/api/v1/enforcement-decisions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/enforcement-decisions/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}

		id := parts[0]
		action := parts[1]

		if action == "execute" {
			var req executeDecisionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			result, err := service.ExecuteDecision(r.Context(), id, req.VLANID, req.DryRun)
			w.Header().Set("Content-Type", "application/json")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(result)
				return
			}
			_ = json.NewEncoder(w).Encode(result)
			return
		}

		var err error
		switch action {
		case "approve":
			err = service.Approve(r.Context(), id)
		case "reject":
			err = service.Reject(r.Context(), id)
		case "retry":
			err = service.Retry(r.Context(), id)
		default:
			http.NotFound(w, r)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
