package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func requestLogger(log *zap.Logger) gin.HandlerFunc {
	log = log.WithOptions(zap.AddStacktrace(zapcore.FatalLevel))

	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		status := c.Writer.Status()
		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()),
			zap.Duration("latency", time.Since(start)),
		}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		switch {
		case status >= http.StatusInternalServerError:
			log.Error("HTTP-запрос", fields...)
		case status >= http.StatusBadRequest:
			log.Warn("HTTP-запрос", fields...)
		default:
			log.Info("HTTP-запрос", fields...)
		}
	}
}

func recovery(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, rec any) {
		log.Error("panic в обработчике",
			zap.Any("panic", rec),
			zap.String("path", c.Request.URL.Path),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
