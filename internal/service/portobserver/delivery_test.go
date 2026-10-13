package portobserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	porteventdomain "nac/internal/domain/portevent"
)

func TestHTTPDelivererClassifiesPermanentAndRetryableResponses(t *testing.T) {
	status := http.StatusBadRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte("validation failed"))
	}))
	defer server.Close()
	deliverer := NewHTTPDeliverer(server.URL)

	err := deliverer.Deliver(context.Background(), porteventdomain.Event{})
	var deliveryErr *DeliveryError
	if !errors.As(err, &deliveryErr) || !deliveryErr.Permanent || deliveryErr.Message != "validation failed" {
		t.Fatalf("400 must be permanent: %v", err)
	}
	status = http.StatusConflict
	err = deliverer.Deliver(context.Background(), porteventdomain.Event{})
	if !errors.As(err, &deliveryErr) || deliveryErr.Permanent {
		t.Fatalf("409 processing conflict must be retried: %v", err)
	}
}
