package config

import "testing"

func TestSophosSyslogRequiresSourceAllowlistWhenEnabled(t *testing.T) {
	t.Setenv("POSTGRES_DB", "nac")
	t.Setenv("POSTGRES_USER", "nac")
	t.Setenv("SOPHOS_RADIUS_SSO_ENABLED", "false")
	t.Setenv("SOPHOS_SYSLOG_ENABLED", "true")
	t.Setenv("SOPHOS_SYSLOG_ALLOWED_SOURCES", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing Sophos syslog source allowlist error")
	}
	t.Setenv("SOPHOS_SYSLOG_ALLOWED_SOURCES", "10.6.0.1,10.7.0.0/16")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SophosAuth.Port != 5514 || cfg.SophosAuth.BindingFreshnessSec != 600 || len(cfg.SophosAuth.AllowedClients) == 0 {
		t.Fatalf("unexpected Sophos auth defaults: %#v", cfg.SophosAuth)
	}
	if cfg.SophosAuth.MaxAttempts != 5 || cfg.SophosAuth.RetryBatchSize != 20 || cfg.SophosAuth.ClaimLeaseSec != 30 {
		t.Fatalf("unexpected Sophos auth retry defaults: %#v", cfg.SophosAuth)
	}
	if cfg.SophosAuth.WorkerCount != 4 || cfg.SophosAuth.QueueSize != 256 {
		t.Fatalf("unexpected Sophos auth backpressure defaults: %#v", cfg.SophosAuth)
	}
}

func TestSophosSyslogRejectsInvalidBackpressureLimits(t *testing.T) {
	t.Setenv("POSTGRES_DB", "nac")
	t.Setenv("POSTGRES_USER", "nac")
	t.Setenv("SOPHOS_RADIUS_SSO_ENABLED", "false")
	t.Setenv("SOPHOS_SYSLOG_ENABLED", "true")
	t.Setenv("SOPHOS_SYSLOG_ALLOWED_SOURCES", "10.6.0.1")
	t.Setenv("SOPHOS_SYSLOG_WORKER_COUNT", "65")
	if _, err := Load(); err == nil {
		t.Fatal("expected Sophos syslog worker count validation error")
	}
	t.Setenv("SOPHOS_SYSLOG_WORKER_COUNT", "4")
	t.Setenv("SOPHOS_SYSLOG_QUEUE_SIZE", "10001")
	if _, err := Load(); err == nil {
		t.Fatal("expected Sophos syslog queue size validation error")
	}
}

func TestSophosSyslogRejectsLeaseShorterThanProcessingTimeout(t *testing.T) {
	t.Setenv("POSTGRES_DB", "nac")
	t.Setenv("POSTGRES_USER", "nac")
	t.Setenv("SOPHOS_RADIUS_SSO_ENABLED", "false")
	t.Setenv("SOPHOS_SYSLOG_ENABLED", "true")
	t.Setenv("SOPHOS_SYSLOG_ALLOWED_SOURCES", "10.6.0.1")
	t.Setenv("SOPHOS_SYSLOG_PROCESSING_TIMEOUT_SECONDS", "10")
	t.Setenv("SOPHOS_SYSLOG_CLAIM_LEASE_SECONDS", "5")
	if _, err := Load(); err == nil {
		t.Fatal("expected Sophos syslog claim lease validation error")
	}
}
