package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/testutil"
)

func createApp(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	a, err := app.NewRepository(pool).Create(context.Background(), app.CreateInput{
		Name: fmt.Sprintf("app-%s", uuid.NewString()[:8]),
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	return a.ID
}

func countActiveResources(t *testing.T, pool *pgxpool.Pool, appID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM resources WHERE app_id = $1 AND deleted_at IS NULL
	`, appID).Scan(&n)
	if err != nil {
		t.Fatalf("count resources: %v", err)
	}
	return n
}

func TestRepository_CreateGetUpdateDelete(t *testing.T) {
	pool := testutil.Pool(t)
	appID := createApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, appID, CreateInput{
		Name:     "  documents  ",
		Metadata: json.RawMessage(`{"kind":"file"}`),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "documents" || created.AppID != appID {
		t.Fatalf("unexpected: %+v", created)
	}

	var dbName string
	var dbAppID uuid.UUID
	err = pool.QueryRow(ctx, `
		SELECT name, app_id FROM resources WHERE id = $1 AND deleted_at IS NULL
	`, created.ID).Scan(&dbName, &dbAppID)
	if err != nil {
		t.Fatalf("db row missing after create: %v", err)
	}
	if dbName != "documents" || dbAppID != appID {
		t.Fatalf("db row = name=%q app=%s", dbName, dbAppID)
	}
	if countActiveResources(t, pool, appID) != 1 {
		t.Fatal("expected 1 active resource row")
	}

	got, err := repo.GetByID(ctx, appID, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("get: %v %+v", err, got)
	}

	name := "files"
	meta := json.RawMessage(`{"kind":"blob"}`)
	updated, err := repo.Update(ctx, appID, created.ID, UpdateInput{Name: &name, Metadata: &meta})
	if err != nil || updated.Name != name {
		t.Fatalf("update: %v %+v", err, updated)
	}
	err = pool.QueryRow(ctx, `SELECT name FROM resources WHERE id = $1`, created.ID).Scan(&dbName)
	if err != nil || dbName != "files" {
		t.Fatalf("db after update: %v name=%q", err, dbName)
	}

	if err := repo.SoftDelete(ctx, appID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var deletedAt *time.Time
	err = pool.QueryRow(ctx, `SELECT deleted_at FROM resources WHERE id = $1`, created.ID).Scan(&deletedAt)
	if err != nil || deletedAt == nil {
		t.Fatalf("expected soft-delete in db: %v deleted_at=%v", err, deletedAt)
	}
	if _, err := repo.GetByID(ctx, appID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
	if countActiveResources(t, pool, appID) != 0 {
		t.Fatal("expected 0 active resource rows")
	}
}

func TestRepository_UniqueNameAndPagination(t *testing.T) {
	pool := testutil.Pool(t)
	appID := createApp(t, pool)
	otherApp := createApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, appID, CreateInput{Name: "docs"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Create(ctx, appID, CreateInput{Name: "docs"}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("dup: %v", err)
	}
	if _, err := repo.Create(ctx, otherApp, CreateInput{Name: "docs"}); err != nil {
		t.Fatalf("other app: %v", err)
	}
	if countActiveResources(t, pool, appID) != 1 || countActiveResources(t, pool, otherApp) != 1 {
		t.Fatal("expected one docs resource per app in db")
	}

	var deletedID uuid.UUID
	for i := 0; i < 4; i++ {
		res, err := repo.Create(ctx, appID, CreateInput{Name: fmt.Sprintf("r-%d", i)})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		if i == 1 {
			deletedID = res.ID
		}
		time.Sleep(2 * time.Millisecond)
	}
	if err := repo.SoftDelete(ctx, appID, deletedID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if countActiveResources(t, pool, appID) != 4 { // docs + 3 remaining
		t.Fatalf("active count = %d", countActiveResources(t, pool, appID))
	}

	page, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 2, Offset: 0})
	if err != nil || len(page) != 2 {
		t.Fatalf("page: %v len=%d", err, len(page))
	}
	all, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 50})
	if err != nil || len(all) != 4 {
		t.Fatalf("all: %v len=%d", err, len(all))
	}

	reused, err := repo.Create(ctx, appID, CreateInput{Name: "r-1"})
	if err != nil {
		t.Fatalf("reuse name: %v", err)
	}
	var n int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM resources WHERE app_id = $1 AND name = 'r-1'
	`, appID).Scan(&n)
	if err != nil || n != 2 {
		t.Fatalf("expected soft-deleted + active r-1 rows, n=%d err=%v", n, err)
	}
	if reused.ID == deletedID {
		t.Fatal("reused name should create a new row id")
	}
}

func TestRepository_InvalidInput(t *testing.T) {
	pool := testutil.Pool(t)
	appID := createApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, appID, CreateInput{Name: "  "}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty name: %v", err)
	}
	if countActiveResources(t, pool, appID) != 0 {
		t.Fatal("invalid create should not insert a row")
	}
	res, err := repo.Create(ctx, appID, CreateInput{Name: "ok"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Update(ctx, appID, res.ID, UpdateInput{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty update: %v", err)
	}
}
