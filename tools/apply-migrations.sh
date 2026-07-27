#!/usr/bin/env bash
# Apply Flyway migrations from migrations/.
#
# Requires either:
#   - flyway on PATH, or
#   - docker (uses redgate/flyway image)
#
# Connection (first match wins):
#   1. FLYWAY_URL / FLYWAY_USER / FLYWAY_PASSWORD
#   2. DATABASE_URL (postgres://user:pass@host:port/db)
#   3. POSTGRES_USER / POSTGRES_PASSWORD / POSTGRES_DB (from .env or environment)
#
# Loads repo-root .env when present (does not override already-set vars).
#
# Usage:
#   ./tools/apply-migrations.sh
#   ./tools/apply-migrations.sh info
#   ./tools/apply-migrations.sh repair
#   DATABASE_URL=postgres://... ./tools/apply-migrations.sh migrate
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-$ROOT_DIR/migrations}"
FLYWAY_IMAGE="${FLYWAY_IMAGE:-redgate/flyway:11}"
COMMAND="${1:-migrate}"

if [[ -f "$ROOT_DIR/.env" ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    local_key="${line%%=*}"
    [[ -z "$local_key" || "$local_key" == "$line" ]] && continue
    if [[ -z "${!local_key+x}" ]]; then
      export "$line"
    fi
  done < "$ROOT_DIR/.env"
fi

parse_database_url() {
  local database_url="$1"
  python3 - "$database_url" <<'PY'
import sys
from urllib.parse import urlparse, unquote, parse_qs

raw = sys.argv[1]
u = urlparse(raw)
if u.scheme not in ("postgres", "postgresql"):
    raise SystemExit(f"unsupported DATABASE_URL scheme: {u.scheme!r}")

user = unquote(u.username or "")
password = unquote(u.password or "")
host = u.hostname or "localhost"
port = u.port or 5432
db = (u.path or "/").lstrip("/") or "postgres"

query = parse_qs(u.query)
sslmode = (query.get("sslmode") or ["prefer"])[0]

jdbc = f"jdbc:postgresql://{host}:{port}/{db}"
if sslmode:
    jdbc += f"?sslmode={sslmode}"

print(jdbc)
print(user)
print(password)
PY
}

resolve_connection() {
  if [[ -n "${FLYWAY_URL:-}" ]]; then
    FLYWAY_URL_RESOLVED="$FLYWAY_URL"
    FLYWAY_USER_RESOLVED="${FLYWAY_USER:-}"
    FLYWAY_PASSWORD_RESOLVED="${FLYWAY_PASSWORD:-}"
    return
  fi

  if [[ -n "${DATABASE_URL:-}" ]]; then
    local parsed
    parsed="$(parse_database_url "$DATABASE_URL")"
    FLYWAY_URL_RESOLVED="$(printf '%s\n' "$parsed" | sed -n '1p')"
    FLYWAY_USER_RESOLVED="$(printf '%s\n' "$parsed" | sed -n '2p')"
    FLYWAY_PASSWORD_RESOLVED="$(printf '%s\n' "$parsed" | sed -n '3p')"
    return
  fi

  local user="${POSTGRES_USER:-postgres}"
  local password="${POSTGRES_PASSWORD:-}"
  local db="${POSTGRES_DB:-postgres}"
  local host="${POSTGRES_HOST:-localhost}"
  local port="${POSTGRES_PORT:-5432}"

  if [[ -z "$password" ]]; then
    echo "error: set DATABASE_URL or POSTGRES_PASSWORD (see .env.example)" >&2
    exit 1
  fi

  FLYWAY_URL_RESOLVED="jdbc:postgresql://${host}:${port}/${db}"
  FLYWAY_USER_RESOLVED="$user"
  FLYWAY_PASSWORD_RESOLVED="$password"
}

run_flyway_local() {
  flyway \
    -url="$FLYWAY_URL_RESOLVED" \
    -user="$FLYWAY_USER_RESOLVED" \
    -password="$FLYWAY_PASSWORD_RESOLVED" \
    -locations="filesystem:$MIGRATIONS_DIR" \
    -connectRetries=10 \
    -cleanDisabled="${CLEAN_DISABLED:-true}" \
    "$COMMAND"
}

run_flyway_docker() {
  local docker_url="$FLYWAY_URL_RESOLVED"
  # Inside the container, localhost is the container itself. Remap to host gateway.
  if [[ "$docker_url" == *"localhost"* ]] || [[ "$docker_url" == *"127.0.0.1"* ]]; then
    docker_url="${docker_url//localhost/host.docker.internal}"
    docker_url="${docker_url//127.0.0.1/host.docker.internal}"
  fi

  docker run --rm \
    --add-host=host.docker.internal:host-gateway \
    -v "$MIGRATIONS_DIR:/flyway/sql:ro" \
    -e "FLYWAY_CLEAN_DISABLED=${CLEAN_DISABLED:-true}" \
    "$FLYWAY_IMAGE" \
    -url="$docker_url" \
    -user="$FLYWAY_USER_RESOLVED" \
    -password="$FLYWAY_PASSWORD_RESOLVED" \
    -locations="filesystem:/flyway/sql" \
    -connectRetries=10 \
    -cleanDisabled="${CLEAN_DISABLED:-true}" \
    "$COMMAND"
}

if [[ ! -d "$MIGRATIONS_DIR" ]]; then
  echo "error: migrations directory not found: $MIGRATIONS_DIR" >&2
  exit 1
fi

resolve_connection

if [[ -z "$FLYWAY_URL_RESOLVED" ]]; then
  echo "error: Flyway JDBC URL is empty" >&2
  exit 1
fi

echo "flyway $COMMAND"
echo "  url:  $FLYWAY_URL_RESOLVED"
echo "  user: ${FLYWAY_USER_RESOLVED:-<empty>}"
echo "  dir:  $MIGRATIONS_DIR"

if command -v flyway >/dev/null 2>&1; then
  run_flyway_local
elif command -v docker >/dev/null 2>&1; then
  run_flyway_docker
else
  echo "error: neither flyway nor docker found on PATH" >&2
  exit 1
fi
