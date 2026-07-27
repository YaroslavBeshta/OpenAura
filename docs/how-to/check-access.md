# How to check access from your backend

The primary integration point for authorization is `POST /access/check`. Call it from your API (or edge worker) before performing a sensitive operation.

## Request

```http
POST /access/check
X-API-Version: 1
X-API-Key: <app-key>
Content-Type: application/json

{
  "user_id": "01912345-6789-7abc-def0-123456789abc",
  "tenant_id": "01912345-6789-7abc-def0-123456789abc",
  "resource": "documents",
  "action": "read"
}
```

| Field | Type | Notes |
|---|---|---|
| `user_id` | UUID | OpenAura user id |
| `tenant_id` | UUID | Tenant where the action applies |
| `resource` | string | Resource **name** (e.g. `documents`) |
| `action` | string | Action **name** (e.g. `read`) |

## Response

HTTP `200` with:

```json
{ "allowed": true }
```

or

```json
{ "allowed": false }
```

Denial is not an error. Treat transport/auth failures (`401`, `5xx`) differently from `allowed: false`.

## What “allowed” means

The check is true when there exists an active chain:

1. Role assignment for `(user_id, tenant_id)`
2. Role still active in the same app
3. Permission on that role for a resource named `resource` and action named `action`
4. All of user, tenant, role, resource, action, and permission are not soft-deleted

Multiple matching roles still yield a single boolean.

## Example: HTTP middleware sketch (pseudo-Go)

```go
func requireAccess(resource, action string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID := currentUserID(r)     // from your session / JWT
            tenantID := currentTenantID(r) // from path or claims

            allowed, err := openaura.Check(r.Context(), CheckInput{
                UserID:   userID,
                TenantID: tenantID,
                Resource: resource,
                Action:   action,
            })
            if err != nil {
                http.Error(w, "authorization unavailable", http.StatusBadGateway)
                return
            }
            if !allowed {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// mux.Handle("GET /docs/{id}", requireAccess("documents", "read")(getDoc))
```

## curl example

```bash
curl -s -X POST "$API/access/check" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID\",\"tenant_id\":\"$TENANT_ID\",\"resource\":\"documents\",\"action\":\"write\"}"
```

## Operational advice

- **Call from the trusted backend**, never from untrusted clients with the app API key.
- **Map your domain ids** to OpenAura UUIDs once (user signup / tenant create) and cache the mapping.
- **Fail closed** on OpenAura outages if the operation is security-sensitive; use timeouts and circuit breakers.
- **Do not** treat `allowed: false` as “user missing” — missing or soft-deleted entities simply do not match.
- Resource/action strings should match the names you created in OpenAura exactly.

## Related

- [Model roles and permissions](model-roles-and-permissions.md)
- [Assign roles](assign-roles.md)
- [Errors and versioning](errors-and-versioning.md)
- [How access evaluation works](../explanation/access-evaluation.md)
