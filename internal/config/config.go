package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	InternalToken string
	LogLevel      string
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8004"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://payments_user:payments_pass@localhost:5432/payments_db?sslmode=disable"
	}

	internalToken := os.Getenv("INTERNAL_TOKEN")
	if internalToken == "" {
		internalToken = "internal-service-secret-entradasya-2026"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	return &Config{
		Port:          port,
		DatabaseURL:   dbURL,
		InternalToken: internalToken,
		LogLevel:      logLevel,
	}
}
