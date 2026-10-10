package httpapi

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type handler struct {
	db            Pinger
	healthTimeout time.Duration
}

func NewRouter(cfg config.HTTP, log *zap.Logger, db Pinger) (*gin.Engine, error) {
	h := &handler{db: db, healthTimeout: cfg.HealthTimeout}

	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("доверенные прокси: %w", err)
	}

	r.Use(requestLogger(log), recovery(log))

	r.GET("/health", h.health)

	return r, nil
}
