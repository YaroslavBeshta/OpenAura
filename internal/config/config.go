package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	BootstrapAdminAPIKey string
	JWTSecret            string
	JWTIssuer            string
	JWTTTL               time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:             envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		BootstrapAdminAPIKey: os.Getenv("BOOTSTRAP_ADMIN_API_KEY"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		JWTIssuer:            envOr("JWT_ISSUER", "openaura"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	ttlRaw := envOr("JWT_TTL", "24h")
	ttl, err := time.ParseDuration(ttlRaw)
	if err != nil {
		return Config{}, fmt.Errorf("JWT_TTL: %w", err)
	}
	if ttl <= 0 {
		return Config{}, fmt.Errorf("JWT_TTL must be positive")
	}
	cfg.JWTTTL = ttl

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
