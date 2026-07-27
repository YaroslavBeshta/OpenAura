#!/usr/bin/env bash
# Generate OpenAPI/Swagger docs from handler annotations via swag.
#
# Usage:
#   ./tools/generate-swagger.sh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

SWAG_VERSION="${SWAG_VERSION:-v1.16.6}"

echo "generating swagger docs with swag ${SWAG_VERSION}"
go run "github.com/swaggo/swag/cmd/swag@${SWAG_VERSION}" init \
  -g cmd/server/main.go \
  -d . \
  -o docs \
  --parseInternal \
  --outputTypes go,json,yaml

echo "wrote docs/swagger.json docs/swagger.yaml docs/docs.go"
