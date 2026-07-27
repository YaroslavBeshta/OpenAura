package permission

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/store"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, appID, roleID uuid.UUID, in CreateInput) (Permission, error) {
	if roleID == uuid.Nil || in.ResourceID == uuid.Nil || in.ActionID == uuid.Nil {
		return Permission{}, fmt.Errorf("%w: role_id, resource_id, and action_id are required", store.ErrInvalidInput)
	}
	if err := r.requireEntitiesInApp(ctx, appID, roleID, in.ResourceID, in.ActionID); err != nil {
		return Permission{}, err
	}

	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return Permission{}, fmt.Errorf("generate uuid: %w", err)
	}

	const q = `
		INSERT INTO permissions (id, role_id, resource_id, action_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, role_id, resource_id, action_id, created_at, updated_at, deleted_at`

	var p Permission
	err = r.pool.QueryRow(ctx, q, id, roleID, in.ResourceID, in.ActionID, now, now).Scan(
		&p.ID, &p.RoleID, &p.ResourceID, &p.ActionID, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil {
		return Permission{}, store.MapWriteError(err)
	}
	return p, nil
}

func (r *Repository) GetByID(ctx context.Context, appID, roleID, id uuid.UUID) (Permission, error) {
	const q = `
		SELECT p.id, p.role_id, p.resource_id, p.action_id, p.created_at, p.updated_at, p.deleted_at
		FROM permissions p
		INNER JOIN roles r ON r.id = p.role_id
		WHERE p.id = $1
		  AND p.role_id = $2
		  AND r.app_id = $3
		  AND p.deleted_at IS NULL
		  AND r.deleted_at IS NULL`

	var p Permission
	err := r.pool.QueryRow(ctx, q, id, roleID, appID).Scan(
		&p.ID, &p.RoleID, &p.ResourceID, &p.ActionID, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Permission{}, store.ErrNotFound
	}
	if err != nil {
		return Permission{}, fmt.Errorf("get permission: %w", err)
	}
	return p, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]Permission, error) {
	if f.RoleID == uuid.Nil {
		return nil, fmt.Errorf("%w: role_id is required", store.ErrInvalidInput)
	}
	limit, offset := clampPagination(f.Limit, f.Offset)
	const q = `
		SELECT p.id, p.role_id, p.resource_id, p.action_id, p.created_at, p.updated_at, p.deleted_at
		FROM permissions p
		INNER JOIN roles r ON r.id = p.role_id
		WHERE p.deleted_at IS NULL
		  AND r.deleted_at IS NULL
		  AND r.app_id = $1
		  AND p.role_id = $2
		  AND ($3::uuid IS NULL OR p.resource_id = $3)
		  AND ($4::uuid IS NULL OR p.action_id = $4)
		ORDER BY p.created_at DESC
		LIMIT $5 OFFSET $6`

	rows, err := r.pool.Query(ctx, q, f.AppID, f.RoleID, f.ResourceID, f.ActionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()

	items := make([]Permission, 0)
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.RoleID, &p.ResourceID, &p.ActionID, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *Repository) SoftDelete(ctx context.Context, appID, roleID, id uuid.UUID) error {
	const q = `
		UPDATE permissions p
		SET deleted_at = $1, updated_at = $1
		FROM roles r
		WHERE p.id = $2
		  AND p.role_id = $3
		  AND p.role_id = r.id
		  AND r.app_id = $4
		  AND p.deleted_at IS NULL
		  AND r.deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, time.Now().UTC(), id, roleID, appID)
	if err != nil {
		return fmt.Errorf("delete permission: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (r *Repository) requireEntitiesInApp(ctx context.Context, appID, roleID, resourceID, actionID uuid.UUID) error {
	const q = `
		SELECT
			EXISTS(SELECT 1 FROM roles WHERE id = $2 AND app_id = $1 AND deleted_at IS NULL),
			EXISTS(SELECT 1 FROM resources WHERE id = $3 AND app_id = $1 AND deleted_at IS NULL),
			EXISTS(SELECT 1 FROM actions WHERE id = $4 AND app_id = $1 AND deleted_at IS NULL)`
	var roleOK, resourceOK, actionOK bool
	if err := r.pool.QueryRow(ctx, q, appID, roleID, resourceID, actionID).Scan(&roleOK, &resourceOK, &actionOK); err != nil {
		return fmt.Errorf("check permission entities: %w", err)
	}
	if !roleOK || !resourceOK || !actionOK {
		return store.ErrFKViolation
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
