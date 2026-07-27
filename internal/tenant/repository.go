package tenant

import (
	"context"
	"errors"
	"fmt"
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

func (r *Repository) Create(ctx context.Context, appID uuid.UUID, in CreateInput) (Tenant, error) {
	if appID == uuid.Nil {
		return Tenant{}, fmt.Errorf("%w: app_id is required", store.ErrInvalidInput)
	}
	metadata, err := httpx.NormalizeMetadata(in.Metadata)
	if err != nil {
		return Tenant{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
	}

	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return Tenant{}, fmt.Errorf("generate uuid: %w", err)
	}

	const q = `
		INSERT INTO tenants (id, app_id, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, app_id, metadata, created_at, updated_at, deleted_at`

	var t Tenant
	err = r.pool.QueryRow(ctx, q, id, appID, metadata, now, now).Scan(
		&t.ID, &t.AppID, &t.Metadata, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if err != nil {
		return Tenant{}, store.MapWriteError(err)
	}
	return t, nil
}

func (r *Repository) GetByID(ctx context.Context, appID, id uuid.UUID) (Tenant, error) {
	const q = `
		SELECT id, app_id, metadata, created_at, updated_at, deleted_at
		FROM tenants
		WHERE id = $1 AND app_id = $2 AND deleted_at IS NULL`

	var t Tenant
	err := r.pool.QueryRow(ctx, q, id, appID).Scan(
		&t.ID, &t.AppID, &t.Metadata, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, store.ErrNotFound
	}
	if err != nil {
		return Tenant{}, fmt.Errorf("get tenant: %w", err)
	}
	return t, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]Tenant, error) {
	limit, offset := clampPagination(f.Limit, f.Offset)
	const q = `
		SELECT id, app_id, metadata, created_at, updated_at, deleted_at
		FROM tenants
		WHERE app_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, f.AppID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	tenants := make([]Tenant, 0)
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.AppID, &t.Metadata, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

func (r *Repository) Update(ctx context.Context, appID, id uuid.UUID, in UpdateInput) (Tenant, error) {
	if in.Metadata == nil {
		return Tenant{}, fmt.Errorf("%w: metadata is required", store.ErrInvalidInput)
	}
	metadata, err := httpx.NormalizeMetadata(*in.Metadata)
	if err != nil {
		return Tenant{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
	}

	const q = `
		UPDATE tenants
		SET metadata = $1, updated_at = $2
		WHERE id = $3 AND app_id = $4 AND deleted_at IS NULL
		RETURNING id, app_id, metadata, created_at, updated_at, deleted_at`

	var t Tenant
	err = r.pool.QueryRow(ctx, q, metadata, time.Now().UTC(), id, appID).Scan(
		&t.ID, &t.AppID, &t.Metadata, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, store.ErrNotFound
	}
	if err != nil {
		return Tenant{}, store.MapWriteError(err)
	}
	return t, nil
}

func (r *Repository) SoftDelete(ctx context.Context, appID, id uuid.UUID) error {
	const q = `
		UPDATE tenants
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND app_id = $3 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, time.Now().UTC(), id, appID)
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
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
