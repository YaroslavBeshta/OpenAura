# API reference

OpenAura is a JSON HTTP API. Interactive docs: `http://localhost:8080/swagger/`. Canonical OpenAPI: [`docs/swagger.yaml`](../swagger.yaml).

## Base URL

Local default: `http://localhost:8080`

There is no `/v1` path prefix. Versioning uses the `X-API-Version` header.

## Authentication

| Audience | Header | Routes |
|---|---|---|
| Admin | `X-API-Key: <admin-secret>` or `Authorization: Bearer <admin-secret>` | `/admin/*` |
| App | `X-API-Key: <app-secret>` or `Authorization: Bearer <app-secret>` | all other versioned routes |

App routes infer `app_id` from the key. Do not send `app_id` in the body.

## Global headers

| Header | Required on versioned routes | Values |
|---|---|---|
| `X-API-Version` | yes | `1` |
| `X-API-Key` / `Authorization` | yes (except health/swagger) | see above |
| `Content-Type` | JSON bodies | `application/json` |

## Unauthenticated routes

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness: `{"status":"ok"}` |
| `GET` | `/swagger/*` | Swagger UI and OpenAPI document |

## Admin routes

All require an admin API key.

### Apps

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/admin/apps` | `{ "name", "metadata"? }` → `201` App |
| `GET` | `/admin/apps` | `?limit&offset` → `{ "apps": […] }` |
| `GET` | `/admin/apps/{id}` | App |
| `PATCH` | `/admin/apps/{id}` | `{ "name"?, "metadata"? }` |
| `DELETE` | `/admin/apps/{id}` | Soft-delete → `204` |
| `POST` | `/admin/apps/{id}/api_keys` | `{ "name"?, "metadata"? }` → `201` includes plaintext `key` |

### Admin API keys

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/admin/api_keys` | `{ "name"? }` → `201` includes plaintext `key` |
| `GET` | `/admin/api_keys` | `{ "admin_api_keys": […] }` |
| `GET` | `/admin/api_keys/{id}` | Metadata only |
| `DELETE` | `/admin/api_keys/{id}` | Revoke → `204` |

## App routes

All require an app API key and are scoped to that app.

### Auth (end-user JWT)

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/auth/register` | `{ "email", "password", "metadata"? }` → JWT + user (`201`) |
| `POST` | `/auth/login` | `{ "email", "password" }` → JWT + user (`200`) |

Password min length is 8. Tokens are HS256 JWTs (`sub`, `app_id`, `email`, `iss`, `iat`, `exp`). See [How to authenticate](../how-to/authenticate.md).

### Users

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/users` | `{ "email", "metadata"? }` — email unique per app; passwordless |
| `GET` | `/users` | `{ "users": […] }` |
| `GET` | `/users/{id}` | |
| `PATCH` | `/users/{id}` | `{ "email"?, "metadata"? }` |
| `DELETE` | `/users/{id}` | Soft-delete → `204` |

### Tenants

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/tenants` | `{ "metadata"? }` |
| `GET` | `/tenants` | `{ "tenants": […] }` |
| `GET` | `/tenants/{id}` | |
| `PATCH` | `/tenants/{id}` | `{ "metadata"? }` |
| `DELETE` | `/tenants/{id}` | Soft-delete → `204` |

### Roles

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/roles` | `{ "metadata"? }` — optional unique `metadata.name` |
| `GET` | `/roles` | `{ "roles": […] }` |
| `GET` | `/roles/{id}` | |
| `PATCH` | `/roles/{id}` | `{ "metadata"? }` |
| `DELETE` | `/roles/{id}` | Soft-delete → `204` |

### Permissions (nested under roles)

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/roles/{id}/permissions` | `{ "resource_id", "action_id" }` |
| `GET` | `/roles/{id}/permissions` | `{ "permissions": […] }` |
| `GET` | `/roles/{id}/permissions/{permission_id}` | |
| `DELETE` | `/roles/{id}/permissions/{permission_id}` | Soft-delete → `204` |

### Role assignments

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/roleassignments` | `{ "user_id", "role_id", "tenant_id" }` |
| `GET` | `/roleassignments` | Filters: `user_id`, `role_id`, `tenant_id`; `{ "roleassignments": […] }` |
| `GET` | `/roleassignments/{id}` | |
| `PATCH` | `/roleassignments/{id}` | `{ "user_id"?, "role_id"?, "tenant_id"? }` |
| `DELETE` | `/roleassignments/{id}` | Soft-delete → `204` |

### Resources

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/resources` | `{ "name", "metadata"? }` — name unique per app |
| `GET` | `/resources` | `{ "resources": […] }` |
| `GET` | `/resources/{id}` | |
| `PATCH` | `/resources/{id}` | `{ "name"?, "metadata"? }` |
| `DELETE` | `/resources/{id}` | Soft-delete → `204` |

### Actions

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/actions` | `{ "name", "metadata"? }` — name unique per app |
| `GET` | `/actions` | `{ "actions": […] }` |
| `GET` | `/actions/{id}` | |
| `PATCH` | `/actions/{id}` | `{ "name"?, "metadata"? }` |
| `DELETE` | `/actions/{id}` | Soft-delete → `204` |

### Access

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/access/check` | `{ "user_id", "tenant_id", "resource", "action" }` → `{ "allowed": bool }` |

`resource` and `action` are **names**, not UUIDs. Always HTTP `200` on a valid check.

### App API keys

| Method | Path | Body / notes |
|---|---|---|
| `POST` | `/api_keys` | `{ "name"?, "metadata"? }` → plaintext `key` once |
| `GET` | `/api_keys` | `{ "api_keys": […] }` |
| `GET` | `/api_keys/{id}` | Metadata only |
| `DELETE` | `/api_keys/{id}` | Revoke → `204` |

## Common types

### Metadata

Most create/update bodies accept `metadata` as a JSON **object** (default `{}`). Arrays and primitives are rejected.

### Soft deletes

`DELETE` on domain entities sets `deleted_at`. Soft-deleted rows are hidden from gets/lists and do not participate in access checks. API key deletes set `revoked_at`.

### Pagination

`limit` (default 50, max 100) and `offset` (default 0) on list endpoints.

### Errors

```json
{ "error": "message" }
```

See [Errors and versioning](../how-to/errors-and-versioning.md) for status-code guidance.
