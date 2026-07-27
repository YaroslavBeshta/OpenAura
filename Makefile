.PHONY: run tidy test test-unit migrate migrate-info migrate-reapply generate-migration swagger

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
