package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "nac/internal/domain/radiusauth"
)

type radiusHandlerService struct {
	accountingErr   error
	authorizeCalls  int
	accountingCalls int
}

func (s *radiusHandlerService) Authorize(context.Context, domain.AuthorizeRequest) (domain.AuthorizeResponse, error) {
	s.authorizeCalls++
	return domain.AuthorizeResponse{Decision: "accept"}, nil
}

func (s *radiusHandlerService) Accounting(context.Context, domain.AccountingRequest) (domain.AccountingResponse, error) {
	s.accountingCalls++
	if s.accountingErr != nil {
		return domain.AccountingResponse{}, s.accountingErr
	}
	return domain.AccountingResponse{Status: "ok"}, nil
}

func TestRadiusAccountingHandlerMapsPersistenceErrorToServerError(t *testing.T) {
	mux := http.NewServeMux()
	registerRadiusRoutes(mux, &radiusHandlerService{accountingErr: errors.New("database unavailable")}, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/radius/accounting", strings.NewReader(`{"acct_status_type":"Start"}`))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "database unavailable") {
		t.Fatalf("response leaks internal error: %q", recorder.Body.String())
	}
}

func TestRadiusAccountingHandlerMapsValidationErrorToBadRequest(t *testing.T) {
	mux := http.NewServeMux()
	registerRadiusRoutes(mux, &radiusHandlerService{accountingErr: domain.ValidationError{Field: "acct_session_id", Message: "is required"}}, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/radius/accounting", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestRadiusAccountingHandlerMapsBusyClaimToServiceUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	registerRadiusRoutes(mux, &radiusHandlerService{accountingErr: domain.ErrAccountingInProgress}, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/radius/accounting", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestRadiusAPITokenAuthentication(t *testing.T) {
	endpoints := []struct {
		name  string
		path  string
		calls func(*radiusHandlerService) int
	}{
		{name: "authorize", path: "/api/v1/radius/authorize", calls: func(s *radiusHandlerService) int { return s.authorizeCalls }},
		{name: "accounting", path: "/api/v1/radius/accounting", calls: func(s *radiusHandlerService) int { return s.accountingCalls }},
	}
	credentials := []struct {
		name       string
		header     string
		wantStatus int
		wantCalls  int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong", header: "Bearer wrong-token", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", header: "Basic shared-token", wantStatus: http.StatusUnauthorized},
		{name: "correct", header: "Bearer shared-token", wantStatus: http.StatusOK, wantCalls: 1},
	}

	for _, endpoint := range endpoints {
		for _, credential := range credentials {
			t.Run(endpoint.name+"/"+credential.name, func(t *testing.T) {
				service := &radiusHandlerService{}
				mux := http.NewServeMux()
				registerRadiusRoutes(mux, service, "shared-token")
				req := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(`{}`))
				if credential.header != "" {
					req.Header.Set("Authorization", credential.header)
				}
				recorder := httptest.NewRecorder()

				mux.ServeHTTP(recorder, req)

				if recorder.Code != credential.wantStatus {
					t.Fatalf("status = %d, want %d", recorder.Code, credential.wantStatus)
				}
				if got := endpoint.calls(service); got != credential.wantCalls {
					t.Fatalf("service calls = %d, want %d", got, credential.wantCalls)
				}
			})
		}
	}
}

func TestRadiusAPITokenUnsetPreservesCompatibility(t *testing.T) {
	service := &radiusHandlerService{}
	mux := http.NewServeMux()
	registerRadiusRoutes(mux, service, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/radius/authorize", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK || service.authorizeCalls != 1 {
		t.Fatalf("status=%d authorize calls=%d", recorder.Code, service.authorizeCalls)
	}
}
