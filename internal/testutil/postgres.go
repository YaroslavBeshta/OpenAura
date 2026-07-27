package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// Fixed advisory lock key so parallel go test packages serialize on one DB.
	testDBLockKey int64 = 872364102
)

// Pool returns a Postgres pool for integration tests.
// Requires DATABASE_URL (typically from .env via `make test`).
// Skips the test when the database is unreachable, unless OPENAURA_REQUIRE_DB
// is set (e.g. by `make test-cover` / CI), in which case it fails instead.
// Takes a Postgres advisory lock so packages tested in parallel do not race.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	requireDB := os.Getenv("OPENAURA_REQUIRE_DB") != ""
	abort := func(format string, args ...any) {
		t.Helper()
		if requireDB {
			t.Fatalf(format, args...)
		}
		t.Skipf(format, args...)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		abort("DATABASE_URL is not set (copy .env.example to .env)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		abort("postgres unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		abort("postgres unavailable: %v", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		t.Fatalf("acquire conn for lock: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, testDBLockKey); err != nil {
		conn.Release()
		pool.Close()
		t.Fatalf("advisory lock: %v", err)
	}

	t.Cleanup(func() {
		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unlockCancel()
		_, _ = conn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, testDBLockKey)
		conn.Release()
		pool.Close()
	})

	return pool
}
