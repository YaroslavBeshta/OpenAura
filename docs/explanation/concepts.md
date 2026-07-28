# Core concepts

OpenAura is an **authorization data plane** for multi-tenant products. It stores the authorization graph and answers: *may this user perform this action on this resource in this tenant?*

It can also optionally authenticate end users with **email/password** and issue JWTs for consumer apps. Machine access to the API still uses API keys — JWTs are for your product session, not for calling OpenAura RBAC routes.

## Apps

An **app** is an isolation boundary — usually one product or environment (`acme-prod`, `acme-staging`). Almost every entity carries `app_id`. App API keys never see data from other apps.

Admin operators create apps; application servers only ever talk to one app via an app key.

## Users

A **user** is an authorization subject inside an app, identified primarily by email (unique per app).

Login methods live separately as **identities** (`user_identities`): password today, with room for Google SSO and other providers later. The same user can have multiple identities.

You can:

- Provision users via `POST /users` (no identity — RBAC subject only), or
- Register via `POST /auth/register` (creates user + password identity → JWT)

Your backend typically proxies register/login, verifies the JWT with `JWT_SECRET`, then calls OpenAura access checks with the app API key and the `user_id` from the token.

## Tenants

A **tenant** is the scope in which roles apply — workspace, organization, account, project, or another domain concept you choose. OpenAura stays agnostic; put naming and hierarchy hints in `metadata`.

Role assignments always include a tenant. The same user can be an admin in one tenant and a viewer in another.

## Roles, resources, actions, permissions

| Concept | Role in the model |
|---|---|
| **Resource** | Named thing being protected (`documents`, `billing`) |
| **Action** | Named verb (`read`, `write`) |
| **Role** | Named bundle of permissions |
| **Permission** | Grant: this role may perform this action on this resource |

Permissions are role-centric. You do not attach permissions directly to users.

## Role assignments

An assignment is the edge `(user, role, tenant)`. That is what makes a permission reachable for an access check.

## API keys

| Kind | Purpose |
|---|---|
| Admin | Platform control plane (`/admin`) |
| App | Product control + data plane for one app (including `/auth/*`) |

Keys are secrets for **your servers**, not for browsers.

## What OpenAura deliberately is not

- Not a full OIDC / SSO provider (no social login, federation, or browser redirects)
- Not a policy language (no ABAC/ReBAC expressions beyond this RBAC graph)
- Not a UI for end users — it is an HTTP API for backends

For how the check query works, see [Access evaluation](access-evaluation.md).
