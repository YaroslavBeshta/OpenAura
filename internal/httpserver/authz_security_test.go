package httpserver_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/access"
	"github.com/openaura/openaura/internal/action"
	"github.com/openaura/openaura/internal/apikey"
	"github.com/openaura/openaura/internal/apitest"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/permission"
	"github.com/openaura/openaura/internal/resource"
	"github.com/openaura/openaura/internal/roleassignments"
	"github.com/openaura/openaura/internal/user"
)

func createSecondAppWithKey(t *testing.T, api *apitest.API) (app.App, string) {
	t.Helper()
	var other app.App
	status := api.AdminJSON(http.MethodPost, "/admin/apps", map[string]any{"name": "other-" + uuid.NewString()[:8]}, &other)
	if status != http.StatusCreated {
		t.Fatalf("create other app: %d", status)
	}
	var key apikey.APIKey
	status = api.AdminJSON(http.MethodPost, "/admin/apps/"+other.ID.String()+"/api_keys", map[string]any{"name": "other"}, &key)
	if status != http.StatusCreated || key.Key == "" {
		t.Fatalf("create other key: %d", status)
	}
	return other, key.Key
}

// Vertical privilege: app keys must not reach admin surfaces; admin keys must not auth as app keys.
func TestSecurity_KeyTypeBoundary(t *testing.T) {
	api := apitest.New(t)

	status := api.JSON(http.MethodGet, "/admin/apps", nil, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("app key on admin list status=%d", status)
	}
	status = api.JSON(http.MethodPost, "/admin/apps", map[string]any{"name": "x"}, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("app key on admin create status=%d", status)
	}

	status = api.AdminJSON(http.MethodGet, "/users", nil, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("admin key on app route status=%d", status)
	}
	status = api.AdminJSON(http.MethodPost, "/access/check", map[string]any{
		"user_id": uuid.Must(uuid.NewV7()), "tenant_id": uuid.Must(uuid.NewV7()),
		"resource": "documents", "action": "read",
	}, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("admin key on access check status=%d", status)
	}
}

func TestSecurity_BearerAuthAccepted(t *testing.T) {
	api := apitest.New(t)
	resp := api.DoBearer(http.MethodGet, "/users", "1", api.AppKey, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bearer status=%d", resp.StatusCode)
	}
}

func TestSecurity_RevokedAndDeletedAppKeysRejected(t *testing.T) {
	api := apitest.New(t)

	var created apikey.APIKey
	status := api.JSON(http.MethodPost, "/api_keys", map[string]any{"name": "temp"}, &created)
	if status != http.StatusCreated {
		t.Fatalf("create key: %d", status)
	}
	status = api.JSONWithKey(http.MethodGet, "/users", created.Key, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("new key auth: %d", status)
	}
	if status := api.JSON(http.MethodDelete, "/api_keys/"+created.ID.String(), nil, nil); status != http.StatusNoContent {
		t.Fatalf("revoke: %d", status)
	}
	status = api.JSONWithKey(http.MethodGet, "/users", created.Key, nil, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("revoked key: %d", status)
	}

	other, otherKey := createSecondAppWithKey(t, api)
	if status := api.AdminJSON(http.MethodDelete, "/admin/apps/"+other.ID.String(), nil, nil); status != http.StatusNoContent {
		t.Fatalf("delete app: %d", status)
	}
	status = api.JSONWithKey(http.MethodGet, "/users", otherKey, nil, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("deleted-app key: %d", status)
	}
}

