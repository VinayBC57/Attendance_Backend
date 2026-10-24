package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	Env                 string
	DBHost              string
	DBPort              string
	DBName              string
	DBUser              string
	DBPassword          string
	JWTSecret           string
	OfficeLatitude      float64
	OfficeLongitude     float64
	AllowedRadiusMeters int
}

var appConfig Config

// Load reads environment values and populates the process-wide application configuration.
func Load() error {
	_ = godotenv.Load()

	appConfig = Config{
		Port:                getEnv("APP_PORT", "8080"),
		Env:                 getEnv("APP_ENV", "development"),
		DBHost:              getEnv("DB_HOST", "localhost"),
		DBPort:              getEnv("DB_PORT", "5432"),
		DBName:              getEnv("DB_NAME", "attendance_db"),
		DBUser:              getEnv("DB_USER", "postgres"),
		DBPassword:          getEnv("DB_PASSWORD", "postgres"),
		JWTSecret:           getEnv("JWT_SECRET", "change-this-super-secret-key"),
		AllowedRadiusMeters: getIntEnv("ALLOWED_RADIUS_METERS", 100),
	}

	lat, err := strconv.ParseFloat(getEnv("OFFICE_LATITUDE", "19.0760"), 64)
	if err != nil {
		lat = 19.0760
	}
	appConfig.OfficeLatitude = lat

	lon, err := strconv.ParseFloat(getEnv("OFFICE_LONGITUDE", "72.8777"), 64)
	if err != nil {
		lon = 72.8777
	}
	appConfig.OfficeLongitude = lon

	return nil
}

// Get returns the currently loaded application configuration.
func Get() Config {
	return appConfig
}

// DatabaseURL builds the PostgreSQL connection string for GORM from config fields.
func (c Config) DatabaseURL() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		c.DBHost,
		c.DBPort,
		c.DBUser,
		c.DBPassword,
		c.DBName,
	)
}

// getEnv reads a string environment variable or returns the supplied fallback.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getIntEnv reads a valid integer environment variable or returns the supplied fallback.
func getIntEnv(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
