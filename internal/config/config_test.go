package config_test

import (
	"testing"
	"time"

	"ripple-note/internal/config"
)

func TestLoadLocalConfigAppliesYAMLAndDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load("../../configs/config.local.yaml")
	if err != nil {
		t.Fatalf("expected config to load, got error: %v", err)
	}

	if cfg.App.Name != "ripple-note" {
		t.Fatalf("expected app name ripple-note, got %q", cfg.App.Name)
	}
	if cfg.HTTP.Port != 8080 {
		t.Fatalf("expected HTTP port 8080, got %d", cfg.HTTP.Port)
	}
	if cfg.HTTP.ReadTimeout != 5*time.Second {
		t.Fatalf("expected read timeout 5s, got %s", cfg.HTTP.ReadTimeout)
	}
	if !cfg.MySQL.Enabled {
		t.Fatal("expected local MySQL to be enabled")
	}
	if cfg.MySQL.DSN == "" {
		t.Fatal("expected MySQL DSN to be present for later stages")
	}
	if cfg.Auth.JWTSecret == "" {
		t.Fatal("expected auth jwt secret to be present")
	}
	if cfg.Auth.JWTTTL != 24*time.Hour {
		t.Fatalf("expected jwt ttl 24h, got %s", cfg.Auth.JWTTTL)
	}
}

func TestValidateRequiresDSNWhenMySQLEnabled(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("expected default config to load, got error: %v", err)
	}
	cfg.MySQL.Enabled = true
	cfg.MySQL.DSN = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when mysql is enabled without dsn")
	}
}