// IDOR: one app's key must not read/write another app's objects by UUID.
func TestSecurity_CrossAppIDOR(t *testing.T) {
	api := apitest.New(t)
	u := createUser(t, api, "owner@example.com")
	ten := createTenant(t, api, "t1")
	ro := createRole(t, api, "admin")
	res := createResource(t, api, "documents")
	act := createAction(t, api, "read")

	var perm permission.Permission
	status := api.JSON(http.MethodPost, "/roles/"+ro.ID.String()+"/permissions", map[string]any{
		"resource_id": res.ID, "action_id": act.ID,
	}, &perm)
	if status != http.StatusCreated {
		t.Fatalf("perm: %d", status)
	}
	var asg roleassignments.RoleAssignment
	status = api.JSON(http.MethodPost, "/roleassignments", map[string]any{
		"user_id": u.ID, "role_id": ro.ID, "tenant_id": ten.ID,
	}, &asg)
	if status != http.StatusCreated {
		t.Fatalf("assign: %d", status)
	}

	_, otherKey := createSecondAppWithKey(t, api)

	paths := []string{
		"/users/" + u.ID.String(),
		"/tenants/" + ten.ID.String(),
		"/roles/" + ro.ID.String(),
		"/resources/" + res.ID.String(),
		"/actions/" + act.ID.String(),
		"/roleassignments/" + asg.ID.String(),
		"/roles/" + ro.ID.String() + "/permissions/" + perm.ID.String(),
	}
	for _, path := range paths {
		status = api.JSONWithKey(http.MethodGet, path, otherKey, nil, nil)
		if status != http.StatusNotFound {
			t.Fatalf("IDOR get %s status=%d", path, status)
		}
	}

	// Mutations against foreign IDs must fail.
	status = api.JSONWithKey(http.MethodPatch, "/users/"+u.ID.String(), otherKey, map[string]any{
		"email": "hacked@example.com",
	}, nil)
	if status != http.StatusNotFound {
		t.Fatalf("IDOR patch user status=%d", status)
	}
	status = api.JSONWithKey(http.MethodDelete, "/resources/"+res.ID.String(), otherKey, nil, nil)
	if status != http.StatusNotFound {
		t.Fatalf("IDOR delete resource status=%d", status)
	}
	status = api.JSONWithKey(http.MethodPost, "/roles/"+ro.ID.String()+"/permissions", otherKey, map[string]any{
		"resource_id": res.ID, "action_id": act.ID,
	}, nil)
	if status != http.StatusBadRequest && status != http.StatusNotFound {
		t.Fatalf("IDOR create permission status=%d", status)
	}

	// Victim objects must remain intact.
	var still user.User
	status = api.JSON(http.MethodGet, "/users/"+u.ID.String(), nil, &still)
	if status != http.StatusOK || still.Email != "owner@example.com" {
		t.Fatalf("victim user mutated: status=%d email=%q", status, still.Email)
	}
}

// Access check must not allow using another app's principals under this app's key.
func TestSecurity_AccessCheckCrossAppPrincipalsDenied(t *testing.T) {
	api := apitest.New(t)
	u := createUser(t, api, "a@example.com")
	ten := createTenant(t, api, "a")
	ro := createRole(t, api, "admin")
	res := createResource(t, api, "documents")
	act := createAction(t, api, "read")
	status := api.JSON(http.MethodPost, "/roles/"+ro.ID.String()+"/permissions", map[string]any{
		"resource_id": res.ID, "action_id": act.ID,
	}, nil)
	if status != http.StatusCreated {
		t.Fatalf("perm: %d", status)
	}
	status = api.JSON(http.MethodPost, "/roleassignments", map[string]any{
		"user_id": u.ID, "role_id": ro.ID, "tenant_id": ten.ID,
	}, nil)
	if status != http.StatusCreated {
		t.Fatalf("assign: %d", status)
	}

	_, otherKey := createSecondAppWithKey(t, api)

	// Other app creates identically named resource/action but no grant for victim IDs.
	status = api.JSONWithKey(http.MethodPost, "/resources", otherKey, map[string]any{"name": "documents"}, nil)
	if status != http.StatusCreated {
		t.Fatalf("other resource: %d", status)
	}
	status = api.JSONWithKey(http.MethodPost, "/actions", otherKey, map[string]any{"name": "read"}, nil)
	if status != http.StatusCreated {
		t.Fatalf("other action: %d", status)
	}

	var check access.CheckResponse
	status = api.JSONWithKey(http.MethodPost, "/access/check", otherKey, map[string]any{
		"user_id": u.ID, "tenant_id": ten.ID, "resource": "documents", "action": "read",
	}, &check)
	if status != http.StatusOK || check.Allowed {
		t.Fatalf("cross-app access check status=%d allowed=%v", status, check.Allowed)
	}

	status = api.JSON(http.MethodPost, "/access/check", map[string]any{
		"user_id": u.ID, "tenant_id": ten.ID, "resource": "documents", "action": "read",
	}, &check)
	if status != http.StatusOK || !check.Allowed {
		t.Fatalf("same-app allow status=%d allowed=%v", status, check.Allowed)
	}
}

func TestSecurity_AccessCheckSoftDeletedUserDenied(t *testing.T) {
	api := apitest.New(t)
	u := createUser(t, api, "soon-gone@example.com")
	ten := createTenant(t, api, "t")
	ro := createRole(t, api, "admin")
	res := createResource(t, api, "documents")
	act := createAction(t, api, "read")
	if status := api.JSON(http.MethodPost, "/roles/"+ro.ID.String()+"/permissions", map[string]any{
		"resource_id": res.ID, "action_id": act.ID,
	}, nil); status != http.StatusCreated {
		t.Fatalf("perm: %d", status)
	}
	if status := api.JSON(http.MethodPost, "/roleassignments", map[string]any{
		"user_id": u.ID, "role_id": ro.ID, "tenant_id": ten.ID,
	}, nil); status != http.StatusCreated {
		t.Fatalf("assign: %d", status)
	}

	var check access.CheckResponse
	status := api.JSON(http.MethodPost, "/access/check", map[string]any{
		"user_id": u.ID, "tenant_id": ten.ID, "resource": "documents", "action": "read",
	}, &check)
	if status != http.StatusOK || !check.Allowed {
		t.Fatalf("pre-delete allow status=%d allowed=%v", status, check.Allowed)
	}

	if status := api.JSON(http.MethodDelete, "/users/"+u.ID.String(), nil, nil); status != http.StatusNoContent {
		t.Fatalf("delete user: %d", status)
	}
	status = api.JSON(http.MethodPost, "/access/check", map[string]any{
		"user_id": u.ID, "tenant_id": ten.ID, "resource": "documents", "action": "read",
	}, &check)
	if status != http.StatusOK || check.Allowed {
		t.Fatalf("post-delete deny status=%d allowed=%v", status, check.Allowed)
	}
}

