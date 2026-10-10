package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
)

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		pingErr  error
		wantCode int
		wantBody string
	}{
		{"db up", nil, http.StatusOK, `{"db":"up","status":"ok"}`},
		{"db down", errors.New("connection refused"), http.StatusServiceUnavailable, `{"db":"down","status":"unavailable"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRouter(config.HTTP{HealthTimeout: time.Second}, zap.NewNop(), fakePinger{err: tt.pingErr})
			if err != nil {
				t.Fatal(err)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
			if w.Body.String() != tt.wantBody {
				t.Errorf("body = %s, want %s", w.Body.String(), tt.wantBody)
			}
		})
	}
}
