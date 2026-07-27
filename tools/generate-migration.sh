#!/usr/bin/env bash
# Create a new Flyway versioned migration under migrations/.
#
# Usage:
#   ./tools/generate-migration.sh create_roles
#   ./tools/generate-migration.sh "add user status"
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-$ROOT_DIR/migrations}"

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <description>" >&2
  echo "example: $0 create_roles" >&2
  exit 1
fi

raw="$*"
description="$(
  printf '%s' "$raw" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/_/g; s/^_+//; s/_+$//'
)"

if [[ -z "$description" ]]; then
  echo "error: description must contain at least one alphanumeric character" >&2
  exit 1
fi

mkdir -p "$MIGRATIONS_DIR"

next_version="$(
  python3 - "$MIGRATIONS_DIR" <<'PY'
import pathlib
import re
import sys

migrations = pathlib.Path(sys.argv[1])
pattern = re.compile(r"^V(\d+)(?:[_.]\d+)*__.*\.sql$")
highest = 0
for path in migrations.glob("V*.sql"):
    match = pattern.match(path.name)
    if match:
        highest = max(highest, int(match.group(1)))
print(highest + 1)
PY
)"

filename="V${next_version}__${description}.sql"
filepath="$MIGRATIONS_DIR/$filename"

if [[ -e "$filepath" ]]; then
  echo "error: migration already exists: $filepath" >&2
  exit 1
fi

cat > "$filepath" <<EOF
-- Migration: V${next_version}__${description}
-- Created: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

EOF

echo "created $filepath"
