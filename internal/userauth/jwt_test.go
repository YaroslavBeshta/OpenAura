package userauth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/user"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := hashPassword("long-enough-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "" || hash == "long-enough-password" {
		t.Fatalf("unexpected hash %q", hash)
	}
	if err := checkPassword(hash, "long-enough-password"); err != nil {
		t.Fatalf("check good: %v", err)
	}
	if err := checkPassword(hash, "wrong-password"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("check bad: %v", err)
	}
	if err := checkPassword("", "long-enough-password"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("empty hash: %v", err)
	}
}

func TestHashPassword_TooShort(t *testing.T) {
	if _, err := hashPassword("short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("got %v, want ErrPasswordTooShort", err)
	}
}

func TestIssueAndParseToken(t *testing.T) {
	cfg := TokenConfig{
		Secret: []byte("test-secret-at-least-32-bytes-long!!"),
		Issuer: "openaura-test",
		TTL:    time.Hour,
	}
	u := user.User{
		ID:    uuid.MustParse("01912345-6789-7abc-def0-123456789abc"),
		AppID: uuid.MustParse("01912345-6789-7abc-def0-123456789abd"),
		Email: "ada@example.com",
	}

	raw, expiresIn, err := issueToken(cfg, u)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if expiresIn != 3600 {
		t.Fatalf("expires_in=%d", expiresIn)
	}
	if raw == "" || strings.Count(raw, ".") != 2 {
		t.Fatalf("unexpected token format: %q", raw)
	}

	claims, err := ParseToken(cfg, raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != u.ID.String() {
		t.Fatalf("sub=%q", claims.Subject)
	}
	if claims.AppID != u.AppID.String() {
		t.Fatalf("app_id=%q", claims.AppID)
	}
	if claims.Email != u.Email {
		t.Fatalf("email=%q", claims.Email)
	}
	if claims.Issuer != cfg.Issuer {
		t.Fatalf("iss=%q", claims.Issuer)
	}
}

func TestParseToken_RejectsBadTokens(t *testing.T) {
	cfg := TokenConfig{
		Secret: []byte("test-secret-at-least-32-bytes-long!!"),
		Issuer: "openaura-test",
		TTL:    time.Hour,
	}
	u := user.User{
		ID:    uuid.MustParse("01912345-6789-7abc-def0-123456789abc"),
		AppID: uuid.MustParse("01912345-6789-7abc-def0-123456789abd"),
		Email: "ada@example.com",
	}
	raw, _, err := issueToken(cfg, u)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	wrongSecret := cfg
	wrongSecret.Secret = []byte("other-secret-at-least-32-bytes-long!")
	if _, err := ParseToken(wrongSecret, raw); err == nil {
		t.Fatal("expected wrong secret to fail")
	}

	wrongIssuer := cfg
	wrongIssuer.Issuer = "other"
	if _, err := ParseToken(wrongIssuer, raw); err == nil {
		t.Fatal("expected wrong issuer to fail")
	}

	expired, _, err := issueToken(TokenConfig{Secret: cfg.Secret, Issuer: cfg.Issuer, TTL: time.Millisecond}, u)
	if err != nil {
		t.Fatalf("issue short: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := ParseToken(cfg, expired); err == nil {
		t.Fatal("expected expired token to fail")
	}

	if _, err := ParseToken(cfg, "not-a-jwt"); err == nil {
		t.Fatal("expected garbage token to fail")
	}
}

func TestIssueToken_RequiresSecretAndTTL(t *testing.T) {
	u := user.User{ID: uuid.New(), AppID: uuid.New(), Email: "a@b.co"}
	if _, _, err := issueToken(TokenConfig{TTL: time.Hour}, u); err == nil {
		t.Fatal("expected missing secret error")
	}
	if _, _, err := issueToken(TokenConfig{Secret: []byte("x"), TTL: 0}, u); err == nil {
		t.Fatal("expected invalid ttl error")
	}
}
