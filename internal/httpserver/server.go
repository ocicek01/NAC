package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

func New(port string, logger *slog.Logger, dhcpEventIngestor dhcpEventIngestor, switchService switchService, portEndpointService portEndpointService, macLookupService macLookupService, macObservationService macObservationService, topologyService topologyService, discoveryJobService discoveryJobService, deviceService deviceService, guestService guestService, portalService portalService, sessionService sessionService, policyService policyService, enforcementService enforcementService, radiusService radiusService) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	registerDHCPEventRoutes(mux, dhcpEventIngestor)
	registerSwitchRoutes(mux, switchService, portEndpointService)
	registerMACLookupRoutes(mux, macLookupService)
	registerMACObservationRoutes(mux, macObservationService)
	registerTopologyRoutes(mux, topologyService)
	registerDiscoveryJobRoutes(mux, discoveryJobService)
	registerDeviceRoutes(mux, deviceService)
	registerGuestRoutes(mux, guestService)
	registerPortalRoutes(mux, portalService)
	registerSessionRoutes(mux, sessionService)
	registerPolicyRoutes(mux, policyService)
	registerEnforcementRoutes(mux, enforcementService)
	registerRadiusRoutes(mux, radiusService)

	return &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%s", port),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("http server starting", "addr", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	return s.httpServer.Shutdown(ctx)
}
