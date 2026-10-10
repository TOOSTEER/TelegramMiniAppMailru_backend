package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

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
	return location(t, get(t, r, "/oauth/login")).Query().Get("state")
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

func TestOAuthLoginRedirectsToVKID(t *testing.T) {
	r := newTestRouter(t, Deps{})

	w := get(t, r, "/oauth/login")

	loc := location(t, w)
	if loc.Host != "id.vk.ru" || loc.Query().Get("provider") != "mail_ru" || loc.Query().Get("state") == "" {
		t.Errorf("Location = %s, want id.vk.ru with provider=mail_ru and state", loc)
	}
	if w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Error("нет заголовка Referrer-Policy: no-referrer")
	}
}

func TestOAuthCallbackSuccess(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	r := newTestRouter(t, Deps{Log: zap.New(core)})
	callback := "/oauth/callback?code=c&device_id=d&state=" + startLogin(t, r)

	w := get(t, r, callback)

	assertBackToMiniApp(t, w, resultOK)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("нет заголовка Cache-Control: no-store")
	}
	if logs.FilterMessage("вход через VK ID выполнен").Len() != 1 {
		t.Error("в логе нет записи об успешном входе")
	}

	assertBackToMiniApp(t, get(t, r, callback), resultExpired)
}

func TestOAuthCallbackErrors(t *testing.T) {
	tests := map[string]struct {
		vk         func(*fakeVKID)
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
			vk:         func(f *fakeVKID) { f.exchangeErr = &vkid.APIError{Status: 200, Code: "invalid_grant"} },
			query:      func(s string) string { return "code=c&device_id=d&state=" + s },
			wantResult: resultFailed,
		},
		"user_info не удался": {
			vk:         func(f *fakeVKID) { f.userInfoErr = errors.New("timeout") },
			query:      func(s string) string { return "code=c&device_id=d&state=" + s },
			wantResult: resultFailed,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			vk := newFakeVKID()
			if tt.vk != nil {
				tt.vk(vk)
			}
			r := newTestRouter(t, Deps{VKID: vk})

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
