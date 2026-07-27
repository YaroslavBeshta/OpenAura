package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidInput = errors.New("invalid input")
	ErrFKViolation  = errors.New("foreign key violation")
	ErrAppMismatch  = errors.New("user, role, and tenant must belong to the same app")
)

func MapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrFKViolation
		case "P0001":
			if strings.Contains(pgErr.Message, "same app") {
				return ErrAppMismatch
			}
		}
	}
	return fmt.Errorf("write: %w", err)
}