func TestSecurity_AccessCheckSoftDeletedResourceDenied(t *testing.T) {
	api := apitest.New(t)
	u := createUser(t, api, "u@example.com")
	ten := createTenant(t, api, "t")
	ro := createRole(t, api, "admin")
	res := createResource(t, api, "documents")
	act := createAction(t, api, "read")
	if status := api.JSON(http.MethodPost, "/roles/"+ro.ID.String()+"/permissions", map[string]any{
		"resource_id": res.ID, "action_id": act.ID,
	}, nil); status != http.StatusCreated {
		t.Fatalf("perm: %d", status)
	}
	if status := api.JSON(http.MethodPost, "/roleassignments", map[string]any{
		"user_id": u.ID, "role_id": ro.ID, "tenant_id": ten.ID,
	}, nil); status != http.StatusCreated {
		t.Fatalf("assign: %d", status)
	}

	if status := api.JSON(http.MethodDelete, "/resources/"+res.ID.String(), nil, nil); status != http.StatusNoContent {
		t.Fatalf("delete resource: %d", status)
	}
	var check access.CheckResponse
	status := api.JSON(http.MethodPost, "/access/check", map[string]any{
		"user_id": u.ID, "tenant_id": ten.ID, "resource": "documents", "action": "read",
	}, &check)
	if status != http.StatusOK || check.Allowed {
		t.Fatalf("deleted resource still allowed status=%d allowed=%v", status, check.Allowed)
	}
}

func TestSecurity_PermissionCreateRejectsForeignIDs(t *testing.T) {
	api := apitest.New(t)
	ro := createRole(t, api, "admin")
	res := createResource(t, api, "documents")
	act := createAction(t, api, "read")

	_, otherKey := createSecondAppWithKey(t, api)
	var otherRes resource.Resource
	status := api.JSONWithKey(http.MethodPost, "/resources", otherKey, map[string]any{"name": "documents"}, &otherRes)
	if status != http.StatusCreated {
		t.Fatalf("other resource: %d", status)
	}
	var otherAct action.Action
	status = api.JSONWithKey(http.MethodPost, "/actions", otherKey, map[string]any{"name": "read"}, &otherAct)
	if status != http.StatusCreated {
		t.Fatalf("other action: %d", status)
	}

	status = api.JSON(http.MethodPost, "/roles/"+ro.ID.String()+"/permissions", map[string]any{
		"resource_id": otherRes.ID, "action_id": act.ID,
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("foreign resource perm status=%d", status)
	}
	status = api.JSON(http.MethodPost, "/roles/"+ro.ID.String()+"/permissions", map[string]any{
		"resource_id": res.ID, "action_id": otherAct.ID,
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("foreign action perm status=%d", status)
	}
}

func TestSecurity_MalformedIDsRejected(t *testing.T) {
	api := apitest.New(t)
	for _, path := range []string{
		"/users/not-a-uuid",
		"/resources/not-a-uuid",
		"/roles/not-a-uuid/permissions",
		"/roles/" + uuid.Must(uuid.NewV7()).String() + "/permissions/not-a-uuid",
		"/access/check",
	} {
		method := http.MethodGet
		var body any
		if path == "/access/check" {
			method = http.MethodPost
			body = map[string]any{"user_id": "bad", "tenant_id": "bad", "resource": "x", "action": "y"}
		}
		status := api.JSON(method, path, body, nil)
		if status != http.StatusBadRequest {
			t.Fatalf("%s %s status=%d", method, path, status)
		}
	}
}

func TestSecurity_UnauthenticatedAndMissingVersion(t *testing.T) {
	api := apitest.New(t)

	resp := api.Do(http.MethodGet, "/users", "1", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("missing key status=%d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = api.Do(http.MethodPost, "/access/check", "1", "", map[string]any{
		"user_id": uuid.Must(uuid.NewV7()), "tenant_id": uuid.Must(uuid.NewV7()),
		"resource": "documents", "action": "read",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("access check without key status=%d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = api.Do(http.MethodGet, "/users", "", api.AppKey, nil)
	if resp.StatusCode != http.StatusBadRequest {
		resp.Body.Close()
		t.Fatalf("missing version status=%d", resp.StatusCode)
	}
	resp.Body.Close()
}
