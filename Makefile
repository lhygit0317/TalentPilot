.PHONY: help setup dev dev-web dev-api test test-api test-web test-e2e lint typecheck build migrate-up migrate-down openapi-generate openapi-check client-generate ci

define require_package
	@test -f "$(1)" || (echo "Missing required package manifest: $(1)" >&2; exit 1)
endef

help:
	@grep -E '^[a-zA-Z_-]+:' Makefile | cut -d: -f1 | sort

setup:
	corepack enable
	pnpm install
	cd apps/api && go mod download

dev:
	$(call require_package,apps/web/package.json)
	pnpm dev

dev-web:
	$(call require_package,apps/web/package.json)
	pnpm --filter @talentpilot/web dev

dev-api:
	cd apps/api && go run ./cmd/api

test:
	$(MAKE) test-api
	$(MAKE) test-web

test-api:
	cd apps/api && go test ./...

test-web:
	$(call require_package,apps/web/package.json)
	pnpm --filter @talentpilot/web test -- --run

test-e2e:
	$(call require_package,apps/web/package.json)
	pnpm --filter @talentpilot/web test:e2e

lint:
	$(call require_package,apps/web/package.json)
	pnpm lint
	cd apps/api && go vet ./...

typecheck:
	$(call require_package,apps/web/package.json)
	$(call require_package,packages/api-client/package.json)
	$(call require_package,packages/api-contract/package.json)
	pnpm typecheck

build:
	$(call require_package,apps/web/package.json)
	pnpm build
	cd apps/api && go build ./cmd/api

migrate-up:
	cd apps/api && go run github.com/pressly/goose/v3/cmd/goose sqlite3 "file:talentpilot_dev.db?_foreign_keys=on" up

migrate-down:
	cd apps/api && go run github.com/pressly/goose/v3/cmd/goose sqlite3 "file:talentpilot_dev.db?_foreign_keys=on" down

openapi-generate:
	cd apps/api && go run ./cmd/openapi > ../../packages/api-contract/openapi.json

openapi-check:
	$(call require_package,packages/api-contract/package.json)
	$(MAKE) openapi-generate
	git diff --exit-code packages/api-contract/openapi.json

client-generate:
	$(call require_package,packages/api-client/package.json)
	pnpm --filter @talentpilot/api-client generate

ci:
	$(MAKE) lint
	$(MAKE) typecheck
	$(MAKE) test
	$(MAKE) openapi-check
	$(MAKE) client-generate
	$(MAKE) build
