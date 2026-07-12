# 010 Quality Hardening SPEC

## 1. Scope and Goals

This SPEC defines the post-E8 quality hardening phase. It covers two engineering controls that were intentionally deferred while product epics E1, IAM, and E2 through E8 were implemented:

- automated frontend enforcement for the project rule that business pages must not use raw interactive HTML elements directly;
- Playwright end-to-end smoke coverage for the highest-risk integrated business workflows.

The goal is to reduce regression risk before the next product phase. This SPEC does not add new business functionality.

## 2. Dependencies and Readiness

Required before this hardening phase:

- implemented frontend business pages for E2, E3, E4, E5, E6, E7, and E8;
- existing Vitest coverage under `apps/web/src`;
- existing `make lint`, `make test-web`, `make test-e2e`, and `make ci` command surfaces;
- backend API behavior and generated client stability from the completed product epics;
- current frontend component convention documented in [000 Foundation Architecture SPEC](000-foundation-architecture.md) and [../../AGENTS.md](../../AGENTS.md).

The current `make test-e2e` target is reserved but not yet backed by installed Playwright dependencies or tests.

## 3. Non-Goals

- New recruiting workflows or API endpoints.
- Visual redesign of existing pages.
- Rewriting existing frontend pages only to satisfy style preferences.
- Full browser matrix coverage in the first pass.
- Requiring Playwright to replace Vitest or backend tests.
- Real W3 identity-provider automation. E2E tests may use existing local/test auth hooks or seeded backend state.
- Large refactors of generated API client code.

## 4. Design Approach

Three implementation scopes were considered:

1. Minimal lint/check hardening plus a small Playwright smoke suite.
2. Playwright-only coverage without component-rule enforcement.
3. Broad UI refactor and full E2E suite across every acceptance criterion.

This SPEC chooses option 1. It closes the two documented quality gaps while keeping the work small, testable, and suitable for TDD. Option 2 misses the explicit frontend architecture rule. Option 3 has too much blast radius for a hardening phase after a large product sequence.

## 5. Workstream A: Frontend Component Rule Enforcement

Business pages must not directly render raw interactive HTML elements such as:

- `button`
- `input`
- `select`
- `textarea`
- `dialog`
- `form`
- `table`

Allowed exceptions:

- project UI primitives and wrappers under `apps/web/src/components/ui`;
- test files;
- setup files;
- non-business infrastructure files where a raw element is explicitly part of the wrapper implementation.

The implementation may use either ESLint configuration or a small repository check script, but it must be run by `make lint` or the existing pnpm lint command so CI catches violations.

Acceptance criteria:

- A failing test or check fixture proves that a business page using a raw interactive element is rejected.
- Existing business pages pass after any required narrow wrapper migrations.
- The failure message identifies the file and disallowed element.
- The rule allows UI wrapper components to render the underlying HTML element.

## 6. Workstream B: Playwright E2E Smoke Coverage

Install and configure Playwright for `apps/web` so `make test-e2e` becomes a meaningful local and CI-ready command.

Initial smoke flows:

| Flow | Coverage |
| --- | --- |
| Authenticated shell | User can enter the app shell with seeded/test auth and see permission-scoped navigation. |
| Resume parse | User selects or imports a resume, selects an on-shelf position, runs parse, and sees match output. |
| Recommendation and notification | User routes a resume, sends a recommendation, and the recipient-facing notification surface updates. |
| Notification read state | User marks one notification read and can mark all read. |
| User and role management | Authorized user can view users, assign/remove a role binding, and inspect roles. |
| Custom role management | Authorized user can create or edit a custom role definition and see validation for invalid role relations. |

The first implementation may seed deterministic local data through test fixtures or backend test setup. It should not rely on external services.

Acceptance criteria:

- `make test-e2e` runs Playwright tests instead of failing because no test runner exists.
- E2E tests run against local services started by Playwright config or documented pretest setup.
- Tests use stable selectors or accessible roles, not fragile CSS implementation details.
- CI can install the required browser dependencies or the SPEC explicitly documents the local-only limitation before CI enablement.
- `docs/project-status.md` records whether E2E is part of the passing gate or still a separate hardening command.

## 7. Testing and TDD Rules

This phase follows the repository TDD rule:

1. Write the failing enforcement test or Playwright smoke test first.
2. Run the targeted command and verify the expected failure.
3. Add the smallest implementation or configuration needed to pass.
4. Run the target command and relevant surrounding checks.
5. Update generated or documentation artifacts only when the implementation requires it.

Configuration-only setup may use verification checks instead of behavior-first tests, but the implementation notes must state that exception explicitly.

## 8. Status Updates

Progress must stay visible in [../project-status.md](../project-status.md):

- mark component-rule enforcement `In Progress` when the first failing check is added;
- mark Playwright setup `In Progress` when dependencies/configuration are added;
- mark items `Done` only after the command evidence is recorded;
- update [../../AGENTS.md](../../AGENTS.md) if command behavior or required agent workflow changes.

## 9. Completion Criteria

The hardening phase is complete when:

- `make lint` or an equivalent CI lint path enforces the raw interactive element rule for business pages;
- `make test-e2e` runs at least the initial Playwright smoke suite;
- `make ci` behavior is clearly documented, including whether E2E is included or remains a separate command;
- `docs/project-status.md` contains current evidence for both hardening items;
- no project-wide consensus has changed without a matching `AGENTS.md` update.
