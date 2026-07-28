# How to authenticate requests

OpenAura has two auth layers:

1. **API keys** — machine auth for your backend calling OpenAura (RBAC management + access checks)
2. **Email/password JWT** — optional end-user auth that your backend proxies; JWT is for your product frontend/session

Your **backend** holds the app API key. Browsers never call OpenAura with that key.

## Headers on every OpenAura API call

| Header | Required | Value |
|---|---|---|
| `X-API-Version` | yes | Currently only `1` |
| `X-API-Key` | yes* | Admin or app secret |
| `Content-Type` | for JSON bodies | `application/json` |

\*Instead of `X-API-Key`, you may send `Authorization: Bearer <key>` (API key, not an end-user JWT).

Public exceptions (no key, no version):

- `GET /healthz`
- `/swagger/` (OpenAPI UI and schema)

## Choose the right key type

| Key | Prefix (generated) | Use for |
|---|---|---|
| **Admin** | `oa_admin_…` | `/admin/*` — create apps, issue first app keys, manage admin keys |
| **App** | `oa_app_…` | Everything else — auth register/login, users, tenants, roles, permissions, access checks |

App keys are scoped to a single app. The server resolves the key to an `app_id`; you never pass `app_id` in path or body for app routes.

## End-user register and login (JWT)

Consumer backends (for example an invoicing app) proxy signup/login to OpenAura with their **app API key**. OpenAura stores credentials in `user_identities` (not on the user row) and returns a JWT for the product frontend to keep as the user session.

```bash
# Register
curl -s -X POST "$API/auth/register" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email":"ada@example.com","password":"correct-horse-battery-staple"}'

# Login
curl -s -X POST "$API/auth/login" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email":"ada@example.com","password":"correct-horse-battery-staple"}'
```

Response shape:

```json
{
  "access_token": "<jwt>",
  "token_type": "Bearer",
  "expires_in": 86400,
  "user": { "id": "...", "app_id": "...", "email": "ada@example.com", "...": "..." }
}
```

JWT claims (HS256, signed with `JWT_SECRET`):

| Claim | Meaning |
|---|---|
| `sub` | OpenAura `user_id` |
| `app_id` | App that owns the user |
| `email` | Normalized email |
| `iss` | `JWT_ISSUER` (default `openaura`) |
| `iat` / `exp` | Issued / expiry |

Your backend should verify the JWT with the shared `JWT_SECRET` and reject tokens whose `app_id` does not match your app. Use `sub` as `user_id` when calling `POST /access/check` with the app API key.

Notes:

- Passwords must be at least 8 characters; stored with bcrypt on a password identity row.
- `POST /users` still creates passwordless RBAC subjects (they cannot log in; use `/auth/register` for users who need credentials).
- End-user JWTs are **not** accepted on OpenAura management or access routes — those stay API-key-only.

## Examples (API keys)

Admin:

```bash
curl -s -X GET "$API/admin/apps" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $ADMIN_KEY"
```

App (Bearer form):

```bash
curl -s -X GET "$API/users" \
  -H "X-API-Version: 1" \
  -H "Authorization: Bearer $APP_KEY"
```

## Common failures

| Status | Meaning |
|---|---|
| `400` | Missing `X-API-Version`, invalid body, or short password |
| `406` | Unsupported version (not `1`) |
| `401` | Missing/invalid API key, or invalid email/password on login |
| `409` | Email already registered in the app |

Keys are stored as SHA-256 hashes. The plaintext secret is returned **only** on create — store it in your secrets manager immediately.

## Related

- [Bootstrap an app](bootstrap-an-app.md)
- [Manage API keys](manage-api-keys.md)
- [Configuration](../reference/configuration.md)
- [Authentication reference](../reference/api.md#authentication)
