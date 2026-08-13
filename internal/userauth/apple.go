package userauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidAppleToken   = errors.New("invalid apple identity token")
	ErrAppleNotConfigured  = errors.New("apple sign-in is not configured")
	ErrAppleEmailRequired  = errors.New("email is required for first apple sign-in")
	ErrAppleAudience       = errors.New("apple identity token audience is not allowed")
)

// AppleClaims are the fields OpenAura needs from a verified Apple identity token.
type AppleClaims struct {
	Subject string
	Email   string
}

// AppleVerifier verifies a Sign in with Apple identity token.
type AppleVerifier interface {
	Verify(ctx context.Context, identityToken string) (AppleClaims, error)
}

// StaticAppleVerifier is a test double. Unknown tokens are rejected.
type StaticAppleVerifier struct {
	tokens map[string]AppleClaims
}

func NewStaticAppleVerifier() *StaticAppleVerifier {
	return &StaticAppleVerifier{tokens: make(map[string]AppleClaims)}
}

// Allow registers a raw identity_token string that Verify will accept.
func (s *StaticAppleVerifier) Allow(identityToken string, claims AppleClaims) {
	if s.tokens == nil {
		s.tokens = make(map[string]AppleClaims)
	}
	s.tokens[identityToken] = claims
}

func (s *StaticAppleVerifier) Verify(_ context.Context, identityToken string) (AppleClaims, error) {
	if s == nil {
		return AppleClaims{}, ErrAppleNotConfigured
	}
	claims, ok := s.tokens[identityToken]
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return AppleClaims{}, ErrInvalidAppleToken
	}
	return claims, nil
}

type AppleInput struct {
	IdentityToken string          `json:"identity_token"`
	Email         string          `json:"email,omitempty" example:"ada@example.com"`
	Metadata      json.RawMessage `json:"metadata,omitempty" swaggertype:"object"`
}

func normalizeAppleEmail(tokenEmail, bodyEmail string) (string, error) {
	email := strings.TrimSpace(tokenEmail)
	if email == "" {
		email = strings.TrimSpace(bodyEmail)
	}
	if email == "" {
		return "", ErrAppleEmailRequired
	}
	return email, nil
}

func appleSubject(raw string) (string, error) {
	subject := strings.TrimSpace(raw)
	if subject == "" {
		return "", fmt.Errorf("%w: missing sub", ErrInvalidAppleToken)
	}
	return subject, nil
}
