package apikey

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

func TestRepository_CreateGetListRevokeResolve(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	appID := testutil.SeedApp(t, pool)
	otherApp := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	name := "ci"
	created, err := repo.Create(ctx, appID, CreateInput{Name: &name})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Key == "" || created.AppID != appID {
		t.Fatalf("unexpected key: %+v", created)
	}
	raw := created.Key

	got, err := repo.GetByID(ctx, appID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Key != "" {
		t.Fatal("get should not return plaintext key")
	}
	if _, err := repo.GetByID(ctx, otherApp, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-app get: %v", err)
	}

	resolved, err := repo.ResolveAppIDByKeyHash(ctx, auth.HashAPIKey(raw))
	if err != nil || resolved != appID {
		t.Fatalf("resolve: %v id=%s", err, resolved)
	}

	for i := 0; i < 3; i++ {
		if _, err := repo.Create(ctx, appID, CreateInput{}); err != nil {
			t.Fatalf("seed: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	page, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d", len(page))
	}

	if err := repo.Revoke(ctx, appID, created.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := repo.GetByID(ctx, appID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get revoked: %v", err)
	}
	if _, err := repo.ResolveAppIDByKeyHash(ctx, auth.HashAPIKey(raw)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("resolve revoked: %v", err)
	}
	if err := repo.Revoke(ctx, appID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second revoke: %v", err)
	}

	all, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 50})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("active keys = %d, want 3", len(all))
	}
}

func TestRepository_CreateRequiresValidApp(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, uuid.Must(uuid.NewV7()), CreateInput{}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing app: %v", err)
	}
}

func TestRepository_ResolveRequiresActiveApp(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	appID := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, appID, CreateInput{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = pool.Exec(ctx, `UPDATE apps SET deleted_at = now() WHERE id = $1`, appID)
	if err != nil {
		t.Fatalf("soft-delete app: %v", err)
	}
	if _, err := repo.ResolveAppIDByKeyHash(ctx, auth.HashAPIKey(created.Key)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("resolve deleted app: %v", err)
	}
}
