package httpapi

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/authstate"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkidsession"
)

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

type fakeSessions struct {
	token string
	err   error
	calls int
}

func (f *fakeSessions) AccessToken(context.Context, int64) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	if f.token == "" {
		return "", vkidsession.ErrNotLinked
	}
	return f.token, nil
}

type fakeUsers struct {
	mu    sync.Mutex
	links map[int64]storage.VKIDLink
	err   error
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{links: make(map[int64]storage.VKIDLink)}
}

func (f *fakeUsers) LinkVKID(_ context.Context, l storage.VKIDLink) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	for tgID, existing := range f.links {
		if existing.VKUserID == l.VKUserID && tgID != l.TelegramID {
			return storage.ErrVKIDAlreadyLinked
		}
	}
	f.links[l.TelegramID] = l
	return nil
}

func (f *fakeUsers) UnlinkVKID(_ context.Context, telegramID int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	l, ok := f.links[telegramID]
	if !ok {
		return "", storage.ErrUserNotFound
	}
	delete(f.links, telegramID)
	return l.AccessToken, nil
}

func (f *fakeUsers) FindByTelegramID(_ context.Context, telegramID int64) (storage.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return storage.User{}, f.err
	}
	l, ok := f.links[telegramID]
	if !ok {
		return storage.User{}, storage.ErrUserNotFound
	}
	return storage.User{TelegramID: l.TelegramID, VKUserID: l.VKUserID, Name: l.Name, Email: l.Email, LinkedAt: time.Now()}, nil
}

type fakeVKID struct {
	*vkid.Client
	tokens      vkid.Tokens
	exchangeErr error
	user        vkid.User
	userInfoErr error

	loggedOut []string
	logoutErr error
}

func (f *fakeVKID) Logout(_ context.Context, accessToken string) error {
	f.loggedOut = append(f.loggedOut, accessToken)
	return f.logoutErr
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

const (
	testMiniAppURL = "https://t.me/test_bot/app"
	testBotToken   = "123456:TEST-TOKEN"
)

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
	if deps.Users == nil {
		deps.Users = newFakeUsers()
	}
	if deps.Sessions == nil {
		deps.Sessions = &fakeSessions{}
	}

	r, err := NewRouter(config.Config{
		HTTP:     config.HTTP{HealthTimeout: time.Second},
		Telegram: config.Telegram{Token: testBotToken, MiniAppURL: testMiniAppURL, InitDataMaxAge: time.Hour},
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
