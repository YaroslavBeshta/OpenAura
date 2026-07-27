package action

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

func (r *Repository) Create(ctx context.Context, appID uuid.UUID, in CreateInput) (Action, error) {
	if appID == uuid.Nil {
		return Action{}, fmt.Errorf("%w: app_id is required", store.ErrInvalidInput)
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return Action{}, err
	}
	metadata, err := httpx.NormalizeMetadata(in.Metadata)
	if err != nil {
		return Action{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
	}

	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return Action{}, fmt.Errorf("generate uuid: %w", err)
	}

	const q = `
		INSERT INTO actions (id, app_id, name, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, app_id, name, metadata, created_at, updated_at, deleted_at`

	var a Action
	err = r.pool.QueryRow(ctx, q, id, appID, name, metadata, now, now).Scan(
		&a.ID, &a.AppID, &a.Name, &a.Metadata, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		return Action{}, store.MapWriteError(err)
	}
	return a, nil
}

func (r *Repository) GetByID(ctx context.Context, appID, id uuid.UUID) (Action, error) {
	const q = `
		SELECT id, app_id, name, metadata, created_at, updated_at, deleted_at
		FROM actions
		WHERE id = $1 AND app_id = $2 AND deleted_at IS NULL`

	var a Action
	err := r.pool.QueryRow(ctx, q, id, appID).Scan(
		&a.ID, &a.AppID, &a.Name, &a.Metadata, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Action{}, store.ErrNotFound
	}
	if err != nil {
		return Action{}, fmt.Errorf("get action: %w", err)
	}
	return a, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]Action, error) {
	limit, offset := clampPagination(f.Limit, f.Offset)
	const q = `
		SELECT id, app_id, name, metadata, created_at, updated_at, deleted_at
		FROM actions
		WHERE app_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, f.AppID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list actions: %w", err)
	}
	defer rows.Close()

	items := make([]Action, 0)
	for rows.Next() {
		var a Action
		if err := rows.Scan(&a.ID, &a.AppID, &a.Name, &a.Metadata, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan action: %w", err)
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *Repository) Update(ctx context.Context, appID, id uuid.UUID, in UpdateInput) (Action, error) {
	if in.Name == nil && in.Metadata == nil {
		return Action{}, fmt.Errorf("%w: name or metadata is required", store.ErrInvalidInput)
	}
	current, err := r.GetByID(ctx, appID, id)
	if err != nil {
		return Action{}, err
	}

	name := current.Name
	if in.Name != nil {
		name, err = normalizeName(*in.Name)
		if err != nil {
			return Action{}, err
		}
	}
	metadata := current.Metadata
	if in.Metadata != nil {
		metadata, err = httpx.NormalizeMetadata(*in.Metadata)
		if err != nil {
			return Action{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
		}
	}

	const q = `
		UPDATE actions
		SET name = $1, metadata = $2, updated_at = $3
		WHERE id = $4 AND app_id = $5 AND deleted_at IS NULL
		RETURNING id, app_id, name, metadata, created_at, updated_at, deleted_at`

	var a Action
	err = r.pool.QueryRow(ctx, q, name, metadata, time.Now().UTC(), id, appID).Scan(
		&a.ID, &a.AppID, &a.Name, &a.Metadata, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Action{}, store.ErrNotFound
	}
	if err != nil {
		return Action{}, store.MapWriteError(err)
	}
	return a, nil
}

func (r *Repository) SoftDelete(ctx context.Context, appID, id uuid.UUID) error {
	const q = `
		UPDATE actions
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND app_id = $3 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, time.Now().UTC(), id, appID)
	if err != nil {
		return fmt.Errorf("delete action: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: name is required", store.ErrInvalidInput)
	}
	return name, nil
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
