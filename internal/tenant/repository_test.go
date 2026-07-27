package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/testutil"
)

func TestRepository_CreateGetUpdateDelete(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	appID := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, appID, CreateInput{
		Metadata: json.RawMessage(`{"name":"acme"}`),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected id")
	}
	assertJSONEqual(t, created.Metadata, `{"name":"acme"}`)

	got, err := repo.GetByID(ctx, appID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("get id mismatch")
	}

	meta := json.RawMessage(`{"name":"acme","plan":"pro"}`)
	updated, err := repo.Update(ctx, appID, created.ID, UpdateInput{Metadata: &meta})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	assertJSONEqual(t, updated.Metadata, `{"name":"acme","plan":"pro"}`)

	if err := repo.SoftDelete(ctx, appID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, appID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	appID := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	var deletedID uuid.UUID
	for i := 0; i < 5; i++ {
		ten, err := repo.Create(ctx, appID, CreateInput{
			Metadata: json.RawMessage(fmt.Sprintf(`{"n":%d}`, i)),
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if i == 1 {
			deletedID = ten.ID
		}
		time.Sleep(2 * time.Millisecond)
	}
	if err := repo.SoftDelete(ctx, appID, deletedID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	page, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d, want 2", len(page))
	}

	all, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("active = %d, want 4", len(all))
	}
	for _, ten := range all {
		if ten.ID == deletedID {
			t.Fatal("soft-deleted tenant in list")
		}
	}
}

func TestRepository_InvalidUpdate(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	appID := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	ten, err := repo.Create(ctx, appID, CreateInput{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Update(ctx, appID, ten.ID, UpdateInput{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty update: %v", err)
	}
	if _, err := repo.GetByID(ctx, appID, uuid.Must(uuid.NewV7())); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func assertJSONEqual(t *testing.T, raw json.RawMessage, want string) {
	t.Helper()
	var gotObj, wantObj any
	if err := json.Unmarshal(raw, &gotObj); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	gotBytes, _ := json.Marshal(gotObj)
	wantBytes, _ := json.Marshal(wantObj)
	if string(gotBytes) != string(wantBytes) {
		t.Fatalf("json = %s, want %s", gotBytes, wantBytes)
	}
}
