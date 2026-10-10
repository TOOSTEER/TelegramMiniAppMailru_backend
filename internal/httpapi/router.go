package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/health", health)

	return r
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
