package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/authstate"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
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
		c.String(http.StatusBadRequest, "VK ID вернул ошибку: %s (%s)", vkErr, c.Query("error_description"))
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	deviceID := c.Query("device_id")
	if code == "" || state == "" || deviceID == "" {
		c.String(http.StatusBadRequest, "в ответе VK ID нет code, state или device_id")
		return
	}

	verifier, err := h.states.Pop(c.Request.Context(), state)
	if errors.Is(err, authstate.ErrNotFound) {
		c.String(http.StatusBadRequest, "неизвестный или просроченный state, начните вход заново")
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "code: %s\nstate: %s\ndevice_id: %s\ncode_verifier найден: %d символов\n",
		code, state, deviceID, len(verifier))
}
