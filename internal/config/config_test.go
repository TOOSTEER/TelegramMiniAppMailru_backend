package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("TG_BOT_TOKEN", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want default :8080", cfg.HTTPAddr)
	}
	if cfg.TelegramToken != "secret" {
		t.Errorf("TelegramToken = %q, want secret", cfg.TelegramToken)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	t.Setenv("TG_BOT_TOKEN", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error about TG_BOT_TOKEN")
	}
}

func TestLoadInvalidEnv(t *testing.T) {
	t.Setenv("TG_BOT_TOKEN", "secret")
	t.Setenv("APP_ENV", "prod")

	cfg, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error about APP_ENV")
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want fallback %q", cfg.Env, EnvDevelopment)
	}
}
