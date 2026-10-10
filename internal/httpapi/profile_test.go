package httpapi

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkidsession"
)

func TestVKIDProfile(t *testing.T) {
	r := newTestRouter(t, Deps{Sessions: &fakeSessions{token: "at"}})

	w := getWithAuth(t, r, "/api/vkid/profile", "tma "+testInitData(testBotToken, time.Now()))

	want := `{"user_id":42,"first_name":"Ivan","last_name":"Ivanov","email":"ivan@mail.ru"}`
	if w.Code != http.StatusOK || w.Body.String() != want {
		t.Errorf("ответ = %d %s, want 200 %s", w.Code, w.Body.String(), want)
	}
}

func TestVKIDProfileErrors(t *testing.T) {
	tests := map[string]struct {
		sessions    *fakeSessions
		userInfoErr error
		wantCode    int
		wantBody    string
	}{
		"не привязан":          {&fakeSessions{}, nil, http.StatusNotFound, `{"error":"vkid_not_linked"}`},
		"нужна новая привязка": {&fakeSessions{err: vkidsession.ErrRelinkRequired}, nil, http.StatusConflict, `{"error":"vkid_relink_required"}`},
		"не удалось обновить":  {&fakeSessions{err: errors.New("timeout")}, nil, http.StatusBadGateway, `{"error":"vkid_unavailable"}`},
		"user_info не удался":  {&fakeSessions{token: "at"}, errors.New("timeout"), http.StatusBadGateway, `{"error":"vkid_unavailable"}`},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			vk := newFakeVKID()
			vk.userInfoErr = tt.userInfoErr
			r := newTestRouter(t, Deps{Sessions: tt.sessions, VKID: vk})

			w := getWithAuth(t, r, "/api/vkid/profile", "tma "+testInitData(testBotToken, time.Now()))

			if w.Code != tt.wantCode || w.Body.String() != tt.wantBody {
				t.Errorf("ответ = %d %s, want %d %s", w.Code, w.Body.String(), tt.wantCode, tt.wantBody)
			}
		})
	}
}
