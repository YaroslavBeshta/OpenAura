# Configuration reference

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres URL, e.g. `postgres://user:pass@localhost:5432/postgres?sslmode=disable` |
| `HTTP_ADDR` | no | `:8080` | HTTP listen address |
| `BOOTSTRAP_ADMIN_API_KEY` | no | empty | If set, ensures this admin key exists on startup |

Compose / local Postgres helpers (used by `docker-compose.yml` and migration scripts):

| Variable | Typical local value |
|---|---|
| `POSTGRES_USER` | `postgres` |
| `POSTGRES_PASSWORD` | `postgres` |
| `POSTGRES_DB` | `postgres` |

Copy `.env.example` to `.env`. Make targets and Compose load `.env` automatically.

### Compose networking note

On the host (`make run`, `make migrate`), `DATABASE_URL` should use host `localhost`.

The `api` service in `docker-compose.yml` overrides `DATABASE_URL` to use hostname `postgres` on the Compose network.

## Make targets

| Target | Purpose |
|---|---|
| `make run` | Run `go run ./cmd/server` with `.env` exported |
| `make migrate` | Apply Flyway migrations from `migrations/` |
| `make migrate-info` | Flyway info |
| `make migrate-reapply` | Destructive reapply (requires `CONFIRM=yes`) |
| `make generate-migration name=…` | Scaffold a new migration |
| `make test` | All tests (`-p 1`) |
| `make test-unit` | Fast unit subset |
| `make swagger` | Regenerate OpenAPI from swag annotations |
| `make tidy` | `go mod tidy` |

Migrations require the Flyway CLI **or** Docker (script falls back to `redgate/flyway`).

## Docker

```bash
docker compose up -d postgres   # database only
docker compose up --build       # api + postgres
```

The API image exposes port `8080` and expects `DATABASE_URL` (and optionally `BOOTSTRAP_ADMIN_API_KEY`).

## Related

- [Getting started](../tutorials/getting-started.md)
- Root [README](../../README.md)
