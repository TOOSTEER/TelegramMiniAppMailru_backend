package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

const testTelegramID = 42

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func location(t *testing.T, w *httptest.ResponseRecorder) *url.URL {
	t.Helper()
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func startLogin(t *testing.T, r *gin.Engine) (state string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/vkid/login", nil)
	req.Header.Set("Authorization", "tma "+testInitData(testBotToken, time.Now()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("/api/vkid/login: status = %d, body: %s", w.Code, w.Body.String())
	}
	var body struct {
		AuthURL string `json:"auth_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(body.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("state")
}

func assertBackToMiniApp(t *testing.T, w *httptest.ResponseRecorder, wantResult string) {
	t.Helper()
	loc := location(t, w)
	if base := loc.Scheme + "://" + loc.Host + loc.Path; base != testMiniAppURL {
		t.Errorf("редирект на %s, want %s", base, testMiniAppURL)
	}
	if got := loc.Query().Get("startapp"); got != wantResult {
		t.Errorf("startapp = %q, want %q", got, wantResult)
	}
}

func TestVKIDLogin(t *testing.T) {
	r := newTestRouter(t, Deps{})

	req := httptest.NewRequest(http.MethodPost, "/api/vkid/login", nil)
	req.Header.Set("Authorization", "tma "+testInitData(testBotToken, time.Now()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body struct {
		AuthURL string `json:"auth_url"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	u, err := url.Parse(body.AuthURL)
	if err != nil || u.Host != "id.vk.ru" || u.Query().Get("provider") != "mail_ru" || u.Query().Get("state") == "" {
		t.Errorf("auth_url = %q", body.AuthURL)
	}
}

func TestVKIDLoginRequiresInitData(t *testing.T) {
	r := newTestRouter(t, Deps{})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/vkid/login", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestOldGetLoginRemoved(t *testing.T) {
	r := newTestRouter(t, Deps{})

	if w := get(t, r, "/oauth/login"); w.Code != http.StatusNotFound {
		t.Errorf("GET /oauth/login: status = %d, want 404", w.Code)
	}
}

func TestOAuthCallbackLinksAccount(t *testing.T) {
	users := newFakeUsers()
	vk := newFakeVKID()
	vk.tokens = vkid.Tokens{AccessToken: "at", RefreshToken: "rt", ExpiresIn: time.Hour, Scope: "email"}
	r := newTestRouter(t, Deps{Users: users, VKID: vk})
	callback := "/oauth/callback?code=c&device_id=dev-1&state=" + startLogin(t, r)

	w := get(t, r, callback)

	assertBackToMiniApp(t, w, resultOK)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("нет заголовка Cache-Control: no-store")
	}

	got, ok := users.links[testTelegramID]
	if !ok {
		t.Fatalf("привязка для telegram_id %d не сохранена", testTelegramID)
	}
	if got.VKUserID != 42 || got.Name != "Ivan Ivanov" || got.Email != "ivan@mail.ru" ||
		got.AccessToken != "at" || got.RefreshToken != "rt" || got.DeviceID != "dev-1" || got.ExpiresIn != time.Hour {
		t.Errorf("сохранено %+v", got)
	}

	assertBackToMiniApp(t, get(t, r, callback), resultExpired)
}

func TestOAuthCallbackErrors(t *testing.T) {
	tests := map[string]struct {
		setup      func(*fakeVKID, *fakeUsers)
		query      func(state string) string
		wantResult string
	}{
		"чужой state": {
			query:      func(string) string { return "code=c&device_id=d&state=forged" },
			wantResult: resultExpired,
		},
		"нет device_id": {
			query:      func(s string) string { return "code=c&state=" + s },
			wantResult: resultFailed,
		},
		"ошибка от VK ID в callback": {
			query:      func(string) string { return "error=access_denied&error_description=denied" },
			wantResult: resultDenied,
		},
		"обмен кода не удался": {
			setup:      func(f *fakeVKID, _ *fakeUsers) { f.exchangeErr = &vkid.APIError{Status: 200, Code: "invalid_grant"} },
			query:      func(s string) string { return "code=c&device_id=d&state=" + s },
			wantResult: resultFailed,
		},
		"user_info не удался": {
			setup:      func(f *fakeVKID, _ *fakeUsers) { f.userInfoErr = errors.New("timeout") },
			query:      func(s string) string { return "code=c&device_id=d&state=" + s },
			wantResult: resultFailed,
		},
		"аккаунт VK ID привязан к другому Telegram": {
			setup: func(f *fakeVKID, u *fakeUsers) {
				u.links[7] = storageLink(7, f.user.ID)
			},
			query:      func(s string) string { return "code=c&device_id=d&state=" + s },
			wantResult: resultConflict,
		},
		"база недоступна": {
			setup:      func(_ *fakeVKID, u *fakeUsers) { u.err = errors.New("connection refused") },
			query:      func(s string) string { return "code=c&device_id=d&state=" + s },
			wantResult: resultFailed,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			vk, users := newFakeVKID(), newFakeUsers()
			if tt.setup != nil {
				tt.setup(vk, users)
			}
			r := newTestRouter(t, Deps{VKID: vk, Users: users})

			assertBackToMiniApp(t, get(t, r, "/oauth/callback?"+tt.query(startLogin(t, r))), tt.wantResult)
		})
	}
}

func TestBackToMiniAppKeepsExistingQuery(t *testing.T) {
	h := &handler{}
	h.miniAppURL, _ = url.Parse("https://t.me/test_bot/app?mode=compact")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/oauth/callback", nil)
	h.backToMiniApp(c, resultOK)

	q := location(t, w).Query()
	if q.Get("mode") != "compact" || q.Get("startapp") != resultOK {
		t.Errorf("query = %v, want mode=compact и startapp=%s", q, resultOK)
	}
}
