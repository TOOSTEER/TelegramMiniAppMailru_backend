package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

type Config struct {
	Env             string
	HTTPAddr        string
	TelegramToken   string
	ShutdownTimeout time.Duration
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

	cfg := Config{
		Env:             getenv("APP_ENV", EnvDevelopment),
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		TelegramToken:   required("TG_BOT_TOKEN"),
		ShutdownTimeout: getenvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	if cfg.Env != EnvDevelopment && cfg.Env != EnvProduction {
		errs = append(errs, fmt.Errorf("APP_ENV=%q: допустимы %s и %s", cfg.Env, EnvDevelopment, EnvProduction))
		cfg.Env = EnvDevelopment
	}

	return cfg, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fallback
		}

		return d
	}

	return fallback
}
