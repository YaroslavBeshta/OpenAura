# How to manage API keys

## App API keys

| Operation | Endpoint | Auth |
|---|---|---|
| Create | `POST /api_keys` | App key |
| Create (bootstrap) | `POST /admin/apps/{id}/api_keys` | Admin key |
| List | `GET /api_keys` | App key |
| Get metadata | `GET /api_keys/{id}` | App key |
| Revoke | `DELETE /api_keys/{id}` | App key |

Create body:

```json
{ "name": "api-server", "metadata": {} }
```

The plaintext `key` appears only in the create response.

## Admin API keys

| Operation | Endpoint | Auth |
|---|---|---|
| Create | `POST /admin/api_keys` | Admin key |
| List | `GET /admin/api_keys` | Admin key |
| Get | `GET /admin/api_keys/{id}` | Admin key |
| Revoke | `DELETE /admin/api_keys/{id}` | Admin key |

Bootstrap via environment:

```bash
BOOTSTRAP_ADMIN_API_KEY=oa_admin_…
```

On startup, OpenAura ensures that key exists (idempotent). Prefer rotating to a generated admin key afterward.

## Rotate an app key safely

1. `POST /api_keys` → store the new secret.
2. Deploy services with the new key (dual-read if needed).
3. Confirm traffic uses the new key.
4. `DELETE /api_keys/{old_id}` to revoke the old one.

Revocation is soft (`revoked_at`); revoked keys fail authentication immediately.

## Security checklist

- Never log full API keys.
- Never ship app or admin keys to browsers or mobile apps.
- Prefer one key per service identity for blast-radius control.
- Store only the OpenAura key id + name in config; keep the secret in a vault.

## Related

- [Authenticate](authenticate.md)
- [Bootstrap an app](bootstrap-an-app.md)
