# How to assign roles to users

A **role assignment** grants a user a role inside a specific **tenant**. The same user can have different roles in different tenants (or multiple roles in one tenant).

## Create users and tenants first

```http
POST /users
{ "email": "ada@example.com", "metadata": { "name": "Ada" } }

POST /tenants
{ "metadata": { "name": "Acme Workspace" } }
```

Emails are unique per app (normalized to lowercase). Tenants are opaque containers — OpenAura does not prescribe org vs workspace vs project; encode that in `metadata`.

## Create an assignment

```http
POST /roleassignments
{
  "user_id": "<uuid>",
  "role_id": "<uuid>",
  "tenant_id": "<uuid>"
}
```

Constraints:

- User, role, and tenant must belong to the same app (DB-enforced).
- The triple `(user_id, role_id, tenant_id)` must be unique among active assignments.

## List and filter

```http
GET /roleassignments?user_id=<uuid>
GET /roleassignments?tenant_id=<uuid>
GET /roleassignments?role_id=<uuid>
GET /roleassignments?limit=50&offset=0
```

Filters may be combined. Pagination defaults to `limit=50` (max 100).

## Update or remove

```http
PATCH  /roleassignments/{id}   # optional user_id, role_id, tenant_id
DELETE /roleassignments/{id}   # soft-delete → access checks stop matching
```

## Integration pattern

When your product invites a user into a workspace:

1. Upsert the OpenAura user (`POST /users` or look up by your own mapping of email → OpenAura id).
2. Ensure the tenant exists (map your workspace id → OpenAura `tenant_id` in your DB).
3. `POST /roleassignments` with the role that matches the invite (e.g. `member`, `admin`).
4. On leave/revoke, `DELETE` the assignment.

Store OpenAura UUIDs next to your domain entities; do not invent parallel auth state that can drift.

## Related

- [Model roles and permissions](model-roles-and-permissions.md)
- [Check access](check-access.md)
