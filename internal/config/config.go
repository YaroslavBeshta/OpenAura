package config

import (
	"fmt"
	"os"
)

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	BootstrapAdminAPIKey string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:             envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		BootstrapAdminAPIKey: os.Getenv("BOOTSTRAP_ADMIN_API_KEY"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
