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

func TestHashAPIKey_EmptyAndUnicode(t *testing.T) {
	empty := HashAPIKey("")
	if len(empty) != 64 {
		t.Fatalf("empty hash len=%d", len(empty))
	}
	uni := HashAPIKey("🔑")
	if uni == empty || len(uni) != 64 {
		t.Fatalf("unicode hash unexpected")
	}
}

func TestNewAPIKey_Uniqueness(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 50; i++ {
		k, err := NewAPIKey("oa_app")
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		if !strings.HasPrefix(k, "oa_app_") {
			t.Fatalf("prefix: %s", k)
		}
		if _, ok := seen[k]; ok {
			t.Fatalf("duplicate key generated")
		}
		seen[k] = struct{}{}
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

func TestResolveAppKey_HeaderEdgeCases(t *testing.T) {
	appID := uuid.Must(uuid.NewV7())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := RequireAppID(w, r); !ok {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := ResolveAppKey(stubAppLookup{appID: appID})(next)

	cases := []struct {
		name   string
		header func(*http.Request)
		want   int
	}{
		{"whitespace_x_api_key", func(r *http.Request) { r.Header.Set(APIKeyHeader, "   ") }, http.StatusUnauthorized},
		{"empty_bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") }, http.StatusUnauthorized},
		{"bearer_only_spaces", func(r *http.Request) { r.Header.Set("Authorization", "Bearer    ") }, http.StatusUnauthorized},
		{"basic_auth_ignored", func(r *http.Request) { r.Header.Set("Authorization", "Basic dXNlcjpwYXNz") }, http.StatusUnauthorized},
		{"bearer_case_insensitive", func(r *http.Request) { r.Header.Set("Authorization", "BEARER good") }, http.StatusNoContent},
		{"bearer_mixed_case", func(r *http.Request) { r.Header.Set("Authorization", "bEaReR good") }, http.StatusNoContent},
		{"x_api_key_trimmed", func(r *http.Request) { r.Header.Set(APIKeyHeader, "  good  ") }, http.StatusNoContent},
		{"x_api_key_wins_over_bearer", func(r *http.Request) {
			r.Header.Set(APIKeyHeader, "good")
			r.Header.Set("Authorization", "Bearer ignored")
		}, http.StatusNoContent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.header(req)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestResolveAppKey_LookupErrors(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "k")
	rec := httptest.NewRecorder()
	ResolveAppKey(stubAppLookup{err: store.ErrNotFound})(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("not found status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	ResolveAppKey(stubAppLookup{err: context.DeadlineExceeded})(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("internal status=%d", rec.Code)
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

func TestResolveAdminKey_LookupErrors(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "k")

	rec := httptest.NewRecorder()
	ResolveAdminKey(stubAdminLookup{err: context.Canceled})(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("internal status=%d", rec.Code)
	}
}

func TestRequireAppIDWithoutContext(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := RequireAppID(rec, req); ok || rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d ok=%v", rec.Code, ok)
	}
}

func TestRequireAppID_InvalidContextValue(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithAppID(req.Context(), "not-a-uuid"))
	if _, ok := RequireAppID(rec, req); ok || rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d ok=%v", rec.Code, ok)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithAppID(req.Context(), ""))
	if _, ok := RequireAppID(rec, req); ok || rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty app id status=%d ok=%v", rec.Code, ok)
	}
}
