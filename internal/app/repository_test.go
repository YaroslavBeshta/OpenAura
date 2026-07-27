package app

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
	repo := NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, CreateInput{
		Name:     "  Acme  ",
		Metadata: json.RawMessage(`{"plan":"pro"}`),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == uuid.Nil || created.Name != "Acme" {
		t.Fatalf("unexpected app: %+v", created)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Fatal("get id mismatch")
	}

	name := "Acme Cloud"
	meta := json.RawMessage(`{"plan":"enterprise"}`)
	updated, err := repo.Update(ctx, created.ID, UpdateInput{Name: &name, Metadata: &meta})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != name {
		t.Fatalf("name = %q", updated.Name)
	}

	if err := repo.SoftDelete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
	if err := repo.SoftDelete(ctx, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	var deletedID uuid.UUID
	for i := 0; i < 5; i++ {
		a, err := repo.Create(ctx, CreateInput{Name: fmt.Sprintf("app-%d", i)})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if i == 2 {
			deletedID = a.ID
		}
		time.Sleep(2 * time.Millisecond)
	}
	if err := repo.SoftDelete(ctx, deletedID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	page, err := repo.List(ctx, ListFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d", len(page))
	}

	all, err := repo.List(ctx, ListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("active = %d, want 4", len(all))
	}
}

func TestRepository_InvalidInput(t *testing.T) {
	pool := testutil.Pool(t)
	testutil.Reset(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, CreateInput{Name: "   "}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty name: %v", err)
	}
	a, err := repo.Create(ctx, CreateInput{Name: "ok"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Update(ctx, a.ID, UpdateInput{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty update: %v", err)
	}
}
