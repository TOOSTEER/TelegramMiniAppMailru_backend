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

type fakeVKID struct {
	*vkid.Client
	tokens      vkid.Tokens
	exchangeErr error
	user        vkid.User
	userInfoErr error
}

func (f *fakeVKID) Exchange(context.Context, string, string, string, string) (vkid.Tokens, error) {
	return f.tokens, f.exchangeErr
}

func (f *fakeVKID) UserInfo(context.Context, string) (vkid.User, error) {
	return f.user, f.userInfoErr
}

func newFakeVKID() *fakeVKID {
	return &fakeVKID{
		Client: vkid.New(testVKIDConfig),
		tokens: vkid.Tokens{AccessToken: "at", UserID: 42},
		user:   vkid.User{ID: 42, FirstName: "Ivan", LastName: "Ivanov", Email: "ivan@mail.ru"},
	}
}

const testMiniAppURL = "https://t.me/test_bot/app"

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
		deps.VKID = newFakeVKID()
	}
	if deps.States == nil {
		deps.States = authstate.NewMemory(time.Minute)
	}

	r, err := NewRouter(config.Config{
		HTTP:     config.HTTP{HealthTimeout: time.Second},
		Telegram: config.Telegram{MiniAppURL: testMiniAppURL},
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
