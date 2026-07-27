# How to bootstrap an app

Use this when provisioning OpenAura for a new product or environment: create the app record, then obtain an app API key your services will use.

## Prerequisites

- A running OpenAura instance
- An **admin** API key (`BOOTSTRAP_ADMIN_API_KEY` or one created via `/admin/api_keys`)

## Steps

### 1. Create the app

```http
POST /admin/apps
X-API-Version: 1
X-API-Key: <admin-key>
Content-Type: application/json

{
  "name": "acme",
  "metadata": { "env": "production" }
}
```

Response `201` includes `id`, `name`, `metadata`, and timestamps. Soft-deleted apps are omitted from later lists.

### 2. Create the first app API key (admin path)

```http
POST /admin/apps/{app_id}/api_keys
X-API-Version: 1
X-API-Key: <admin-key>
Content-Type: application/json

{
  "name": "api-server",
  "metadata": { "owner": "platform" }
}
```

Response `201` includes a `key` field with the plaintext secret. Persist it securely; subsequent `GET` responses never include `key`.

### 3. Use the app key for day-to-day API calls

From this point, prefer the app key for users, RBAC, and access checks. Keep the admin key limited to operators and automation that manage apps.

### 4. (Optional) Create additional app keys from the app itself

Once you have one app key:

```http
POST /api_keys
X-API-Version: 1
X-API-Key: <app-key>
Content-Type: application/json

{ "name": "worker" }
```

Useful for per-service keys without needing admin credentials in every deploy pipeline.

## Suggested production pattern

1. Bootstrap one admin key via config (`BOOTSTRAP_ADMIN_API_KEY`) or an out-of-band process.
2. Create the app and first app key with a one-shot provisioner.
3. Store the app key in your secret store; inject it into API servers.
4. Create additional admin keys via `POST /admin/api_keys`, then revoke the bootstrap key if desired.
5. Rotate app keys with create-then-revoke (see [Manage API keys](manage-api-keys.md)).

## Related

- [Authenticate requests](authenticate.md)
- [Getting started tutorial](../tutorials/getting-started.md)
