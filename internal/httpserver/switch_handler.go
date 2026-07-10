package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"nac/internal/domain/switchasset"
	switchport "nac/internal/domain/switchport"
	portendpointservice "nac/internal/service/portendpoint"
)

type switchService interface {
	Create(ctx context.Context, asset switchasset.Switch) (switchasset.Switch, error)
	CreateBulk(ctx context.Context, defaults switchasset.Switch, assets []switchasset.Switch) ([]switchasset.Switch, error)
	UpdateIdentity(ctx context.Context, id, systemName, baseMAC string, aliases []string) (switchasset.Switch, error)
	UpdateRoutingSwitch(ctx context.Context, id, routingSwitchID string) (switchasset.Switch, error)
	UpdateRadiusSecret(ctx context.Context, id, radiusSecret string) (switchasset.Switch, error)
	UpdateSSHConfig(ctx context.Context, id, username, password string, port int) (switchasset.Switch, error)
	RefreshIdentities(ctx context.Context) ([]switchasset.Switch, error)
	List(ctx context.Context) ([]switchasset.Switch, error)
	FindByID(ctx context.Context, id string) (*switchasset.Switch, error)
	FindByName(ctx context.Context, name string) (*switchasset.Switch, error)
	FindByManagementIP(ctx context.Context, managementIP string) (*switchasset.Switch, error)
	LiveDetail(ctx context.Context, id string) (*switchasset.LiveDetail, error)
	ListPortsBySwitch(ctx context.Context, switchID string) ([]switchport.Port, error)
	PortSummaryBySwitch(ctx context.Context, switchID string) (any, error)
	LivePortLookup(ctx context.Context, switchID string, ifIndex int) (*switchasset.LivePortLookup, error)
	RefreshPortSnapshot(ctx context.Context, switchID string, ifIndex int) (*switchasset.LivePortLookup, error)
}

type portEndpointService interface {
	ListBySwitch(ctx context.Context, switchID string) ([]portendpointservice.PortView, error)
}

type bulkSwitchCreateRequest struct {
	Defaults switchasset.Switch   `json:"defaults"`
	Switches []switchasset.Switch `json:"switches"`
}

type switchIdentityUpdateRequest struct {
	ID         string   `json:"id"`
	SystemName string   `json:"system_name"`
	BaseMAC    string   `json:"base_mac"`
	Aliases    []string `json:"aliases"`
}

type switchRadiusSecretUpdateRequest struct {
	ID           string `json:"id"`
	RadiusSecret string `json:"radius_secret"`
}

type switchRoutingUpdateRequest struct {
	ID              string `json:"id"`
	RoutingSwitchID string `json:"routing_switch_id"`
}

type switchSSHConfigUpdateRequest struct {
	ID          string `json:"id"`
	SSHUsername string `json:"ssh_username"`
	SSHPassword string `json:"ssh_password"`
	SSHPort     int    `json:"ssh_port"`
}

