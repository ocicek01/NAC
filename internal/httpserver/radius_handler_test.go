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
	accountingErr error
}

func (s radiusHandlerService) Authorize(context.Context, domain.AuthorizeRequest) (domain.AuthorizeResponse, error) {
	return domain.AuthorizeResponse{}, nil
}

func (s radiusHandlerService) Accounting(context.Context, domain.AccountingRequest) (domain.AccountingResponse, error) {
	if s.accountingErr != nil {
		return domain.AccountingResponse{}, s.accountingErr
	}
	return domain.AccountingResponse{Status: "ok"}, nil
}

func TestRadiusAccountingHandlerMapsPersistenceErrorToServerError(t *testing.T) {
	mux := http.NewServeMux()
	registerRadiusRoutes(mux, radiusHandlerService{accountingErr: errors.New("database unavailable")})
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
	registerRadiusRoutes(mux, radiusHandlerService{accountingErr: domain.ValidationError{Field: "acct_session_id", Message: "is required"}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/radius/accounting", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestRadiusAccountingHandlerMapsBusyClaimToServiceUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	registerRadiusRoutes(mux, radiusHandlerService{accountingErr: domain.ErrAccountingInProgress})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/radius/accounting", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
