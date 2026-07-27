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
	repo := NewRepository(pool)
	ctx := context.Background()

	name := testutil.Name("root")
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

	activeIDs := make(map[uuid.UUID]struct{})
	for i := 0; i < 3; i++ {
		key, err := repo.Create(ctx, CreateInput{})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		activeIDs[key.ID] = struct{}{}
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
	for _, key := range all {
		delete(activeIDs, key.ID)
	}
	if len(activeIDs) != 0 {
		t.Fatalf("created keys missing from list: %v", activeIDs)
	}
}

func TestRepository_EnsureBootstrapKey(t *testing.T) {
	pool := testutil.Pool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	raw := "oa_admin_bootstrap_" + testutil.Unique()
	name := testutil.Name("bootstrap")
	if err := repo.EnsureBootstrapKey(ctx, raw, name); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if err := repo.EnsureBootstrapKey(ctx, raw, name); err != nil {
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
	found := false
	for _, key := range all {
		if key.Name != nil && *key.Name == name {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("bootstrap key missing from list")
	}
}
