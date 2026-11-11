package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/tgauth"
)

func testContactResponse(token string, authDate time.Time, userID int64, phone string) string {
	contactJSON := `{"phone_number":"` + phone +
		`","first_name":"Иван","last_name":"Хвостов","user_id":` +
		strconv.FormatInt(userID, 10) + `}`
	return tgauth.Sign(url.Values{
		"contact":   {contactJSON},
		"auth_date": {strconv.FormatInt(authDate.Unix(), 10)},
	}, token)
}

func putWithAuth(t *testing.T, h http.Handler, target, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, target, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func postJSONWithAuth(t *testing.T, h http.Handler, target, authorization string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestTelegramProfileRequiresInitData(t *testing.T) {
	r := newTestRouter(t, Deps{})

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/api/telegram/profile"},
		{http.MethodGet, "/api/telegram/profile"},
		{http.MethodPost, "/api/telegram/contact"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", w.Code)
			}
		})
	}
}

func TestTelegramProfileSync(t *testing.T) {
	profiles := newFakeTelegramProfiles()
	r := newTestRouter(t, Deps{TelegramProfiles: profiles})
	auth := "tma " + testInitData(testBotToken, time.Now())

	w := putWithAuth(t, r, "/api/telegram/profile", auth)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var body struct {
		TelegramID      int64  `json:"telegram_id"`
		FirstName       string `json:"first_name"`
		Username        string `json:"username"`
		IsPremium       bool   `json:"is_premium"`
		AllowsWriteToPM bool   `json:"allows_write_to_pm"`
		HasPhone        bool   `json:"has_phone"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TelegramID != 42 || body.FirstName != "Иван" || body.Username != "ivan" {
		t.Errorf("body = %+v", body)
	}
	if body.HasPhone {
		t.Error("has_phone должно быть false до отправки контакта")
	}

	saved, err := profiles.Find(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if saved.FirstName != "Иван" || saved.Username != "ivan" {
		t.Errorf("сохранённый профиль = %+v", saved)
	}
}

func TestTelegramProfileGet(t *testing.T) {
	profiles := newFakeTelegramProfiles()
	r := newTestRouter(t, Deps{TelegramProfiles: profiles})
	auth := "tma " + testInitData(testBotToken, time.Now())

	w := getWithAuth(t, r, "/api/telegram/profile", auth)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"telegram_profile_not_found"`) {
		t.Errorf("без профиля = %d %s, want 404", w.Code, w.Body.String())
	}

	if w := putWithAuth(t, r, "/api/telegram/profile", auth); w.Code != http.StatusOK {
		t.Fatalf("PUT profile: status = %d, body: %s", w.Code, w.Body.String())
	}

	w = getWithAuth(t, r, "/api/telegram/profile", auth)
	if w.Code != http.StatusOK {
		t.Fatalf("GET profile: status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestTelegramContactSuccess(t *testing.T) {
	profiles := newFakeTelegramProfiles()
	r := newTestRouter(t, Deps{TelegramProfiles: profiles})
	auth := "tma " + testInitData(testBotToken, time.Now())

	contactRaw := testContactResponse(testBotToken, time.Now(), 42, "+7 (999) 123-45-67")
	w := postJSONWithAuth(t, r, "/api/telegram/contact", auth, map[string]string{"contact": contactRaw})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var body struct {
		HasPhone    bool   `json:"has_phone"`
		PhoneNumber string `json:"phone_number"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.HasPhone || body.PhoneNumber != "+79991234567" {
		t.Errorf("has_phone = %v, phone_number = %q, want true, +79991234567", body.HasPhone, body.PhoneNumber)
	}
}

func TestTelegramContactErrors(t *testing.T) {
	tests := map[string]struct {
		contact string
		want    int
		code    string
	}{
		"чужой user_id":  {testContactResponse(testBotToken, time.Now(), 999, "+7999"), http.StatusForbidden, "contact_mismatch"},
		"битая подпись":  {testContactResponse("654321:OTHER", time.Now(), 42, "+7999"), http.StatusBadRequest, "contact_invalid"},
		"просрочка":      {testContactResponse(testBotToken, time.Now().Add(-2*time.Hour), 42, "+7999"), http.StatusBadRequest, "contact_expired"},
		"пустой телефон": {testContactResponse(testBotToken, time.Now(), 42, ""), http.StatusBadRequest, "contact_invalid"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := newTestRouter(t, Deps{})
			auth := "tma " + testInitData(testBotToken, time.Now())
			w := postJSONWithAuth(t, r, "/api/telegram/contact", auth, map[string]string{"contact": tt.contact})
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d, body: %s", w.Code, tt.want, w.Body.String())
			}
			if tt.code != "" && !strings.Contains(w.Body.String(), `"error":"`+tt.code+`"`) {
				t.Errorf("body = %s, want error %s", w.Body.String(), tt.code)
			}
		})
	}
}

var _ = storage.TelegramProfile{}
