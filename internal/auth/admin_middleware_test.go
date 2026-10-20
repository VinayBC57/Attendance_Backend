package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestAdminMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret"

	tests := []struct {
		name       string
		role       string
		withToken  bool
		wantStatus int
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized},
		{name: "employee token", role: "employee", withToken: true, wantStatus: http.StatusForbidden},
		{name: "admin token", role: "admin", withToken: true, wantStatus: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/admin", AdminMiddleware(secret), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodPost, "/admin", nil)
			if test.withToken {
				request.Header.Set("Authorization", "Bearer "+newMiddlewareTestToken(t, secret, test.role))
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, response.Code)
			}
		})
	}
}

func TestUserMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret"

	for _, role := range []string{"employee", "admin"} {
		t.Run(role, func(t *testing.T) {
			router := gin.New()
			router.GET("/me", UserMiddleware(secret), func(c *gin.Context) {
				c.Header("X-Employee-ID", c.GetString("employee_id"))
				c.Header("X-Role", c.GetString("role"))
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/me", nil)
			request.Header.Set("Authorization", "Bearer "+newMiddlewareTestToken(t, secret, role))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
			}
			if response.Header().Get("X-Employee-ID") != "TEST-001" || response.Header().Get("X-Role") != role {
				t.Fatalf("expected middleware to expose authenticated user claims")
			}
		})
	}
}

func newMiddlewareTestToken(t *testing.T, secret, role string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"employee_id": "TEST-001",
		"role":        role,
		"exp":         time.Now().Add(time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return signed
}
