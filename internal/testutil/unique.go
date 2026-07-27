package testutil

import (
	"fmt"

	"github.com/google/uuid"
)

// Unique returns a short random token suitable for isolating test fixtures.
func Unique() string {
	return uuid.NewString()[:8]
}

// Email returns a unique example email with the supplied prefix.
func Email(prefix string) string {
	return fmt.Sprintf("%s-%s@example.com", prefix, Unique())
}

// Name returns a unique fixture name with the supplied prefix.
func Name(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, Unique())
}
