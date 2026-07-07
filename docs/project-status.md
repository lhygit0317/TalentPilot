# Project Status

This file is the live checklist for foundation work and PRD delivery. Update it whenever scope, status, evidence, or next actions change.

## Status Legend

| Status | Meaning |
| --- | --- |
| Done | Implemented or documented, with evidence. |
| In Progress | Actively being designed or implemented. |
| Not Started | Planned but no concrete work yet. |
| Blocked | Cannot progress without a decision or external dependency. |
| Deferred | Intentionally postponed. |

## Foundation Checklist

| Area | Item | Status | Evidence | Next |
| --- | --- | --- | --- | --- |
| Product | PRD captured | Done | `PRD.md` exists. | Keep as product source of truth. |
| Architecture | Foundation SPEC | Done | `docs/specs/000-foundation-architecture.md` reviewed and approved. | Use as implementation planning input. |
| Agent Infrastructure | Main Agent memory | Done | `AGENTS.md` | Keep updated when project-wide consensus changes. |
| Agent Infrastructure | Live project checklist | Done | `docs/project-status.md` | Update after each design or implementation milestone. |
| Architecture Decisions | ADR set for foundation choices | Done | `docs/adr/0001` through `0006` exist. | Add ADRs for future durable decisions. |
| Repository | Monorepo skeleton | Done | `apps/web`, `apps/api`, `packages/api-contract`, `packages/api-client`, `pnpm-workspace.yaml`, and `Makefile` exist. | Use foundation for next SPEC work. |
| Frontend | React/Vite app | Done | `apps/web/package.json`, `apps/web/src/app/App.tsx`, Vite, Tailwind, and Dockerfile exist. | Extend through business SPECs. |
| Frontend | Component rules documented | In Progress | Component convention is documented in `AGENTS.md` and SPEC; `apps/web/src/components/ui/button.tsx` exists. No lint/check enforces raw interactive HTML yet. | Add stricter lint/check when business pages expand. |
| Frontend | i18n foundation | Done | `apps/web/src/i18n/zh-CN.ts` and `apps/web/src/i18n/en-US.ts` exist. | Add feature messages with each business SPEC. |
| Backend | Go/Echo API app | Done | `apps/api/cmd/api/main.go`, `internal/app/server.go`, and `/healthz` tests exist. | Extend through auth and IAM SPECs. |
| Backend | GORM DB adapter | Done | `apps/api/internal/platform/db/db.go` provides SQLite/PostgreSQL GORM adapter. | Add domain repositories with business SPECs. |
| Backend | goose migrations | Done | `apps/api/migrations/000001_create_foundation_tables.sql` and SQLite/PostgreSQL migration integration tests exist. | Add migrations per schema change. |
| API Contract | Backend-generated OpenAPI | Done | Huma/Echo OpenAPI generation in `apps/api/cmd/openapi`; `make openapi-check` verifies drift. | Keep generated contract current. |
| API Contract | Generated frontend client | Done | `packages/api-client/src/schema.d.ts`, `src/index.ts`, and `make client-check` exist. | Regenerate after API changes. |
| Testing | TDD rules documented | Done | `AGENTS.md`, foundation SPEC, ADR 0005 | Enforce in implementation workflow. |
| Testing | Backend tests | Done | Go unit tests plus SQLite and `DATABASE_URL`-gated PostgreSQL migration integration tests exist under `apps/api`; `make test-api` runs them. | Add failing tests before new behavior. |
| Testing | Frontend tests | Done | Vitest and Testing Library setup exist under `apps/web`; `make test-web` runs them. | Add MSW/Playwright coverage when workflows need it. |
| CI | Quality gate workflow | Done | `.github/workflows/ci.yml` runs pnpm setup, Go setup, PostgreSQL service-backed tests, lint, typecheck, OpenAPI/client drift checks, and build. | Keep CI aligned with Makefile. |
| Containers | Local compose stack | Done | `docker-compose.yml`, `apps/api/Dockerfile`, and `apps/web/Dockerfile` exist. | Keep image builds green as services grow. |

## Business Epic Checklist

| Epic | Scope | Status | Foundation Dependency | Next SPEC |
| --- | --- | --- | --- | --- |
| E1 | Login and identity authentication | Done | `docs/specs/001-auth-session-w3.md`; backend auth schema/service/routes, generated OpenAPI/client, and frontend W3 login shell are implemented. Verification: `make test-api`, `make test-web`, `make openapi-check`, `make client-check`, `make typecheck`, `make lint`, `make build`. | Use as dependency for IAM implementation. |
| IAM | Permission model foundation | Done | `docs/specs/002-iam-permission-model.md` implemented: preset role seeds, Permission whitelist, RoleRelation expansion, SQL principal loading, `/me` IAM fields, backend guard, mutation audit/cache hooks, frontend shell scope display, OpenAPI/client generation. Verification: `make test-api`, `make test-web`, `make openapi-check`, `make client-check`, `make typecheck`, `make lint`, `make build`, `git diff --check`. | Use as authorization and data-scope foundation for E4/E5/E2/E3/E6/E8. |
| E2 | Resume parsing | In Progress | `docs/specs/005-resume-parse-workspace.md` drafted for full Story 2.1-2.5 delivery using synchronous backend matching plus deterministic interview-question generation. Depends on E4 resumes and E5 positions. | Create implementation plan, then implement with TDD. |
| E3 | Resume recommendation and routing | Not Started | IAM, resumes, positions, notifications | `006-resume-recommendation-notification.md` |
| E4 | Resume library | Done | `docs/specs/003-resume-library-import.md` implemented: permission-filtered list/detail/delete, import jobs, audit coverage, API routes, generated OpenAPI/client wrappers, and frontend list/search/detail/import/delete workflows. Verification: `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make test-api`, `CI=true pnpm --filter @talentpilot/web test -- --run`, `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-check`, `make client-check`, `make typecheck`, `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make lint`, `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make build`. Playwright remains outside the passing gate; `make test-e2e` is still reserved. | Use E4 as dependency for E5/E2/E3/E7. |
| E5 | Department and position management | Done | `docs/specs/004-position-department-management.md` implemented: permission-filtered department and position list/detail, super-admin writes, delete protection, audit coverage, API routes, generated OpenAPI/client wrappers, and frontend management workflows. Verification: `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH CI=true make ci`, `git diff --check`; the CI gate includes lint, typecheck, backend/frontend tests, OpenAPI/client drift checks, and builds. Playwright remains outside the passing gate; `make test-e2e` is still reserved. | Use E5 as dependency for E2/E3/E6. |
| E6 | User and role management | Not Started | IAM, user list, role list | `007-user-role-management.md` |
| E7 | Notifications | Not Started | Recommendation events, notification service | `006-resume-recommendation-notification.md` |
| E8 | Custom role management | Not Started | IAM model, permissions, RoleRelation validation | `008-custom-role-management.md` |

## Current Recommended Order

1. Create the E2 implementation plan from `docs/specs/005-resume-parse-workspace.md`, then implement with TDD.
2. Plan E3 after E2 parsing output and matching handoff are implemented.
3. Keep `make test-e2e` reserved until Playwright is installed and configured.
