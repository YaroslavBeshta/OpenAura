package apikey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/auth"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/store"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, appID uuid.UUID, in CreateInput) (APIKey, error) {
	if err := r.requireActiveApp(ctx, appID); err != nil {
		return APIKey{}, err
	}
	metadata, err := httpx.NormalizeMetadata(in.Metadata)
	if err != nil {
		return APIKey{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
	}
	raw, err := auth.NewAPIKey("oa_app")
	if err != nil {
		return APIKey{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return APIKey{}, fmt.Errorf("generate uuid: %w", err)
	}
	now := time.Now().UTC()

	const q = `
		INSERT INTO api_keys (id, app_id, key_hash, name, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, app_id, name, metadata, created_at, revoked_at`

	var k APIKey
	err = r.pool.QueryRow(ctx, q, id, appID, auth.HashAPIKey(raw), in.Name, metadata, now).Scan(
		&k.ID, &k.AppID, &k.Name, &k.Metadata, &k.CreatedAt, &k.RevokedAt,
	)
	if err != nil {
		return APIKey{}, store.MapWriteError(err)
	}
	k.Key = raw
	return k, nil
}

func (r *Repository) GetByID(ctx context.Context, appID, id uuid.UUID) (APIKey, error) {
	const q = `
		SELECT id, app_id, name, metadata, created_at, revoked_at
		FROM api_keys
		WHERE id = $1 AND app_id = $2 AND revoked_at IS NULL`
	var k APIKey
	err := r.pool.QueryRow(ctx, q, id, appID).Scan(
		&k.ID, &k.AppID, &k.Name, &k.Metadata, &k.CreatedAt, &k.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, store.ErrNotFound
	}
	if err != nil {
		return APIKey{}, fmt.Errorf("get api key: %w", err)
	}
	return k, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]APIKey, error) {
	limit, offset := clampPagination(f.Limit, f.Offset)
	const q = `
		SELECT id, app_id, name, metadata, created_at, revoked_at
		FROM api_keys
		WHERE app_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, q, f.AppID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	keys := make([]APIKey, 0)
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.AppID, &k.Name, &k.Metadata, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *Repository) Revoke(ctx context.Context, appID, id uuid.UUID) error {
	const q = `
		UPDATE api_keys
		SET revoked_at = $1
		WHERE id = $2 AND app_id = $3 AND revoked_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, time.Now().UTC(), id, appID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (r *Repository) ResolveAppIDByKeyHash(ctx context.Context, keyHash string) (uuid.UUID, error) {
	// Key must be active and its app must still exist (not soft-deleted).
	const q = `
		SELECT k.app_id
		FROM api_keys k
		INNER JOIN apps a ON a.id = k.app_id
		WHERE k.key_hash = $1
		  AND k.revoked_at IS NULL
		  AND a.deleted_at IS NULL`
	var appID uuid.UUID
	err := r.pool.QueryRow(ctx, q, keyHash).Scan(&appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, store.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve app key: %w", err)
	}
	return appID, nil
}

func (r *Repository) requireActiveApp(ctx context.Context, appID uuid.UUID) error {
	const q = `SELECT 1 FROM apps WHERE id = $1 AND deleted_at IS NULL`
	var one int
	err := r.pool.QueryRow(ctx, q, appID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check app: %w", err)
	}
	return nil
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
