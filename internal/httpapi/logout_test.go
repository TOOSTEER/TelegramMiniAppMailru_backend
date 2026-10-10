package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func postWithAuth(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, nil)
	req.Header.Set("Authorization", "tma "+testInitData(testBotToken, time.Now()))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestVKIDLogout(t *testing.T) {
	tests := map[string]struct {
		linked     bool
		freshToken string
		logoutErr  error
		wantLogout []string
	}{
		"привязан, токен обновлён перед отвязкой": {linked: true, freshToken: "fresh-at", wantLogout: []string{"fresh-at"}},
		"привязан": {linked: true, wantLogout: []string{"at-42"}},
		"привязан, VK ID ответил ошибкой": {linked: true, logoutErr: errors.New("invalid_token"), wantLogout: []string{"at-42"}},
		"не привязан":                     {linked: false, wantLogout: nil},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			users, vk := newFakeUsers(), newFakeVKID()
			vk.logoutErr = tt.logoutErr
			if tt.linked {
				l := storageLink(testTelegramID, 1001)
				l.AccessToken = "at-42"
				users.links[testTelegramID] = l
			}
			r := newTestRouter(t, Deps{Users: users, VKID: vk, Sessions: &fakeSessions{token: tt.freshToken}})

			w := postWithAuth(t, r, "/api/vkid/logout")

			if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
				t.Errorf("ответ = %d %q, want 204 без тела", w.Code, w.Body.String())
			}
			if !slices.Equal(vk.loggedOut, tt.wantLogout) {
				t.Errorf("Logout в VK ID вызван с %q, want %q", vk.loggedOut, tt.wantLogout)
			}
			if _, ok := users.links[testTelegramID]; ok {
				t.Error("привязка осталась после отвязки")
			}
		})
	}
}

func TestVKIDLogoutStorageError(t *testing.T) {
	users, vk := newFakeUsers(), newFakeVKID()
	users.err = errors.New("connection refused")
	r := newTestRouter(t, Deps{Users: users, VKID: vk})

	w := postWithAuth(t, r, "/api/vkid/logout")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if len(vk.loggedOut) != 0 {
		t.Error("при ошибке базы не должно быть запроса в VK ID")
	}
}

func TestVKIDLogoutRequiresInitData(t *testing.T) {
	r := newTestRouter(t, Deps{})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/vkid/logout", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
