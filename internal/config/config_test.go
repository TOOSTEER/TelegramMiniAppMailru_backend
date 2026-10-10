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
	"VKID_AUTH_URL", "VKID_TOKEN_URL", "VKID_USERINFO_URL", "VKID_HTTP_TIMEOUT", "VKID_REDIRECT_URL", "VKID_SCOPES", "VKID_PROVIDER", "OAUTH_STATE_TTL",
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range optionalKeys {
		t.Setenv(k, "")
	}
	t.Setenv("TG_BOT_TOKEN", "secret")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("VKID_CLIENT_ID", "12345")
	t.Setenv("TG_MINIAPP_URL", "https://t.me/test_bot/app")
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
		Telegram: Telegram{Token: "secret", MiniAppURL: "https://t.me/test_bot/app"},
		VKID: VKID{
			ClientID:    "12345",
			AuthURL:     "https://id.vk.ru/authorize",
			TokenURL:    "https://id.vk.ru/oauth2/auth",
			UserInfoURL: "https://id.vk.ru/oauth2/user_info",
			RedirectURL: "http://localhost/oauth/callback",
			Scopes:      "vkid.personal_info email",
			Provider:    "mail_ru",
			StateTTL:    10 * time.Minute,
			HTTPTimeout: 10 * time.Second,
		},
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
		"missing required":      {"TG_BOT_TOKEN": "", "DATABASE_URL": "", "VKID_CLIENT_ID": "", "TG_MINIAPP_URL": ""},
		"relative redirect URL": {"VKID_REDIRECT_URL": "/oauth/callback"},
		"unknown provider":      {"VKID_PROVIDER": "google"},
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
		a.DB == b.DB && a.Telegram == b.Telegram && a.VKID == b.VKID
}
