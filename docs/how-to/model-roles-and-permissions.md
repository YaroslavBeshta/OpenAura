# How to model roles and permissions

OpenAura authorization is classic RBAC with named resources and actions. You define the vocabulary once, then grant combinations to roles.

## Mental model

```text
permission = role + resource + action
```

At check time you pass **resource name** and **action name** (strings). IDs are used when creating permissions.

## Recommended order

1. Create **resources** (`documents`, `invoices`, `settings`, …)
2. Create **actions** (`read`, `write`, `delete`, `admin`, …)
3. Create **roles** (metadata often holds a `name` like `editor`)
4. Attach **permissions** to each role
5. Later: [assign roles to users in tenants](assign-roles.md)

Reusing the same action names across resources (`read` on `documents` and `read` on `invoices`) is intentional — actions are app-scoped names, not global enums.

## Create resources and actions

```http
POST /resources
{ "name": "documents", "metadata": {} }

POST /actions
{ "name": "read" }

POST /actions
{ "name": "write" }
```

Names are unique per app (case-sensitive as stored). Soft-delete removes them from active use.

## Create a role

Roles have no required `name` field on the object itself; put a display or stable name in `metadata` if you need one:

```http
POST /roles
{
  "metadata": { "name": "editor", "description": "Can read and write documents" }
}
```

If `metadata.name` is set, it must be unique among active roles in the app.

## Grant permissions

```http
POST /roles/{role_id}/permissions
{
  "resource_id": "<documents-uuid>",
  "action_id": "<read-uuid>"
}
```

Repeat for each `(resource, action)` pair. Duplicate triples return a conflict-style error.

List or remove:

```http
GET    /roles/{role_id}/permissions
GET    /roles/{role_id}/permissions/{permission_id}
DELETE /roles/{role_id}/permissions/{permission_id}
```

## Design tips

- Prefer a small, stable action vocabulary (`read`, `write`, `delete`, `manage`) over one action per UI button.
- Prefer resource names that match your domain objects, not HTTP routes.
- Keep role permission sets coarse; use multiple roles and assignments for finer control.
- All entities in a permission must belong to the same app — the database enforces this.

## Related

- [Assign roles to users](assign-roles.md)
- [Check access](check-access.md)
- [Core concepts](../explanation/concepts.md)
