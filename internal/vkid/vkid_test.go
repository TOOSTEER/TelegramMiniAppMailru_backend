package vkid

import (
	"net/url"
	"regexp"
	"testing"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
)

func TestChallengeRFC7636(t *testing.T) {
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const want = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	if got := Challenge(verifier); got != want {
		t.Errorf("Challenge() = %q, want %q", got, want)
	}
}

func TestRandomValuesFormat(t *testing.T) {
	valid := regexp.MustCompile(`^[A-Za-z0-9_-]{43,128}$`)

	for name, v := range map[string]string{"state": NewState(), "verifier": NewVerifier()} {
		if !valid.MatchString(v) {
			t.Errorf("%s = %q: нужно 43–128 символов из [A-Za-z0-9_-]", name, v)
		}
	}
	if a, b := NewState(), NewState(); a == b {
		t.Error("NewState() вернул одинаковые значения")
	}
}

func TestAuthURL(t *testing.T) {
	c := New(config.VKID{
		ClientID:    "12345",
		AuthURL:     "https://id.vk.ru/authorize",
		RedirectURL: "http://localhost/oauth/callback",
		Scopes:      "vkid.personal_info email",
		Provider:    "mail_ru",
	})

	u, err := url.Parse(c.AuthURL("the-state", "the-verifier"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme+"://"+u.Host+u.Path != "https://id.vk.ru/authorize" {
		t.Errorf("base = %s, want https://id.vk.ru/authorize", u.Scheme+"://"+u.Host+u.Path)
	}

	want := map[string]string{
		"response_type":         "code",
		"client_id":             "12345",
		"redirect_uri":          "http://localhost/oauth/callback",
		"state":                 "the-state",
		"code_challenge":        Challenge("the-verifier"),
		"code_challenge_method": "S256",
		"scope":                 "vkid.personal_info email",
		"provider":              "mail_ru",
	}
	q := u.Query()
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	if q.Has("code_verifier") {
		t.Error("code_verifier не должен попадать в URL авторизации")
	}
}
