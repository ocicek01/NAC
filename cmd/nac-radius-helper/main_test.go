package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestAuthorizeAddsBearerTokenAndNeverSendsPassword(t *testing.T) {
	var capturedBody string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer shared-token" {
			t.Fatalf("Authorization = %q", got)
		}
		body, _ := io.ReadAll(req.Body)
		capturedBody = string(body)
		return jsonResponse(http.StatusOK, `{"decision":"accept","reply_attributes":{"Reply-Message":"ok"}}`), nil
	})}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := run([]string{"--username", "omer.cicek", "--mac", "00:11:22:33:44:55"}, envMap(map[string]string{
		"NAC_RADIUS_API_TOKEN": "shared-token",
	}), stdout, stderr, client)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if strings.Contains(strings.ToLower(capturedBody), "password") || strings.Contains(capturedBody, "shared-token") {
		t.Fatalf("sensitive value leaked in payload: %s", capturedBody)
	}
}

func TestAccountingIncludesTimestampAndDelay(t *testing.T) {
	var body string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(req.Body)
		body = string(data)
		return jsonResponse(http.StatusOK, `{"status":"ok"}`), nil
	})}
	code := run([]string{
		"--mode", "accounting", "--acct-status-type", "Interim-Update",
		"--acct-session-id", "session-1", "--event-timestamp", "2026-10-13T12:00:00Z",
		"--acct-delay-time", "7",
	}, envMap(nil), io.Discard, io.Discard, client)
	if code != 0 || !strings.Contains(body, `"event_timestamp":"2026-10-13T12:00:00Z"`) || !strings.Contains(body, `"acct_delay_time":7`) {
		t.Fatalf("code=%d body=%s", code, body)
	}
}

func TestHelperFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		body string
		code int
	}{
		{name: "remote endpoint", env: map[string]string{"NAC_RADIUS_AUTHORIZE_URL": "http://10.0.0.2:8080/api/v1/radius/authorize"}, code: 1},
		{name: "invalid decision", body: `{"decision":""}`, code: 1},
		{name: "explicit reject", body: `{"decision":"reject"}`, code: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, tt.body), nil
			})}
			if got := run(nil, envMap(tt.env), io.Discard, io.Discard, client); got != tt.code {
				t.Fatalf("code=%d want=%d", got, tt.code)
			}
		})
	}
}

func TestHelperFailsClosedWhenNACIsUnavailable(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}
	stderr := &bytes.Buffer{}
	if code := run(nil, envMap(nil), io.Discard, stderr, client); code != 1 {
		t.Fatalf("code=%d", code)
	}
	if strings.Contains(stderr.String(), "connection refused") {
		t.Fatalf("transport detail should not leak: %q", stderr.String())
	}
}

func TestHelperTimeoutIsBounded(t *testing.T) {
	if got := helperTimeout(envMap(map[string]string{"NAC_RADIUS_HELPER_TIMEOUT_SECONDS": "30"})); got != defaultTimeout {
		t.Fatalf("timeout=%s", got)
	}
	if got := helperTimeout(envMap(map[string]string{"NAC_RADIUS_HELPER_TIMEOUT_SECONDS": "2"})); got != 2*time.Second {
		t.Fatalf("timeout=%s", got)
	}
}

func TestDeploymentTemplatesSeparateMABAndUserAuthentication(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "freeradius")
	var combined strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			combined.Write(data)
		}
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	config := combined.String()
	for _, required := range []string{"nac_mab_authorize", "ldaps://", "require_cert = \"demand\"", "ttls", "event-timestamp", "acct-delay-time", "NAC_RADIUS_API_TOKEN"} {
		if !strings.Contains(config, required) {
			t.Errorf("deployment templates missing %q", required)
		}
	}
	forbidden := "Auth-Type" + " Accept"
	if strings.Contains(config, forbidden) {
		t.Fatalf("deployment contains authentication bypass")
	}

	outer, err := os.ReadFile(filepath.Join(root, "sites-available", "nac"))
	if err != nil {
		t.Fatal(err)
	}
	inner, err := os.ReadFile(filepath.Join(root, "sites-available", "nac-inner-tunnel"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(outer), "nac_ldap") {
		t.Fatal("MAB/outer site must not invoke LDAP")
	}
	if !strings.Contains(string(inner), "nac_ldap") || !strings.Contains(string(inner), "nac_user_authorize") {
		t.Fatal("inner tunnel must authenticate with LDAP before NAC authorization")
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func envMap(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
