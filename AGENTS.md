# TalentPilot Agent Guide

## Read This First

This repository is the TalentPilot recruiting intelligence assistant for the Computing Power Business Unit. The product requirements live in [PRD.md](PRD.md). The current work phase is E6 User and Role Management SPEC planning, building on the implemented IAM, E2, E3, E4, and E5 foundations.

Agents must read this file before making changes. If project-wide consensus changes, update this file in the same change set.

## Current Project Phase

- Phase: E6 User and Role Management SPEC planning, after the E3 resume recommendation workflow implementation.
- Current source of truth: PRD E6 user/role management scope in [PRD.md](PRD.md), [docs/project-status.md](docs/project-status.md), implemented E3 behavior from [docs/specs/006-resume-recommendation-notification.md](docs/specs/006-resume-recommendation-notification.md) and [docs/superpowers/plans/2026-07-12-e3-resume-recommendation-implementation.md](docs/superpowers/plans/2026-07-12-e3-resume-recommendation-implementation.md), implemented E2 behavior from [docs/specs/005-resume-parse-workspace.md](docs/specs/005-resume-parse-workspace.md) and [docs/superpowers/plans/2026-07-07-e2-resume-parse-workspace-implementation.md](docs/superpowers/plans/2026-07-07-e2-resume-parse-workspace-implementation.md), implemented E5 behavior from [docs/specs/004-position-department-management.md](docs/specs/004-position-department-management.md) and [docs/superpowers/plans/2026-07-04-e5-department-position-management-implementation.md](docs/superpowers/plans/2026-07-04-e5-department-position-management-implementation.md), implemented E4 behavior from [docs/specs/003-resume-library-import.md](docs/specs/003-resume-library-import.md), implemented IAM behavior from [docs/specs/002-iam-permission-model.md](docs/specs/002-iam-permission-model.md), and [docs/specs/001-auth-session-w3.md](docs/specs/001-auth-session-w3.md).
- Current status checklist: [docs/project-status.md](docs/project-status.md).
- PRD scope: W3 login, IAM, resume parsing, recommendation, resume library, department/position management, user/role management, notifications, custom role management.

## Stable Project Decisions

- Repository shape: monorepo.
- Frontend app: `apps/web`.
- Backend app: `apps/api`.
- Shared/generated packages: `packages/*`.
- JavaScript package manager: pnpm.
- Runtime baseline: Node 24 LTS for frontend tooling; Go 1.26 for backend tooling.
- Frontend stack: React, TypeScript, Vite, Tailwind CSS, shadcn/ui, Vercel AI SDK, lucide-react, and a React-compatible motion library.
- Backend stack: Go, Echo, GORM, goose.
- Database strategy: SQLite for default local automated testing; PostgreSQL for production and CI migration compatibility when `DATABASE_URL` is set.
- Migration strategy: goose migrations are the schema source of truth. Do not rely on GORM AutoMigrate for production schema changes.
- API contract strategy: backend code is the source of truth; Huma with the Echo adapter generates OpenAPI from backend route/DTO definitions; frontend TypeScript client is generated from OpenAPI.
- OpenAPI generation library: Huma with Echo adapter.
- Container ownership model: API image runs as non-root `talentpilot` uid/gid `10001`; web image uses `nginxinc/nginx-unprivileged`; Compose Postgres data ownership is managed by the official Postgres image and named volume.
- Testing strategy: red-green-refactor TDD. No production behavior without a failing test first.
- Frontend component rule: business pages must not use raw interactive HTML elements directly. Use shadcn/ui or project-wrapped components.
- Error strategy: backend returns stable error codes, default Chinese messages, request IDs, and structured details; frontend translates display text through i18n.
- Production credential safety: W3 login requires HTTPS before password handling. `X-Forwarded-Proto` is trusted only when `TRUST_FORWARDED_PROTO=true` is explicitly set behind a trusted proxy that sanitizes forwarded headers.

## Required Working Rules for Agents

