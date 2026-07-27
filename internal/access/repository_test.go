package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/action"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/permission"
	"github.com/openaura/openaura/internal/resource"
	"github.com/openaura/openaura/internal/role"
	"github.com/openaura/openaura/internal/roleassignments"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/tenant"
	"github.com/openaura/openaura/internal/testutil"
	"github.com/openaura/openaura/internal/user"
)

type fixtures struct {
	pool      *pgxpool.Pool
	apps      *app.Repository
	users     *user.Repository
	tenants   *tenant.Repository
	roles     *role.Repository
	resources *resource.Repository
	actions   *action.Repository
	perms     *permission.Repository
	assigns   *roleassignments.Repository
	access    *Repository
}

func newFixtures(t *testing.T) (*fixtures, context.Context, uuid.UUID) {
	t.Helper()
	pool := testutil.Pool(t)
	f := &fixtures{
		pool:      pool,
		apps:      app.NewRepository(pool),
		users:     user.NewRepository(pool),
		tenants:   tenant.NewRepository(pool),
		roles:     role.NewRepository(pool),
		resources: resource.NewRepository(pool),
		actions:   action.NewRepository(pool),
		perms:     permission.NewRepository(pool),
		assigns:   roleassignments.NewRepository(pool),
		access:    NewRepository(pool),
	}
	a, err := f.apps.Create(context.Background(), app.CreateInput{Name: "access-app"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	return f, context.Background(), a.ID
}

func (f *fixtures) seed(t *testing.T, ctx context.Context, appID uuid.UUID) (user.User, tenant.Tenant, role.Role, resource.Resource, action.Action) {
	t.Helper()
	u, err := f.users.Create(ctx, appID, user.CreateInput{Email: fmt.Sprintf("ada-%s@example.com", uuid.NewString()[:8])})
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	ten, err := f.tenants.Create(ctx, appID, tenant.CreateInput{
		Metadata: json.RawMessage(`{"name":"acme"}`),
	})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	ro, err := f.roles.Create(ctx, appID, role.CreateInput{
		Metadata: json.RawMessage(`{"name":"admin"}`),
	})
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	res, err := f.resources.Create(ctx, appID, resource.CreateInput{Name: "documents"})
	if err != nil {
		t.Fatalf("resource: %v", err)
	}
	act, err := f.actions.Create(ctx, appID, action.CreateInput{Name: "read"})
	if err != nil {
		t.Fatalf("action: %v", err)
	}

	// Prove the graph rows exist in Postgres before access checks.
	var n int
	err = f.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users u
		JOIN tenants t ON t.app_id = u.app_id
		JOIN roles r ON r.app_id = u.app_id
		JOIN resources res ON res.app_id = u.app_id
		JOIN actions a ON a.app_id = u.app_id
		WHERE u.id = $1 AND t.id = $2 AND r.id = $3 AND res.id = $4 AND a.id = $5
		  AND u.deleted_at IS NULL AND t.deleted_at IS NULL AND r.deleted_at IS NULL
		  AND res.deleted_at IS NULL AND a.deleted_at IS NULL
	`, u.ID, ten.ID, ro.ID, res.ID, act.ID).Scan(&n)
	if err != nil || n != 1 {
		t.Fatalf("seed graph missing in db: n=%d err=%v", n, err)
	}
	return u, ten, ro, res, act
}

func TestRepository_CheckAllowAndDeny(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u, ten, ro, res, act := f.seed(t, ctx, appID)

	perm, err := f.perms.Create(ctx, appID, ro.ID, permission.CreateInput{
		ResourceID: res.ID, ActionID: act.ID,
	})
	if err != nil {
		t.Fatalf("perm: %v", err)
	}
	asg, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{
		UserID: u.ID, RoleID: ro.ID, TenantID: ten.ID,
	})
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	var n int
	err = f.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM permissions p
		JOIN roleassignments ra ON ra.role_id = p.role_id
		WHERE p.id = $1 AND ra.id = $2 AND p.deleted_at IS NULL AND ra.deleted_at IS NULL
	`, perm.ID, asg.ID).Scan(&n)
	if err != nil || n != 1 {
		t.Fatalf("permission+assignment missing in db: n=%d err=%v", n, err)
	}

	allowed, err := f.access.Check(ctx, appID, CheckInput{
		UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read",
	})
	if err != nil || !allowed {
		t.Fatalf("allow: %v allowed=%v", err, allowed)
	}

	allowed, err = f.access.Check(ctx, appID, CheckInput{
		UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "write",
	})
	if err != nil || allowed {
		t.Fatalf("deny missing action: %v allowed=%v", err, allowed)
	}

	allowed, err = f.access.Check(ctx, appID, CheckInput{
		UserID: u.ID, TenantID: ten.ID, Resource: "billing", Action: "read",
	})
	if err != nil || allowed {
		t.Fatalf("deny unknown resource: %v allowed=%v", err, allowed)
	}

	otherTenant, err := f.tenants.Create(ctx, appID, tenant.CreateInput{
		Metadata: json.RawMessage(`{"name":"other"}`),
	})
	if err != nil {
		t.Fatalf("other tenant: %v", err)
	}
	allowed, err = f.access.Check(ctx, appID, CheckInput{
		UserID: u.ID, TenantID: otherTenant.ID, Resource: "documents", Action: "read",
	})
	if err != nil || allowed {
		t.Fatalf("deny wrong tenant: %v allowed=%v", err, allowed)
	}
}

func TestRepository_CheckRequiresAssignmentAndActiveEntities(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u, ten, ro, res, act := f.seed(t, ctx, appID)

	perm, err := f.perms.Create(ctx, appID, ro.ID, permission.CreateInput{
		ResourceID: res.ID, ActionID: act.ID,
	})
	if err != nil {
		t.Fatalf("perm: %v", err)
	}

	allowed, err := f.access.Check(ctx, appID, CheckInput{
		UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read",
	})
	if err != nil || allowed {
		t.Fatalf("deny without assignment: %v allowed=%v", err, allowed)
	}

	asg, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{
		UserID: u.ID, RoleID: ro.ID, TenantID: ten.ID,
	})
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	allowed, err = f.access.Check(ctx, appID, CheckInput{
		UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read",
	})
	if err != nil || !allowed {
		t.Fatalf("allow: %v allowed=%v", err, allowed)
	}

	if err := f.assigns.SoftDelete(ctx, appID, asg.ID); err != nil {
		t.Fatalf("delete assign: %v", err)
	}
	allowed, err = f.access.Check(ctx, appID, CheckInput{
		UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read",
	})
	if err != nil || allowed {
		t.Fatalf("deny after soft-delete assignment: %v allowed=%v", err, allowed)
	}

	// Recreate assignment, then soft-delete permission and confirm deny.
	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{
		UserID: u.ID, RoleID: ro.ID, TenantID: ten.ID,
	}); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if err := f.perms.SoftDelete(ctx, appID, ro.ID, perm.ID); err != nil {
		t.Fatalf("delete perm: %v", err)
	}
	allowed, err = f.access.Check(ctx, appID, CheckInput{
		UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read",
	})
	if err != nil || allowed {
		t.Fatalf("deny after soft-delete permission: %v allowed=%v", err, allowed)
	}
}

func TestRepository_CheckInvalidInput(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	if _, err := f.access.Check(ctx, appID, CheckInput{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := f.access.Check(ctx, appID, CheckInput{
		UserID: uuid.Must(uuid.NewV7()), TenantID: uuid.Must(uuid.NewV7()),
	}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("missing names: %v", err)
	}
}

func TestRepository_CheckCrossAppDenied(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u, ten, ro, res, act := f.seed(t, ctx, appID)
	if _, err := f.perms.Create(ctx, appID, ro.ID, permission.CreateInput{
		ResourceID: res.ID, ActionID: act.ID,
	}); err != nil {
		t.Fatalf("perm: %v", err)
	}
	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{
		UserID: u.ID, RoleID: ro.ID, TenantID: ten.ID,
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	other, err := f.apps.Create(ctx, app.CreateInput{Name: "other-access-app"})
	if err != nil {
		t.Fatalf("other app: %v", err)
	}
	allowed, err := f.access.Check(ctx, other.ID, CheckInput{
		UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read",
	})
	if err != nil || allowed {
		t.Fatalf("deny other app scope: %v allowed=%v", err, allowed)
	}
}
