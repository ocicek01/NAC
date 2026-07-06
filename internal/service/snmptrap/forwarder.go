package snmptrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	domain "nac/internal/domain/snmptrap"
)

type HTTPPortStatusForwarder struct {
	url    string
	token  string
	client *http.Client
}

type trapForwardPayload struct {
	SwitchIP       string           `json:"switch_ip,omitempty"`
	SourceIP       string           `json:"source_ip,omitempty"`
	SwitchHostname string           `json:"switch_hostname,omitempty"`
	IfIndex        int              `json:"if_index"`
	AdminStatus    string           `json:"admin_status,omitempty"`
	OperStatus     string           `json:"oper_status,omitempty"`
	TrapOID        string           `json:"trap_oid,omitempty"`
	TrapType       string           `json:"trap_type,omitempty"`
	OccurredAt     string           `json:"occurred_at,omitempty"`
	Varbinds       []domain.VarBind `json:"varbinds,omitempty"`
}

func NewHTTPPortStatusForwarder(enabled bool, url, token string, timeout time.Duration) *HTTPPortStatusForwarder {
	url = strings.TrimSpace(url)
	if !enabled || url == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPPortStatusForwarder{
		url:   url,
		token: strings.TrimSpace(token),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (f *HTTPPortStatusForwarder) ForwardPortStatus(ctx context.Context, event domain.Event) error {
	if f == nil {
		return nil
	}

	payload := trapForwardPayload{
		SwitchIP:       strings.TrimSpace(event.SourceIP),
		SourceIP:       strings.TrimSpace(event.SourceIP),
		SwitchHostname: strings.TrimSpace(event.SwitchName),
		IfIndex:        event.IfIndex,
		AdminStatus:    "unknown",
		OperStatus:     operStatusFromCategory(event.Category),
		TrapOID:        strings.TrimSpace(event.TrapOID),
		TrapType:       trapTypeFromCategory(event.Category),
		OccurredAt:     resolveOccurredAt(event).Format(time.RFC3339),
		Varbinds:       append([]domain.VarBind{}, event.VarBinds...),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if f.token != "" {
		req.Header.Set("X-TRAP-TOKEN", f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("trap forward returned status %d", resp.StatusCode)
	}

	return nil
}

func operStatusFromCategory(category string) string {
	switch strings.TrimSpace(category) {
	case "link-up":
		return "up"
	case "link-down":
		return "down"
	default:
		return "unknown"
	}
}

func trapTypeFromCategory(category string) string {
	switch strings.TrimSpace(category) {
	case "link-up":
		return "linkUp"
	case "link-down":
		return "linkDown"
	default:
		return strings.TrimSpace(category)
	}
}

func resolveOccurredAt(event domain.Event) time.Time {
	if !event.ReceivedAt.IsZero() {
		return event.ReceivedAt.UTC()
	}
	if !event.CreatedAt.IsZero() {
		return event.CreatedAt.UTC()
	}
	return time.Now().UTC()
}
