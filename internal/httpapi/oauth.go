package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/authstate"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

const (
	resultOK       = "vkid_ok"
	resultExpired  = "vkid_expired"
	resultDenied   = "vkid_denied"
	resultConflict = "vkid_conflict"
	resultFailed   = "vkid_failed"
)

func (h *handler) vkidLogin(c *gin.Context) {
	state := vkid.NewState()
	verifier := vkid.NewVerifier()

	entry := authstate.Entry{Verifier: verifier, TelegramID: initData(c).User.ID}
	if err := h.states.Save(c.Request.Context(), state, entry); err != nil {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	c.PureJSON(http.StatusOK, gin.H{"auth_url": h.vkid.AuthURL(state, verifier)})
}

func (h *handler) vkidLogout(c *gin.Context) {
	ctx := c.Request.Context()
	telegramID := initData(c).User.ID

	accessToken, err := h.users.UnlinkVKID(ctx, telegramID)
	if errors.Is(err, storage.ErrUserNotFound) {
		c.Status(http.StatusNoContent)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	if accessToken != "" {
		if err := h.vkid.Logout(ctx, accessToken); err != nil {
			h.log.Warn("не удалось завершить сессию в VK ID, привязка удалена локально",
				zap.Int64("telegram_id", telegramID),
				zap.Error(err),
			)
		}
	}

	h.log.Info("аккаунт VK ID отвязан", zap.Int64("telegram_id", telegramID))
	c.Status(http.StatusNoContent)
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

	entry, err := h.states.Pop(ctx, state)
	if errors.Is(err, authstate.ErrNotFound) {
		h.backToMiniApp(c, resultExpired)
		return
	}
	if err != nil {
		_ = c.Error(fmt.Errorf("чтение state: %w", err))
		h.backToMiniApp(c, resultFailed)
		return
	}

	tokens, err := h.vkid.Exchange(ctx, code, entry.Verifier, deviceID, state)
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

	err = h.users.LinkVKID(ctx, storage.VKIDLink{
		TelegramID:   entry.TelegramID,
		VKUserID:     user.ID,
		Name:         strings.TrimSpace(user.FirstName + " " + user.LastName),
		Email:        user.Email,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		DeviceID:     deviceID,
		Scope:        tokens.Scope,
		ExpiresIn:    tokens.ExpiresIn,
	})
	if errors.Is(err, storage.ErrVKIDAlreadyLinked) {
		h.log.Warn("аккаунт VK ID уже привязан к другому пользователю Telegram",
			zap.Int64("telegram_id", entry.TelegramID),
			zap.Int64("vk_user_id", user.ID),
		)
		h.backToMiniApp(c, resultConflict)
		return
	}
	if err != nil {
		_ = c.Error(err)
		h.backToMiniApp(c, resultFailed)
		return
	}

	h.log.Info("аккаунт VK ID привязан",
		zap.Int64("telegram_id", entry.TelegramID),
		zap.Int64("vk_user_id", user.ID),
		zap.Bool("has_email", user.Email != ""),
		zap.String("scope", tokens.Scope),
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
