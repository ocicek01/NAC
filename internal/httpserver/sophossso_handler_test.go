package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "nac/internal/domain/sophossso"
)

type sophosHandlerService struct{}

func (sophosHandlerService) List(context.Context, int, int) ([]domain.Item, error) {
	return []domain.Item{}, nil
}
func (sophosHandlerService) Health(context.Context) (domain.Health, error) {
	return domain.Health{Enabled: false}, nil
}

func TestSophosSSOReadOnlyRoutesRespond(t *testing.T) {
	mux := http.NewServeMux()
	registerSophosSSORoutes(mux, sophosHandlerService{})
	for _, path := range []string{"/api/v1/sophos-sso/outbox?limit=10", "/api/v1/sophos-sso/health"} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d", path, recorder.Code)
		}
	}
}
