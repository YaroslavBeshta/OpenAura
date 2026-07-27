package access

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/store"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Check returns whether the user is allowed to perform action on resource within tenant,
// scoped to the authenticated app.
func (r *Repository) Check(ctx context.Context, appID uuid.UUID, in CheckInput) (bool, error) {
	if appID == uuid.Nil || in.UserID == uuid.Nil || in.TenantID == uuid.Nil {
		return false, fmt.Errorf("%w: user_id and tenant_id are required", store.ErrInvalidInput)
	}
	resourceName := strings.TrimSpace(in.Resource)
	actionName := strings.TrimSpace(in.Action)
	if resourceName == "" || actionName == "" {
		return false, fmt.Errorf("%w: resource and action are required", store.ErrInvalidInput)
	}

	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM roleassignments ra
			INNER JOIN roles ro ON ro.id = ra.role_id
			INNER JOIN users u ON u.id = ra.user_id
			INNER JOIN tenants t ON t.id = ra.tenant_id
			INNER JOIN permissions p ON p.role_id = ra.role_id
			INNER JOIN resources res ON res.id = p.resource_id
			INNER JOIN actions act ON act.id = p.action_id
			WHERE ra.deleted_at IS NULL
			  AND ro.deleted_at IS NULL
			  AND u.deleted_at IS NULL
			  AND t.deleted_at IS NULL
			  AND p.deleted_at IS NULL
			  AND res.deleted_at IS NULL
			  AND act.deleted_at IS NULL
			  AND u.app_id = $1
			  AND t.app_id = $1
			  AND ro.app_id = $1
			  AND res.app_id = $1
			  AND act.app_id = $1
			  AND ra.user_id = $2
			  AND ra.tenant_id = $3
			  AND res.name = $4
			  AND act.name = $5
		)`

	var allowed bool
	if err := r.pool.QueryRow(ctx, q, appID, in.UserID, in.TenantID, resourceName, actionName).Scan(&allowed); err != nil {
		return false, fmt.Errorf("access check: %w", err)
	}
	return allowed, nil
}
