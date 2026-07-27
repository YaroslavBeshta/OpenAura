package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/store"
)

const APIKeyHeader = "X-API-Key"

// AppKeyLookup resolves an app API key hash to an app id.
type AppKeyLookup interface {
	ResolveAppIDByKeyHash(ctx context.Context, keyHash string) (uuid.UUID, error)
}

// AdminKeyLookup verifies an admin API key hash exists and is active.
type AdminKeyLookup interface {
	AdminKeyExists(ctx context.Context, keyHash string) (bool, error)
}

// ResolveAppKey authenticates app-scoped requests via X-API-Key.
func ResolveAppKey(lookup AppKeyLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := rawAPIKey(r)
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, APIKeyHeader+" header is required")
				return
			}
			appID, err := lookup.ResolveAppIDByKeyHash(r.Context(), HashAPIKey(raw))
			if errors.Is(err, store.ErrNotFound) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid api key")
				return
			}
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithAppID(r.Context(), appID.String())))
		})
	}
}

// ResolveAdminKey authenticates admin requests via X-API-Key.
func ResolveAdminKey(lookup AdminKeyLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := rawAPIKey(r)
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, APIKeyHeader+" header is required")
				return
			}
			ok, err := lookup.AdminKeyExists(r.Context(), HashAPIKey(raw))
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid api key")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithAdmin(r.Context())))
		})
	}
}

func rawAPIKey(r *http.Request) (string, bool) {
	if v := strings.TrimSpace(r.Header.Get(APIKeyHeader)); v != "" {
		return v, true
	}
	// Also accept Authorization: Bearer <key>
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		v := strings.TrimSpace(authz[7:])
		if v != "" {
			return v, true
		}
	}
	return "", false
}

// RequireAppID extracts the authenticated app id or writes 401.
func RequireAppID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw, ok := AppID(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "app authentication required")
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid app authentication")
		return uuid.Nil, false
	}
	return id, true
}
