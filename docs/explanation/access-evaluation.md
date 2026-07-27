# Access evaluation

This page explains what `POST /access/check` computes and why denials are quiet.

## Inputs

The caller supplies:

- Authenticated **app** (from the API key)
- `user_id`, `tenant_id` (UUIDs)
- `resource`, `action` (**names**)

## Decision rule

Access is allowed if and only if at least one matching path exists in the database:

```text
user ──assignment──► role ──permission──► (resource.name, action.name)
         │
         └── scoped to tenant
```

All of the following must be active (`deleted_at` / `revoked_at` null as applicable):

- assignment, role, user, tenant, permission, resource, action
- every entity’s `app_id` equals the authenticated app

Otherwise the result is `allowed: false`.

## Why names for resource and action?

Management APIs use UUIDs for stability under renames-by-id. Runtime checks use names so your application code can stay readable:

```text
Check(user, tenant, "documents", "write")
```

rather than embedding permission UUIDs in every service. Keep names stable once you ship them.

## Why HTTP 200 on deny?

Authorization denial is a successful evaluation, not a client protocol error. That lets clients distinguish:

| Outcome | Signal |
|---|---|
| Not allowed | `200` + `allowed: false` |
| Bad request | `400` |
| Unauthenticated to OpenAura | `401` |
| OpenAura failure | `5xx` |

Your product should map `allowed: false` to its own `403 Forbidden` (or equivalent).

## Soft deletes and revocation

Deleting a role, permission, assignment, user, tenant, resource, or action removes it from the join. Revoking an API key stops authentication entirely. There is no separate “deny” rule — absence of a matching allow path is denial.

## Consistency guarantees

App consistency for permissions and assignments is enforced in Postgres triggers: user/role/tenant (and role/resource/action) must share an `app_id`. Cross-app graphs cannot be constructed through the API under normal operation.

## Related

- [How to check access](../how-to/check-access.md)
- [Core concepts](concepts.md)
