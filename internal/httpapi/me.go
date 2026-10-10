package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"telegram_user": initData(c).User})
}
