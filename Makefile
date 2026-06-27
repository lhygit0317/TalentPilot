.PHONY: help setup dev dev-web dev-api test test-api test-web test-e2e lint typecheck build migrate-up migrate-down openapi-generate openapi-check client-generate ci

help:
	@grep -E '^[a-zA-Z_-]+:' Makefile | cut -d: -f1 | sort

setup:
	corepack enable
	pnpm install
	cd apps/api && go mod download

dev:
	pnpm dev

dev-web:
	pnpm --filter @talentpilot/web dev

dev-api:
	cd apps/api && go run ./cmd/api

test:
	$(MAKE) test-api
	$(MAKE) test-web

test-api:
	cd apps/api && go test ./...

test-web:
	pnpm --filter @talentpilot/web test -- --run

test-e2e:
	pnpm --filter @talentpilot/web test:e2e

lint:
	pnpm lint
	cd apps/api && go vet ./...

typecheck:
	pnpm typecheck

build:
	pnpm build
	cd apps/api && go build ./cmd/api

migrate-up:
	cd apps/api && go run github.com/pressly/goose/v3/cmd/goose sqlite3 "file:talentpilot_dev.db?_foreign_keys=on" up

migrate-down:
	cd apps/api && go run github.com/pressly/goose/v3/cmd/goose sqlite3 "file:talentpilot_dev.db?_foreign_keys=on" down

openapi-generate:
	cd apps/api && go run ./cmd/openapi > ../../packages/api-contract/openapi.json

openapi-check:
	$(MAKE) openapi-generate
	git diff --exit-code packages/api-contract/openapi.json

client-generate:
	pnpm --filter @talentpilot/api-client generate

ci:
	$(MAKE) lint
	$(MAKE) typecheck
	$(MAKE) test
	$(MAKE) openapi-check
	$(MAKE) client-generate
	$(MAKE) build
