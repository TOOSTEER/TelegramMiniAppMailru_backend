package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(log *zap.Logger) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil)

	r.Use(requestLogger(log), recovery(log))

	r.GET("/health", health)

	return r
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
