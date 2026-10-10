package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.healthTimeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "db": "down"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "up"})
}
