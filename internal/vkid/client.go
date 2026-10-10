package vkid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
)

const maxResponseSize = 1 << 20

type Client struct {
	cfg  config.VKID
	http *http.Client
}

func New(cfg config.VKID) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.HTTPTimeout}}
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	ExpiresIn    time.Duration
	UserID       int64
	Scope        string
}

type User struct {
	ID        int64
	FirstName string
	LastName  string
	Email     string
	Avatar    string
}

type APIError struct {
	Status      int
	Code        string
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("VK ID: HTTP %d: %s: %s", e.Status, e.Code, e.Description)
}

var ErrStateMismatch = errors.New("VK ID вернул state, не совпадающий с отправленным")

func (c *Client) AuthURL(state, verifier string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.cfg.ClientID},
		"redirect_uri":          {c.cfg.RedirectURL},
		"state":                 {state},
		"code_challenge":        {Challenge(verifier)},
		"code_challenge_method": {"S256"},
		"scope":                 {c.cfg.Scopes},
		"provider":              {c.cfg.Provider},
	}

	return c.cfg.AuthURL + "?" + q.Encode()
}

func (c *Client) Exchange(ctx context.Context, code, verifier, deviceID, state string) (Tokens, error) {
	tokens, err := c.requestTokens(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"client_id":     {c.cfg.ClientID},
		"device_id":     {deviceID},
		"redirect_uri":  {c.cfg.RedirectURL},
		"state":         {state},
	})
	if err != nil {
		return Tokens{}, fmt.Errorf("обмен кода на токены: %w", err)
	}
	return tokens, nil
}

func (c *Client) Refresh(ctx context.Context, refreshToken, deviceID string) (Tokens, error) {
	tokens, err := c.requestTokens(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.cfg.ClientID},
		"device_id":     {deviceID},
		"state":         {NewState()},
	})
	if err != nil {
		return Tokens{}, fmt.Errorf("обновление токенов: %w", err)
	}
	return tokens, nil
}

func (c *Client) requestTokens(ctx context.Context, form url.Values) (Tokens, error) {
	var resp struct {
		AccessToken  string  `json:"access_token"`
		RefreshToken string  `json:"refresh_token"`
		IDToken      string  `json:"id_token"`
		ExpiresIn    int64   `json:"expires_in"`
		UserID       flexInt `json:"user_id"`
		State        string  `json:"state"`
		Scope        string  `json:"scope"`
	}

	if err := c.post(ctx, c.cfg.TokenURL, form, &resp); err != nil {
		return Tokens{}, err
	}
	if resp.State != form.Get("state") {
		return Tokens{}, ErrStateMismatch
	}
	if resp.AccessToken == "" {
		return Tokens{}, errors.New("в ответе VK ID нет access_token")
	}

	return Tokens{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		IDToken:      resp.IDToken,
		ExpiresIn:    time.Duration(resp.ExpiresIn) * time.Second,
		UserID:       int64(resp.UserID),
		Scope:        resp.Scope,
	}, nil
}

func (c *Client) UserInfo(ctx context.Context, accessToken string) (User, error) {
	var resp struct {
		User struct {
			UserID    flexInt `json:"user_id"`
			FirstName string  `json:"first_name"`
			LastName  string  `json:"last_name"`
			Email     string  `json:"email"`
			Avatar    string  `json:"avatar"`
		} `json:"user"`
	}

	err := c.post(ctx, c.cfg.UserInfoURL, url.Values{
		"client_id":    {c.cfg.ClientID},
		"access_token": {accessToken},
	}, &resp)
	if err != nil {
		return User{}, fmt.Errorf("данные пользователя: %w", err)
	}

	u := resp.User
	if u.UserID == 0 {
		return User{}, errors.New("данные пользователя: в ответе VK ID нет user_id")
	}

	return User{
		ID:        int64(u.UserID),
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		Avatar:    u.Avatar,
	}, nil
}

func (c *Client) Logout(ctx context.Context, accessToken string) error {
	var resp struct {
		Response int `json:"response"`
	}

	err := c.post(ctx, c.cfg.LogoutURL, url.Values{
		"client_id":    {c.cfg.ClientID},
		"access_token": {accessToken},
	}, &resp)
	if err != nil {
		return fmt.Errorf("завершение сессии: %w", err)
	}
	if resp.Response != 1 {
		return fmt.Errorf("завершение сессии: VK ID ответил response=%d", resp.Response)
	}
	return nil
}

func (c *Client) post(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("чтение ответа: %w", err)
	}

	var apiErr struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &apiErr)
	if apiErr.Error != "" {
		return &APIError{Status: resp.StatusCode, Code: apiErr.Error, Description: apiErr.Description}
	}
	if resp.StatusCode != http.StatusOK {
		return &APIError{Status: resp.StatusCode, Code: "unexpected_status", Description: http.StatusText(resp.StatusCode)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("разбор ответа: %w", err)
	}
	return nil
}

type flexInt int64

func (n *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("ожидается число, получено %s", b)
	}
	*n = flexInt(v)
	return nil
}
