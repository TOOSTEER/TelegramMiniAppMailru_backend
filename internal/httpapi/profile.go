package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkidsession"
)

type profileResponse struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
}

func (h *handler) vkidProfile(c *gin.Context) {
	ctx := c.Request.Context()

	token, err := h.sessions.AccessToken(ctx, initData(c).User.ID)
	switch {
	case errors.Is(err, vkidsession.ErrNotLinked):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "vkid_not_linked"})
		return
	case errors.Is(err, vkidsession.ErrRelinkRequired):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "vkid_relink_required"})
		return
	case err != nil:
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "vkid_unavailable"})
		return
	}

	u, err := h.vkid.UserInfo(ctx, token)
	if err != nil {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "vkid_unavailable"})
		return
	}

	c.JSON(http.StatusOK, profileResponse{
		UserID:    u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		Avatar:    u.Avatar,
	})
}
