.PHONY: run tidy test test-cover test-unit migrate migrate-info migrate-reapply generate-migration swagger

ifneq (,$(wildcard .env))
include .env
export
endif

run:
	go run ./cmd/server

tidy:
	go mod tidy

test:
	go test ./... -count=1 -p 1

# Packages included in coverage (exclude generated docs and test helpers).
COVER_PKGS := $(shell go list ./internal/... | grep -vE '/(apitest|testutil)$$' | paste -sd, -)

test-cover:
	@test -n "$(DATABASE_URL)" || (echo 'DATABASE_URL is required (copy .env.example to .env and start Postgres)' >&2; exit 1)
	OPENAURA_REQUIRE_DB=1 go test ./... -count=1 -p 1 -covermode=atomic -coverpkg=$(COVER_PKGS) -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -n 1
	@echo "HTML report: go tool cover -html=coverage.out"

test-unit:
	go test ./internal/httpx ./internal/store -count=1

migrate:
	./tools/apply-migrations.sh migrate

migrate-info:
	./tools/apply-migrations.sh info

migrate-reapply:
	CONFIRM=yes ./tools/reapply-migrations.sh

generate-migration:
	@test -n "$(name)" || (echo 'usage: make generate-migration name=create_roles' >&2; exit 1)
	./tools/generate-migration.sh "$(name)"

swagger:
	./tools/generate-swagger.sh
