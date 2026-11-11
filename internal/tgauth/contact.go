package tgauth

import (
	"crypto/hmac"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type Contact struct {
	PhoneNumber string
	FirstName   string
	LastName    string
	UserID      int64
}

func ValidateContact(raw, botToken string, maxAge time.Duration, now time.Time) (Contact, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return Contact{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	for k, v := range values {
		if len(v) > 1 {
			return Contact{}, fmt.Errorf("%w: поле %s повторяется", ErrMalformed, k)
		}
	}

	gotHash, err := hex.DecodeString(values.Get("hash"))
	if err != nil || len(gotHash) == 0 {
		return Contact{}, ErrMissingHash
	}
	if !hmac.Equal(gotHash, sign(values, botToken)) {
		return Contact{}, ErrBadSignature
	}

	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return Contact{}, fmt.Errorf("%w: auth_date", ErrMalformed)
	}
	authDate := time.Unix(authUnix, 0)
	if now.Sub(authDate) > maxAge || authDate.Sub(now) > maxClockSkew {
		return Contact{}, ErrExpired
	}

	var payload struct {
		PhoneNumber string `json:"phone_number"`
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
		UserID      int64  `json:"user_id"`
	}
	if err := json.Unmarshal([]byte(values.Get("contact")), &payload); err != nil {
		return Contact{}, fmt.Errorf("%w: contact", ErrMalformed)
	}
	if payload.PhoneNumber == "" {
		return Contact{}, fmt.Errorf("%w: phone_number пуст", ErrMalformed)
	}
	if payload.UserID == 0 {
		return Contact{}, fmt.Errorf("%w: user_id пуст", ErrMalformed)
	}

	return Contact{
		PhoneNumber: payload.PhoneNumber,
		FirstName:   payload.FirstName,
		LastName:    payload.LastName,
		UserID:      payload.UserID,
	}, nil
}
