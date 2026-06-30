package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"nac/internal/domain/discoveryjob"
)

type discoveryJobService interface {
	Create(ctx context.Context, job discoveryjob.Job) (discoveryjob.Job, error)
	FindByID(ctx context.Context, id string) (*discoveryjob.Job, error)
	DispatchByID(ctx context.Context, id, workerID string) (*discoveryjob.Job, error)
	StartNext(ctx context.Context, workerID string) (*discoveryjob.Job, error)
	StartByID(ctx context.Context, id, workerID string) (*discoveryjob.Job, error)
}

type discoveryJobCreateRequest struct {
	SwitchID        string `json:"switch_id"`
	Scope           string `json:"scope"`
	RequestedSource string `json:"requested_source"`
	RequestedBy     string `json:"requested_by"`
}

type discoveryJobRunNextRequest struct {
	WorkerID string `json:"worker_id"`
}

func registerDiscoveryJobRoutes(mux *http.ServeMux, service discoveryJobService) {
	if service == nil {
		return
	}

	mux.HandleFunc("/api/v1/discovery/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req discoveryJobCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		created, err := service.Create(r.Context(), discoveryjob.Job{
			SwitchID:        req.SwitchID,
			Scope:           req.Scope,
			RequestedSource: req.RequestedSource,
			RequestedBy:     req.RequestedBy,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	})

	mux.HandleFunc("/api/v1/discovery/jobs/run-next", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req discoveryJobRunNextRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		job, err := service.StartNext(r.Context(), req.WorkerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if job == nil {
			http.Error(w, "no queued jobs", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/api/v1/discovery/jobs/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/discovery/jobs/")
		id = strings.Trim(id, "/")
		if id == "" {
			http.Error(w, "job id is required", http.StatusBadRequest)
			return
		}

		if strings.HasSuffix(id, "/run") {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			jobID := strings.Trim(strings.TrimSuffix(id, "/run"), "/")
			if jobID == "" {
				http.Error(w, "job id is required", http.StatusBadRequest)
				return
			}

			var req discoveryJobRunNextRequest
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&req)
			}

			job, err := service.StartByID(r.Context(), jobID, req.WorkerID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if job == nil {
				http.Error(w, "job not queued", http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(job)
			return
		}

		if strings.HasSuffix(id, "/dispatch") {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			jobID := strings.Trim(strings.TrimSuffix(id, "/dispatch"), "/")
			if jobID == "" {
				http.Error(w, "job id is required", http.StatusBadRequest)
				return
			}

			var req discoveryJobRunNextRequest
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&req)
			}

			job, err := service.DispatchByID(r.Context(), jobID, req.WorkerID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if job == nil {
				http.Error(w, "job not queued", http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(job)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		job, err := service.FindByID(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if job == nil {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(job)
	})
}
