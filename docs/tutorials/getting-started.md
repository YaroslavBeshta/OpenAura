# Getting started with OpenAura

This tutorial walks you from a blank machine to a successful access check. You will run OpenAura, create an app, define a permission model, and call `POST /access/check`.

## What you will build

A tiny authorization setup for a fictional app:

- One **user** (`ada@example.com`)
- One **tenant** (a workspace)
- One **role** (`editor`) that can `read` and `write` the `documents` resource
- An access check that returns `"allowed": true`

## Prerequisites

- Docker and Docker Compose
- `curl` and `jq` (optional but helpful)
- Go 1.26+ if you run the API with `make run` instead of Compose

## 1. Start the stack

From the repository root:

```bash
cp .env.example .env
```

Set a bootstrap admin key in `.env` (required for the admin steps below):

```bash
BOOTSTRAP_ADMIN_API_KEY=oa_admin_dev_bootstrap_change_me
```

Start Postgres and apply migrations, then run the API:

```bash
docker compose up -d postgres
make migrate
make run
```

Alternatively, start everything with Compose (API image + Postgres):

```bash
docker compose up --build
```

Confirm health:

```bash
curl -s http://localhost:8080/healthz
# {"status":"ok"}
```

## 2. Create an app and an app API key

Admin routes require `X-API-Version: 1` and your admin key in `X-API-Key`.

```bash
export ADMIN_KEY=oa_admin_dev_bootstrap_change_me
export API=http://localhost:8080

# Create an app
curl -s -X POST "$API/admin/apps" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"acme","metadata":{"env":"dev"}}' | tee /tmp/oa-app.json

export APP_ID=$(jq -r .id /tmp/oa-app.json)

# Issue an app API key (plaintext secret is returned once)
curl -s -X POST "$API/admin/apps/$APP_ID/api_keys" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"local-dev"}' | tee /tmp/oa-app-key.json

export APP_KEY=$(jq -r .key /tmp/oa-app-key.json)
echo "Save this key; it is only shown once: $APP_KEY"
```

All further calls in this tutorial use the **app** key.

## 3. Create the RBAC building blocks

```bash
# User
curl -s -X POST "$API/users" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email":"ada@example.com","metadata":{"name":"Ada"}}' | tee /tmp/oa-user.json
export USER_ID=$(jq -r .id /tmp/oa-user.json)

# Tenant (workspace / org / account — your choice of meaning)
curl -s -X POST "$API/tenants" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d '{"metadata":{"name":"Acme Workspace"}}' | tee /tmp/oa-tenant.json
export TENANT_ID=$(jq -r .id /tmp/oa-tenant.json)

# Role (optional display name in metadata)
curl -s -X POST "$API/roles" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d '{"metadata":{"name":"editor"}}' | tee /tmp/oa-role.json
export ROLE_ID=$(jq -r .id /tmp/oa-role.json)

# Resource + actions
curl -s -X POST "$API/resources" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"documents"}' | tee /tmp/oa-resource.json
export RESOURCE_ID=$(jq -r .id /tmp/oa-resource.json)

curl -s -X POST "$API/actions" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"read"}' | tee /tmp/oa-action-read.json
export ACTION_READ=$(jq -r .id /tmp/oa-action-read.json)

curl -s -X POST "$API/actions" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"write"}' | tee /tmp/oa-action-write.json
export ACTION_WRITE=$(jq -r .id /tmp/oa-action-write.json)
```

## 4. Attach permissions and assign the role

```bash
# Grant editor: documents.read and documents.write
curl -s -X POST "$API/roles/$ROLE_ID/permissions" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"resource_id\":\"$RESOURCE_ID\",\"action_id\":\"$ACTION_READ\"}"

curl -s -X POST "$API/roles/$ROLE_ID/permissions" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"resource_id\":\"$RESOURCE_ID\",\"action_id\":\"$ACTION_WRITE\"}"

# Assign editor to Ada in the tenant
curl -s -X POST "$API/roleassignments" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID\",\"role_id\":\"$ROLE_ID\",\"tenant_id\":\"$TENANT_ID\"}"
```

## 5. Check access

Access checks use **resource and action names** (strings), not IDs:

```bash
curl -s -X POST "$API/access/check" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID\",\"tenant_id\":\"$TENANT_ID\",\"resource\":\"documents\",\"action\":\"read\"}"
```

Expected response:

```json
{"allowed":true}
```

A check for an action the role does not have returns `"allowed":false` (HTTP 200), not an error:

```bash
curl -s -X POST "$API/access/check" \
  -H "X-API-Version: 1" \
  -H "X-API-Key: $APP_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID\",\"tenant_id\":\"$TENANT_ID\",\"resource\":\"documents\",\"action\":\"delete\"}"
# {"allowed":false}
```

## What you learned

1. Admin keys manage apps; app keys manage everything inside an app.
2. Permissions are `(role, resource, action)` triples.
3. Assignments bind `(user, role, tenant)`.
4. Runtime authorization is a single `POST /access/check` call.

## Next steps

- [Bootstrap apps and keys in production-like flows](../how-to/bootstrap-an-app.md)
- [Call access checks from your backend](../how-to/check-access.md)
- [Understand apps, tenants, and roles](../explanation/concepts.md)
- [API reference](../reference/api.md)
