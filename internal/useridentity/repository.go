package useridentity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/user"
)

type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, appID uuid.UUID, in CreateInput) (Identity, error) {
	return r.create(ctx, r.pool, appID, in)
}

// CreateTx creates an identity inside an existing transaction.
func (r *Repository) CreateTx(ctx context.Context, tx pgx.Tx, appID uuid.UUID, in CreateInput) (Identity, error) {
	return r.create(ctx, tx, appID, in)
}

func (r *Repository) create(ctx context.Context, q querier, appID uuid.UUID, in CreateInput) (Identity, error) {
	if appID == uuid.Nil {
		return Identity{}, fmt.Errorf("%w: app_id is required", store.ErrInvalidInput)
	}
	if in.UserID == uuid.Nil {
		return Identity{}, fmt.Errorf("%w: user_id is required", store.ErrInvalidInput)
	}
	if in.Provider == "" {
		return Identity{}, fmt.Errorf("%w: provider is required", store.ErrInvalidInput)
	}
	subject := strings.TrimSpace(in.ProviderSubject)
	if subject == "" {
		return Identity{}, fmt.Errorf("%w: provider_subject is required", store.ErrInvalidInput)
	}
	if in.Provider == ProviderPassword {
		subject = strings.ToLower(subject)
		if in.SecretHash == "" {
			return Identity{}, fmt.Errorf("%w: secret_hash is required for password identities", store.ErrInvalidInput)
		}
	}

	metadata, err := httpx.NormalizeMetadata(in.Metadata)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
	}

	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return Identity{}, fmt.Errorf("generate uuid: %w", err)
	}

	var secretHash *string
	if in.SecretHash != "" {
		secretHash = &in.SecretHash
	}

	const sql = `
		INSERT INTO user_identities (
			id, app_id, user_id, provider, provider_subject, secret_hash, metadata, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, app_id, user_id, provider, provider_subject, secret_hash, metadata, created_at, updated_at, deleted_at`

	var ident Identity
	var hash *string
	err = q.QueryRow(ctx, sql, id, appID, in.UserID, in.Provider, subject, secretHash, metadata, now, now).Scan(
		&ident.ID, &ident.AppID, &ident.UserID, &ident.Provider, &ident.ProviderSubject,
		&hash, &ident.Metadata, &ident.CreatedAt, &ident.UpdatedAt, &ident.DeletedAt,
	)
	if err != nil {
		return Identity{}, store.MapWriteError(err)
	}
	if hash != nil {
		ident.SecretHash = *hash
	}
	return ident, nil
}

// GetPasswordByEmail returns the user and password hash for login.
func (r *Repository) GetPasswordByEmail(ctx context.Context, appID uuid.UUID, email string) (PasswordCredential, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return PasswordCredential{}, user.ErrInvalidEmail
	}

	const sql = `
		SELECT
			u.id, u.app_id, u.email, u.metadata, u.created_at, u.updated_at, u.deleted_at,
			i.secret_hash
		FROM user_identities i
		INNER JOIN users u ON u.id = i.user_id
		WHERE i.app_id = $1
		  AND i.provider = $2
		  AND i.provider_subject = $3
		  AND i.deleted_at IS NULL
		  AND u.deleted_at IS NULL`

	var cred PasswordCredential
	var hash *string
	err := r.pool.QueryRow(ctx, sql, appID, ProviderPassword, email).Scan(
		&cred.User.ID, &cred.User.AppID, &cred.User.Email, &cred.User.Metadata,
		&cred.User.CreatedAt, &cred.User.UpdatedAt, &cred.User.DeletedAt,
		&hash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PasswordCredential{}, store.ErrNotFound
	}
	if err != nil {
		return PasswordCredential{}, fmt.Errorf("get password identity: %w", err)
	}
	if hash != nil {
		cred.SecretHash = *hash
	}
	return cred, nil
}
