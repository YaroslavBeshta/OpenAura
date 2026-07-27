# How to authenticate requests

OpenAura uses API keys, not end-user sessions. Your **backend** holds the key and calls OpenAura on behalf of your product.

## Headers on every authenticated call

| Header | Required | Value |
|---|---|---|
| `X-API-Version` | yes | Currently only `1` |
| `X-API-Key` | yes* | Admin or app secret |
| `Content-Type` | for JSON bodies | `application/json` |

\*Instead of `X-API-Key`, you may send `Authorization: Bearer <key>`.

Public exceptions (no key, no version):

- `GET /healthz`
- `/swagger/` (OpenAPI UI and schema)

## Choose the right key type

| Key | Prefix (generated) | Use for |
|---|---|---|
| **Admin** | `oa_admin_…` | `/admin/*` — create apps, issue first app keys, manage admin keys |
| **App** | `oa_app_…` | Everything else — users, tenants, roles, permissions, access checks |

App keys are scoped to a single app. The server resolves the key to an `app_id`; you never pass `app_id` in path or body for app routes.

## Examples

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
| `400` | Missing `X-API-Version` |
| `406` | Unsupported version (not `1`) |
| `401` | Missing or invalid API key |

Keys are stored as SHA-256 hashes. The plaintext secret is returned **only** on create — store it in your secrets manager immediately.

## Related

- [Bootstrap an app](bootstrap-an-app.md)
- [Manage API keys](manage-api-keys.md)
- [Authentication reference](../reference/api.md#authentication)
