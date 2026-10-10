package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/tgauth"
)

const initDataKey = "tgauth.initData"

func telegramAuth(botToken string, maxAge time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := strings.CutPrefix(c.GetHeader("Authorization"), "tma ")
		if !ok || raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "init_data_missing"})
			return
		}

		data, err := tgauth.Validate(raw, botToken, maxAge, time.Now())
		if err != nil {
			_ = c.Error(err)
			code := "init_data_invalid"
			if errors.Is(err, tgauth.ErrExpired) {
				code = "init_data_expired"
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": code})
			return
		}

		c.Set(initDataKey, data)
		c.Next()
	}
}

func initData(c *gin.Context) tgauth.InitData {
	return c.MustGet(initDataKey).(tgauth.InitData)
}
