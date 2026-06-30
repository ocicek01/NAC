package identitysource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	Matched      bool
	Source       string
	IdentityType string
	ExternalID   string
	Username     string
	FullName     string
	TargetVLAN   int
	ExpiresAt    time.Time
	Attributes   map[string]any
}

type Resolver interface {
	Resolve(ctx context.Context, identifier, password string) (*Result, error)
}

type HTTPResolver struct {
	name   string
	url    string
	client *http.Client
}

type verifyRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type verifyResponse struct {
	Matched      bool           `json:"matched"`
	IdentityType string         `json:"identity_type"`
	ExternalID   string         `json:"external_id"`
	Username     string         `json:"username"`
	FullName     string         `json:"full_name"`
	TargetVLAN   int            `json:"target_vlan"`
	ExpiresAt    string         `json:"expires_at"`
	Attributes   map[string]any `json:"attributes"`
}

func NewHTTPResolver(name, url string, timeout time.Duration) *HTTPResolver {
	return &HTTPResolver{
		name: strings.TrimSpace(name),
		url:  strings.TrimSpace(url),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (r *HTTPResolver) Resolve(ctx context.Context, identifier, password string) (*Result, error) {
	if strings.TrimSpace(r.url) == "" {
		return nil, nil
	}

	payload, err := json.Marshal(verifyRequest{
		Identifier: strings.TrimSpace(identifier),
		Password:   password,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s verify returned status %d", r.name, resp.StatusCode)
	}

	var body verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if !body.Matched {
		return nil, nil
	}

	var expiresAt time.Time
	if strings.TrimSpace(body.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(body.ExpiresAt))
		if err == nil {
			expiresAt = parsed.UTC()
		}
	}

	return &Result{
		Matched:      true,
		Source:       r.name,
		IdentityType: strings.TrimSpace(body.IdentityType),
		ExternalID:   strings.TrimSpace(body.ExternalID),
		Username:     strings.TrimSpace(body.Username),
		FullName:     strings.TrimSpace(body.FullName),
		TargetVLAN:   body.TargetVLAN,
		ExpiresAt:    expiresAt,
		Attributes:   body.Attributes,
	}, nil
}
