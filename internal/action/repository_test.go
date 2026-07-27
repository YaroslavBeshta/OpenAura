package action

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

func countActiveActions(t *testing.T, pool *pgxpool.Pool, appID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM actions WHERE app_id = $1 AND deleted_at IS NULL
	`, appID).Scan(&n)
	if err != nil {
		t.Fatalf("count actions: %v", err)
	}
	return n
}

func TestRepository_CreateGetUpdateDelete(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	appID := createApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, appID, CreateInput{
		Name:     "  read  ",
		Metadata: json.RawMessage(`{"level":1}`),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "read" || created.AppID != appID {
		t.Fatalf("unexpected: %+v", created)
	}

	var dbName string
	err = pool.QueryRow(ctx, `
		SELECT name FROM actions WHERE id = $1 AND app_id = $2 AND deleted_at IS NULL
	`, created.ID, appID).Scan(&dbName)
	if err != nil || dbName != "read" {
		t.Fatalf("db row after create: %v name=%q", err, dbName)
	}

	got, err := repo.GetByID(ctx, appID, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("get: %v %+v", err, got)
	}

	name := "write"
	meta := json.RawMessage(`{"level":2}`)
	updated, err := repo.Update(ctx, appID, created.ID, UpdateInput{Name: &name, Metadata: &meta})
	if err != nil || updated.Name != name {
		t.Fatalf("update: %v %+v", err, updated)
	}
	err = pool.QueryRow(ctx, `SELECT name FROM actions WHERE id = $1`, created.ID).Scan(&dbName)
	if err != nil || dbName != "write" {
		t.Fatalf("db after update: %v name=%q", err, dbName)
	}

	if err := repo.SoftDelete(ctx, appID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var deletedAt *time.Time
	err = pool.QueryRow(ctx, `SELECT deleted_at FROM actions WHERE id = $1`, created.ID).Scan(&deletedAt)
	if err != nil || deletedAt == nil {
		t.Fatalf("expected soft-delete in db: %v", err)
	}
	if _, err := repo.GetByID(ctx, appID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
}

func TestRepository_UniqueNameAndPagination(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	appID := createApp(t, pool)
	otherApp := createApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, appID, CreateInput{Name: "read"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Create(ctx, appID, CreateInput{Name: "read"}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("dup: %v", err)
	}
	if _, err := repo.Create(ctx, otherApp, CreateInput{Name: "read"}); err != nil {
		t.Fatalf("other app: %v", err)
	}
	if countActiveActions(t, pool, appID) != 1 || countActiveActions(t, pool, otherApp) != 1 {
		t.Fatal("expected one read action per app in db")
	}

	var deletedID uuid.UUID
	for i := 0; i < 4; i++ {
		a, err := repo.Create(ctx, appID, CreateInput{Name: fmt.Sprintf("a-%d", i)})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		if i == 1 {
			deletedID = a.ID
		}
		time.Sleep(2 * time.Millisecond)
	}
	if err := repo.SoftDelete(ctx, appID, deletedID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	page, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 2, Offset: 0})
	if err != nil || len(page) != 2 {
		t.Fatalf("page: %v len=%d", err, len(page))
	}
	all, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 50})
	if err != nil || len(all) != 4 {
		t.Fatalf("all: %v len=%d", err, len(all))
	}
	if countActiveActions(t, pool, appID) != 4 {
		t.Fatalf("db active count = %d", countActiveActions(t, pool, appID))
	}

	if _, err := repo.Create(ctx, appID, CreateInput{Name: "a-1"}); err != nil {
		t.Fatalf("reuse name: %v", err)
	}
}

func TestRepository_InvalidInput(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	appID := createApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, appID, CreateInput{Name: "  "}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty name: %v", err)
	}
	if countActiveActions(t, pool, appID) != 0 {
		t.Fatal("invalid create should not insert a row")
	}
	a, err := repo.Create(ctx, appID, CreateInput{Name: "ok"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Update(ctx, appID, a.ID, UpdateInput{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty update: %v", err)
	}
}
