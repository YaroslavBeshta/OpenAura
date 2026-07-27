package httpserver

import (
	"log"
	"net/http"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/openaura/openaura/internal/access"
	"github.com/openaura/openaura/internal/action"
	"github.com/openaura/openaura/internal/adminapikey"
	"github.com/openaura/openaura/internal/apikey"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/auth"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/permission"
	"github.com/openaura/openaura/internal/resource"
	"github.com/openaura/openaura/internal/role"
	"github.com/openaura/openaura/internal/roleassignments"
	"github.com/openaura/openaura/internal/tenant"
	"github.com/openaura/openaura/internal/user"
)

const (
	apiVersionHeader    = "X-API-Version"
	supportedAPIVersion = "1"
)

type Handlers struct {
	Apps            *app.Handler
	Users           *user.Handler
	Tenants         *tenant.Handler
	Roles           *role.Handler
	RoleAssignments *roleassignments.Handler
	Resources       *resource.Handler
	Actions         *action.Handler
	Permissions     *permission.Handler
	Access          *access.Handler
	APIKeys         *apikey.Handler
	AdminAPIKeys    *adminapikey.Handler
}

type KeyLookups struct {
	App   auth.AppKeyLookup
	Admin auth.AdminKeyLookup
}

func New(h Handlers, keys KeyLookups) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	admin := http.NewServeMux()
	admin.HandleFunc("POST /admin/apps", h.Apps.Create)
	admin.HandleFunc("GET /admin/apps", h.Apps.List)
	admin.HandleFunc("GET /admin/apps/{id}", h.Apps.Get)
	admin.HandleFunc("PATCH /admin/apps/{id}", h.Apps.Update)
	admin.HandleFunc("DELETE /admin/apps/{id}", h.Apps.Delete)
	admin.HandleFunc("POST /admin/apps/{id}/api_keys", h.APIKeys.AdminCreate)

	admin.HandleFunc("POST /admin/api_keys", h.AdminAPIKeys.Create)
	admin.HandleFunc("GET /admin/api_keys", h.AdminAPIKeys.List)
	admin.HandleFunc("GET /admin/api_keys/{id}", h.AdminAPIKeys.Get)
	admin.HandleFunc("DELETE /admin/api_keys/{id}", h.AdminAPIKeys.Delete)

	appAPI := http.NewServeMux()
	appAPI.HandleFunc("POST /users", h.Users.Create)
	appAPI.HandleFunc("GET /users", h.Users.List)
	appAPI.HandleFunc("GET /users/{id}", h.Users.Get)
	appAPI.HandleFunc("PATCH /users/{id}", h.Users.Update)
	appAPI.HandleFunc("DELETE /users/{id}", h.Users.Delete)

	appAPI.HandleFunc("POST /tenants", h.Tenants.Create)
	appAPI.HandleFunc("GET /tenants", h.Tenants.List)
	appAPI.HandleFunc("GET /tenants/{id}", h.Tenants.Get)
	appAPI.HandleFunc("PATCH /tenants/{id}", h.Tenants.Update)
	appAPI.HandleFunc("DELETE /tenants/{id}", h.Tenants.Delete)

	appAPI.HandleFunc("POST /roles", h.Roles.Create)
	appAPI.HandleFunc("GET /roles", h.Roles.List)
	appAPI.HandleFunc("GET /roles/{id}", h.Roles.Get)
	appAPI.HandleFunc("PATCH /roles/{id}", h.Roles.Update)
	appAPI.HandleFunc("DELETE /roles/{id}", h.Roles.Delete)

	appAPI.HandleFunc("POST /roles/{id}/permissions", h.Permissions.Create)
	appAPI.HandleFunc("GET /roles/{id}/permissions", h.Permissions.List)
	appAPI.HandleFunc("GET /roles/{id}/permissions/{permission_id}", h.Permissions.Get)
	appAPI.HandleFunc("DELETE /roles/{id}/permissions/{permission_id}", h.Permissions.Delete)

	appAPI.HandleFunc("POST /roleassignments", h.RoleAssignments.Create)
	appAPI.HandleFunc("GET /roleassignments", h.RoleAssignments.List)
	appAPI.HandleFunc("GET /roleassignments/{id}", h.RoleAssignments.Get)
	appAPI.HandleFunc("PATCH /roleassignments/{id}", h.RoleAssignments.Update)
	appAPI.HandleFunc("DELETE /roleassignments/{id}", h.RoleAssignments.Delete)

	appAPI.HandleFunc("POST /resources", h.Resources.Create)
	appAPI.HandleFunc("GET /resources", h.Resources.List)
	appAPI.HandleFunc("GET /resources/{id}", h.Resources.Get)
	appAPI.HandleFunc("PATCH /resources/{id}", h.Resources.Update)
	appAPI.HandleFunc("DELETE /resources/{id}", h.Resources.Delete)

	appAPI.HandleFunc("POST /actions", h.Actions.Create)
	appAPI.HandleFunc("GET /actions", h.Actions.List)
	appAPI.HandleFunc("GET /actions/{id}", h.Actions.Get)
	appAPI.HandleFunc("PATCH /actions/{id}", h.Actions.Update)
	appAPI.HandleFunc("DELETE /actions/{id}", h.Actions.Delete)

	appAPI.HandleFunc("POST /access/check", h.Access.Check)

	appAPI.HandleFunc("POST /api_keys", h.APIKeys.Create)
	appAPI.HandleFunc("GET /api_keys", h.APIKeys.List)
	appAPI.HandleFunc("GET /api_keys/{id}", h.APIKeys.Get)
	appAPI.HandleFunc("DELETE /api_keys/{id}", h.APIKeys.Delete)

	versioned := http.NewServeMux()
	versioned.Handle("/admin/", auth.ResolveAdminKey(keys.Admin)(admin))
	versioned.Handle("/", auth.ResolveAppKey(keys.App)(appAPI))

	mux.Handle("/", requireAPIVersion(versioned))

	return withLogging(mux)
}

func requireAPIVersion(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := r.Header.Get(apiVersionHeader)
		if version == "" {
			httpx.WriteError(w, http.StatusBadRequest, apiVersionHeader+" header is required")
			return
		}
		if version != supportedAPIVersion {
			httpx.WriteError(w, http.StatusNotAcceptable, "unsupported API version")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