func registerSwitchRoutes(mux *http.ServeMux, service switchService, portEndpointService portEndpointService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/switches/bulk", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req bulkSwitchCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		created, err := service.CreateBulk(r.Context(), req.Defaults, req.Switches)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(redactSwitchSecrets(created))
	})

	mux.HandleFunc("/api/v1/switches/identity", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req switchIdentityUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		updated, err := service.UpdateIdentity(r.Context(), req.ID, req.SystemName, req.BaseMAC, req.Aliases)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(redactSwitchSecret(updated))
	})

	mux.HandleFunc("/api/v1/switches/identity/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		updated, err := service.RefreshIdentities(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(redactSwitchSecrets(updated))
	})

	mux.HandleFunc("/api/v1/switches/radius-secret", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req switchRadiusSecretUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		updated, err := service.UpdateRadiusSecret(r.Context(), req.ID, req.RadiusSecret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(redactSwitchSecret(updated))
	})

	mux.HandleFunc("/api/v1/switches/routing", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req switchRoutingUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		updated, err := service.UpdateRoutingSwitch(r.Context(), req.ID, req.RoutingSwitchID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(redactSwitchSecret(updated))
	})

	mux.HandleFunc("/api/v1/switches/ssh-config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req switchSSHConfigUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		updated, err := service.UpdateSSHConfig(r.Context(), req.ID, req.SSHUsername, req.SSHPassword, req.SSHPort)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(redactSwitchSecret(updated))
	})

	mux.HandleFunc("/api/v1/switches/resolve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		managementIP := strings.TrimSpace(r.URL.Query().Get("management_ip"))

		if name == "" && managementIP == "" {
			http.Error(w, "name or management_ip is required", http.StatusBadRequest)
			return
		}

		var asset *switchasset.Switch
		var err error
		if managementIP != "" {
			asset, err = service.FindByManagementIP(r.Context(), managementIP)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if asset == nil && name != "" {
			asset, err = service.FindByName(r.Context(), name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if asset == nil {
			http.Error(w, "switch not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(redactSwitchSecret(*asset))
	})

	mux.HandleFunc("/api/v1/switches", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			assets, err := service.List(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(redactSwitchSecrets(assets))
		case http.MethodPost:
			var asset switchasset.Switch
			if err := json.NewDecoder(r.Body).Decode(&asset); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			created, err := service.Create(r.Context(), asset)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(redactSwitchSecret(created))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/switches/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/switches/")
		switch {
		case strings.Contains(path, "/ports/") && strings.HasSuffix(path, "/refresh") && r.Method == http.MethodPost:
			trimmed := strings.Trim(strings.TrimSuffix(path, "/refresh"), "/")
			parts := strings.Split(trimmed, "/ports/")
			if len(parts) != 2 {
				http.Error(w, "invalid switch port path", http.StatusBadRequest)
				return
			}
			id := strings.TrimSpace(parts[0])
			ifIndex, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil || ifIndex <= 0 {
				http.Error(w, "valid if_index is required", http.StatusBadRequest)
				return
			}

			go func() {
				_, _ = service.RefreshPortSnapshot(context.Background(), id, ifIndex)
			}()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "queued", "switch_id": id, "if_index": ifIndex})
		case strings.Contains(path, "/ports/") && strings.HasSuffix(path, "/live") && r.Method == http.MethodGet:
			trimmed := strings.Trim(strings.TrimSuffix(path, "/live"), "/")
			parts := strings.Split(trimmed, "/ports/")
			if len(parts) != 2 {
				http.Error(w, "invalid switch port path", http.StatusBadRequest)
				return
			}
			id := strings.TrimSpace(parts[0])
			ifIndex, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil || ifIndex <= 0 {
				http.Error(w, "valid if_index is required", http.StatusBadRequest)
				return
			}

			item, err := service.LivePortLookup(r.Context(), id, ifIndex)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(item)
		case strings.HasSuffix(path, "/live") && r.Method == http.MethodGet:
			id := strings.Trim(strings.TrimSuffix(path, "/live"), "/")
			if id == "" {
				http.Error(w, "switch id is required", http.StatusBadRequest)
				return
			}

			detail, err := service.LiveDetail(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(detail)
		case strings.HasSuffix(path, "/ports") && r.Method == http.MethodGet:
			id := strings.Trim(strings.TrimSuffix(path, "/ports"), "/")
			if id == "" {
				http.Error(w, "switch id is required", http.StatusBadRequest)
				return
			}

			ports, err := service.ListPortsBySwitch(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ports)
		case strings.HasSuffix(path, "/ports/endpoints") && r.Method == http.MethodGet:
			if portEndpointService == nil {
				http.Error(w, "port endpoint service is not configured", http.StatusNotImplemented)
				return
			}

			id := strings.Trim(strings.TrimSuffix(path, "/ports/endpoints"), "/")
			if id == "" {
				http.Error(w, "switch id is required", http.StatusBadRequest)
				return
			}

			items, err := portEndpointService.ListBySwitch(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(items)
		case strings.HasSuffix(path, "/ports/summary") && r.Method == http.MethodGet:
			id := strings.Trim(strings.TrimSuffix(path, "/ports/summary"), "/")
			if id == "" {
				http.Error(w, "switch id is required", http.StatusBadRequest)
				return
			}

			summary, err := service.PortSummaryBySwitch(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(summary)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
}

func redactSwitchSecret(asset switchasset.Switch) switchasset.Switch {
	asset.RadiusSecret = ""
	asset.SSHPassword = ""
	return asset
}

func redactSwitchSecrets(assets []switchasset.Switch) []switchasset.Switch {
	if len(assets) == 0 {
		return assets
	}

	out := make([]switchasset.Switch, len(assets))
	for i, asset := range assets {
		out[i] = redactSwitchSecret(asset)
	}
	return out
}