- Read [PRD.md](PRD.md), this file, the active SPEC, and [docs/project-status.md](docs/project-status.md) before planning non-trivial work.
- Keep architecture boundaries intact. Frontend state is not a security boundary; backend authorization is authoritative.
- Preserve frontend/backend separation. Frontend calls generated clients, not handwritten business fetch calls.
- Keep project-wide decisions documented. If a stable consensus changes, update this file.
- Keep progress visible. When an item moves to Done, In Progress, Blocked, or Deferred, update [docs/project-status.md](docs/project-status.md).
- Add ADRs for durable architecture decisions that affect future work.
- Do not bury long design details in this file. Link to SPECs and ADRs instead.
- Keep AI output and edits small enough to be reliable. Do not emit or apply more than about 1000 lines of code in one response, one patch, or one implementation step. Split large changes into smaller reviewed batches and state the split points before continuing.

## When to Update This File

Update `AGENTS.md` whenever work changes any project-wide consensus, including:

- Technology stack or major library choice.
- Repository layout.
- API contract generation flow.
- Database, ORM, migration, or environment strategy.
- Testing/TDD rules or CI gates.
- Security, authorization, audit, session, or privacy rules.
- Frontend component rules or design-system constraints.
- Commands that future Agents must know.
- Documentation index or active SPEC order.

This file should contain stable rules and indexes only. Put detailed designs in `docs/specs/`, durable decisions in `docs/adr/`, and live status in `docs/project-status.md`.

## TDD Rules

- Write the failing test first.
- Run the test and verify it fails for the expected reason.
- Write the smallest production code needed to pass.
- Run the target test and relevant surrounding tests.
- Refactor only after tests are green.
- New bugs require a failing regression test before the fix.
- Generated code, one-off scaffolding, and configuration-only changes may use verification checks instead of behavior-first tests, but this exception must be explicit in the task notes.

## Output Size and Change Batching

- Do not output more than about 1000 lines of code in a single assistant response.
- Do not create a single patch larger than about 1000 lines of code unless the user explicitly approves it.
- Do not design implementation tasks that require one Agent to write a very large code dump in one step.
- Split large work by verifiable boundaries: package setup, failing test, minimal implementation, generated artifact, documentation update, or CI check.
- Prefer several small commits over one large commit.
- If generated files exceed this size, generate them with tools and review summaries instead of pasting their full contents into chat.

## Architecture Boundaries

- `apps/web` owns UI, interaction, form pre-validation, local state, i18n, accessibility, and generated client consumption.
- `apps/api` owns authentication, sessions, authorization, data filtering, transactions, audit logging, file safety checks, and business rules.
- `packages/api-contract` stores generated OpenAPI output.
- `packages/api-client` stores generated frontend API client code.
- Backend DTOs must not expose GORM persistence models directly.
- Backend handlers must not contain business authorization logic; authorization must be centralized through middleware/guards and IAM services.
- List APIs must apply permission predicates in backend queries. Do not fetch all data and filter permissions in the frontend.
- IAM implementation source: [docs/specs/002-iam-permission-model.md](docs/specs/002-iam-permission-model.md). It defines the implemented resource/action enums, preset roles, Permission whitelist, RoleRelation expansion, data-scope predicates, `/me` IAM additions, error codes, audit boundaries, and cache invalidation.

## Commands Index

Implemented command surface:

