package config

import (
	"testing"
	"time"
)

func TestLoad_RequiresDatabaseAndJWT(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "secret")
	if _, err := Load(); err == nil {
		t.Fatal("expected DATABASE_URL error")
	}

	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected JWT_SECRET error")
	}
}

func TestLoad_DefaultsAndTTL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("JWT_SECRET", "super-secret")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("JWT_ISSUER", "")
	t.Setenv("JWT_TTL", "")
	t.Setenv("BOOTSTRAP_ADMIN_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.JWTIssuer != "openaura" || cfg.JWTTTL != 24*time.Hour {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.JWTSecret != "super-secret" {
		t.Fatalf("secret=%q", cfg.JWTSecret)
	}

	t.Setenv("JWT_TTL", "15m")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("load ttl: %v", err)
	}
	if cfg.JWTTTL != 15*time.Minute {
		t.Fatalf("ttl=%s", cfg.JWTTTL)
	}

	t.Setenv("JWT_TTL", "not-a-duration")
	if _, err := Load(); err == nil {
		t.Fatal("expected JWT_TTL parse error")
	}

	t.Setenv("JWT_TTL", "0s")
	if _, err := Load(); err == nil {
		t.Fatal("expected JWT_TTL positive error")
	}
}
