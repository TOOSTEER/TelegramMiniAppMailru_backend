package vkid

import (
	"net/url"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
)

type Client struct {
	cfg config.VKID
}

func New(cfg config.VKID) *Client {
	return &Client{cfg: cfg}
}

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
