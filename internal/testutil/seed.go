package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedApp creates an active app for repository tests that require app_id.
// Uses raw SQL so testutil does not import internal/app (avoids import cycles
// when app's own tests use testutil).
func SeedApp(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("seed app id: %v", err)
	}
	_, err = pool.Exec(context.Background(), `
		INSERT INTO apps (id, name, metadata)
		VALUES ($1, $2, '{}'::jsonb)
	`, id, Name("test-app"))
	if err != nil {
		t.Fatalf("seed app: %v", err)
	}
	return id
}
