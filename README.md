# OpenAura

Open-source Authentication User Roles & Access management service. Your application calls OpenAura over HTTP to store users, tenants, roles, and permissions, then asks `POST /access/check` whether a user may perform an action on a resource within a tenant.

## Requirements

- Go 1.26+
- Docker and Docker Compose (for Postgres, or the full stack)
- Flyway for migrations (`make migrate` uses the Flyway CLI if installed, otherwise a Docker image)

## Quick start

```bash
cp .env.example .env
# Optionally set BOOTSTRAP_ADMIN_API_KEY in .env

docker compose up -d postgres
make migrate
make run
```

Or run API + Postgres together:

```bash
cp .env.example .env
# Set BOOTSTRAP_ADMIN_API_KEY, then:
docker compose up --build
```

The server listens on `:8080` by default.

- Health: `GET /healthz`
- Interactive API docs: [http://localhost:8080/swagger/](http://localhost:8080/swagger/)

## Configuration

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | yes | Postgres connection string |
| `HTTP_ADDR` | no | Listen address (default `:8080`) |
| `BOOTSTRAP_ADMIN_API_KEY` | no | Seeds an admin API key on startup |
| `POSTGRES_*` | compose | Used by `docker-compose.yml` for the database |

See `.env.example` for a local template.

## Common commands

```bash
make run              # start the API (loads .env)
make migrate          # apply SQL migrations
make test             # run all tests
make swagger          # regenerate OpenAPI from code annotations
docker compose up -d postgres   # Postgres only
```

## Documentation

Integration and deeper material follow [Diátaxis](https://diataxis.fr/):

| Need | Start here |
|---|---|
| Learn by doing | [Tutorials](docs/tutorials/getting-started.md) |
| Solve a specific task | [How-to guides](docs/how-to/README.md) |
| Look up endpoints & config | [Reference](docs/reference/README.md) |
| Understand the model | [Explanation](docs/explanation/README.md) |

Docs index: [docs/README.md](docs/README.md)

## License

Apache 2.0 (as declared in the OpenAPI metadata).
