package access

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/action"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/permission"
	"github.com/openaura/openaura/internal/resource"
	"github.com/openaura/openaura/internal/role"
	"github.com/openaura/openaura/internal/roleassignments"
	"github.com/openaura/openaura/internal/store"
	"github.com/openaura/openaura/internal/tenant"
	"github.com/openaura/openaura/internal/user"
)

func mustAllow(t *testing.T, f *fixtures, appID uuid.UUID, in CheckInput) {
	t.Helper()
	allowed, err := f.access.Check(t.Context(), appID, in)
	if err != nil || !allowed {
		t.Fatalf("want allow: err=%v allowed=%v input=%+v", err, allowed, in)
	}
}

func mustDeny(t *testing.T, f *fixtures, appID uuid.UUID, in CheckInput) {
	t.Helper()
	allowed, err := f.access.Check(t.Context(), appID, in)
	if err != nil || allowed {
		t.Fatalf("want deny: err=%v allowed=%v input=%+v", err, allowed, in)
	}
}

// Soft-deleted principals or authz objects must never grant access (common RBAC bypass).
func TestSecurity_SoftDeletedEntitiesDenyAccess(t *testing.T) {
	cases := []struct {
		name string
		kill func(t *testing.T, f *fixtures, appID uuid.UUID, u user.User, ten tenant.Tenant, ro role.Role, res resource.Resource, act action.Action, permID, asgID uuid.UUID)
	}{
		{"user", func(t *testing.T, f *fixtures, appID uuid.UUID, u user.User, _ tenant.Tenant, _ role.Role, _ resource.Resource, _ action.Action, _, _ uuid.UUID) {
			if err := f.users.SoftDelete(t.Context(), appID, u.ID); err != nil {
				t.Fatalf("delete user: %v", err)
			}
		}},
		{"tenant", func(t *testing.T, f *fixtures, appID uuid.UUID, _ user.User, ten tenant.Tenant, _ role.Role, _ resource.Resource, _ action.Action, _, _ uuid.UUID) {
			if err := f.tenants.SoftDelete(t.Context(), appID, ten.ID); err != nil {
				t.Fatalf("delete tenant: %v", err)
			}
		}},
		{"role", func(t *testing.T, f *fixtures, appID uuid.UUID, _ user.User, _ tenant.Tenant, ro role.Role, _ resource.Resource, _ action.Action, _, _ uuid.UUID) {
			if err := f.roles.SoftDelete(t.Context(), appID, ro.ID); err != nil {
				t.Fatalf("delete role: %v", err)
			}
		}},
		{"resource", func(t *testing.T, f *fixtures, appID uuid.UUID, _ user.User, _ tenant.Tenant, _ role.Role, res resource.Resource, _ action.Action, _, _ uuid.UUID) {
			if err := f.resources.SoftDelete(t.Context(), appID, res.ID); err != nil {
				t.Fatalf("delete resource: %v", err)
			}
		}},
		{"action", func(t *testing.T, f *fixtures, appID uuid.UUID, _ user.User, _ tenant.Tenant, _ role.Role, _ resource.Resource, act action.Action, _, _ uuid.UUID) {
			if err := f.actions.SoftDelete(t.Context(), appID, act.ID); err != nil {
				t.Fatalf("delete action: %v", err)
			}
		}},
		{"permission", func(t *testing.T, f *fixtures, appID uuid.UUID, _ user.User, _ tenant.Tenant, ro role.Role, _ resource.Resource, _ action.Action, permID, _ uuid.UUID) {
			if err := f.perms.SoftDelete(t.Context(), appID, ro.ID, permID); err != nil {
				t.Fatalf("delete permission: %v", err)
			}
		}},
		{"assignment", func(t *testing.T, f *fixtures, appID uuid.UUID, _ user.User, _ tenant.Tenant, _ role.Role, _ resource.Resource, _ action.Action, _, asgID uuid.UUID) {
			if err := f.assigns.SoftDelete(t.Context(), appID, asgID); err != nil {
				t.Fatalf("delete assignment: %v", err)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, ctx, appID := newFixtures(t)
			u, ten, ro, res, act := f.seed(t, ctx, appID)
			perm, err := f.perms.Create(ctx, appID, ro.ID, permission.CreateInput{ResourceID: res.ID, ActionID: act.ID})
			if err != nil {
				t.Fatalf("perm: %v", err)
			}
			asg, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{UserID: u.ID, RoleID: ro.ID, TenantID: ten.ID})
			if err != nil {
				t.Fatalf("assign: %v", err)
			}
			in := CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read"}
			mustAllow(t, f, appID, in)
			tc.kill(t, f, appID, u, ten, ro, res, act, perm.ID, asg.ID)
			mustDeny(t, f, appID, in)
		})
	}
}

