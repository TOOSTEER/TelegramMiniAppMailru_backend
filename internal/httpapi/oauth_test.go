package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func TestOAuthLoginRedirectsToVKID(t *testing.T) {
	r := newTestRouter(t, Deps{})

	w := get(t, r, "/oauth/login")

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Host != "id.vk.ru" || loc.Query().Get("provider") != "mail_ru" || loc.Query().Get("state") == "" {
		t.Errorf("Location = %s, want id.vk.ru with provider=mail_ru and state", loc)
	}
}

func TestOAuthCallback(t *testing.T) {
	r := newTestRouter(t, Deps{})

	loc, _ := url.Parse(get(t, r, "/oauth/login").Header().Get("Location"))
	state := loc.Query().Get("state")
	callback := "/oauth/callback?code=the-code&device_id=dev-1&state=" + state

	w := get(t, r, callback)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "code: the-code") {
		t.Fatalf("первый callback: %d %q, want 200 с code", w.Code, w.Body.String())
	}

	if w := get(t, r, callback); w.Code != http.StatusBadRequest {
		t.Errorf("повторный callback: status = %d, want 400", w.Code)
	}

	tests := map[string]string{
		"unknown state":     "/oauth/callback?code=c&device_id=d&state=forged",
		"missing device_id": "/oauth/callback?code=c&state=" + state,
		"error from VK ID":  "/oauth/callback?error=access_denied&error_description=denied",
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			if w := get(t, r, target); w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
		})
	}
}
