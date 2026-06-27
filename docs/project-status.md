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
| Architecture Decisions | ADR set for foundation choices | Done | `docs/adr/0001` through `0005` reviewed and approved. | Add ADRs for future durable decisions. |
| Repository | Monorepo skeleton | Not Started | No `apps/` or `packages/` code yet. | Create foundation scaffold after implementation plan approval. |
| Frontend | React/Vite app | Not Started | No `apps/web`. | Scaffold with TypeScript, Vite, Tailwind, shadcn/ui. |
| Frontend | Component rules enforced | Not Started | No ESLint/checks yet. | Add lint/review rule against raw interactive HTML in business pages. |
| Frontend | i18n foundation | Not Started | No `apps/web/src/i18n`. | Add `zh-CN` first, `en-US` structure reserved. |
| Backend | Go/Echo API app | Not Started | No `apps/api`. | Scaffold Echo app with config, middleware, health endpoint. |
| Backend | GORM repositories | Not Started | No persistence layer. | Add DB adapter and repository pattern. |
| Backend | goose migrations | Not Started | No migrations. | Add first schema migration set. |
| API Contract | Backend-generated OpenAPI | Not Started | No generator configured. | Choose generator and add generate/check commands. |
| API Contract | Generated frontend client | Not Started | No `packages/api-client`. | Generate TypeScript client from OpenAPI. |
| Testing | TDD rules documented | Done | `AGENTS.md`, foundation SPEC, ADR 0005 | Enforce in implementation workflow. |
| Testing | Backend tests | Not Started | No backend code. | Add Go unit/integration test harness. |
| Testing | Frontend tests | Not Started | No frontend code. | Add Vitest, RTL, MSW, Playwright. |
| CI | Quality gate workflow | Not Started | No workflow implementation. | Add lint, tests, OpenAPI drift, builds, Docker check. |
| Containers | Local compose stack | Not Started | No compose services implemented. | Add API, web, PostgreSQL; defer optional Redis/MinIO if needed. |

## Business Epic Checklist

| Epic | Scope | Status | Foundation Dependency | Next SPEC |
| --- | --- | --- | --- | --- |
| E1 | Login and identity authentication | Not Started | API app, auth/session foundation, IAM basics | `001-auth-session-w3.md` |
| E2 | Resume parsing | Not Started | Resume library, positions, matching, jobs | `005-resume-parse-workspace.md` |
| E3 | Resume recommendation and routing | Not Started | IAM, resumes, positions, notifications | `006-resume-recommendation-notification.md` |
| E4 | Resume library | Not Started | IAM, DB schema, resume service | `003-resume-library-import.md` |
| E5 | Department and position management | Not Started | IAM, departments, positions | `004-position-department-management.md` |
| E6 | User and role management | Not Started | IAM, user list, role list | `007-user-role-management.md` |
| E7 | Notifications | Not Started | Recommendation events, notification service | `006-resume-recommendation-notification.md` |
| E8 | Custom role management | Not Started | IAM model, permissions, RoleRelation validation | `008-custom-role-management.md` |

## Current Recommended Order

1. Create implementation plan for monorepo foundation.
2. Scaffold `apps/web`, `apps/api`, `packages/api-contract`, and `packages/api-client`.
3. Add first CI, test harnesses, and generated contract checks.
4. Write `001-auth-session-w3.md` and `002-iam-permission-model.md`.
