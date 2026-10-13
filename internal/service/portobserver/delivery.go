package portobserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	porteventdomain "nac/internal/domain/portevent"
)

type Outbox interface {
	PendingPortEvents(context.Context, int) ([]porteventdomain.Event, error)
	MarkPortEventDelivered(context.Context, string) error
	RetryPortEvent(context.Context, string, string, bool) error
}

type DeliveryError struct {
	StatusCode int
	Permanent  bool
	Message    string
}

func (e *DeliveryError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("port event API returned %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("port event API returned %d", e.StatusCode)
}

type Deliverer interface {
	Deliver(context.Context, porteventdomain.Event) error
}

type HTTPDeliverer struct {
	endpoint string
	client   *http.Client
}

func NewHTTPDeliverer(baseURL string) *HTTPDeliverer {
	return &HTTPDeliverer{
		endpoint: strings.TrimRight(baseURL, "/") + "/api/v1/port-events",
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (d *HTTPDeliverer) Deliver(ctx context.Context, event porteventdomain.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := d.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return &DeliveryError{
			StatusCode: response.StatusCode,
			Permanent:  response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != http.StatusConflict && response.StatusCode != http.StatusTooManyRequests,
			Message:    strings.TrimSpace(string(message)),
		}
	}
	return nil
}
