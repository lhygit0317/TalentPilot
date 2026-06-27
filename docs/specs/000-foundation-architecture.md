# 000 Foundation Architecture SPEC

## 1. Scope and Goals

This SPEC defines the project foundation for TalentPilot. It covers repository structure, frontend/backend separation, Agent-friendly project memory, backend and frontend architecture, API contract generation, database and migration strategy, error codes, i18n, TDD workflow, CI gates, containers, and future SPEC breakdown.

The foundation must support the PRD in [../../PRD.md](../../PRD.md), especially strict backend authorization, explicit relationship entities, auditability, async AI-heavy workflows, and a high-density enterprise UI.

## 2. Non-Goals

- This SPEC does not implement PRD Story details.
- This SPEC does not define every API endpoint.
- This SPEC does not choose a production W3 provider integration detail beyond reserving an adapter boundary.
- This SPEC does not require microservices.
- This SPEC does not require production object storage, cache, or queue in the first implementation slice, but it reserves adapter boundaries for them.

## 3. Repository Structure

The project uses a monorepo:

```text
TalentPilot/
  AGENTS.md
  README.md
  Makefile
  docker-compose.yml
  .env.example
  .github/workflows/
  apps/
    web/
    api/
  packages/
    api-contract/
    api-client/
    shared/
  docs/
    specs/
    adr/
    testing/
    runbooks/
    project-status.md
  scripts/
```

`apps/web` and `apps/api` are independently runnable. `packages/api-contract` stores generated OpenAPI output. `packages/api-client` stores the generated TypeScript client consumed by the frontend. `packages/shared` is reserved for stable shared assets that do not couple frontend and backend runtime code.

## 4. Agent-Friendly Project Memory

`AGENTS.md` is the main Agent memory entrypoint. It contains stable project consensus, required working rules, TDD rules, architecture boundaries, command indexes, and documentation indexes.

Agents must update `AGENTS.md` when project-wide consensus changes. Examples include technology stack changes, repository layout changes, API contract flow changes, database strategy changes, test/CI rule changes, frontend component rules, and security or authorization rules.

Detailed designs belong in `docs/specs/`. Durable architecture decisions belong in `docs/adr/`. Live progress belongs in `docs/project-status.md`.

Agents must keep output and edits small enough to be reliable. A single assistant response, patch, or implementation step should not contain more than about 1000 lines of code unless the user explicitly approves it. Large work must be split by verifiable boundaries such as setup, failing test, minimal implementation, generated artifact, documentation update, or CI check.

## 5. System Architecture

TalentPilot starts as a modular monolith backend with a separate frontend:

```text
apps/web -> generated API client -> apps/api -> database / storage / queue / cache
```

The backend is the only trusted business boundary:

- Authentication and session checks happen in the backend.
- Authorization and data-range filtering happen in the backend.
- Frontend permission gates improve UX only.
- List APIs push permission predicates into backend queries.
- Get, Update, and Delete APIs validate the target resource against the user's authorized scope.
- Business APIs must be explainable as `Resource + Action + data scope`.

This avoids microservice overhead while keeping module boundaries clear enough for later extraction if justified by an ADR.

## 6. Backend Architecture

Backend stack:

- Go
- Echo
- GORM
- goose

Planned layout:

```text
apps/api/
  cmd/api/main.go
  cmd/worker/main.go
  internal/
    app/
    config/
    platform/
      db/
      logger/
      transaction/
      storage/
      queue/
      openapi/
    http/
      middleware/
      response/
      validation/
    auth/
    iam/
    departments/
    positions/
    resumes/
    matching/
    notifications/
    audit/
    jobs/
  migrations/
  test/
    integration/
    fixtures/
```

Layering rules:

- Handlers bind HTTP input, call services/usecases, and return uniform responses.
- Services/usecases hold business rules, transaction orchestration, and authorization context.
- Repositories wrap GORM queries. GORM chains must not leak into service callers.
- Platform packages wrap infrastructure such as DB, logger, clock, ID generation, transaction helpers, storage, queue, and OpenAPI generation.
- Persistence models and API DTOs are separate.

Authorization rules:

- Session middleware runs before business handlers.
- Permission guard calls IAM to resolve roles, RoleRelation inheritance, Permission records, department scope, and AttributeCondition scope.
- Handlers must not implement ad hoc permission checks.
- IAM exposes central capabilities such as `Can`, `Scope`, and `RoleSummary`.
- RoleRelation cycle detection and maximum-depth protection are required.
- Permission cache invalidates when UserDepartmentRole, Permission, or RoleRelation changes.

