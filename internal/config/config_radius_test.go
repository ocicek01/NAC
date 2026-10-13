package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresRadiusTokenWhenHelperEnabled(t *testing.T) {
	t.Setenv("POSTGRES_DB", "nac")
	t.Setenv("POSTGRES_USER", "nac")
	t.Setenv("RADIUS_HELPER_ENABLED", "true")
	t.Setenv("NAC_RADIUS_API_TOKEN", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "RADIUS API token") {
		t.Fatalf("Load() error = %v, want missing RADIUS API token error", err)
	}
}

func TestLoadAcceptsRadiusHelperWithToken(t *testing.T) {
	t.Setenv("POSTGRES_DB", "nac")
	t.Setenv("POSTGRES_USER", "nac")
	t.Setenv("RADIUS_HELPER_ENABLED", "true")
	t.Setenv("NAC_RADIUS_API_TOKEN", "shared-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Radius.HelperEnabled || cfg.Radius.APIToken != "shared-token" {
		t.Fatalf("unexpected RADIUS config: %+v", cfg.Radius)
	}
}
