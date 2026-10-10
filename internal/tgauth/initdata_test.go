package tgauth

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

const botToken = "123456:TEST-TOKEN"

var now = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func signedInitData(authDate time.Time, extra map[string]string) string {
	v := url.Values{
		"query_id":  {"AAH-test"},
		"user":      {`{"id":42,"first_name":"Иван","last_name":"Хвостов","username":"ivan","language_code":"ru"}`},
		"auth_date": {strconv.FormatInt(authDate.Unix(), 10)},
		"signature": {"ed25519-signature-is-part-of-hmac-data"},
	}
	for k, val := range extra {
		v.Set(k, val)
	}
	return Sign(v, botToken)
}

func TestValidate(t *testing.T) {
	raw := signedInitData(now.Add(-time.Hour), map[string]string{"start_param": "vkid_ok"})

	data, err := Validate(raw, botToken, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	want := User{ID: 42, FirstName: "Иван", LastName: "Хвостов", Username: "ivan", LanguageCode: "ru"}
	if data.User != want {
		t.Errorf("User = %+v, want %+v", data.User, want)
	}
	if data.StartParam != "vkid_ok" || data.QueryID != "AAH-test" || !data.AuthDate.Equal(now.Add(-time.Hour)) {
		t.Errorf("InitData = %+v", data)
	}
}

func TestValidateErrors(t *testing.T) {
	valid := signedInitData(now.Add(-time.Hour), nil)

	tests := map[string]struct {
		raw     string
		token   string
		wantErr error
	}{
		"чужой токен бота":   {valid, "654321:OTHER", ErrBadSignature},
		"подменён user":      {strings.Replace(valid, "%22id%22%3A42", "%22id%22%3A43", 1), botToken, ErrBadSignature},
		"удалена signature":  {removeField(valid, "signature"), botToken, ErrBadSignature},
		"нет hash":           {removeField(valid, "hash"), botToken, ErrMissingHash},
		"hash не hex":        {removeField(valid, "hash") + "&hash=zz", botToken, ErrMissingHash},
		"повторный hash":     {valid + "&hash=00", botToken, ErrMalformed},
		"повторный user":     {valid + "&user=%7B%22id%22%3A1%7D", botToken, ErrMalformed},
		"устарел":            {signedInitData(now.Add(-25*time.Hour), nil), botToken, ErrExpired},
		"из будущего":        {signedInitData(now.Add(time.Hour), nil), botToken, ErrExpired},
		"нет user":           {Sign(url.Values{"auth_date": {strconv.FormatInt(now.Unix(), 10)}}, botToken), botToken, ErrMalformed},
		"пустая строка":      {"", botToken, ErrMissingHash},
		"мусор вместо query": {"%zz", botToken, ErrMalformed},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Validate(tt.raw, tt.token, 24*time.Hour, now); !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataCheckStringFormat(t *testing.T) {
	v := url.Values{"b": {"2"}, "a": {"1"}, "hash": {"ignored"}}
	want := hmacSHA256(hmacSHA256([]byte("WebAppData"), []byte(botToken)), []byte("a=1\nb=2"))

	if got := sign(v, botToken); string(got) != string(want) {
		t.Error("data-check-string должен быть отсортированными парами key=value через \\n без hash")
	}
}

func removeField(raw, key string) string {
	v, _ := url.ParseQuery(raw)
	v.Del(key)
	return v.Encode()
}
