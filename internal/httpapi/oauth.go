package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/authstate"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

const (
	resultOK      = "vkid_ok"
	resultExpired = "vkid_expired"
	resultDenied  = "vkid_denied"
	resultFailed  = "vkid_failed"
)

func (h *handler) oauthLogin(c *gin.Context) {
	state := vkid.NewState()
	verifier := vkid.NewVerifier()

	if err := h.states.Save(c.Request.Context(), state, verifier); err != nil {
		_ = c.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Redirect(http.StatusFound, h.vkid.AuthURL(state, verifier))
}

func (h *handler) oauthCallback(c *gin.Context) {
	if vkErr := c.Query("error"); vkErr != "" {
		h.log.Warn("VK ID вернул ошибку в callback",
			zap.String("error", vkErr),
			zap.String("description", c.Query("error_description")),
		)
		h.backToMiniApp(c, resultDenied)
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	deviceID := c.Query("device_id")
	if code == "" || state == "" || deviceID == "" {
		_ = c.Error(errors.New("в callback нет code, state или device_id"))
		h.backToMiniApp(c, resultFailed)
		return
	}

	ctx := c.Request.Context()

	verifier, err := h.states.Pop(ctx, state)
	if errors.Is(err, authstate.ErrNotFound) {
		h.backToMiniApp(c, resultExpired)
		return
	}
	if err != nil {
		_ = c.Error(fmt.Errorf("чтение state: %w", err))
		h.backToMiniApp(c, resultFailed)
		return
	}

	tokens, err := h.vkid.Exchange(ctx, code, verifier, deviceID, state)
	if err != nil {
		_ = c.Error(err)
		h.backToMiniApp(c, resultFailed)
		return
	}

	user, err := h.vkid.UserInfo(ctx, tokens.AccessToken)
	if err != nil {
		_ = c.Error(err)
		h.backToMiniApp(c, resultFailed)
		return
	}

	h.log.Info("вход через VK ID выполнен",
		zap.Int64("vk_user_id", user.ID),
		zap.String("name", user.FirstName+" "+user.LastName),
		zap.Bool("has_email", user.Email != ""),
		zap.String("scope", tokens.Scope),
		zap.Duration("access_ttl", tokens.ExpiresIn),
		zap.Bool("has_refresh", tokens.RefreshToken != ""),
	)

	h.backToMiniApp(c, resultOK)
}

func (h *handler) backToMiniApp(c *gin.Context, result string) {
	u := *h.miniAppURL
	q := u.Query()
	q.Set("startapp", result)
	u.RawQuery = q.Encode()

	c.Redirect(http.StatusFound, u.String())
}
