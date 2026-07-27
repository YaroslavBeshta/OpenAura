// Package main is the OpenAura API server.
//
//	@title						OpenAura API
//	@version					1.0
//	@description				Open-source user, role, and access management service.
//	@description				API version is selected via the X-API-Version header (not the URL path).
//	@description				App routes require an app API key (X-API-Key). Admin routes live under /admin and require an admin API key.
//
//	@contact.name				OpenAura
//
//	@license.name				Apache 2.0
//	@license.url				https://www.apache.org/licenses/LICENSE-2.0.html
//
//	@host						localhost:8080
//	@BasePath					/
//	@schemes					http
//
//	@tag.name					users
//	@tag.name					tenants
//	@tag.name					roles
//	@tag.name					roleassignments
//	@tag.name					api_keys
//	@tag.name					admin-apps
//	@tag.name					admin-api_keys
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/openaura/openaura/docs"
	"github.com/openaura/openaura/internal/adminapikey"
	"github.com/openaura/openaura/internal/app"
	"github.com/openaura/openaura/internal/apikey"
	"github.com/openaura/openaura/internal/config"
	"github.com/openaura/openaura/internal/db"
	"github.com/openaura/openaura/internal/httpserver"
	"github.com/openaura/openaura/internal/role"
	"github.com/openaura/openaura/internal/roleassignments"
	"github.com/openaura/openaura/internal/tenant"
	"github.com/openaura/openaura/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	appRepo := app.NewRepository(pool)
	userRepo := user.NewRepository(pool)
	tenantRepo := tenant.NewRepository(pool)
	roleRepo := role.NewRepository(pool)
	assignmentRepo := roleassignments.NewRepository(pool)
	apiKeyRepo := apikey.NewRepository(pool)
	adminKeyRepo := adminapikey.NewRepository(pool)

	if cfg.BootstrapAdminAPIKey != "" {
		if err := adminKeyRepo.EnsureBootstrapKey(ctx, cfg.BootstrapAdminAPIKey, "bootstrap"); err != nil {
			log.Fatalf("bootstrap admin api key: %v", err)
		}
		log.Printf("bootstrap admin api key ensured")
	}

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpserver.New(httpserver.Handlers{
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
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("openaura listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