Transactions and audit:

- Multi-record writes use transaction helpers.
- `Create Resume + Create DepartmentResume`, `Create Position + Create DepartmentPosition`, and `Create Role + Permission + RoleRelation` are atomic.
- Sensitive writes produce audit logs with request ID, actor, action, resource, target, result, and before/after summaries where applicable.

## 7. Frontend Architecture

Frontend stack:

- React
- TypeScript
- Vite
- Tailwind CSS
- shadcn/ui
- Vercel AI SDK
- lucide-react
- React-compatible motion library

Planned layout:

```text
apps/web/
  src/
    app/
      router.tsx
      providers.tsx
      layout/
    pages/
      login/
      resume-parse/
      resume-recommend/
      resume-library/
      departments-positions/
      users/
      roles/
    features/
      auth/
      iam/
      resumes/
      positions/
      matching/
      notifications/
      audit/
    components/
      ui/
      shell/
      feedback/
      data-display/
    api/
      client.ts
      errors.ts
    i18n/
      zh-CN.ts
      en-US.ts
      error-messages.ts
    styles/
      globals.css
      tokens.css
    test/
      setup.ts
      factories/
```

Frontend rules:

- Business pages must not call `fetch` directly.
- Business requests must use the generated client from `packages/api-client`.
- `apps/web/src/api/client.ts` may wrap the generated client for auth, request IDs, error normalization, and toast strategy.
- Business pages must not directly use raw interactive HTML elements such as `button`, `input`, `select`, `dialog`, `table`, or `form`.
- Use shadcn/ui or project-wrapped components for interactive UI. Semantic layout elements such as `main`, `section`, `header`, `nav`, `div`, `span`, `p`, and headings are allowed.
- UI tokens live in Tailwind theme and `tokens.css`.
- User-visible text, toast text, empty states, and error messages live in i18n dictionaries.
- lucide-react is the default icon source.
- Emoji icons and marketing-style landing pages are not allowed.

## 8. API Contract Generation

The backend code is the source of truth:

```text
Go routes + DTOs + annotations -> OpenAPI -> generated TypeScript client -> apps/web
```

Contract rules:

- OpenAPI output is stored in `packages/api-contract`.
- The frontend TypeScript client is stored in `packages/api-client`.
- The frontend must not handwrite business API DTOs.
- CI must fail when generated OpenAPI or generated client output drifts from the committed output.
- Standard response envelopes, pagination, error responses, and request ID propagation must be declared consistently in the generated contract.

## 9. Database and Migration Strategy

SQLite is used for local automated tests. PostgreSQL is used for production. SQL and schema design should prefer features supported by both databases.

goose migrations are the schema source of truth. GORM AutoMigrate must not be used for production schema evolution.

Initial tables:

- `users`
- `departments`
- `roles`
- `permissions`
- `role_relations`
- `user_department_roles`
- `positions`
- `department_positions`
- `resumes`
- `department_resumes`
- `position_resumes`
- `notifications`
- `audit_logs`
- `jobs`

JSON strategy:

- Query-critical fields must be normal columns.
- Extension/display fields may use JSON.
- `Resume.chan`, `Resume.expired`, `Position.chan`, and `Position.status` should be columns.
- `Permission.attributeConditions`, `Resume.profile`, and less-frequently queried structured payloads may use JSON.

Key constraints:

- `users.employee_id` is unique.
- `departments.id='__system__'` is the system department.
- `user_department_roles(user_id, department_id, role_id)` is unique.
- `department_resumes(resume_id)` is unique for current ownership.
- `position_resumes(resume_id, position_id, kind)` is unique where upsert behavior is required.
- `role_relations(parent_role_id, child_role_id)` is unique.
- Service logic prevents RoleRelation cycles.
- Deleting Resume cascades DepartmentResume and PositionResume records, but not Notification records.

Seed strategy:

- System department, system roles, Permission whitelist, system Permission records, and preset RoleRelation records are foundation seed data.
- Production must not include sample users, sample resumes, or sample positions.
- Development and test seed data must be isolated from production migrations.

## 10. Error Codes and i18n

Backend errors use stable codes and request IDs:

```json
{
  "code": "ROLE_RELATION_CYCLE",
  "message": "角色包含关系不能形成循环",
  "requestId": "req_xxx",
  "details": {
    "roleLabel": "高级评审者"
  }
}
```

Rules:

