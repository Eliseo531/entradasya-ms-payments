package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInternalAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expectedToken := "test-secret-token"

	setupRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(InternalAuth(expectedToken))
		r.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		return r
	}

	t.Run("should reject request with 403 when X-Internal-Token is missing", func(t *testing.T) {
		r := setupRouter()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("should reject request with 403 when X-Internal-Token is invalid", func(t *testing.T) {
		r := setupRouter()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("X-Internal-Token", "wrong-token")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("should allow request with 200 when X-Internal-Token is valid", func(t *testing.T) {
		r := setupRouter()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("X-Internal-Token", expectedToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})
}
