# TalentPilot Agent Guide

## Read This First

This repository is the TalentPilot recruiting intelligence assistant for the Computing Power Business Unit. The product requirements live in [PRD.md](PRD.md). The current work phase is foundation design: repository structure, frontend/backend separation, engineering practices, testing discipline, API contracts, database strategy, and Agent-friendly project memory.

Agents must read this file before making changes. If project-wide consensus changes, update this file in the same change set.

## Current Project Phase

- Phase: Foundation architecture and SPEC creation.
- Current source of truth: [docs/specs/000-foundation-architecture.md](docs/specs/000-foundation-architecture.md).
- Current status checklist: [docs/project-status.md](docs/project-status.md).
- PRD scope: W3 login, IAM, resume parsing, recommendation, resume library, department/position management, user/role management, notifications, custom role management.

## Stable Project Decisions

- Repository shape: monorepo.
- Frontend app: `apps/web`.
- Backend app: `apps/api`.
- Shared/generated packages: `packages/*`.
- Frontend stack: React, TypeScript, Vite, Tailwind CSS, shadcn/ui, Vercel AI SDK, lucide-react, and a React-compatible motion library.
- Backend stack: Go, Echo, GORM, goose.
- Database strategy: SQLite for local automated testing; PostgreSQL for production.
- Migration strategy: goose migrations are the schema source of truth. Do not rely on GORM AutoMigrate for production schema changes.
- API contract strategy: backend code is the source of truth; backend route/DTO annotations generate OpenAPI; frontend TypeScript client is generated from OpenAPI.
- Testing strategy: red-green-refactor TDD. No production behavior without a failing test first.
- Frontend component rule: business pages must not use raw interactive HTML elements directly. Use shadcn/ui or project-wrapped components.
- Error strategy: backend returns stable error codes, default Chinese messages, request IDs, and structured details; frontend translates display text through i18n.

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

## Commands Index

Commands are not implemented yet. Planned command surface:

- `make setup`
- `make dev`
- `make dev-web`
- `make dev-api`
- `make test`
- `make test-api`
- `make test-web`
- `make test-e2e`
- `make lint`
- `make typecheck`
- `make migrate-up`
- `make migrate-down`
- `make openapi-generate`
- `make openapi-check`
- `make client-generate`
- `make ci`

When these commands become available, update this section with exact behavior and prerequisites.

## Documentation Index

- Product requirements: [PRD.md](PRD.md)
- Foundation SPEC: [docs/specs/000-foundation-architecture.md](docs/specs/000-foundation-architecture.md)
- Project checklist: [docs/project-status.md](docs/project-status.md)
- ADR index:
  - [docs/adr/0001-use-monorepo.md](docs/adr/0001-use-monorepo.md)
  - [docs/adr/0002-use-react-vite-shadcn.md](docs/adr/0002-use-react-vite-shadcn.md)
  - [docs/adr/0003-use-go-echo-gorm-goose.md](docs/adr/0003-use-go-echo-gorm-goose.md)
  - [docs/adr/0004-backend-code-generates-openapi.md](docs/adr/0004-backend-code-generates-openapi.md)
  - [docs/adr/0005-use-tdd-red-green-refactor.md](docs/adr/0005-use-tdd-red-green-refactor.md)
