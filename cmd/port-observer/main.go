package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"nac/internal/config"
	"nac/internal/database"
	switchrepo "nac/internal/repository/switchasset"
	portrepo "nac/internal/repository/switchport"
	"nac/internal/service/portobserver"
	"nac/internal/snmp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.NewPostgresPool(startup, cfg.Postgres)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	workers, err := envInt("PORT_OBSERVER_WORKERS", 4)
	if err != nil {
		return err
	}
	interval, err := envInt("PORT_OBSERVER_INTERVAL_SECONDS", 15)
	if err != nil {
		return err
	}
	timeout, err := envInt("PORT_OBSERVER_TIMEOUT_SECONDS", 30)
	if err != nil {
		return err
	}
	portRepository := portrepo.NewPostgresRepository(pool)
	service, err := portobserver.New(switchrepo.NewPostgresRepository(pool), portRepository, snmp.NewClient(), portobserver.Config{Workers: workers, Interval: time.Duration(interval) * time.Second, Timeout: time.Duration(timeout) * time.Second})
	if err != nil {
		return err
	}
	apiURL := os.Getenv("PORT_OBSERVER_API_URL")
	if apiURL == "" {
		apiURL = "http://127.0.0.1:8080"
	}
	service.ConfigureDelivery(portRepository, portobserver.NewHTTPDeliverer(apiURL))
	address := os.Getenv("PORT_OBSERVER_HEALTH_ADDR")
	if address == "" {
		address = "127.0.0.1:9091"
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		health := service.Health()
		w.Header().Set("Content-Type", "application/json")
		if health.Heartbeat.IsZero() || time.Since(health.Heartbeat) > 5*time.Second || health.LastError != "" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(health)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, WriteTimeout: 5 * time.Second}
	serverErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			stop()
		}
	}()
	log.Printf("port observer started: workers=%d interval=%ds timeout=%ds health=%s", workers, interval, timeout, address)
	service.Run(ctx)
	shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	_ = server.Shutdown(shutdown)
	select {
	case err := <-serverErr:
		return err
	default:
		return nil
	}
}
func envInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(value)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return v, nil
}
