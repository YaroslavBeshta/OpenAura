package permission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/action"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/resource"
	"github.com/openaura/openaura/internal/role"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/testutil"
)

type fixtures struct {
	pool      *pgxpool.Pool
	apps      *app.Repository
	roles     *role.Repository
	resources *resource.Repository
	actions   *action.Repository
	perms     *Repository
}

func newFixtures(t *testing.T) (*fixtures, context.Context, uuid.UUID) {
	t.Helper()
	pool := testutil.Pool(t)
	f := &fixtures{
		pool:      pool,
		apps:      app.NewRepository(pool),
		roles:     role.NewRepository(pool),
		resources: resource.NewRepository(pool),
		actions:   action.NewRepository(pool),
		perms:     NewRepository(pool),
	}
	a, err := f.apps.Create(context.Background(), app.CreateInput{Name: "perm-app"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	return f, context.Background(), a.ID
}

func (f *fixtures) createApp(t *testing.T, name string) uuid.UUID {
	t.Helper()
	a, err := f.apps.Create(context.Background(), app.CreateInput{Name: name})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	return a.ID
}

func (f *fixtures) seedRole(t *testing.T, ctx context.Context, appID uuid.UUID, name string) role.Role {
	t.Helper()
	r, err := f.roles.Create(ctx, appID, role.CreateInput{
		Metadata: json.RawMessage(fmt.Sprintf(`{"name":%q}`, name)),
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	return r
}

func (f *fixtures) seedResource(t *testing.T, ctx context.Context, appID uuid.UUID, name string) resource.Resource {
	t.Helper()
	res, err := f.resources.Create(ctx, appID, resource.CreateInput{Name: name})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}
	return res
}

func (f *fixtures) seedAction(t *testing.T, ctx context.Context, appID uuid.UUID, name string) action.Action {
	t.Helper()
	a, err := f.actions.Create(ctx, appID, action.CreateInput{Name: name})
	if err != nil {
		t.Fatalf("create action: %v", err)
	}
	return a
}

func (f *fixtures) countActivePermissions(t *testing.T, roleID uuid.UUID) int {
	t.Helper()
	var n int
	err := f.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM permissions WHERE role_id = $1 AND deleted_at IS NULL
	`, roleID).Scan(&n)
	if err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	return n
}

func TestRepository_CreateGetListDelete(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	admin := f.seedRole(t, ctx, appID, "admin")
	docs := f.seedResource(t, ctx, appID, "documents")
	read := f.seedAction(t, ctx, appID, "read")
	write := f.seedAction(t, ctx, appID, "write")

	created, err := f.perms.Create(ctx, appID, admin.ID, CreateInput{
		ResourceID: docs.ID, ActionID: read.ID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var dbRole, dbRes, dbAct uuid.UUID
	err = f.pool.QueryRow(ctx, `
		SELECT role_id, resource_id, action_id
		FROM permissions
		WHERE id = $1 AND deleted_at IS NULL
	`, created.ID).Scan(&dbRole, &dbRes, &dbAct)
	if err != nil {
		t.Fatalf("permission row missing in db: %v", err)
	}
	if dbRole != admin.ID || dbRes != docs.ID || dbAct != read.ID {
		t.Fatalf("db permission mismatch: role=%s res=%s act=%s", dbRole, dbRes, dbAct)
	}

	got, err := f.perms.GetByID(ctx, appID, admin.ID, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("get: %v %+v", err, got)
	}

	if _, err := f.perms.Create(ctx, appID, admin.ID, CreateInput{
		ResourceID: docs.ID, ActionID: write.ID,
	}); err != nil {
		t.Fatalf("create write: %v", err)
	}
	if f.countActivePermissions(t, admin.ID) != 2 {
		t.Fatalf("expected 2 active permissions in db, got %d", f.countActivePermissions(t, admin.ID))
	}

	list, err := f.perms.List(ctx, ListFilter{AppID: appID, RoleID: admin.ID, Limit: 50})
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}

	byAction, err := f.perms.List(ctx, ListFilter{
		AppID: appID, RoleID: admin.ID, ActionID: &read.ID, Limit: 50,
	})
	if err != nil || len(byAction) != 1 {
		t.Fatalf("filter: %v len=%d", err, len(byAction))
	}

	if err := f.perms.SoftDelete(ctx, appID, admin.ID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var deletedAt *time.Time
	err = f.pool.QueryRow(ctx, `SELECT deleted_at FROM permissions WHERE id = $1`, created.ID).Scan(&deletedAt)
	if err != nil || deletedAt == nil {
		t.Fatalf("expected soft-delete in db: %v", err)
	}
	if _, err := f.perms.GetByID(ctx, appID, admin.ID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
	if f.countActivePermissions(t, admin.ID) != 1 {
		t.Fatalf("expected 1 active permission after delete, got %d", f.countActivePermissions(t, admin.ID))
	}

	recreated, err := f.perms.Create(ctx, appID, admin.ID, CreateInput{
		ResourceID: docs.ID, ActionID: read.ID,
	})
	if err != nil {
		t.Fatalf("recreate: %v", err)
	}
	if recreated.ID == created.ID {
		t.Fatal("recreate should insert a new permission row")
	}
}

func TestRepository_RejectsCrossAppAndConflict(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	otherApp := f.createApp(t, "other-app")

	admin := f.seedRole(t, ctx, appID, "admin")
	docs := f.seedResource(t, ctx, appID, "documents")
	read := f.seedAction(t, ctx, appID, "read")

	otherRole := f.seedRole(t, ctx, otherApp, "other")
	otherRes := f.seedResource(t, ctx, otherApp, "documents")
	otherAct := f.seedAction(t, ctx, otherApp, "read")

	if _, err := f.perms.Create(ctx, appID, admin.ID, CreateInput{
		ResourceID: docs.ID, ActionID: read.ID,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.perms.Create(ctx, appID, admin.ID, CreateInput{
		ResourceID: docs.ID, ActionID: read.ID,
	}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("dup: %v", err)
	}
	if f.countActivePermissions(t, admin.ID) != 1 {
		t.Fatal("conflict must not insert a second row")
	}

	before := f.countActivePermissions(t, admin.ID)
	if _, err := f.perms.Create(ctx, appID, otherRole.ID, CreateInput{
		ResourceID: docs.ID, ActionID: read.ID,
	}); !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("cross-app role: %v", err)
	}
	if _, err := f.perms.Create(ctx, appID, admin.ID, CreateInput{
		ResourceID: otherRes.ID, ActionID: read.ID,
	}); !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("cross-app resource: %v", err)
	}
	if _, err := f.perms.Create(ctx, appID, admin.ID, CreateInput{
		ResourceID: docs.ID, ActionID: otherAct.ID,
	}); !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("cross-app action: %v", err)
	}
	if f.countActivePermissions(t, admin.ID) != before {
		t.Fatal("cross-app rejects must not insert rows")
	}
}
