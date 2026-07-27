package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/store"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (App, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return App{}, fmt.Errorf("%w: name is required", store.ErrInvalidInput)
	}
	metadata, err := httpx.NormalizeMetadata(in.Metadata)
	if err != nil {
		return App{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
	}

	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return App{}, fmt.Errorf("generate uuid: %w", err)
	}

	const q = `
		INSERT INTO apps (id, name, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, metadata, created_at, updated_at, deleted_at`

	var a App
	err = r.pool.QueryRow(ctx, q, id, name, metadata, now, now).Scan(
		&a.ID, &a.Name, &a.Metadata, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		return App{}, store.MapWriteError(err)
	}
	return a, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (App, error) {
	const q = `
		SELECT id, name, metadata, created_at, updated_at, deleted_at
		FROM apps
		WHERE id = $1 AND deleted_at IS NULL`

	var a App
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.Name, &a.Metadata, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return App{}, store.ErrNotFound
	}
	if err != nil {
		return App{}, fmt.Errorf("get app: %w", err)
	}
	return a, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]App, error) {
	limit, offset := clampPagination(f.Limit, f.Offset)
	const q = `
		SELECT id, name, metadata, created_at, updated_at, deleted_at
		FROM apps
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	defer rows.Close()

	apps := make([]App, 0)
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Name, &a.Metadata, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan app: %w", err)
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (App, error) {
	if in.Name == nil && in.Metadata == nil {
		return App{}, fmt.Errorf("%w: name or metadata is required", store.ErrInvalidInput)
	}
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return App{}, err
	}

	name := current.Name
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return App{}, fmt.Errorf("%w: name is required", store.ErrInvalidInput)
		}
	}
	metadata := current.Metadata
	if in.Metadata != nil {
		metadata, err = httpx.NormalizeMetadata(*in.Metadata)
		if err != nil {
			return App{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
		}
	}

	const q = `
		UPDATE apps
		SET name = $1, metadata = $2, updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING id, name, metadata, created_at, updated_at, deleted_at`

	var a App
	err = r.pool.QueryRow(ctx, q, name, metadata, time.Now().UTC(), id).Scan(
		&a.ID, &a.Name, &a.Metadata, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return App{}, store.ErrNotFound
	}
	if err != nil {
		return App{}, store.MapWriteError(err)
	}
	return a, nil
}

func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE apps
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("delete app: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
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
