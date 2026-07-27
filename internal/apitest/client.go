package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/adminapikey"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/apikey"
	"github.com/openaura/openaura/internal/httpserver"
	"github.com/openaura/openaura/internal/role"
	"github.com/openaura/openaura/internal/roleassignments"
	"github.com/openaura/openaura/internal/tenant"
	"github.com/openaura/openaura/internal/testutil"
	"github.com/openaura/openaura/internal/user"
)

// API is an httptest-backed OpenAura server for integration tests.
type API struct {
	t         *testing.T
	server    *httptest.Server
	Pool      *pgxpool.Pool
	AdminKey  string
	AppKey    string
	AppID     uuid.UUID
}

// New wires the full server against a clean database with bootstrap keys.
func New(t *testing.T) *API {
	t.Helper()

	pool := testutil.Pool(t)
	testutil.Reset(t, pool)

	appRepo := app.NewRepository(pool)
	userRepo := user.NewRepository(pool)
	tenantRepo := tenant.NewRepository(pool)
	roleRepo := role.NewRepository(pool)
	assignmentRepo := roleassignments.NewRepository(pool)
	apiKeyRepo := apikey.NewRepository(pool)
	adminKeyRepo := adminapikey.NewRepository(pool)

	adminKey := "oa_admin_test_bootstrap_key_0001"
	if err := adminKeyRepo.EnsureBootstrapKey(context.Background(), adminKey, "test"); err != nil {
		t.Fatalf("bootstrap admin key: %v", err)
	}

	handler := httpserver.New(httpserver.Handlers{
		Apps:            app.NewHandler(appRepo),
		Users:           user.NewHandler(userRepo),
		Tenants:         tenant.NewHandler(tenantRepo),
		Roles:           role.NewHandler(roleRepo),
		RoleAssignments: roleassignments.NewHandler(assignmentRepo),
		APIKeys:         apikey.NewHandler(apiKeyRepo),
		AdminAPIKeys:    adminapikey.NewHandler(adminKeyRepo),
	}, httpserver.KeyLookups{
		App:   apiKeyRepo,
		Admin: adminKeyRepo,
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	api := &API{t: t, server: server, Pool: pool, AdminKey: adminKey}

	var created app.App
	status := api.adminJSON(http.MethodPost, "/admin/apps", map[string]any{
		"name": "test-app",
	}, &created)
	if status != http.StatusCreated {
		t.Fatalf("create app status=%d", status)
	}
	api.AppID = created.ID

	var key apikey.APIKey
	status = api.adminJSON(http.MethodPost, "/admin/apps/"+created.ID.String()+"/api_keys", map[string]any{
		"name": "test",
	}, &key)
	if status != http.StatusCreated || key.Key == "" {
		t.Fatalf("create app key status=%d key=%q", status, key.Key)
	}
	api.AppKey = key.Key

	return api
}

func (a *API) Do(method, path, apiVersion, apiKey string, body any) *http.Response {
	a.t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, a.server.URL+path, reader)
	if err != nil {
		a.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiVersion != "" {
		req.Header.Set("X-API-Version", apiVersion)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatalf("do request: %v", err)
	}
	return resp
}

func (a *API) JSON(method, path string, body, out any) int {
	a.t.Helper()
	return a.jsonWithKey(method, path, "1", a.AppKey, body, out)
}

func (a *API) AdminJSON(method, path string, body, out any) int {
	a.t.Helper()
	return a.adminJSON(method, path, body, out)
}

func (a *API) adminJSON(method, path string, body, out any) int {
	a.t.Helper()
	return a.jsonWithKey(method, path, "1", a.AdminKey, body, out)
}

func (a *API) JSONVersion(method, path, apiVersion string, body, out any) int {
	a.t.Helper()
	return a.jsonWithKey(method, path, apiVersion, a.AppKey, body, out)
}

func (a *API) jsonWithKey(method, path, apiVersion, apiKey string, body, out any) int {
	a.t.Helper()

	resp := a.Do(method, path, apiVersion, apiKey, body)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		a.t.Fatalf("read body: %v", err)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil && resp.StatusCode < 300 {
			a.t.Fatalf("decode %s %s (%d): %v\nbody: %s", method, path, resp.StatusCode, err, raw)
		}
	}
	return resp.StatusCode
}

// DecodeError reads an error response payload.
func DecodeError(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	return payload.Error
}