// Horizontal IDOR-style checks: correct app scope but wrong user/tenant combination.
func TestSecurity_WrongPrincipalOrTenantDenied(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u1, ten1, ro, res, act := f.seed(t, ctx, appID)
	if _, err := f.perms.Create(ctx, appID, ro.ID, permission.CreateInput{ResourceID: res.ID, ActionID: act.ID}); err != nil {
		t.Fatalf("perm: %v", err)
	}
	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{UserID: u1.ID, RoleID: ro.ID, TenantID: ten1.ID}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	u2, err := f.users.Create(ctx, appID, user.CreateInput{Email: "other@example.com"})
	if err != nil {
		t.Fatalf("user2: %v", err)
	}
	ten2, err := f.tenants.Create(ctx, appID, tenant.CreateInput{Metadata: json.RawMessage(`{"name":"other"}`)})
	if err != nil {
		t.Fatalf("tenant2: %v", err)
	}

	mustAllow(t, f, appID, CheckInput{UserID: u1.ID, TenantID: ten1.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appID, CheckInput{UserID: u2.ID, TenantID: ten1.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appID, CheckInput{UserID: u1.ID, TenantID: ten2.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appID, CheckInput{UserID: u2.ID, TenantID: ten2.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appID, CheckInput{UserID: uuid.Must(uuid.NewV7()), TenantID: ten1.ID, Resource: "documents", Action: "read"})
}

// Same resource/action names in another app must not authorize cross-app (confused deputy / IDOR).
func TestSecurity_HomonymousResourcesAcrossApps(t *testing.T) {
	f, ctx, appA := newFixtures(t)
	uA, tenA, roA, resA, actA := f.seed(t, ctx, appA)
	if _, err := f.perms.Create(ctx, appA, roA.ID, permission.CreateInput{ResourceID: resA.ID, ActionID: actA.ID}); err != nil {
		t.Fatalf("perm A: %v", err)
	}
	if _, err := f.assigns.Create(ctx, appA, roleassignments.CreateInput{UserID: uA.ID, RoleID: roA.ID, TenantID: tenA.ID}); err != nil {
		t.Fatalf("assign A: %v", err)
	}

	appB, err := f.apps.Create(ctx, app.CreateInput{Name: "app-b"})
	if err != nil {
		t.Fatalf("app B: %v", err)
	}
	uB, err := f.users.Create(ctx, appB.ID, user.CreateInput{Email: "b@example.com"})
	if err != nil {
		t.Fatalf("user B: %v", err)
	}
	tenB, err := f.tenants.Create(ctx, appB.ID, tenant.CreateInput{Metadata: json.RawMessage(`{"name":"b"}`)})
	if err != nil {
		t.Fatalf("tenant B: %v", err)
	}
	roB, err := f.roles.Create(ctx, appB.ID, role.CreateInput{Metadata: json.RawMessage(`{"name":"admin"}`)})
	if err != nil {
		t.Fatalf("role B: %v", err)
	}
	resB, err := f.resources.Create(ctx, appB.ID, resource.CreateInput{Name: "documents"})
	if err != nil {
		t.Fatalf("resource B: %v", err)
	}
	actB, err := f.actions.Create(ctx, appB.ID, action.CreateInput{Name: "read"})
	if err != nil {
		t.Fatalf("action B: %v", err)
	}
	if _, err := f.perms.Create(ctx, appB.ID, roB.ID, permission.CreateInput{ResourceID: resB.ID, ActionID: actB.ID}); err != nil {
		t.Fatalf("perm B: %v", err)
	}
	if _, err := f.assigns.Create(ctx, appB.ID, roleassignments.CreateInput{UserID: uB.ID, RoleID: roB.ID, TenantID: tenB.ID}); err != nil {
		t.Fatalf("assign B: %v", err)
	}

	mustAllow(t, f, appA, CheckInput{UserID: uA.ID, TenantID: tenA.ID, Resource: "documents", Action: "read"})
	mustAllow(t, f, appB.ID, CheckInput{UserID: uB.ID, TenantID: tenB.ID, Resource: "documents", Action: "read"})

	mustDeny(t, f, appA, CheckInput{UserID: uB.ID, TenantID: tenB.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appB.ID, CheckInput{UserID: uA.ID, TenantID: tenA.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appA, CheckInput{UserID: uA.ID, TenantID: tenB.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appB.ID, CheckInput{UserID: uB.ID, TenantID: tenA.ID, Resource: "documents", Action: "read"})
}

// Permissions from multiple assigned roles are unioned (additive RBAC).
func TestSecurity_MultipleRolesUnionPermissions(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u, ten, reader, res, read := f.seed(t, ctx, appID)
	writer, err := f.roles.Create(ctx, appID, role.CreateInput{Metadata: json.RawMessage(`{"name":"writer"}`)})
	if err != nil {
		t.Fatalf("writer role: %v", err)
	}
	write, err := f.actions.Create(ctx, appID, action.CreateInput{Name: "write"})
	if err != nil {
		t.Fatalf("write action: %v", err)
	}

	if _, err := f.perms.Create(ctx, appID, reader.ID, permission.CreateInput{ResourceID: res.ID, ActionID: read.ID}); err != nil {
		t.Fatalf("read perm: %v", err)
	}
	if _, err := f.perms.Create(ctx, appID, writer.ID, permission.CreateInput{ResourceID: res.ID, ActionID: write.ID}); err != nil {
		t.Fatalf("write perm: %v", err)
	}
	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{UserID: u.ID, RoleID: reader.ID, TenantID: ten.ID}); err != nil {
		t.Fatalf("assign reader: %v", err)
	}

	mustAllow(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "write"})

	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{UserID: u.ID, RoleID: writer.ID, TenantID: ten.ID}); err != nil {
		t.Fatalf("assign writer: %v", err)
	}
	mustAllow(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read"})
	mustAllow(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "write"})
}

// Name matching trims whitespace but remains case-sensitive.
func TestSecurity_ResourceActionNameMatching(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u, ten, ro, res, act := f.seed(t, ctx, appID)
	if _, err := f.perms.Create(ctx, appID, ro.ID, permission.CreateInput{ResourceID: res.ID, ActionID: act.ID}); err != nil {
		t.Fatalf("perm: %v", err)
	}
	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{UserID: u.ID, RoleID: ro.ID, TenantID: ten.ID}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	mustAllow(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "  documents  ", Action: " read "})
	mustDeny(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "Documents", Action: "read"})
	mustDeny(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "Read"})

	newName := "files"
	if _, err := f.resources.Update(ctx, appID, res.ID, resource.UpdateInput{Name: &newName}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	mustDeny(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read"})
	mustAllow(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "files", Action: "read"})
}

// Assignment without matching permission, and permission without assignment, both deny.
func TestSecurity_IncompleteAuthzGraphDenied(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u, ten, ro, res, act := f.seed(t, ctx, appID)

	mustDeny(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read"})

	if _, err := f.perms.Create(ctx, appID, ro.ID, permission.CreateInput{ResourceID: res.ID, ActionID: act.ID}); err != nil {
		t.Fatalf("perm: %v", err)
	}
	mustDeny(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read"})

	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{UserID: u.ID, RoleID: ro.ID, TenantID: ten.ID}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	mustAllow(t, f, appID, CheckInput{UserID: u.ID, TenantID: ten.ID, Resource: "documents", Action: "read"})
}

func TestSecurity_NilAndEmptyInputsRejected(t *testing.T) {
	f, _, appID := newFixtures(t)
	uid := uuid.Must(uuid.NewV7())
	tid := uuid.Must(uuid.NewV7())

	cases := []CheckInput{
		{},
		{UserID: uid, TenantID: tid},
		{UserID: uid, TenantID: tid, Resource: "documents"},
		{UserID: uid, TenantID: tid, Action: "read"},
		{UserID: uuid.Nil, TenantID: tid, Resource: "documents", Action: "read"},
		{UserID: uid, TenantID: uuid.Nil, Resource: "documents", Action: "read"},
		{UserID: uid, TenantID: tid, Resource: "   ", Action: "read"},
		{UserID: uid, TenantID: tid, Resource: "documents", Action: "   "},
	}
	for _, in := range cases {
		if _, err := f.access.Check(t.Context(), appID, in); !errors.Is(err, store.ErrInvalidInput) {
			t.Fatalf("input %+v: err=%v", in, err)
		}
	}
	if _, err := f.access.Check(t.Context(), uuid.Nil, CheckInput{
		UserID: uid, TenantID: tid, Resource: "documents", Action: "read",
	}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("nil app: %v", err)
	}
}

// Role assigned in tenant A does not grant access in tenant B (tenant isolation).
func TestSecurity_AssignmentIsTenantScoped(t *testing.T) {
	f, ctx, appID := newFixtures(t)
	u, tenA, ro, res, act := f.seed(t, ctx, appID)
	tenB, err := f.tenants.Create(ctx, appID, tenant.CreateInput{Metadata: json.RawMessage(`{"name":"b"}`)})
	if err != nil {
		t.Fatalf("tenant B: %v", err)
	}
	if _, err := f.perms.Create(ctx, appID, ro.ID, permission.CreateInput{ResourceID: res.ID, ActionID: act.ID}); err != nil {
		t.Fatalf("perm: %v", err)
	}
	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{UserID: u.ID, RoleID: ro.ID, TenantID: tenA.ID}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	mustAllow(t, f, appID, CheckInput{UserID: u.ID, TenantID: tenA.ID, Resource: "documents", Action: "read"})
	mustDeny(t, f, appID, CheckInput{UserID: u.ID, TenantID: tenB.ID, Resource: "documents", Action: "read"})

	if _, err := f.assigns.Create(ctx, appID, roleassignments.CreateInput{UserID: u.ID, RoleID: ro.ID, TenantID: tenB.ID}); err != nil {
		t.Fatalf("assign B: %v", err)
	}
	mustAllow(t, f, appID, CheckInput{UserID: u.ID, TenantID: tenB.ID, Resource: "documents", Action: "read"})
}
