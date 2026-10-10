package httpapi

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/authstate"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
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
	Save(ctx context.Context, state string, entry authstate.Entry) error
	Pop(ctx context.Context, state string) (authstate.Entry, error)
}

type Users interface {
	LinkVKID(ctx context.Context, link storage.VKIDLink) error
	FindByTelegramID(ctx context.Context, telegramID int64) (storage.User, error)
}

type Deps struct {
	Log    *zap.Logger
	DB     Pinger
	VKID   VKID
	States StateStore
	Users  Users
}

type handler struct {
	log           *zap.Logger
	db            Pinger
	vkid          VKID
	states        StateStore
	users         Users
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
		users:         deps.Users,
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
	oauth.GET("/callback", h.oauthCallback)

	api := r.Group("/api", telegramAuth(cfg.Telegram.Token, cfg.Telegram.InitDataMaxAge))
	api.GET("/me", h.me)
	api.POST("/vkid/login", h.vkidLogin)

	return r, nil
}
