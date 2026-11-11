package httpapi

import (
	"context"
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

func TestMeContainsTelegramProfile(t *testing.T) {
	profiles := newFakeTelegramProfiles()
	r := newTestRouter(t, Deps{TelegramProfiles: profiles})
	auth := "tma " + testInitData(testBotToken, time.Now())

	w := getWithAuth(t, r, "/api/me", auth)
	if strings.Contains(w.Body.String(), `"telegram_profile"`) {
		t.Errorf("без профиля body = %s, telegram_profile не должен появляться", w.Body.String())
	}

	if w := putWithAuth(t, r, "/api/telegram/profile", auth); w.Code != http.StatusOK {
		t.Fatalf("PUT profile: status = %d, body: %s", w.Code, w.Body.String())
	}
	contactRaw := testContactResponse(testBotToken, time.Now(), 42, "+79991234567")
	if w := postJSONWithAuth(t, r, "/api/telegram/contact", auth, map[string]string{"contact": contactRaw}); w.Code != http.StatusOK {
		t.Fatalf("POST contact: status = %d, body: %s", w.Code, w.Body.String())
	}

	w = getWithAuth(t, r, "/api/me", auth)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var body struct {
		TelegramProfile *struct {
			TelegramID int64  `json:"telegram_id"`
			FirstName  string `json:"first_name"`
			HasPhone   bool   `json:"has_phone"`
			Phone      string `json:"phone_number"`
		} `json:"telegram_profile"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TelegramProfile == nil {
		t.Fatalf("telegram_profile отсутствует в body: %s", w.Body.String())
	}
	if body.TelegramProfile.TelegramID != 42 || body.TelegramProfile.FirstName != "Иван" {
		t.Errorf("telegram_profile = %+v", body.TelegramProfile)
	}
	if !body.TelegramProfile.HasPhone {
		t.Error("has_phone должен быть true после отправки контакта")
	}
	if body.TelegramProfile.Phone != "" {
		t.Errorf("phone_number не должен отдаваться в /api/me, got %q", body.TelegramProfile.Phone)
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

var _ = context.Background
