package roleassignments

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

func (r *Repository) Create(ctx context.Context, appID uuid.UUID, in CreateInput) (RoleAssignment, error) {
	if in.UserID == uuid.Nil || in.RoleID == uuid.Nil || in.TenantID == uuid.Nil {
		return RoleAssignment{}, fmt.Errorf("%w: user_id, role_id, and tenant_id are required", store.ErrInvalidInput)
	}
	if err := r.requireEntitiesInApp(ctx, appID, in.UserID, in.RoleID, in.TenantID); err != nil {
		return RoleAssignment{}, err
	}

	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return RoleAssignment{}, fmt.Errorf("generate uuid: %w", err)
	}

	const q = `
		INSERT INTO roleassignments (id, user_id, role_id, tenant_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, role_id, tenant_id, created_at, updated_at, deleted_at`

	var a RoleAssignment
	err = r.pool.QueryRow(ctx, q, id, in.UserID, in.RoleID, in.TenantID, now, now).Scan(
		&a.ID, &a.UserID, &a.RoleID, &a.TenantID, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		return RoleAssignment{}, store.MapWriteError(err)
	}
	return a, nil
}

func (r *Repository) GetByID(ctx context.Context, appID, id uuid.UUID) (RoleAssignment, error) {
	const q = `
		SELECT ra.id, ra.user_id, ra.role_id, ra.tenant_id, ra.created_at, ra.updated_at, ra.deleted_at
		FROM roleassignments ra
		INNER JOIN tenants t ON t.id = ra.tenant_id
		WHERE ra.id = $1 AND t.app_id = $2 AND ra.deleted_at IS NULL`

	var a RoleAssignment
	err := r.pool.QueryRow(ctx, q, id, appID).Scan(
		&a.ID, &a.UserID, &a.RoleID, &a.TenantID, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoleAssignment{}, store.ErrNotFound
	}
	if err != nil {
		return RoleAssignment{}, fmt.Errorf("get role assignment: %w", err)
	}
	return a, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]RoleAssignment, error) {
	limit, offset := clampPagination(f.Limit, f.Offset)

	const q = `
		SELECT ra.id, ra.user_id, ra.role_id, ra.tenant_id, ra.created_at, ra.updated_at, ra.deleted_at
		FROM roleassignments ra
		INNER JOIN tenants t ON t.id = ra.tenant_id
		WHERE ra.deleted_at IS NULL
		  AND t.app_id = $1
		  AND ($2::uuid IS NULL OR ra.user_id = $2)
		  AND ($3::uuid IS NULL OR ra.role_id = $3)
		  AND ($4::uuid IS NULL OR ra.tenant_id = $4)
		ORDER BY ra.created_at DESC
		LIMIT $5 OFFSET $6`

	rows, err := r.pool.Query(ctx, q, f.AppID, f.UserID, f.RoleID, f.TenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list role assignments: %w", err)
	}
	defer rows.Close()

	assignments := make([]RoleAssignment, 0)
	for rows.Next() {
		var a RoleAssignment
		if err := rows.Scan(&a.ID, &a.UserID, &a.RoleID, &a.TenantID, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan role assignment: %w", err)
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

func (r *Repository) Update(ctx context.Context, appID, id uuid.UUID, in UpdateInput) (RoleAssignment, error) {
	current, err := r.GetByID(ctx, appID, id)
	if err != nil {
		return RoleAssignment{}, err
	}

	userID := current.UserID
	roleID := current.RoleID
	tenantID := current.TenantID
	if in.UserID != nil {
		userID = *in.UserID
	}
	if in.RoleID != nil {
		roleID = *in.RoleID
	}
	if in.TenantID != nil {
		tenantID = *in.TenantID
	}
	if userID == uuid.Nil || roleID == uuid.Nil || tenantID == uuid.Nil {
		return RoleAssignment{}, fmt.Errorf("%w: user_id, role_id, and tenant_id are required", store.ErrInvalidInput)
	}
	if err := r.requireEntitiesInApp(ctx, appID, userID, roleID, tenantID); err != nil {
		return RoleAssignment{}, err
	}

	const q = `
		UPDATE roleassignments ra
		SET user_id = $1, role_id = $2, tenant_id = $3, updated_at = $4
		FROM tenants t
		WHERE ra.id = $5
		  AND ra.tenant_id = t.id
		  AND t.app_id = $6
		  AND ra.deleted_at IS NULL
		RETURNING ra.id, ra.user_id, ra.role_id, ra.tenant_id, ra.created_at, ra.updated_at, ra.deleted_at`

	var a RoleAssignment
	err = r.pool.QueryRow(ctx, q, userID, roleID, tenantID, time.Now().UTC(), id, appID).Scan(
		&a.ID, &a.UserID, &a.RoleID, &a.TenantID, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoleAssignment{}, store.ErrNotFound
	}
	if err != nil {
		return RoleAssignment{}, store.MapWriteError(err)
	}
	return a, nil
}

func (r *Repository) SoftDelete(ctx context.Context, appID, id uuid.UUID) error {
	const q = `
		UPDATE roleassignments ra
		SET deleted_at = $1, updated_at = $1
		FROM tenants t
		WHERE ra.id = $2
		  AND ra.tenant_id = t.id
		  AND t.app_id = $3
		  AND ra.deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, time.Now().UTC(), id, appID)
	if err != nil {
		return fmt.Errorf("delete role assignment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

// requireEntitiesInApp ensures user, role, and tenant exist in the authenticated app.
// Missing or cross-app IDs are treated as FK violations so callers cannot probe other apps.
func (r *Repository) requireEntitiesInApp(ctx context.Context, appID, userID, roleID, tenantID uuid.UUID) error {
	const q = `
		SELECT
			EXISTS(SELECT 1 FROM users WHERE id = $2 AND app_id = $1 AND deleted_at IS NULL),
			EXISTS(SELECT 1 FROM roles WHERE id = $3 AND app_id = $1 AND deleted_at IS NULL),
			EXISTS(SELECT 1 FROM tenants WHERE id = $4 AND app_id = $1 AND deleted_at IS NULL)`
	var userOK, roleOK, tenantOK bool
	if err := r.pool.QueryRow(ctx, q, appID, userID, roleID, tenantID).Scan(&userOK, &roleOK, &tenantOK); err != nil {
		return fmt.Errorf("check assignment entities: %w", err)
	}
	if !userOK || !roleOK || !tenantOK {
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
