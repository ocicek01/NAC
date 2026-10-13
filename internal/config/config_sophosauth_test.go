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
}
