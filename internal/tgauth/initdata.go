package tgauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxClockSkew = time.Minute

var (
	ErrMissingHash  = errors.New("в initData нет hash")
	ErrBadSignature = errors.New("подпись initData не совпадает")
	ErrExpired      = errors.New("initData устарел")
	ErrMalformed    = errors.New("initData повреждён")
)

type User struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
	IsPremium    bool   `json:"is_premium,omitempty"`
	PhotoURL     string `json:"photo_url,omitempty"`
}

type InitData struct {
	User       User
	AuthDate   time.Time
	StartParam string
	QueryID    string
}

func Validate(raw, botToken string, maxAge time.Duration, now time.Time) (InitData, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return InitData{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	for k, v := range values {
		if len(v) > 1 {
			return InitData{}, fmt.Errorf("%w: поле %s повторяется", ErrMalformed, k)
		}
	}

	gotHash, err := hex.DecodeString(values.Get("hash"))
	if err != nil || len(gotHash) == 0 {
		return InitData{}, ErrMissingHash
	}
	if !hmac.Equal(gotHash, sign(values, botToken)) {
		return InitData{}, ErrBadSignature
	}

	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return InitData{}, fmt.Errorf("%w: auth_date", ErrMalformed)
	}
	authDate := time.Unix(authUnix, 0)
	if now.Sub(authDate) > maxAge || authDate.Sub(now) > maxClockSkew {
		return InitData{}, ErrExpired
	}

	var user User
	if err := json.Unmarshal([]byte(values.Get("user")), &user); err != nil || user.ID == 0 {
		return InitData{}, fmt.Errorf("%w: user", ErrMalformed)
	}

	return InitData{
		User:       user,
		AuthDate:   authDate,
		StartParam: values.Get("start_param"),
		QueryID:    values.Get("query_id"),
	}, nil
}

func Sign(values url.Values, botToken string) string {
	signed := url.Values{}
	for k, v := range values {
		if k != "hash" {
			signed[k] = v
		}
	}
	signed.Set("hash", hex.EncodeToString(sign(signed, botToken)))
	return signed.Encode()
}

func sign(values url.Values, botToken string) []byte {
	keys := make([]string, 0, len(values))
	for k := range values {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	pairs := make([]string, len(keys))
	for i, k := range keys {
		pairs[i] = k + "=" + values.Get(k)
	}

	return hmacSHA256(hmacSHA256([]byte("WebAppData"), []byte(botToken)), []byte(strings.Join(pairs, "\n")))
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
