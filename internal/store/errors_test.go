package store

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapWriteError(t *testing.T) {
	if got := MapWriteError(&pgconn.PgError{Code: "23505"}); !errors.Is(got, ErrConflict) {
		t.Fatalf("23505 => %v", got)
	}
	if got := MapWriteError(&pgconn.PgError{Code: "23503"}); !errors.Is(got, ErrFKViolation) {
		t.Fatalf("23503 => %v", got)
	}
	if got := MapWriteError(&pgconn.PgError{Code: "P0001", Message: "user, role, and tenant must belong to the same app"}); !errors.Is(got, ErrAppMismatch) {
		t.Fatalf("P0001 => %v", got)
	}
	err := errors.New("other")
	if got := MapWriteError(err); errors.Is(got, ErrConflict) || errors.Is(got, ErrFKViolation) {
		t.Fatalf("unexpected mapped error: %v", got)
	}
}
