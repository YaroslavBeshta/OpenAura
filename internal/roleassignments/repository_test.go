package roleassignments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/role"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/tenant"
	"github.com/openaura/openaura/internal/testutil"
	"github.com/openaura/openaura/internal/user"
)

type fixtures struct {
	users   *user.Repository
	tenants *tenant.Repository
	roles   *role.Repository
	assign  *Repository
}

func newFixtures(t *testing.T) (*fixtures, context.Context, uuid.UUID) {
	t.Helper()
	pool := testutil.Pool(t)
	appID := testutil.SeedApp(t, pool)
	return &fixtures{
		users:   user.NewRepository(pool),
		tenants: tenant.NewRepository(pool),
		roles:   role.NewRepository(pool),
		assign:  NewRepository(pool),
	}, context.Background(), appID
}

func (f *fixtures) seedUser(t *testing.T, ctx context.Context, appID uuid.UUID, _ string) user.User {
	t.Helper()
	u, err := f.users.Create(ctx, appID, user.CreateInput{Email: testutil.Email("user")})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func (f *fixtures) seedTenant(t *testing.T, ctx context.Context, appID uuid.UUID, name string) tenant.Tenant {
	t.Helper()
	ten, err := f.tenants.Create(ctx, appID, tenant.CreateInput{
		Metadata: json.RawMessage(fmt.Sprintf(`{"name":%q}`, name)),
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return ten
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

func TestRepository_CreateGetUpdateDelete(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u := f.seedUser(t, ctx, appID, "a@example.com")
	ten := f.seedTenant(t, ctx, appID, "acme")
	admin := f.seedRole(t, ctx, appID, "admin")
	member := f.seedRole(t, ctx, appID, "member")

	created, err := f.assign.Create(ctx, appID, CreateInput{
		UserID:   u.ID,
		RoleID:   admin.ID,
		TenantID: ten.ID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected id")
	}
	if created.UserID != u.ID || created.RoleID != admin.ID || created.TenantID != ten.ID {
		t.Fatalf("unexpected assignment: %+v", created)
	}

	got, err := f.assign.GetByID(ctx, appID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Fatal("get id mismatch")
	}

	updated, err := f.assign.Update(ctx, appID, created.ID, UpdateInput{RoleID: &member.ID})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.RoleID != member.ID {
		t.Fatalf("role_id = %s, want %s", updated.RoleID, member.ID)
	}

	if err := f.assign.SoftDelete(ctx, appID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := f.assign.GetByID(ctx, appID, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
}

func TestRepository_ConflictAndFK(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u := f.seedUser(t, ctx, appID, "a@example.com")
	ten := f.seedTenant(t, ctx, appID, "acme")
	admin := f.seedRole(t, ctx, appID, "admin")

	if _, err := f.assign.Create(ctx, appID, CreateInput{
		UserID:   u.ID,
		RoleID:   admin.ID,
		TenantID: ten.ID,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.assign.Create(ctx, appID, CreateInput{
		UserID:   u.ID,
		RoleID:   admin.ID,
		TenantID: ten.ID,
	}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate: %v, want ErrConflict", err)
	}

	missing := uuid.Must(uuid.NewV7())
	if _, err := f.assign.Create(ctx, appID, CreateInput{
		UserID:   missing,
		RoleID:   admin.ID,
		TenantID: ten.ID,
	}); !errors.Is(err, store.ErrFKViolation) && !errors.Is(err, store.ErrAppMismatch) {
		t.Fatalf("missing user fk: %v, want ErrFKViolation or ErrAppMismatch", err)
	}

	if _, err := f.assign.Create(ctx, appID, CreateInput{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("empty create: %v", err)
	}
}

func TestRepository_ListFiltersAndPagination(t *testing.T) {
	f, ctx, appID := newFixtures(t)

	u1 := f.seedUser(t, ctx, appID, "u1@example.com")
	u2 := f.seedUser(t, ctx, appID, "u2@example.com")
	t1 := f.seedTenant(t, ctx, appID, "t1")
	t2 := f.seedTenant(t, ctx, appID, "t2")
	r1 := f.seedRole(t, ctx, appID, "r1")
	r2 := f.seedRole(t, ctx, appID, "r2")

	type triple struct {
		user, role, tenant uuid.UUID
	}
	seeds := []triple{
		{u1.ID, r1.ID, t1.ID},
		{u1.ID, r2.ID, t1.ID},
		{u2.ID, r1.ID, t1.ID},
		{u2.ID, r1.ID, t2.ID},
		{u2.ID, r2.ID, t2.ID},
	}
	var deletedID uuid.UUID
	for i, s := range seeds {
		a, err := f.assign.Create(ctx, appID, CreateInput{
			UserID:   s.user,
			RoleID:   s.role,
			TenantID: s.tenant,
		})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		if i == 0 {
			deletedID = a.ID
		}
		time.Sleep(2 * time.Millisecond)
	}
	if err := f.assign.SoftDelete(ctx, appID, deletedID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	byUser, err := f.assign.List(ctx, ListFilter{AppID: appID, UserID: &u1.ID, Limit: 50})
	if err != nil {
		t.Fatalf("filter user: %v", err)
	}
	if len(byUser) != 1 {
		t.Fatalf("user filter len = %d, want 1 (one soft-deleted)", len(byUser))
	}
	if byUser[0].RoleID != r2.ID {
		t.Fatalf("unexpected remaining assignment for u1")
	}

	byTenant, err := f.assign.List(ctx, ListFilter{AppID: appID, TenantID: &t2.ID, Limit: 50})
	if err != nil {
		t.Fatalf("filter tenant: %v", err)
	}
	if len(byTenant) != 2 {
		t.Fatalf("tenant filter len = %d, want 2", len(byTenant))
	}

	byRole, err := f.assign.List(ctx, ListFilter{AppID: appID, RoleID: &r1.ID, Limit: 50})
	if err != nil {
		t.Fatalf("filter role: %v", err)
	}
	if len(byRole) != 2 {
		t.Fatalf("role filter len = %d, want 2", len(byRole))
	}

	combined, err := f.assign.List(ctx, ListFilter{
		AppID:    appID,
		UserID:   &u2.ID,
		RoleID:   &r1.ID,
		TenantID: &t2.ID,
		Limit:    50,
	})
	if err != nil {
		t.Fatalf("combined filter: %v", err)
	}
	if len(combined) != 1 {
		t.Fatalf("combined len = %d, want 1", len(combined))
	}

	page, err := f.assign.List(ctx, ListFilter{AppID: appID, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d, want 2", len(page))
	}

	all, err := f.assign.List(ctx, ListFilter{AppID: appID, Limit: 100})
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("active assignments = %d, want 4", len(all))
	}
}

func TestRepository_ReuseAfterSoftDelete(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u := f.seedUser(t, ctx, appID, "a@example.com")
	ten := f.seedTenant(t, ctx, appID, "acme")
	admin := f.seedRole(t, ctx, appID, "admin")

	first, err := f.assign.Create(ctx, appID, CreateInput{
		UserID: u.ID, RoleID: admin.ID, TenantID: ten.ID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.assign.SoftDelete(ctx, appID, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	second, err := f.assign.Create(ctx, appID, CreateInput{
		UserID: u.ID, RoleID: admin.ID, TenantID: ten.ID,
	})
	if err != nil {
		t.Fatalf("recreate after soft delete: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("expected new assignment id")
	}
}

func TestRepository_RejectsCrossAppEntities(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	otherApp := testutil.SeedApp(t, f.assign.pool)

	u := f.seedUser(t, ctx, appID, "a@example.com")
	ten := f.seedTenant(t, ctx, appID, "acme")
	admin := f.seedRole(t, ctx, appID, "admin")

	otherUser := f.seedUser(t, ctx, otherApp, "other@example.com")
	otherTenant := f.seedTenant(t, ctx, otherApp, "other")
	otherRole := f.seedRole(t, ctx, otherApp, "other-role")

	if _, err := f.assign.Create(ctx, appID, CreateInput{
		UserID: otherUser.ID, RoleID: admin.ID, TenantID: ten.ID,
	}); !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("cross-app user: %v", err)
	}
	if _, err := f.assign.Create(ctx, appID, CreateInput{
		UserID: u.ID, RoleID: otherRole.ID, TenantID: ten.ID,
	}); !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("cross-app role: %v", err)
	}
	if _, err := f.assign.Create(ctx, appID, CreateInput{
		UserID: u.ID, RoleID: admin.ID, TenantID: otherTenant.ID,
	}); !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("cross-app tenant: %v", err)
	}

	// Other app's consistent trio still rejected for this app's key scope.
	if _, err := f.assign.Create(ctx, appID, CreateInput{
		UserID: otherUser.ID, RoleID: otherRole.ID, TenantID: otherTenant.ID,
	}); !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("other app entities: %v", err)
	}

	created, err := f.assign.Create(ctx, appID, CreateInput{
		UserID: u.ID, RoleID: admin.ID, TenantID: ten.ID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.assign.Update(ctx, appID, created.ID, UpdateInput{UserID: &otherUser.ID}); !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("update cross-app user: %v", err)
	}
	if _, err := f.assign.GetByID(ctx, otherApp, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-app get: %v", err)
	}
}
