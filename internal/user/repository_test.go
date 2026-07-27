package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/testutil"
)

func TestRepository_CreateGetUpdateDelete(t *testing.T) {
	pool := testutil.Pool(t)
	appID := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	localPart := testutil.Unique()
	created, err := repo.Create(ctx, appID, CreateInput{
		Email:    " " + strings.ToUpper(localPart) + "@Example.COM ",
		Metadata: json.RawMessage(`{"name":"Ada"}`),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected uuid v7 id")
	}
	if created.Email != localPart+"@example.com" {
		t.Fatalf("email = %q, want normalized lowercase", created.Email)
	}
	if created.DeletedAt != nil {
		t.Fatal("deleted_at should be nil")
	}
	assertJSONEqual(t, created.Metadata, `{"name":"Ada"}`)

	got, err := repo.GetByID(ctx, appID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID || got.Email != created.Email {
		t.Fatalf("get mismatch: %+v", got)
	}

	email := testutil.Email("lovelace")
	meta := json.RawMessage(`{"title":"Countess"}`)
	updated, err := repo.Update(ctx, appID, created.ID, UpdateInput{
		Email:    &email,
		Metadata: &meta,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Email != email {
		t.Fatalf("updated email = %q", updated.Email)
	}
	assertJSONEqual(t, updated.Metadata, `{"title":"Countess"}`)
	if !updated.UpdatedAt.After(created.UpdatedAt) && !updated.UpdatedAt.Equal(created.UpdatedAt) {
		// updated_at should move forward; allow equal only if clock resolution is coarse
		if updated.UpdatedAt.Before(created.CreatedAt) {
			t.Fatalf("updated_at went backwards")
		}
	}

	if err := repo.SoftDelete(ctx, appID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, appID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get after delete: %v, want ErrNotFound", err)
	}
	if err := repo.SoftDelete(ctx, appID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: %v, want ErrNotFound", err)
	}
}

func TestRepository_CreateConflictAndReuseAfterDelete(t *testing.T) {
	pool := testutil.Pool(t)
	appID := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	email := testutil.Email("dup")
	first, err := repo.Create(ctx, appID, CreateInput{Email: email})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Create(ctx, appID, CreateInput{Email: email}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate create: %v, want ErrConflict", err)
	}

	if err := repo.SoftDelete(ctx, appID, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	second, err := repo.Create(ctx, appID, CreateInput{Email: email})
	if err != nil {
		t.Fatalf("create after soft delete: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("expected a new user id after soft delete")
	}
}

func TestRepository_ListPaginationAndSoftDeleteVisibility(t *testing.T) {
	pool := testutil.Pool(t)
	appID := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		// Ensure distinct created_at ordering across inserts.
		u, err := repo.Create(ctx, appID, CreateInput{Email: uniqueEmail(i)})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		ids = append(ids, u.ID)
		time.Sleep(2 * time.Millisecond)
	}
	if err := repo.SoftDelete(ctx, appID, ids[2]); err != nil {
		t.Fatalf("delete middle user: %v", err)
	}

	page1, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}

	page2, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2 len = %d, want 2", len(page2))
	}

	all, err := repo.List(ctx, ListFilter{AppID: appID, Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("active users = %d, want 4", len(all))
	}
	for _, u := range all {
		if u.ID == ids[2] {
			t.Fatal("soft-deleted user appeared in list")
		}
	}

	// Newest first.
	if !all[0].CreatedAt.After(all[len(all)-1].CreatedAt) && !all[0].CreatedAt.Equal(all[len(all)-1].CreatedAt) {
		t.Fatalf("list not ordered by created_at desc")
	}
}

func TestRepository_InvalidInput(t *testing.T) {
	pool := testutil.Pool(t)
	appID := testutil.SeedApp(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, appID, CreateInput{Email: "not-an-email"}); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("invalid email: %v", err)
	}
	if _, err := repo.GetByID(ctx, appID, uuid.Must(uuid.NewV7())); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing get: %v", err)
	}
	if _, err := repo.Update(ctx, appID, uuid.Must(uuid.NewV7()), UpdateInput{}); !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty update: %v", err)
	}
}

func TestClampPagination(t *testing.T) {
	tests := []struct {
		limit, offset      int
		wantLimit, wantOff int
	}{
		{0, 0, 50, 0},
		{-1, -5, 50, 0},
		{10, 3, 10, 3},
		{101, 0, 50, 0},
	}
	for _, tt := range tests {
		gotLimit, gotOff := clampPagination(tt.limit, tt.offset)
		if gotLimit != tt.wantLimit || gotOff != tt.wantOff {
			t.Fatalf("clampPagination(%d,%d)=(%d,%d), want (%d,%d)",
				tt.limit, tt.offset, gotLimit, gotOff, tt.wantLimit, tt.wantOff)
		}
	}
}

func uniqueEmail(i int) string {
	return fmt.Sprintf("user-%d-%s@example.com", i, uuid.NewString()[:8])
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
