package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/store"
)

func TestHashAndNewAPIKey(t *testing.T) {
	a := HashAPIKey("secret")
	b := HashAPIKey("secret")
	c := HashAPIKey("other")
	if a != b || a == c || len(a) != 64 {
		t.Fatalf("hash behavior unexpected: %s %s %s", a, b, c)
	}

	key, err := NewAPIKey("oa_app")
	if err != nil {
		t.Fatalf("new key: %v", err)
	}
	if !strings.HasPrefix(key, "oa_app_") || len(key) < 20 {
		t.Fatalf("unexpected key format: %s", key)
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	if _, ok := AppID(ctx); ok {
		t.Fatal("expected no app id")
	}
	if IsAdmin(ctx) {
		t.Fatal("expected not admin")
	}

	id := uuid.Must(uuid.NewV7()).String()
	ctx = WithAppID(ctx, id)
	got, ok := AppID(ctx)
	if !ok || got != id {
		t.Fatalf("app id = %q ok=%v", got, ok)
	}

	ctx = WithAdmin(context.Background())
	if !IsAdmin(ctx) {
		t.Fatal("expected admin")
	}
}

type stubAppLookup struct {
	appID uuid.UUID
	err   error
}

func (s stubAppLookup) ResolveAppIDByKeyHash(context.Context, string) (uuid.UUID, error) {
	return s.appID, s.err
}

type stubAdminLookup struct {
	ok  bool
	err error
}

func (s stubAdminLookup) AdminKeyExists(context.Context, string) (bool, error) {
	return s.ok, s.err
}

func TestResolveAppKey(t *testing.T) {
	appID := uuid.Must(uuid.NewV7())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := RequireAppID(w, r)
		if !ok {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"app_id": got.String()})
	})
	h := ResolveAppKey(stubAppLookup{appID: appID})(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing key status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "bad")
	rec = httptest.NewRecorder()
	ResolveAppKey(stubAppLookup{err: store.ErrNotFound})(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid key status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer good-key")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid bearer status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), appID.String()) {
		t.Fatalf("body missing app id: %s", rec.Body.String())
	}
}

func TestResolveAdminKey(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdmin(r.Context()) {
			t.Fatal("expected admin context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := ResolveAdminKey(stubAdminLookup{ok: true})(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing key status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "admin")
	rec = httptest.NewRecorder()
	ResolveAdminKey(stubAdminLookup{ok: false})(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid admin status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "admin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("valid admin status=%d", rec.Code)
	}
}

func TestRequireAppIDWithoutContext(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := RequireAppID(rec, req); ok || rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d ok=%v", rec.Code, ok)
	}
}
