package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(zap.NewNop())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if want := `{"status":"ok"}`; w.Body.String() != want {
		t.Fatalf("body = %s, want %s", w.Body.String(), want)
	}
}
