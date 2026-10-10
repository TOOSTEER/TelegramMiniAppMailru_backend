package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/tgauth"
)

type meResponse struct {
	TelegramUser tgauth.User `json:"telegram_user"`
	VKID         vkidStatus  `json:"vkid"`
}

type vkidStatus struct {
	Linked   bool       `json:"linked"`
	UserID   int64      `json:"user_id,omitempty"`
	Name     string     `json:"name,omitempty"`
	Email    string     `json:"email,omitempty"`
	LinkedAt *time.Time `json:"linked_at,omitempty"`
}

func (h *handler) me(c *gin.Context) {
	tgUser := initData(c).User
	resp := meResponse{TelegramUser: tgUser}

	u, err := h.users.FindByTelegramID(c.Request.Context(), tgUser.ID)
	switch {
	case errors.Is(err, storage.ErrUserNotFound):
	case err != nil:
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	default:
		resp.VKID = vkidStatus{Linked: true, UserID: u.VKUserID, Name: u.Name, Email: u.Email, LinkedAt: &u.LinkedAt}
	}

	c.JSON(http.StatusOK, resp)
}
