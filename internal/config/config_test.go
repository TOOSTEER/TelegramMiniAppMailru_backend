package config

import (
	"slices"
	"testing"
	"time"
)

var optionalKeys = []string{
	"APP_ENV", "LOG_LEVEL",
	"HTTP_ADDR", "HTTP_READ_HEADER_TIMEOUT", "SHUTDOWN_TIMEOUT", "HEALTH_TIMEOUT", "HTTP_TRUSTED_PROXIES",
	"DB_CONNECT_TIMEOUT",
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range optionalKeys {
		t.Setenv(k, "")
	}
	t.Setenv("TG_BOT_TOKEN", "secret")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, nil)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		Env:      EnvDevelopment,
		LogLevel: "debug",
		HTTP: HTTP{
			Addr:              ":8080",
			ReadHeaderTimeout: 5 * time.Second,
			ShutdownTimeout:   10 * time.Second,
			HealthTimeout:     2 * time.Second,
		},
		DB: DB{
			URL:            "postgres://localhost/db",
			ConnectTimeout: 5 * time.Second,
		},
		Telegram: Telegram{Token: "secret"},
	}
	if !equal(cfg, want) {
		t.Errorf("Load() = %+v,\nwant %+v", cfg, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	setEnv(t, map[string]string{
		"APP_ENV":                  "production",
		"HTTP_READ_HEADER_TIMEOUT": "1s",
		"HEALTH_TIMEOUT":           "500ms",
		"DB_CONNECT_TIMEOUT":       "1m",
		"HTTP_TRUSTED_PROXIES":     " 10.0.0.1, 192.168.0.0/16 ,",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want production default info", cfg.LogLevel)
	}
	if cfg.HTTP.ReadHeaderTimeout != time.Second || cfg.HTTP.HealthTimeout != 500*time.Millisecond || cfg.DB.ConnectTimeout != time.Minute {
		t.Errorf("timeouts = %v/%v/%v, want 1s/500ms/1m", cfg.HTTP.ReadHeaderTimeout, cfg.HTTP.HealthTimeout, cfg.DB.ConnectTimeout)
	}
	if want := []string{"10.0.0.1", "192.168.0.0/16"}; !slices.Equal(cfg.HTTP.TrustedProxies, want) {
		t.Errorf("TrustedProxies = %q, want %q", cfg.HTTP.TrustedProxies, want)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := map[string]map[string]string{
		"missing required":      {"TG_BOT_TOKEN": "", "DATABASE_URL": ""},
		"invalid APP_ENV":       {"APP_ENV": "prod"},
		"invalid LOG_LEVEL":     {"LOG_LEVEL": "verbose"},
		"duration without unit": {"SHUTDOWN_TIMEOUT": "10"},
		"negative duration":     {"DB_CONNECT_TIMEOUT": "-1s"},
		"invalid proxy":         {"HTTP_TRUSTED_PROXIES": "10.0.0.1,nginx"},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			setEnv(t, env)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want error")
			}
		})
	}
}

func equal(a, b Config) bool {
	return a.Env == b.Env && a.LogLevel == b.LogLevel &&
		a.HTTP.Addr == b.HTTP.Addr &&
		a.HTTP.ReadHeaderTimeout == b.HTTP.ReadHeaderTimeout &&
		a.HTTP.ShutdownTimeout == b.HTTP.ShutdownTimeout &&
		a.HTTP.HealthTimeout == b.HTTP.HealthTimeout &&
		slices.Equal(a.HTTP.TrustedProxies, b.HTTP.TrustedProxies) &&
		a.DB == b.DB && a.Telegram == b.Telegram
}
