package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type ctxKey int

const (
	appIDKey ctxKey = iota + 1
	adminKey
)

// WithAppID attaches the authenticated app id to the request context.
func WithAppID(ctx context.Context, appID string) context.Context {
	return context.WithValue(ctx, appIDKey, appID)
}

// AppID returns the authenticated app id, if present.
func AppID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(appIDKey).(string)
	return v, ok && v != ""
}

// WithAdmin marks the request as authenticated via an admin API key.
func WithAdmin(ctx context.Context) context.Context {
	return context.WithValue(ctx, adminKey, true)
}

// IsAdmin reports whether the request was authenticated as admin.
func IsAdmin(ctx context.Context) bool {
	v, ok := ctx.Value(adminKey).(bool)
	return ok && v
}

// HashAPIKey returns a hex-encoded SHA-256 digest of the raw key.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// NewAPIKey generates a random API key with the given prefix (e.g. "oa_app", "oa_admin").
func NewAPIKey(prefix string) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(b[:]), nil
}
