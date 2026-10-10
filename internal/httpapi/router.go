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

type AuthURLBuilder interface {
	AuthURL(state, verifier string) string
}

type StateStore interface {
	Save(ctx context.Context, state, verifier string) error
	Pop(ctx context.Context, state string) (verifier string, err error)
}

type Deps struct {
	Log    *zap.Logger
	DB     Pinger
	VKID   AuthURLBuilder
	States StateStore
}

type handler struct {
	db            Pinger
	vkid          AuthURLBuilder
	states        StateStore
	healthTimeout time.Duration
}

func NewRouter(cfg config.HTTP, deps Deps) (*gin.Engine, error) {
	h := &handler{
		db:            deps.DB,
		vkid:          deps.VKID,
		states:        deps.States,
		healthTimeout: cfg.HealthTimeout,
	}

	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("доверенные прокси: %w", err)
	}

	r.Use(requestLogger(deps.Log), recovery(deps.Log))

	r.GET("/health", h.health)

	oauth := r.Group("/oauth")
	oauth.GET("/login", h.oauthLogin)
	oauth.GET("/callback", h.oauthCallback)

	return r, nil
}
