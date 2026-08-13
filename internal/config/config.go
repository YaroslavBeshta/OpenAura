package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	BootstrapAdminAPIKey string
	JWTSecret            string
	JWTIssuer            string
	JWTTTL               time.Duration
	AppleClientIDs       []string
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
	cfg.AppleClientIDs = splitCSV(os.Getenv("APPLE_CLIENT_IDS"))

	return cfg, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
