package adminapikey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/auth"
	"github.com/openaura/openaura/internal/store"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (AdminAPIKey, error) {
	raw, err := auth.NewAPIKey("oa_admin")
	if err != nil {
		return AdminAPIKey{}, err
	}
	return r.insert(ctx, raw, in.Name)
}

// EnsureBootstrapKey inserts the given raw key if its hash is not already present.
func (r *Repository) EnsureBootstrapKey(ctx context.Context, raw, name string) error {
	hash := auth.HashAPIKey(raw)
	const existsQ = `SELECT 1 FROM admin_api_keys WHERE key_hash = $1`
	var one int
	err := r.pool.QueryRow(ctx, existsQ, hash).Scan(&one)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check bootstrap key: %w", err)
	}
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	_, err = r.insertWithHash(ctx, raw, hash, namePtr)
	return err
}

func (r *Repository) insert(ctx context.Context, raw string, name *string) (AdminAPIKey, error) {
	return r.insertWithHash(ctx, raw, auth.HashAPIKey(raw), name)
}

func (r *Repository) insertWithHash(ctx context.Context, raw, hash string, name *string) (AdminAPIKey, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return AdminAPIKey{}, fmt.Errorf("generate uuid: %w", err)
	}
	now := time.Now().UTC()
	const q = `
		INSERT INTO admin_api_keys (id, key_hash, name, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, created_at, revoked_at`
	var k AdminAPIKey
	err = r.pool.QueryRow(ctx, q, id, hash, name, now).Scan(
		&k.ID, &k.Name, &k.CreatedAt, &k.RevokedAt,
	)
	if err != nil {
		return AdminAPIKey{}, store.MapWriteError(err)
	}
	k.Key = raw
	return k, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (AdminAPIKey, error) {
	const q = `
		SELECT id, name, created_at, revoked_at
		FROM admin_api_keys
		WHERE id = $1 AND revoked_at IS NULL`
	var k AdminAPIKey
	err := r.pool.QueryRow(ctx, q, id).Scan(&k.ID, &k.Name, &k.CreatedAt, &k.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminAPIKey{}, store.ErrNotFound
	}
	if err != nil {
		return AdminAPIKey{}, fmt.Errorf("get admin api key: %w", err)
	}
	return k, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]AdminAPIKey, error) {
	limit, offset := clampPagination(f.Limit, f.Offset)
	const q = `
		SELECT id, name, created_at, revoked_at
		FROM admin_api_keys
		WHERE revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list admin api keys: %w", err)
	}
	defer rows.Close()
	keys := make([]AdminAPIKey, 0)
	for rows.Next() {
		var k AdminAPIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan admin api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *Repository) Revoke(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE admin_api_keys
		SET revoked_at = $1
		WHERE id = $2 AND revoked_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("revoke admin api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (r *Repository) AdminKeyExists(ctx context.Context, keyHash string) (bool, error) {
	const q = `SELECT 1 FROM admin_api_keys WHERE key_hash = $1 AND revoked_at IS NULL`
	var one int
	err := r.pool.QueryRow(ctx, q, keyHash).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("resolve admin key: %w", err)
	}
	return true, nil
}

func clampPagination(limit, offset int) (int, int) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
