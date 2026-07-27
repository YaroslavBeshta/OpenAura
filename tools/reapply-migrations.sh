#!/usr/bin/env bash
# Drop all objects in the schema (flyway clean) and re-apply migrations.
#
# WARNING: destroys all data in the target database schema.
#
# Usage:
#   ./tools/reapply-migrations.sh
#   DATABASE_URL=postgres://... ./tools/reapply-migrations.sh
#   CONFIRM=yes ./tools/reapply-migrations.sh   # skip interactive prompt
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

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

if [[ "${CONFIRM:-}" != "yes" ]]; then
  echo "This will DROP all objects in the database schema and re-run migrations."
  read -r -p "Type 'yes' to continue: " answer
  if [[ "$answer" != "yes" ]]; then
    echo "aborted"
    exit 1
  fi
fi

echo "cleaning schema..."
# Flyway clean is disabled by default; enable it for this destructive local/dev workflow.
CLEAN_DISABLED=false "$ROOT_DIR/tools/apply-migrations.sh" clean

echo "applying migrations..."
"$ROOT_DIR/tools/apply-migrations.sh" migrate

echo "reapply complete"
