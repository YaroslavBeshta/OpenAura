package adminapikey

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/auth"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/testutil"
)

func TestRepository_CreateGetListRevoke(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	name := "root"
	created, err := repo.Create(ctx, CreateInput{Name: &name})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Key == "" || created.ID == uuid.Nil {
		t.Fatalf("unexpected key: %+v", created)
	}
	raw := created.Key

	ok, err := repo.AdminKeyExists(ctx, auth.HashAPIKey(raw))
	if err != nil || !ok {
		t.Fatalf("exists: ok=%v err=%v", ok, err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Key != "" {
		t.Fatal("get should not return plaintext key")
	}

	for i := 0; i < 3; i++ {
		if _, err := repo.Create(ctx, CreateInput{}); err != nil {
			t.Fatalf("seed: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	page, err := repo.List(ctx, ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d", len(page))
	}

	if err := repo.Revoke(ctx, created.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	ok, err = repo.AdminKeyExists(ctx, auth.HashAPIKey(raw))
	if err != nil || ok {
		t.Fatalf("exists after revoke: ok=%v err=%v", ok, err)
	}
	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get revoked: %v", err)
	}

	all, err := repo.List(ctx, ListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("active = %d, want 3", len(all))
	}
}

func TestRepository_EnsureBootstrapKey(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	raw := "oa_admin_bootstrap_fixed_key_for_tests"
	if err := repo.EnsureBootstrapKey(ctx, raw, "bootstrap"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if err := repo.EnsureBootstrapKey(ctx, raw, "bootstrap"); err != nil {
		t.Fatalf("ensure idempotent: %v", err)
	}

	ok, err := repo.AdminKeyExists(ctx, auth.HashAPIKey(raw))
	if err != nil || !ok {
		t.Fatalf("exists: ok=%v err=%v", ok, err)
	}

	all, err := repo.List(ctx, ListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected one bootstrap key, got %d", len(all))
	}
}
