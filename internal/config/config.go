package config

import (
	"errors"
	"fmt"
	"net"
	"os"
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
	Token string
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
			Token: required("TG_BOT_TOKEN"),
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
