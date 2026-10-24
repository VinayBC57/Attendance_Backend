package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"attendance-backend/internal/models"
)

// HashPassword hashes a plaintext password using bcrypt for secure database storage.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPasswordHash reports whether a plaintext password matches a stored bcrypt hash.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePassword enforces the application's minimum password strength policy.
func ValidatePassword(password string) bool {
	if len(password) < 7 {
		return false
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return false
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return false
	}
	if !regexp.MustCompile(`[^A-Za-z0-9]`).MatchString(password) {
		return false
	}
	return true
}

// GenerateTemporaryPassword creates a random password that satisfies the password policy.
func GenerateTemporaryPassword() (string, error) {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
	password := make([]byte, 12)
	for i := range password {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		password[i] = letters[num.Int64()]
	}

	candidate := string(password)
	if !ValidatePassword(candidate) {
		return GenerateTemporaryPassword()
	}
	return candidate, nil
}

// CreateToken creates a signed HS256 JWT containing the user's identity and role.
func CreateToken(user models.User, secret string) (string, error) {
	claims := jwt.MapClaims{
		"employee_id": user.EmployeeID,
		"role":        user.Role,
		"exp":         time.Now().Add(24 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GetJWTSecret returns the configured JWT secret or the development fallback.
func GetJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "change-this-super-secret-key"
	}
	return secret
}

// GenerateEmployeeID creates the next sequential employee identifier for a count.
func GenerateEmployeeID(count int) string {
	return fmt.Sprintf("EMP-%03d", count+1)
}
