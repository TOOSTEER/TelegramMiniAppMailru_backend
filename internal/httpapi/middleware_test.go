package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
)

func TestMiddlewareLogsAndRecoversPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	r, err := NewRouter(config.HTTP{HealthTimeout: time.Second}, zap.New(core), fakePinger{})
	if err != nil {
		t.Fatal(err)
	}
	r.GET("/panic", func(c *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic?code=secret", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if n := logs.FilterMessage("panic в обработчике").Len(); n != 1 {
		t.Errorf("panic logged %d times, want 1", n)
	}

	reqLogs := logs.FilterMessage("HTTP-запрос").All()
	if len(reqLogs) != 1 {
		t.Fatalf("request logged %d times, want 1", len(reqLogs))
	}
	fields := reqLogs[0].ContextMap()
	if fields["status"] != int64(http.StatusInternalServerError) {
		t.Errorf("status field = %v, want 500", fields["status"])
	}
	if fields["path"] != "/panic" {
		t.Errorf("path field = %v, want /panic without query", fields["path"])
	}
}