- `code` is stable and must not change when display text changes.
- `message` is a default Chinese fallback.
- `details` contains structured interpolation values.
- Frontend display text is selected from i18n dictionaries by `code`.
- Logs and audit records use error codes, not only display text.
- CI should verify known backend error codes have frontend i18n mappings.

Error code groups:

- `AUTH_*`
- `IAM_*`
- `RESUME_*`
- `POSITION_*`
- `NOTIFICATION_*`
- `JOB_*`
- `VALIDATION_*`
- `INTERNAL_*`

Frontend i18n starts with `zh-CN` and reserves `en-US` structure. UI text, toast text, empty states, validation messages, and API error messages are centralized.

## 11. Testing and TDD Workflow

TDD is mandatory for production behavior:

1. RED: write one minimal failing test.
2. Verify RED: run it and confirm the failure reason is expected.
3. GREEN: write the minimum production code to pass.
4. Verify GREEN: run the target test and relevant surrounding tests.
5. REFACTOR: clean up only while tests remain green.

Backend tests:

- Unit: IAM expansion, RoleRelation cycle detection, AttributeCondition merging, matching scores, recommendation deduplication, error code mapping.
- Integration: Echo handlers with SQLite and goose migrations.
- Migration: goose up/down with SQLite and PostgreSQL in CI.
- Contract: OpenAPI generation and drift checks.

Frontend tests:

- Vitest for utilities, hooks, reducers, and formatting.
- React Testing Library for component behavior.
- MSW for API mocks around generated client usage.
- Playwright for critical E2E smoke flows.
- Accessibility checks for focus, keyboard behavior, ARIA, and reduced motion.

Exceptions:

- Generated code and pure configuration may use generation/drift/validation checks instead of behavior-first tests.
- Any exception must be explicit in task notes.

## 12. CI and Quality Gates

Planned CI stages:

1. Lint.
2. Format check.
3. Typecheck.
4. Backend unit tests.
5. Backend integration tests with SQLite.
6. Backend migration tests with PostgreSQL service.
7. OpenAPI generate and drift check.
8. Frontend client generate and drift check.
9. Frontend unit/component tests.
10. Frontend build.
11. E2E smoke tests.
12. Docker build check.

Quality gates:

- OpenAPI drift fails CI.
- Generated client drift fails CI.
- goose migration failure fails CI.
- Missing i18n mapping for known error codes fails CI.
- Frontend raw interactive HTML in business pages fails lint or review.
- New project-wide consensus without `AGENTS.md` update fails review.
- New production behavior without a red-green test trail fails review.
- Implementation steps or patches that exceed the Agent output-size rule fail review unless explicitly approved.

## 13. Container and Environment Strategy

The project must be container friendly and low-resource.

Planned local services:

```text
api
web
postgres
redis/cache    optional after needed
minio/storage  optional after upload flow begins
```

Configuration rules:

- `.env.example` lists all required variables with safe example values.
- Secrets must not be committed.
- API supports `APP_ENV=development|test|staging|production`.
- Development and test may use W3 mock and AI mock, clearly labeled by environment.
- Production uses real W3 and production-safe AI/storage configuration.

## 14. Project Status Checklist

The live checklist is [../project-status.md](../project-status.md). It tracks:

- Foundation readiness.
- Frontend readiness.
- Backend readiness.
- Database and migration readiness.
- API contract readiness.
- Testing and CI readiness.
- Container readiness.
- Business Epic progress.

Status values are `Done`, `In Progress`, `Not Started`, `Blocked`, and `Deferred`.

## 15. Future SPEC Breakdown

Future SPECs should map directly to PRD Stories and ACs.

Recommended order:

1. `001-auth-session-w3.md`
2. `002-iam-permission-model.md`
3. `004-position-department-management.md`
4. `003-resume-library-import.md`
5. `005-resume-parse-workspace.md`
6. `006-resume-recommendation-notification.md`
7. `007-user-role-management.md`
8. `008-custom-role-management.md`
9. `009-ui-design-system.md`
10. `010-observability-audit-jobs.md`

Each business SPEC must include a PRD Story/AC mapping table, API contract expectations, backend authorization requirements, frontend states, TDD test list, and checklist updates.

## 16. Decisions

- Use Foundation-First delivery before implementing business Epics.
- Use monorepo for frontend, backend, generated clients, docs, and scripts.
- Use modular monolith backend instead of microservices for the first production path.
- Use backend-code-generated OpenAPI instead of OpenAPI-first.
- Use stable error codes plus frontend i18n.
- Use query-critical columns and JSON only for extensions/display payloads.
- Treat Agent memory as foundation infrastructure.
