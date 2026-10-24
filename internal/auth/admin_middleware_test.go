package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// TestAdminMiddleware verifies missing, employee, and admin authorization outcomes.
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

// TestUserMiddleware verifies that valid employee and admin claims reach protected handlers.
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

// TestAdminMiddlewareRejectsMissingEmployeeID verifies that admin claims require employee identity.
func TestAdminMiddlewareRejectsMissingEmployeeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": "admin",
		"exp":  time.Now().Add(time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.GET("/admin", AdminMiddleware(secret), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("Authorization", "Bearer "+signed)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

// TestMiddlewareRejectsExpiredAndInvalidTokens verifies expiration and algorithm enforcement.
func TestMiddlewareRejectsExpiredAndInvalidTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret"

	tests := []struct {
		name  string
		token string
	}{
		{name: "expired", token: newMiddlewareTestTokenAt(t, secret, "employee", time.Now().Add(-time.Minute))},
		{name: "wrong algorithm", token: newMiddlewareTestHMACToken(t, secret, jwt.SigningMethodHS384, "employee", time.Now().Add(time.Minute))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/me", UserMiddleware(secret), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/me", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
			}
		})
	}
}

// newMiddlewareTestToken creates a valid short-lived JWT for middleware tests.
func newMiddlewareTestToken(t *testing.T, secret, role string) string {
	return newMiddlewareTestTokenAt(t, secret, role, time.Now().Add(time.Minute))
}

// newMiddlewareTestTokenAt creates a middleware test JWT with an explicit expiry.
func newMiddlewareTestTokenAt(t *testing.T, secret, role string, expiresAt time.Time) string {
	t.Helper()
	return newMiddlewareTestHMACToken(t, secret, jwt.SigningMethodHS256, role, expiresAt)
}

// newMiddlewareTestHMACToken creates a JWT with the supplied signing method for negative tests.
func newMiddlewareTestHMACToken(t *testing.T, secret string, method jwt.SigningMethod, role string, expiresAt time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(method, jwt.MapClaims{
		"employee_id": "TEST-001",
		"role":        role,
		"exp":         expiresAt.Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return signed
}
