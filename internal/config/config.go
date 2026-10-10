package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

type Config struct {
	Env      string
	LogLevel string
	HTTP     HTTP
	DB       DB
	Telegram Telegram
	VKID     VKID
}

type HTTP struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
	HealthTimeout     time.Duration
	TrustedProxies    []string
}

type DB struct {
	URL            string
	ConnectTimeout time.Duration
}

type Telegram struct {
	Token          string
	MiniAppURL     string
	InitDataMaxAge time.Duration
}

type VKID struct {
	ClientID      string
	AuthURL       string
	TokenURL      string
	UserInfoURL   string
	LogoutURL     string
	RedirectURL   string
	Scopes        string
	Provider      string
	StateTTL      time.Duration
	HTTPTimeout   time.Duration
	RefreshBefore time.Duration
}

func Load() (Config, error) {
	var errs []error

	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("переменная окружения %s не задана", key))
		}
		return v
	}

	duration := func(key string, fallback time.Duration) time.Duration {
		v := os.Getenv(key)
		if v == "" {
			return fallback
		}

		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s=%q: ожидается положительная длительность вида 10s или 1m", key, v))
			return fallback
		}

		return d
	}

	ipList := func(key string) []string {
		var list []string
		for item := range strings.SplitSeq(os.Getenv(key), ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if net.ParseIP(item) == nil {
				if _, _, err := net.ParseCIDR(item); err != nil {
					errs = append(errs, fmt.Errorf("%s: %q не IP-адрес и не подсеть CIDR", key, item))
					continue
				}
			}
			list = append(list, item)
		}
		return list
	}

	absURL := func(key, fallback string) string {
		v := getenv(key, fallback)
		if v == "" {
			return required(key)
		}
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, fmt.Errorf("%s=%q: ожидается абсолютный http(s) URL", key, v))
		}
		return v
	}

	oneOf := func(key, fallback string, allowed ...string) string {
		v := getenv(key, fallback)
		if !slices.Contains(allowed, v) {
			errs = append(errs, fmt.Errorf("%s=%q: допустимы %s", key, v, strings.Join(allowed, ", ")))
			return fallback
		}
		return v
	}

	cfg := Config{
		Env: getenv("APP_ENV", EnvDevelopment),
		HTTP: HTTP{
			Addr:              getenv("HTTP_ADDR", ":8080"),
			ReadHeaderTimeout: duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ShutdownTimeout:   duration("SHUTDOWN_TIMEOUT", 10*time.Second),
			HealthTimeout:     duration("HEALTH_TIMEOUT", 2*time.Second),
			TrustedProxies:    ipList("HTTP_TRUSTED_PROXIES"),
		},
		DB: DB{
			URL:            required("DATABASE_URL"),
			ConnectTimeout: duration("DB_CONNECT_TIMEOUT", 5*time.Second),
		},
		Telegram: Telegram{
			Token:          required("TG_BOT_TOKEN"),
			MiniAppURL:     absURL("TG_MINIAPP_URL", ""),
			InitDataMaxAge: duration("TG_INITDATA_MAX_AGE", 24*time.Hour),
		},
		VKID: VKID{
			ClientID:      required("VKID_CLIENT_ID"),
			AuthURL:       absURL("VKID_AUTH_URL", "https://id.vk.ru/authorize"),
			TokenURL:      absURL("VKID_TOKEN_URL", "https://id.vk.ru/oauth2/auth"),
			UserInfoURL:   absURL("VKID_USERINFO_URL", "https://id.vk.ru/oauth2/user_info"),
			LogoutURL:     absURL("VKID_LOGOUT_URL", "https://id.vk.ru/oauth2/logout"),
			RedirectURL:   absURL("VKID_REDIRECT_URL", "http://localhost/oauth/callback"),
			Scopes:        getenv("VKID_SCOPES", "vkid.personal_info email"),
			Provider:      oneOf("VKID_PROVIDER", "mail_ru", "mail_ru", "ok_ru", "vkid"),
			StateTTL:      duration("OAUTH_STATE_TTL", 10*time.Minute),
			HTTPTimeout:   duration("VKID_HTTP_TIMEOUT", 10*time.Second),
			RefreshBefore: duration("VKID_REFRESH_BEFORE", time.Minute),
		},
	}

	if cfg.Env != EnvDevelopment && cfg.Env != EnvProduction {
		errs = append(errs, fmt.Errorf("APP_ENV=%q: допустимы %s и %s", cfg.Env, EnvDevelopment, EnvProduction))
		cfg.Env = EnvDevelopment
	}

	defaultLevel := "debug"
	if cfg.Env == EnvProduction {
		defaultLevel = "info"
	}
	cfg.LogLevel = getenv("LOG_LEVEL", defaultLevel)

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL=%q: допустимы debug, info, warn, error", cfg.LogLevel))
		cfg.LogLevel = defaultLevel
	}

	return cfg, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
