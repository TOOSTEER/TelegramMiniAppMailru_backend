package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/authstate"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

var testVKIDConfig = config.VKID{
	ClientID:    "12345",
	AuthURL:     "https://id.vk.ru/authorize",
	RedirectURL: "http://localhost/oauth/callback",
	Scopes:      "vkid.personal_info email",
	Provider:    "mail_ru",
}

func newTestRouter(t *testing.T, deps Deps) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	if deps.Log == nil {
		deps.Log = zap.NewNop()
	}
	if deps.DB == nil {
		deps.DB = fakePinger{}
	}
	if deps.VKID == nil {
		deps.VKID = vkid.New(testVKIDConfig)
	}
	if deps.States == nil {
		deps.States = authstate.NewMemory(time.Minute)
	}

	r, err := NewRouter(config.HTTP{HealthTimeout: time.Second}, deps)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
