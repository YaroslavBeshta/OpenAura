package httpserver_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/adminapikey"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/apikey"
	"github.com/openaura/openaura/internal/apitest"
	"github.com/openaura/openaura/internal/role"
	"github.com/openaura/openaura/internal/roleassignments"
	"github.com/openaura/openaura/internal/tenant"
	"github.com/openaura/openaura/internal/user"
)

func TestHealthz(t *testing.T) {
	api := apitest.New(t)
	resp := api.Do(http.MethodGet, "/healthz", "", "", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestAPIVersionRequired(t *testing.T) {
	api := apitest.New(t)

	resp := api.Do(http.MethodGet, "/users", "", "", nil)
	if got := apitest.DecodeError(t, resp); resp.StatusCode != http.StatusBadRequest || got == "" {
		t.Fatalf("missing version: status=%d error=%q", resp.StatusCode, got)
	}

	var errBody map[string]string
	status := api.JSONVersion(http.MethodGet, "/users", "99", nil, &errBody)
	if status != http.StatusNotAcceptable {
		t.Fatalf("unsupported version status = %d", status)
	}

	resp = api.Do(http.MethodGet, "/users", "1", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("missing api key status=%d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestUsersAPI_CRUDPaginationAndErrors(t *testing.T) {
	api := apitest.New(t)

	var created user.User
	status := api.JSON(http.MethodPost, "/users", map[string]any{
		"email":    " Ada@Example.COM ",
		"metadata": map[string]any{"name": "Ada"},
	}, &created)
	if status != http.StatusCreated {
		t.Fatalf("create status = %d", status)
	}
	if created.Email != "ada@example.com" || created.ID == uuid.Nil {
		t.Fatalf("unexpected user: %+v", created)
	}

	var got user.User
	status = api.JSON(http.MethodGet, "/users/"+created.ID.String(), nil, &got)
	if status != http.StatusOK || got.ID != created.ID {
		t.Fatalf("get status=%d user=%+v", status, got)
	}

	var updated user.User
	status = api.JSON(http.MethodPatch, "/users/"+created.ID.String(), map[string]any{
		"email":    "lovelace@example.com",
		"metadata": map[string]any{"title": "Countess"},
	}, &updated)
	if status != http.StatusOK || updated.Email != "lovelace@example.com" {
		t.Fatalf("update status=%d user=%+v", status, updated)
	}

	// Seed more users for pagination (newest first).
	var ids []uuid.UUID
	ids = append(ids, created.ID)
	for i := 0; i < 4; i++ {
		var u user.User
		status = api.JSON(http.MethodPost, "/users", map[string]any{
			"email": fmt.Sprintf("user-%d-%s@example.com", i, uuid.NewString()[:8]),
		}, &u)
		if status != http.StatusCreated {
			t.Fatalf("seed create %d: %d", i, status)
		}
		ids = append(ids, u.ID)
		time.Sleep(2 * time.Millisecond)
	}

	status = api.JSON(http.MethodDelete, "/users/"+ids[2].String(), nil, nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete status = %d", status)
	}

	var page user.ListResponse
	status = api.JSON(http.MethodGet, "/users?limit=2&offset=0", nil, &page)
	if status != http.StatusOK || len(page.Users) != 2 {
		t.Fatalf("page1 status=%d len=%d", status, len(page.Users))
	}

	status = api.JSON(http.MethodGet, "/users?limit=50", nil, &page)
	if status != http.StatusOK || len(page.Users) != 4 {
		t.Fatalf("list active status=%d len=%d want 4", status, len(page.Users))
	}
	for _, u := range page.Users {
		if u.ID == ids[2] {
			t.Fatal("soft-deleted user visible in list")
		}
	}

	status = api.JSON(http.MethodGet, "/users/"+ids[2].String(), nil, nil)
	if status != http.StatusNotFound {
		t.Fatalf("get deleted status = %d", status)
	}

	status = api.JSON(http.MethodPost, "/users", map[string]any{
		"email": "lovelace@example.com",
	}, nil)
	if status != http.StatusConflict {
		t.Fatalf("duplicate status = %d", status)
	}

	status = api.JSON(http.MethodPost, "/users", map[string]any{
		"email": "not-an-email",
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid email status = %d", status)
	}

	status = api.JSON(http.MethodGet, "/users/not-a-uuid", nil, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("bad id status = %d", status)
	}
}

func TestTenantsAPI_CRUDAndPagination(t *testing.T) {
	api := apitest.New(t)

	var created tenant.Tenant
	status := api.JSON(http.MethodPost, "/tenants", map[string]any{
		"metadata": map[string]any{"name": "acme"},
	}, &created)
	if status != http.StatusCreated || created.ID == uuid.Nil {
		t.Fatalf("create status=%d tenant=%+v", status, created)
	}

	var got tenant.Tenant
	status = api.JSON(http.MethodGet, "/tenants/"+created.ID.String(), nil, &got)
	if status != http.StatusOK || got.ID != created.ID {
		t.Fatalf("get status=%d", status)
	}

	var updated tenant.Tenant
	status = api.JSON(http.MethodPatch, "/tenants/"+created.ID.String(), map[string]any{
		"metadata": map[string]any{"name": "acme", "plan": "pro"},
	}, &updated)
	if status != http.StatusOK {
		t.Fatalf("update status=%d", status)
	}
	assertJSONContains(t, updated.Metadata, "plan", "pro")

	var deletedID uuid.UUID
	for i := 0; i < 4; i++ {
		var ten tenant.Tenant
		status = api.JSON(http.MethodPost, "/tenants", map[string]any{
			"metadata": map[string]any{"n": i},
		}, &ten)
		if status != http.StatusCreated {
			t.Fatalf("seed %d: %d", i, status)
		}
		if i == 1 {
			deletedID = ten.ID
		}
		time.Sleep(2 * time.Millisecond)
	}
	status = api.JSON(http.MethodDelete, "/tenants/"+deletedID.String(), nil, nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete status=%d", status)
	}

	var page tenant.ListResponse
	status = api.JSON(http.MethodGet, "/tenants?limit=2&offset=0", nil, &page)
	if status != http.StatusOK || len(page.Tenants) != 2 {
		t.Fatalf("page status=%d len=%d", status, len(page.Tenants))
	}
	status = api.JSON(http.MethodGet, "/tenants?limit=50", nil, &page)
	if status != http.StatusOK || len(page.Tenants) != 4 {
		t.Fatalf("list status=%d len=%d want 4", status, len(page.Tenants))
	}
}

func TestRolesAPI_CRUDAndPagination(t *testing.T) {
	api := apitest.New(t)

	var created role.Role
	status := api.JSON(http.MethodPost, "/roles", map[string]any{
		"metadata": map[string]any{"name": "admin"},
	}, &created)
	if status != http.StatusCreated || created.ID == uuid.Nil {
		t.Fatalf("create status=%d", status)
	}

	var got role.Role
	status = api.JSON(http.MethodGet, "/roles/"+created.ID.String(), nil, &got)
	if status != http.StatusOK || got.ID != created.ID {
		t.Fatalf("get status=%d", status)
	}

	var updated role.Role
	status = api.JSON(http.MethodPatch, "/roles/"+created.ID.String(), map[string]any{
		"metadata": map[string]any{"name": "admin", "level": 1},
	}, &updated)
	if status != http.StatusOK {
		t.Fatalf("update status=%d", status)
	}

	var deletedID uuid.UUID
	for i := 0; i < 4; i++ {
		var r role.Role
		status = api.JSON(http.MethodPost, "/roles", map[string]any{
			"metadata": map[string]any{"n": i},
		}, &r)
		if status != http.StatusCreated {
			t.Fatalf("seed %d: %d", i, status)
		}
		if i == 0 {
			deletedID = r.ID
		}
		time.Sleep(2 * time.Millisecond)
	}
	if status := api.JSON(http.MethodDelete, "/roles/"+deletedID.String(), nil, nil); status != http.StatusNoContent {
		t.Fatalf("delete status=%d", status)
	}

	var page role.ListResponse
	status = api.JSON(http.MethodGet, "/roles?limit=2&offset=1", nil, &page)
	if status != http.StatusOK || len(page.Roles) != 2 {
		t.Fatalf("page status=%d len=%d", status, len(page.Roles))
	}
	status = api.JSON(http.MethodGet, "/roles?limit=50", nil, &page)
	if status != http.StatusOK || len(page.Roles) != 4 {
		t.Fatalf("list status=%d len=%d want 4", status, len(page.Roles))
	}

	status = api.JSON(http.MethodGet, "/roles/"+deletedID.String(), nil, nil)
	if status != http.StatusNotFound {
		t.Fatalf("get deleted status=%d", status)
	}
}

func TestRoleAssignmentsAPI_CRUDFiltersAndErrors(t *testing.T) {
	api := apitest.New(t)

	u1 := createUser(t, api, "u1@example.com")
	u2 := createUser(t, api, "u2@example.com")
	t1 := createTenant(t, api, "t1")
	t2 := createTenant(t, api, "t2")
	r1 := createRole(t, api, "r1")
	r2 := createRole(t, api, "r2")

	var created roleassignments.RoleAssignment
	status := api.JSON(http.MethodPost, "/roleassignments", map[string]any{
		"user_id":   u1.ID,
		"role_id":   r1.ID,
		"tenant_id": t1.ID,
	}, &created)
	if status != http.StatusCreated || created.ID == uuid.Nil {
		t.Fatalf("create status=%d", status)
	}

	var got roleassignments.RoleAssignment
	status = api.JSON(http.MethodGet, "/roleassignments/"+created.ID.String(), nil, &got)
	if status != http.StatusOK || got.RoleID != r1.ID {
		t.Fatalf("get status=%d assignment=%+v", status, got)
	}

	var updated roleassignments.RoleAssignment
	status = api.JSON(http.MethodPatch, "/roleassignments/"+created.ID.String(), map[string]any{
		"role_id": r2.ID,
	}, &updated)
	if status != http.StatusOK || updated.RoleID != r2.ID {
		t.Fatalf("update status=%d assignment=%+v", status, updated)
	}

	seeds := []struct {
		user, role, tenant uuid.UUID
	}{
		{u1.ID, r1.ID, t1.ID},
		{u2.ID, r1.ID, t1.ID},
		{u2.ID, r1.ID, t2.ID},
		{u2.ID, r2.ID, t2.ID},
	}
	for i, s := range seeds {
		var a roleassignments.RoleAssignment
		status = api.JSON(http.MethodPost, "/roleassignments", map[string]any{
			"user_id":   s.user,
			"role_id":   s.role,
			"tenant_id": s.tenant,
		}, &a)
		if status != http.StatusCreated {
			t.Fatalf("seed %d status=%d", i, status)
		}
		time.Sleep(2 * time.Millisecond)
	}

	status = api.JSON(http.MethodDelete, "/roleassignments/"+created.ID.String(), nil, nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete status=%d", status)
	}

	var list roleassignments.ListResponse
	status = api.JSON(http.MethodGet, "/roleassignments?user_id="+u1.ID.String(), nil, &list)
	if status != http.StatusOK || len(list.RoleAssignments) != 1 {
		t.Fatalf("filter user status=%d len=%d", status, len(list.RoleAssignments))
	}

	status = api.JSON(http.MethodGet, "/roleassignments?tenant_id="+t2.ID.String(), nil, &list)
	if status != http.StatusOK || len(list.RoleAssignments) != 2 {
		t.Fatalf("filter tenant status=%d len=%d", status, len(list.RoleAssignments))
	}

	status = api.JSON(http.MethodGet, "/roleassignments?role_id="+r1.ID.String(), nil, &list)
	if status != http.StatusOK || len(list.RoleAssignments) != 3 {
		t.Fatalf("filter role status=%d len=%d", status, len(list.RoleAssignments))
	}

	q := fmt.Sprintf("/roleassignments?user_id=%s&role_id=%s&tenant_id=%s", u2.ID, r1.ID, t2.ID)
	status = api.JSON(http.MethodGet, q, nil, &list)
	if status != http.StatusOK || len(list.RoleAssignments) != 1 {
		t.Fatalf("combined filter status=%d len=%d", status, len(list.RoleAssignments))
	}

	status = api.JSON(http.MethodGet, "/roleassignments?limit=2&offset=0", nil, &list)
	if status != http.StatusOK || len(list.RoleAssignments) != 2 {
		t.Fatalf("page status=%d len=%d", status, len(list.RoleAssignments))
	}

	status = api.JSON(http.MethodGet, "/roleassignments?limit=50", nil, &list)
	if status != http.StatusOK || len(list.RoleAssignments) != 4 {
		t.Fatalf("list status=%d len=%d want 4", status, len(list.RoleAssignments))
	}

	// Conflict on unique active assignment.
	status = api.JSON(http.MethodPost, "/roleassignments", map[string]any{
		"user_id":   u2.ID,
		"role_id":   r1.ID,
		"tenant_id": t1.ID,
	}, nil)
	if status != http.StatusConflict {
		t.Fatalf("conflict status=%d", status)
	}

	// FK violation.
	status = api.JSON(http.MethodPost, "/roleassignments", map[string]any{
		"user_id":   uuid.Must(uuid.NewV7()),
		"role_id":   r1.ID,
		"tenant_id": t1.ID,
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("fk status=%d", status)
	}

	// Soft-deleted slot can be reused.
	status = api.JSON(http.MethodPost, "/roleassignments", map[string]any{
		"user_id":   u1.ID,
		"role_id":   r2.ID,
		"tenant_id": t1.ID,
	}, &created)
	if status != http.StatusCreated {
		t.Fatalf("recreate after delete status=%d", status)
	}
}

func createUser(t *testing.T, api *apitest.API, email string) user.User {
	t.Helper()
	var u user.User
	status := api.JSON(http.MethodPost, "/users", map[string]any{"email": email}, &u)
	if status != http.StatusCreated {
		t.Fatalf("create user: %d", status)
	}
	return u
}

func createTenant(t *testing.T, api *apitest.API, name string) tenant.Tenant {
	t.Helper()
	var ten tenant.Tenant
	status := api.JSON(http.MethodPost, "/tenants", map[string]any{
		"metadata": map[string]any{"name": name},
	}, &ten)
	if status != http.StatusCreated {
		t.Fatalf("create tenant: %d", status)
	}
	return ten
}

func createRole(t *testing.T, api *apitest.API, name string) role.Role {
	t.Helper()
	var r role.Role
	status := api.JSON(http.MethodPost, "/roles", map[string]any{
		"metadata": map[string]any{"name": name},
	}, &r)
	if status != http.StatusCreated {
		t.Fatalf("create role: %d", status)
	}
	return r
}

func assertJSONContains(t *testing.T, raw json.RawMessage, key string, want any) {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	got, ok := obj[key]
	if !ok {
		t.Fatalf("missing key %q in %s", key, raw)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("%s = %s, want %s", key, gotJSON, wantJSON)
	}
}

func TestAdminAppsAPI_CRUDAndPagination(t *testing.T) {
	api := apitest.New(t)

	var created app.App
	status := api.AdminJSON(http.MethodPost, "/admin/apps", map[string]any{
		"name":     "Acme",
		"metadata": map[string]any{"tier": "gold"},
	}, &created)
	if status != http.StatusCreated || created.ID == uuid.Nil || created.Name != "Acme" {
		t.Fatalf("create status=%d app=%+v", status, created)
	}

	var got app.App
	status = api.AdminJSON(http.MethodGet, "/admin/apps/"+created.ID.String(), nil, &got)
	if status != http.StatusOK || got.ID != created.ID {
		t.Fatalf("get status=%d", status)
	}

	var updated app.App
	status = api.AdminJSON(http.MethodPatch, "/admin/apps/"+created.ID.String(), map[string]any{
		"name": "Acme Inc",
	}, &updated)
	if status != http.StatusOK || updated.Name != "Acme Inc" {
		t.Fatalf("update status=%d app=%+v", status, updated)
	}

	var deletedID uuid.UUID
	for i := 0; i < 4; i++ {
		var a app.App
		status = api.AdminJSON(http.MethodPost, "/admin/apps", map[string]any{
			"name": fmt.Sprintf("extra-%d-%s", i, uuid.NewString()[:8]),
		}, &a)
		if status != http.StatusCreated {
			t.Fatalf("seed %d: %d", i, status)
		}
		if i == 1 {
			deletedID = a.ID
		}
		time.Sleep(2 * time.Millisecond)
	}
	if status := api.AdminJSON(http.MethodDelete, "/admin/apps/"+deletedID.String(), nil, nil); status != http.StatusNoContent {
		t.Fatalf("delete status=%d", status)
	}

	var page app.ListResponse
	status = api.AdminJSON(http.MethodGet, "/admin/apps?limit=2&offset=0", nil, &page)
	if status != http.StatusOK || len(page.Apps) != 2 {
		t.Fatalf("page status=%d len=%d", status, len(page.Apps))
	}

	// New() already created one app; plus created + 3 remaining extras = 5
	status = api.AdminJSON(http.MethodGet, "/admin/apps?limit=50", nil, &page)
	if status != http.StatusOK || len(page.Apps) != 5 {
		t.Fatalf("list status=%d len=%d want 5", status, len(page.Apps))
	}

	status = api.JSON(http.MethodGet, "/admin/apps", nil, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("app key on admin route status=%d", status)
	}
}

func TestAdminAPIKeysAPI_CRUD(t *testing.T) {
	api := apitest.New(t)

	var created adminapikey.AdminAPIKey
	status := api.AdminJSON(http.MethodPost, "/admin/api_keys", map[string]any{
		"name": "ops",
	}, &created)
	if status != http.StatusCreated || created.Key == "" {
		t.Fatalf("create status=%d key=%q", status, created.Key)
	}
	raw := created.Key

	var got adminapikey.AdminAPIKey
	status = api.AdminJSON(http.MethodGet, "/admin/api_keys/"+created.ID.String(), nil, &got)
	if status != http.StatusOK || got.Key != "" {
		t.Fatalf("get status=%d key=%q", status, got.Key)
	}

	var list adminapikey.ListResponse
	status = api.AdminJSON(http.MethodGet, "/admin/api_keys?limit=50", nil, &list)
	if status != http.StatusOK || len(list.AdminAPIKeys) < 2 {
		t.Fatalf("list status=%d len=%d", status, len(list.AdminAPIKeys))
	}

	if status := api.AdminJSON(http.MethodDelete, "/admin/api_keys/"+created.ID.String(), nil, nil); status != http.StatusNoContent {
		t.Fatalf("delete status=%d", status)
	}
	status = api.AdminJSON(http.MethodGet, "/admin/api_keys/"+created.ID.String(), nil, nil)
	if status != http.StatusNotFound {
		t.Fatalf("get revoked status=%d", status)
	}

	// Revoked key cannot access admin routes.
	resp := api.Do(http.MethodGet, "/admin/apps", "1", raw, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("revoked key status=%d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAppAPIKeysAPI_CRUDAndIsolation(t *testing.T) {
	api := apitest.New(t)

	var created apikey.APIKey
	status := api.JSON(http.MethodPost, "/api_keys", map[string]any{
		"name": "rotate",
	}, &created)
	if status != http.StatusCreated || created.Key == "" || created.AppID != api.AppID {
		t.Fatalf("create status=%d key=%+v", status, created)
	}
	newKey := created.Key

	var got apikey.APIKey
	status = api.JSON(http.MethodGet, "/api_keys/"+created.ID.String(), nil, &got)
	if status != http.StatusOK || got.Key != "" {
		t.Fatalf("get status=%d key=%q", status, got.Key)
	}

	var list apikey.ListResponse
	status = api.JSON(http.MethodGet, "/api_keys?limit=50", nil, &list)
	if status != http.StatusOK || len(list.APIKeys) < 2 {
		t.Fatalf("list status=%d len=%d", status, len(list.APIKeys))
	}

	// New key can authenticate app routes.
	resp := api.Do(http.MethodGet, "/users", "1", newKey, nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("new key auth status=%d", resp.StatusCode)
	}
	resp.Body.Close()

	if status := api.JSON(http.MethodDelete, "/api_keys/"+created.ID.String(), nil, nil); status != http.StatusNoContent {
		t.Fatalf("revoke status=%d", status)
	}
	resp = api.Do(http.MethodGet, "/users", "1", newKey, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("revoked app key status=%d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestUsersAPI_EmailUniquePerApp(t *testing.T) {
	api := apitest.New(t)

	status := api.JSON(http.MethodPost, "/users", map[string]any{"email": "shared@example.com"}, nil)
	if status != http.StatusCreated {
		t.Fatalf("create user app1: %d", status)
	}
	status = api.JSON(http.MethodPost, "/users", map[string]any{"email": "shared@example.com"}, nil)
	if status != http.StatusConflict {
		t.Fatalf("dup in same app: %d", status)
	}

	// Second app can reuse the email.
	var other app.App
	status = api.AdminJSON(http.MethodPost, "/admin/apps", map[string]any{"name": "other-app"}, &other)
	if status != http.StatusCreated {
		t.Fatalf("create other app: %d", status)
	}
	var otherKey apikey.APIKey
	status = api.AdminJSON(http.MethodPost, "/admin/apps/"+other.ID.String()+"/api_keys", map[string]any{}, &otherKey)
	if status != http.StatusCreated || otherKey.Key == "" {
		t.Fatalf("create other key: %d", status)
	}

	resp := api.Do(http.MethodPost, "/users", "1", otherKey.Key, map[string]any{"email": "shared@example.com"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("same email other app status=%d", resp.StatusCode)
	}
}
