package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "nac/internal/domain/sophosauth"
)

type sophosAuthHandlerStub struct{ limit, offset int }

func (s *sophosAuthHandlerStub) ListEvents(_ context.Context, limit, offset int) ([]domain.Event, error) {
	s.limit, s.offset = limit, offset
	return []domain.Event{}, nil
}
func (s *sophosAuthHandlerStub) ListSessions(_ context.Context, limit, offset int) ([]domain.Session, error) {
	s.limit, s.offset = limit, offset
	return []domain.Session{}, nil
}
func (s *sophosAuthHandlerStub) Health(context.Context) (domain.Health, error) {
	return domain.Health{Enabled: true}, nil
}

func TestSophosAuthRoutesPaginationAndHealth(t *testing.T) {
	service := &sophosAuthHandlerStub{}
	mux := http.NewServeMux()
	registerSophosAuthRoutes(mux, service)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/sophos-auth/sessions?limit=20&offset=40", nil))
	if response.Code != http.StatusOK || service.limit != 20 || service.offset != 40 {
		t.Fatalf("unexpected pagination response: status=%d limit=%d offset=%d", response.Code, service.limit, service.offset)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/sophos-auth/events?limit=999", nil))
	if response.Code != http.StatusOK || service.limit != 200 {
		t.Fatalf("expected capped limit, got status=%d limit=%d", response.Code, service.limit)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/sophos-auth/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", response.Code, response.Body.String())
	}
}