- `make setup`: enable Corepack, install pnpm dependencies, and download Go modules. Requires Node/Corepack and Go; Corepack provides pnpm.
- `make dev`: run workspace development services through pnpm.
- `make dev-web`: run the Vite web app.
- `make dev-api`: run the Echo API with `go run`.
- `make test`: run backend and frontend tests.
- `make test-api`: run `go test ./...` in `apps/api`; migration tests also exercise PostgreSQL when `DATABASE_URL` is set. Requires Go.
- `make test-web`: run Vitest for `apps/web`. Requires pnpm dependencies.
- `make test-e2e`: reserved for future Playwright coverage; Playwright dependency/config is not installed yet and this command is not part of current passing gates.
- `make lint`: run pnpm lint plus `go vet ./...`. Requires pnpm dependencies and Go.
- `make typecheck`: run TypeScript type checks.
- `make build`: build frontend packages and the Go API binary at `apps/api/bin/api`. Requires pnpm dependencies and Go.
- `make migrate-up`: run goose SQLite migrations for local development. Requires Go.
- `make migrate-down`: roll back one goose SQLite migration for local development. Requires Go.
- `make openapi-generate`: generate `packages/api-contract/openapi.json` from the Go API via Huma. Requires Go.
- `make openapi-check`: regenerate OpenAPI and fail on committed contract drift. Requires Go and a clean expected contract.
- `make client-generate`: generate the TypeScript API client from OpenAPI.
- `make client-check`: regenerate the TypeScript API client and fail on committed client drift.
- `make ci`: run lint, typecheck, tests, OpenAPI drift check, client drift check, and builds. Requires pnpm dependencies and Go.
- `docker compose config`: validate local compose configuration. Requires Docker Compose.
- `docker compose build`: build local API and web images. Requires Docker.

## Documentation Index

- Product requirements: [PRD.md](PRD.md)
- Foundation SPEC: [docs/specs/000-foundation-architecture.md](docs/specs/000-foundation-architecture.md)
- E1 Auth Session W3 SPEC: [docs/specs/001-auth-session-w3.md](docs/specs/001-auth-session-w3.md)
- IAM Permission Model SPEC: [docs/specs/002-iam-permission-model.md](docs/specs/002-iam-permission-model.md)
- IAM implementation plan: [docs/superpowers/plans/2026-07-03-iam-permission-model-implementation.md](docs/superpowers/plans/2026-07-03-iam-permission-model-implementation.md)
- E4 Resume Library SPEC: [docs/specs/003-resume-library-import.md](docs/specs/003-resume-library-import.md)
- E4 implementation plan: [docs/superpowers/plans/2026-07-04-e4-resume-library-implementation.md](docs/superpowers/plans/2026-07-04-e4-resume-library-implementation.md)
- E5 Department and Position Management SPEC: [docs/specs/004-position-department-management.md](docs/specs/004-position-department-management.md)
- E5 implementation plan: [docs/superpowers/plans/2026-07-04-e5-department-position-management-implementation.md](docs/superpowers/plans/2026-07-04-e5-department-position-management-implementation.md)
- E2 Resume Parse Workspace SPEC: [docs/specs/005-resume-parse-workspace.md](docs/specs/005-resume-parse-workspace.md)
- E2 implementation plan: [docs/superpowers/plans/2026-07-07-e2-resume-parse-workspace-implementation.md](docs/superpowers/plans/2026-07-07-e2-resume-parse-workspace-implementation.md)
- E3 Resume Recommendation and Notification SPEC: [docs/specs/006-resume-recommendation-notification.md](docs/specs/006-resume-recommendation-notification.md)
- E3 implementation plan: [docs/superpowers/plans/2026-07-12-e3-resume-recommendation-implementation.md](docs/superpowers/plans/2026-07-12-e3-resume-recommendation-implementation.md)
- Project checklist: [docs/project-status.md](docs/project-status.md)
- ADR index:
  - [docs/adr/0001-use-monorepo.md](docs/adr/0001-use-monorepo.md)
  - [docs/adr/0002-use-react-vite-shadcn.md](docs/adr/0002-use-react-vite-shadcn.md)
  - [docs/adr/0003-use-go-echo-gorm-goose.md](docs/adr/0003-use-go-echo-gorm-goose.md)
  - [docs/adr/0004-backend-code-generates-openapi.md](docs/adr/0004-backend-code-generates-openapi.md)
  - [docs/adr/0005-use-tdd-red-green-refactor.md](docs/adr/0005-use-tdd-red-green-refactor.md)
  - [docs/adr/0006-use-huma-for-openapi-generation.md](docs/adr/0006-use-huma-for-openapi-generation.md)
