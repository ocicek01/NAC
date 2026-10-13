package sophossyslog

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"io"
	"log/slog"

	"nac/internal/config"
	domain "nac/internal/domain/sophosauth"
)

func TestParseFusionAndLegacyFields(t *testing.T) {
	received := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	payload := []byte(`log_component="Firewall Authentication" log_id=17701 user_name="omer.cicek" user_group="Personel" client_used="Captive Portal" auth_mechanism=LDAP status=Success src_ip=10.6.9.93 src_mac=30:9C:23:9B:97:AA message="Login \"successful\"" eventtime=1791892800`)
	event, err := Parse(payload, received)
	if err != nil {
		t.Fatal(err)
	}
	if event.EventType != "login" || event.Username != "omer.cicek" || event.SourceIP != "10.6.9.93" || event.Message != `Login "successful"` {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.Fingerprint == "" || event.EventAt.Unix() != 1791892800 {
		t.Fatalf("fingerprint/time not parsed: %#v", event)
	}

	legacy, err := Parse([]byte(`component=Authentication message_id=17703 user=student auth_client="Web Client" sourceip=10.6.9.94 status=Success message=logout`), received)
	if err != nil || legacy.EventType != "logout" || legacy.Username != "student" {
		t.Fatalf("legacy parse failed: %#v %v", legacy, err)
	}
}

func TestParseRejectsMalformedAndUnrelatedRecords(t *testing.T) {
	if _, err := Parse([]byte(`component=Authentication user="broken`), time.Now()); err == nil {
		t.Fatal("expected malformed quote error")
	}
	if _, err := Parse([]byte(`component=Firewall log_id=1 status=Success`), time.Now()); !errors.Is(err, errNotAuthentication) {
		t.Fatalf("expected unrelated record error, got %v", err)
	}
}

type collectorProcessor struct {
	processed int
	rejected  int
	failed    int
}

func (p *collectorProcessor) Process(context.Context, domain.ParsedEvent) error {
	p.processed++
	return nil
}
func (p *collectorProcessor) ProcessDue(context.Context, int) error             { return nil }
func (p *collectorProcessor) RecordRejectedSource(context.Context, string)      { p.rejected++ }
func (p *collectorProcessor) RecordParseFailure(context.Context, string, error) { p.failed++ }
func (p *collectorProcessor) RecordQueueDrop(context.Context, string, int)      {}

func TestCollectorEnforcesSourceAllowlistBeforeParsing(t *testing.T) {
	processor := &collectorProcessor{}
	collector, err := New(config.SophosAuthConfig{AllowedSources: []string{"10.6.0.1", "192.168.10.0/24"}, ProcessingTimeoutSec: 1}, slog.New(slog.NewTextHandler(io.Discard, nil)), processor)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`component=Authentication log_id=17701 user_name=test client_used="Captive Portal" status=Success src_ip=10.6.9.93`)
	collector.processDatagram(context.Background(), net.ParseIP("10.6.0.2"), payload)
	collector.processDatagram(context.Background(), net.ParseIP("192.168.10.9"), payload)
	if processor.rejected != 1 || processor.processed != 1 {
		t.Fatalf("unexpected calls: %#v", processor)
	}
	collector.processDatagram(context.Background(), net.ParseIP("10.6.0.1"), []byte(`component=Authentication user="broken`))
	if processor.failed != 1 {
		t.Fatalf("parse failure was not reported: %#v", processor)
	}
}
