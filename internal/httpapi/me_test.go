package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/tgauth"
)

func testInitData(token string, authDate time.Time) string {
	return tgauth.Sign(url.Values{
		"user":      {`{"id":42,"first_name":"Иван","username":"ivan"}`},
		"auth_date": {strconv.FormatInt(authDate.Unix(), 10)},
	}, token)
}

func getWithAuth(t *testing.T, h http.Handler, target, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestMe(t *testing.T) {
	r := newTestRouter(t, Deps{})

	w := getWithAuth(t, r, "/api/me", "tma "+testInitData(testBotToken, time.Now()))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	var body struct {
		TelegramUser tgauth.User `json:"telegram_user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TelegramUser.ID != 42 || body.TelegramUser.Username != "ivan" {
		t.Errorf("telegram_user = %+v", body.TelegramUser)
	}
}

func TestMeLinked(t *testing.T) {
	users := newFakeUsers()
	users.links[42] = storageLink(42, 1001)
	r := newTestRouter(t, Deps{Users: users})

	w := getWithAuth(t, r, "/api/me", "tma "+testInitData(testBotToken, time.Now()))

	var body struct {
		VKID struct {
			Linked bool   `json:"linked"`
			UserID int64  `json:"user_id"`
			Email  string `json:"email"`
		} `json:"vkid"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.VKID.Linked || body.VKID.UserID != 1001 || body.VKID.Email != "ivan@mail.ru" {
		t.Errorf("vkid = %+v, body: %s", body.VKID, w.Body.String())
	}
}

func TestMeNotLinked(t *testing.T) {
	r := newTestRouter(t, Deps{})

	w := getWithAuth(t, r, "/api/me", "tma "+testInitData(testBotToken, time.Now()))

	if want := `"vkid":{"linked":false}`; !strings.Contains(w.Body.String(), want) {
		t.Errorf("body = %s, want %s", w.Body.String(), want)
	}
}

func TestMeUnauthorized(t *testing.T) {
	tests := map[string]struct {
		authorization string
		wantError     string
	}{
		"нет заголовка":       {"", "init_data_missing"},
		"другая схема":        {"Bearer " + testInitData(testBotToken, time.Now()), "init_data_missing"},
		"чужой токен бота":    {"tma " + testInitData("654321:OTHER", time.Now()), "init_data_invalid"},
		"устаревший initData": {"tma " + testInitData(testBotToken, time.Now().Add(-2*time.Hour)), "init_data_expired"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			r := newTestRouter(t, Deps{Log: zap.New(core)})

			w := getWithAuth(t, r, "/api/me", tt.authorization)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", w.Code)
			}
			if want := `{"error":"` + tt.wantError + `"}`; w.Body.String() != want {
				t.Errorf("body = %s, want %s", w.Body.String(), want)
			}
			if entries := logs.FilterMessage("HTTP-запрос").All(); len(entries) != 1 || entries[0].Level != zapcore.WarnLevel {
				t.Errorf("запрос с 401 должен логироваться один раз на уровне WARN, got %v", entries)
			}
		})
	}
}

func storageLink(telegramID, vkUserID int64) storage.VKIDLink {
	return storage.VKIDLink{TelegramID: telegramID, VKUserID: vkUserID, Name: "Ivan Ivanov", Email: "ivan@mail.ru"}
}
