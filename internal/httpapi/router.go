package httpapi

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type VKID interface {
	AuthURL(state, verifier string) string
	Exchange(ctx context.Context, code, verifier, deviceID, state string) (vkid.Tokens, error)
	UserInfo(ctx context.Context, accessToken string) (vkid.User, error)
}

type StateStore interface {
	Save(ctx context.Context, state, verifier string) error
	Pop(ctx context.Context, state string) (verifier string, err error)
}

type Deps struct {
	Log    *zap.Logger
	DB     Pinger
	VKID   VKID
	States StateStore
}

type handler struct {
	log           *zap.Logger
	db            Pinger
	vkid          VKID
	states        StateStore
	healthTimeout time.Duration
	miniAppURL    *url.URL
}

func NewRouter(cfg config.Config, deps Deps) (*gin.Engine, error) {
	miniAppURL, err := url.Parse(cfg.Telegram.MiniAppURL)
	if err != nil {
		return nil, fmt.Errorf("адрес Mini App: %w", err)
	}

	h := &handler{
		log:           deps.Log,
		db:            deps.DB,
		vkid:          deps.VKID,
		states:        deps.States,
		healthTimeout: cfg.HTTP.HealthTimeout,
		miniAppURL:    miniAppURL,
	}

	r := gin.New()
	if err := r.SetTrustedProxies(cfg.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("доверенные прокси: %w", err)
	}

	r.Use(requestLogger(deps.Log), recovery(deps.Log))

	r.GET("/health", h.health)

	oauth := r.Group("/oauth", privatePage)
	oauth.GET("/login", h.oauthLogin)
	oauth.GET("/callback", h.oauthCallback)

	return r, nil
}
