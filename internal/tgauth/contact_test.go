package tgauth

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func signedContact(authDate time.Time, contactJSON string) string {
	v := url.Values{
		"contact":   {contactJSON},
		"auth_date": {strconv.FormatInt(authDate.Unix(), 10)},
	}
	return Sign(v, botToken)
}

func TestValidateContact(t *testing.T) {
	raw := signedContact(now.Add(-time.Minute),
		`{"phone_number":"+79991234567","first_name":"Иван","last_name":"Хвостов","user_id":42}`)

	c, err := ValidateContact(raw, botToken, time.Hour, now)
	if err != nil {
		t.Fatalf("ValidateContact() error = %v", err)
	}

	want := Contact{PhoneNumber: "+79991234567", FirstName: "Иван", LastName: "Хвостов", UserID: 42}
	if c != want {
		t.Errorf("Contact = %+v, want %+v", c, want)
	}
}

func TestValidateContactErrors(t *testing.T) {
	valid := signedContact(now.Add(-time.Minute), `{"phone_number":"+7999","user_id":42}`)

	tests := map[string]struct {
		raw     string
		token   string
		wantErr error
	}{
		"чужой токен бота": {valid, "654321:OTHER", ErrBadSignature},
		"подменён телефон": {
			strings.Replace(valid, "%2B7999", "%2B7000", 1),
			botToken,
			ErrBadSignature,
		},
		"нет hash":    {removeField(valid, "hash"), botToken, ErrMissingHash},
		"устарел":     {signedContact(now.Add(-2*time.Hour), `{"phone_number":"+7999","user_id":42}`), botToken, ErrExpired},
		"из будущего": {signedContact(now.Add(time.Hour), `{"phone_number":"+7999","user_id":42}`), botToken, ErrExpired},
		"нет contact": {
			Sign(url.Values{"auth_date": {strconv.FormatInt(now.Unix(), 10)}}, botToken),
			botToken,
			ErrMalformed,
		},
		"пустой телефон": {signedContact(now, `{"phone_number":"","user_id":42}`), botToken, ErrMalformed},
		"нет user_id":    {signedContact(now, `{"phone_number":"+7999","user_id":0}`), botToken, ErrMalformed},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateContact(tt.raw, tt.token, time.Hour, now); !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateContact() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
