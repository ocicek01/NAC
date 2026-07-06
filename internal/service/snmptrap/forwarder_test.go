package snmptrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "nac/internal/domain/snmptrap"
)

func TestHTTPPortStatusForwarderPostsNormalizedPayload(t *testing.T) {
	var receivedMethod string
	var receivedToken string
	var payload trapForwardPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedToken = r.Header.Get("X-TRAP-TOKEN")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	forwarder := NewHTTPPortStatusForwarder(true, server.URL, "secret-token", 2*time.Second)
	if forwarder == nil {
		t.Fatal("expected forwarder instance")
	}

	err := forwarder.ForwardPortStatus(context.Background(), domain.Event{
		SourceIP:   "10.6.8.19",
		SwitchName: "sw-10-6-8-19",
		TrapOID:    ".1.3.6.1.6.3.1.1.5.3",
		Category:   "link-down",
		IfIndex:    45,
		VarBinds: []domain.VarBind{
			{OID: ".1.3.6.1.2.1.2.2.1.1.45", Type: "Integer", Value: "45"},
			{OID: ".1.3.6.1.2.1.2.2.1.7.45", Type: "Integer", Value: "2"},
			{OID: ".1.3.6.1.2.1.2.2.1.8.45", Type: "Integer", Value: "2"},
			{OID: ".1.3.6.1.2.1.2.2.1.2.45", Type: "OctetString", Value: "45"},
			{OID: ".1.3.6.1.2.1.31.1.1.1.18.45", Type: "OctetString", Value: "User port 45"},
		},
		ReceivedAt: time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ForwardPortStatus returned error: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", receivedMethod)
	}
	if receivedToken != "secret-token" {
		t.Fatalf("expected token header, got %q", receivedToken)
	}
	if payload.SwitchIP != "10.6.8.19" {
		t.Fatalf("expected switch_ip, got %q", payload.SwitchIP)
	}
	if payload.SwitchHostname != "sw-10-6-8-19" {
		t.Fatalf("expected switch_hostname, got %q", payload.SwitchHostname)
	}
	if payload.IfIndex != 45 {
		t.Fatalf("expected if_index 45, got %d", payload.IfIndex)
	}
	if payload.IfName != "45" {
		t.Fatalf("expected if_name 45, got %q", payload.IfName)
	}
	if payload.IfDescr != "User port 45" {
		t.Fatalf("expected if_descr from alias, got %q", payload.IfDescr)
	}
	if payload.AdminStatus != "down" {
		t.Fatalf("expected admin_status down, got %q", payload.AdminStatus)
	}
	if payload.OperStatus != "down" {
		t.Fatalf("expected oper_status down, got %q", payload.OperStatus)
	}
	if payload.TrapType != "linkDown" {
		t.Fatalf("expected trap_type linkDown, got %q", payload.TrapType)
	}
	if payload.OccurredAt != "2026-07-06T12:00:00Z" {
		t.Fatalf("expected occurred_at to use received time, got %q", payload.OccurredAt)
	}
	if len(payload.Varbinds) != 5 || payload.Varbinds[0].Value != "45" {
		t.Fatalf("expected varbind payload to be preserved, got %+v", payload.Varbinds)
	}
}
