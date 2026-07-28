package useridentity

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/testutil"
	"github.com/openaura/openaura/internal/user"
)

func TestRepository_CreatePasswordAndLookup(t *testing.T) {
	pool := testutil.Pool(t)
	appID := testutil.SeedApp(t, pool)
	users := user.NewRepository(pool)
	idents := NewRepository(pool)
	ctx := context.Background()

	email := testutil.Email("ident")
	u, err := users.Create(ctx, appID, user.CreateInput{Email: email})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	ident, err := idents.Create(ctx, appID, CreateInput{
		UserID:          u.ID,
		Provider:        ProviderPassword,
		ProviderSubject: email,
		SecretHash:      "bcrypt-hash-placeholder",
	})
	if err != nil {
		t.Fatalf("create identity: %v", err)
	}
	if ident.Provider != ProviderPassword || ident.ProviderSubject != email {
		t.Fatalf("identity: %+v", ident)
	}

	cred, err := idents.GetPasswordByEmail(ctx, appID, " "+email+" ")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if cred.User.ID != u.ID || cred.SecretHash != "bcrypt-hash-placeholder" {
		t.Fatalf("cred: %+v", cred)
	}

	if _, err := idents.GetPasswordByEmail(ctx, appID, testutil.Email("missing")); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}

	plain, err := users.Create(ctx, appID, user.CreateInput{Email: testutil.Email("plain")})
	if err != nil {
		t.Fatalf("plain user: %v", err)
	}
	if _, err := idents.GetPasswordByEmail(ctx, appID, plain.Email); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("passwordless lookup: %v", err)
	}

	if _, err := idents.Create(ctx, appID, CreateInput{
		UserID:          u.ID,
		Provider:        ProviderPassword,
		ProviderSubject: email,
		SecretHash:      "other",
	}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate identity: %v", err)
	}

	if _, err := idents.Create(ctx, appID, CreateInput{
		UserID:          uuid.Nil,
		Provider:        ProviderPassword,
		ProviderSubject: testutil.Email("bad"),
		SecretHash:      "x",
	}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("nil user: %v", err)
	}
}
