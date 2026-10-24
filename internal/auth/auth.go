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

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

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

func ParseToken(tokenValue, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenValue, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func GetJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "change-this-super-secret-key"
	}
	return secret
}

func GenerateEmployeeID(count int) string {
	return fmt.Sprintf("EMP-%03d", count+1)
}
