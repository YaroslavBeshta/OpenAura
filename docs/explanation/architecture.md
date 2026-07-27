# Architecture

## High-level shape

```text
┌─────────────────┐     X-API-Key + X-API-Version      ┌──────────────────┐
│  Your backends  │ ─────────────────────────────────► │  OpenAura HTTP   │
│  (trusted)      │ ◄───────────────────────────────── │  API (:8080)     │
└─────────────────┘           JSON                     └────────┬─────────┘
                                                                │
                                                                ▼
                                                         ┌──────────────┐
                                                         │  PostgreSQL  │
                                                         └──────────────┘
```

OpenAura is a single Go binary (`cmd/server`) with handlers organized by domain under `internal/` (users, tenants, roles, access, …). Postgres is the source of truth; Flyway migrations live in `migrations/`.

## Auth boundaries

```text
                 require X-API-Version = 1
                            │
              ┌─────────────┴─────────────┐
              ▼                           ▼
        /admin/*                     other routes
     ResolveAdminKey              ResolveAppKey
     (admin key hash)             (app key → app_id)
```

Health and Swagger sit outside version and key middleware.

## Data model (simplified)

```text
apps
 ├── tenants
 ├── users
 ├── roles
 │    └── permissions ──► resources
 │                    └──► actions
 ├── roleassignments (user × role × tenant)
 └── api_keys

admin_api_keys   (global)
```

Soft deletes (`deleted_at`) are used for domain entities; API keys use `revoked_at`. Secrets are never stored plaintext — only SHA-256 hashes.

## Integration posture

Typical product integration:

1. Provision app + app key once per environment.
2. On domain events (signup, workspace create, role change), mutate OpenAura via the app API.
3. On every authorized request path, call `POST /access/check` (or a cached projection you build yourself — OpenAura does not ship a cache).

OpenAura stays synchronous and request/response oriented so it can sit behind your existing API gateway or service mesh without a sidecar protocol.

## Related

- [Core concepts](concepts.md)
- [Configuration reference](../reference/configuration.md)
- [API reference](../reference/api.md)
