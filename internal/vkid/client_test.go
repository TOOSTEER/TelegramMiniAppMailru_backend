package vkid

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return New(config.VKID{
		ClientID:    "12345",
		TokenURL:    srv.URL + "/oauth2/auth",
		UserInfoURL: srv.URL + "/oauth2/user_info",
		LogoutURL:   srv.URL + "/oauth2/logout",
		RedirectURL: "http://localhost/oauth/callback",
		HTTPTimeout: time.Second,
	})
}

func TestExchange(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/oauth2/auth" {
			t.Errorf("запрос %s %s, want POST /oauth2/auth", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", ct)
		}

		want := map[string]string{
			"grant_type":    "authorization_code",
			"code":          "the-code",
			"code_verifier": "the-verifier",
			"client_id":     "12345",
			"device_id":     "dev-1",
			"redirect_uri":  "http://localhost/oauth/callback",
			"state":         "the-state",
		}
		for k, v := range want {
			if got := r.PostFormValue(k); got != v {
				t.Errorf("%s = %q, want %q", k, got, v)
			}
		}

		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","id_token":"it",
			"token_type":"Bearer","expires_in":3600,"user_id":1234567890,"state":"the-state","scope":"email"}`))
	})

	tok, err := c.Exchange(context.Background(), "the-code", "the-verifier", "dev-1", "the-state")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}

	want := Tokens{AccessToken: "at", RefreshToken: "rt", IDToken: "it", ExpiresIn: time.Hour, UserID: 1234567890, Scope: "email"}
	if tok != want {
		t.Errorf("Exchange() = %+v, want %+v", tok, want)
	}
}

func TestExchangeErrors(t *testing.T) {
	tests := map[string]struct {
		status  int
		body    string
		wantErr func(error) bool
	}{
		"ошибка VK ID": {http.StatusOK, `{"error":"invalid_grant","error_description":"code expired"}`, func(err error) bool {
			var apiErr *APIError
			return errors.As(err, &apiErr) && apiErr.Code == "invalid_grant"
		}},
		"чужой state": {http.StatusOK, `{"access_token":"at","state":"other"}`, func(err error) bool {
			return errors.Is(err, ErrStateMismatch)
		}},
		"HTTP 500 без JSON": {http.StatusInternalServerError, `oops`, func(err error) bool {
			var apiErr *APIError
			return errors.As(err, &apiErr) && apiErr.Status == http.StatusInternalServerError
		}},
		"нет access_token": {http.StatusOK, `{"state":"the-state"}`, func(err error) bool { return err != nil }},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := c.Exchange(context.Background(), "c", "v", "d", "the-state")
			if !tt.wantErr(err) {
				t.Errorf("Exchange() error = %v", err)
			}
		})
	}
}

func TestUserInfo(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/user_info" {
			t.Errorf("path = %s, want /oauth2/user_info", r.URL.Path)
		}
		if r.PostFormValue("access_token") != "at" || r.PostFormValue("client_id") != "12345" {
			t.Errorf("form = %v", r.PostForm)
		}

		_, _ = w.Write([]byte(`{"user":{"user_id":"1234567890","first_name":"Ivan","last_name":"Ivanov",
			"email":"ivan@mail.ru","avatar":"https://example.com/a.png","sex":2,"verified":false}}`))
	})

	u, err := c.UserInfo(context.Background(), "at")
	if err != nil {
		t.Fatalf("UserInfo() error = %v", err)
	}

	want := User{ID: 1234567890, FirstName: "Ivan", LastName: "Ivanov", Email: "ivan@mail.ru", Avatar: "https://example.com/a.png"}
	if u != want {
		t.Errorf("UserInfo() = %+v, want %+v", u, want)
	}
}

func TestUserInfoInvalidToken(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_token","error_description":"bad token"}`))
	})

	var apiErr *APIError
	if _, err := c.UserInfo(context.Background(), "bad"); !errors.As(err, &apiErr) || apiErr.Code != "invalid_token" {
		t.Errorf("UserInfo() error = %v, want APIError invalid_token", err)
	}
}

func TestLogout(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/logout" || r.PostFormValue("access_token") != "at" || r.PostFormValue("client_id") != "12345" {
			t.Errorf("запрос %s, form = %v", r.URL.Path, r.PostForm)
		}
		_, _ = w.Write([]byte(`{"response":1}`))
	})

	if err := c.Logout(context.Background(), "at"); err != nil {
		t.Errorf("Logout() error = %v", err)
	}
}

func TestLogoutErrors(t *testing.T) {
	for name, body := range map[string]string{
		"invalid_token": `{"error":"invalid_token","error_description":"token expired"}`,
		"response=0":    `{"response":0}`,
	} {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			})
			if err := c.Logout(context.Background(), "at"); err == nil {
				t.Error("Logout() error = nil, want error")
			}
		})
	}
}
