package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AdminMiddleware validates a bearer JWT and requires an authenticated admin role.
func AdminMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := parseBearerToken(c, secret)
		if !ok {
			return
		}

		role, ok := claims["role"].(string)
		if !ok || role != "admin" {
			writeAuthError(c, http.StatusForbidden, "FORBIDDEN", "Admin access is required.")
			return
		}
		employeeID, ok := claims["employee_id"].(string)
		if !ok || strings.TrimSpace(employeeID) == "" {
			writeAuthError(c, http.StatusUnauthorized, "INVALID_TOKEN", "The bearer token is missing required user claims.")
			return
		}

		c.Set("employee_id", employeeID)
		c.Set("role", role)
		c.Next()
	}
}

// UserMiddleware validates a bearer JWT and requires supported user role claims.
func UserMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := parseBearerToken(c, secret)
		if !ok {
			return
		}

		employeeID, hasEmployeeID := claims["employee_id"].(string)
		role, hasRole := claims["role"].(string)
		if !hasEmployeeID || employeeID == "" || !hasRole || (role != "admin" && role != "employee") {
			writeAuthError(c, http.StatusUnauthorized, "INVALID_TOKEN", "The bearer token is missing required user claims.")
			return
		}

		c.Set("employee_id", employeeID)
		c.Set("role", role)
		c.Next()
	}
}

// parseBearerToken verifies the Authorization header and signed HS256 JWT claims.
func parseBearerToken(c *gin.Context, secret string) (jwt.MapClaims, bool) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) == "" {
		writeAuthError(c, http.StatusUnauthorized, "UNAUTHORIZED", "A valid bearer token is required.")
		return nil, false
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(
		strings.TrimPrefix(header, "Bearer "),
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || token == nil || !token.Valid {
		writeAuthError(c, http.StatusUnauthorized, "UNAUTHORIZED", "The bearer token is invalid or expired.")
		return nil, false
	}

	return claims, true
}

// writeAuthError writes a consistent JSON authentication or authorization failure.
func writeAuthError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
