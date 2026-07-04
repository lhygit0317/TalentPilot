# E5 Department and Position Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build E5 department and position management from `docs/specs/004-position-department-management.md`.

**Architecture:** Add a backend `organization` domain package for departments, positions, department-position relations, validation, scope-pushed SQL queries, and audit events. Expose Huma/Echo routes as the OpenAPI source of truth, regenerate the TypeScript client, then build a frontend `DepartmentPositionPage` using generated client wrappers and project UI components.

**Tech Stack:** Go, Echo, Huma, GORM, goose, SQLite/PostgreSQL-compatible SQL, React, TypeScript, Vite, Vitest, Testing Library, pnpm.

---

## Task 0: Baseline and Branch Guard

**Files:**
- Read: `AGENTS.md`
- Read: `docs/specs/004-position-department-management.md`
- Read: `docs/project-status.md`

- [ ] **Step 1: Confirm branch and worktree**

Run:

```bash
git branch --show-current
git status --short
```

Expected: branch is `codex/e5-department-position-management`; only unrelated `.codex-work/` may be untracked.

- [ ] **Step 2: Run baseline verification**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH CI=true make ci
```

Expected: PASS before implementation.

## Task 1: IAM and Error-Code Foundation

**Files:**
- Modify: `apps/api/internal/iam/whitelist.go`
- Modify: `apps/api/internal/iam/presets.go`
- Modify: `apps/api/internal/iam/whitelist_test.go`
- Modify: `apps/api/internal/iam/presets_test.go`
- Modify: `apps/api/internal/http/apperror/error.go`
- Modify: `apps/api/internal/http/apperror/error_test.go`
- Create: `apps/api/migrations/000006_seed_department_write_permissions.sql`
- Modify: `apps/api/test/integration/migrations_test.go`

- [ ] **Step 1: Write failing tests**

Add tests asserting:

```go
func TestDepartmentWritePermissionsAreWhitelisted(t *testing.T) {
	for _, action := range []iam.Action{iam.ActionCreate, iam.ActionUpdate, iam.ActionDelete} {
		if err := iam.ValidatePermissionGrant(iam.PermissionGrant{RoleID: iam.RoleSuperAdmin, Resource: iam.ResourceDepartment, Action: action}); err != nil {
			t.Fatalf("Department.%s should be whitelisted: %v", action, err)
		}
	}
}

func TestOnlySuperAdminPresetGetsDepartmentWrites(t *testing.T) {
	grants := iam.PresetRolePermissions()
	for roleID, roleGrants := range grants {
		hasWrite := hasGrant(roleGrants, iam.ResourceDepartment, iam.ActionCreate) ||
			hasGrant(roleGrants, iam.ResourceDepartment, iam.ActionUpdate) ||
			hasGrant(roleGrants, iam.ResourceDepartment, iam.ActionDelete)
		if roleID == iam.RoleSuperAdmin && !hasWrite {
			t.Fatalf("super admin should have department writes")
		}
		if roleID != iam.RoleSuperAdmin && hasWrite {
			t.Fatalf("%s should not have department writes", roleID)
		}
	}
}

func TestOrganizationErrorCodesUseStableMessages(t *testing.T) {
	for _, code := range []apperror.Code{
		apperror.DepartmentNotFound,
		apperror.DepartmentNameRequired,
		apperror.DepartmentNameDuplicate,
		apperror.DepartmentDeleteHasRelations,
		apperror.DepartmentSystemProtected,
		apperror.PositionNotFound,
		apperror.PositionDeleteHasHistory,
	} {
		problem := apperror.NewProblem(code, "", "", nil)
		if problem.Code != string(code) || problem.Message == "" {
			t.Fatalf("missing stable problem for %s", code)
		}
	}
}
```

- [ ] **Step 2: Run red tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam ./internal/http/apperror ./test/integration -run 'DepartmentWrite|OnlySuperAdmin|OrganizationError|Migration' -count=1
```

Expected: FAIL because department write permissions and E5 error codes are missing.

- [ ] **Step 3: Implement minimal foundation**

Add `ActionCreate`, `ActionUpdate`, and `ActionDelete` under `ResourceDepartment` in `PermissionWhitelist()`.

Add E5 error codes in `apperror/error.go` with Chinese fallback messages:

```go
DepartmentNotFound             Code = "DEPARTMENT_NOT_FOUND"
DepartmentNameRequired         Code = "DEPARTMENT_NAME_REQUIRED"
DepartmentNameDuplicate        Code = "DEPARTMENT_NAME_DUPLICATE"
DepartmentDeleteHasRelations   Code = "DEPARTMENT_DELETE_HAS_RELATIONS"
DepartmentSystemProtected      Code = "DEPARTMENT_SYSTEM_PROTECTED"
PositionNotFound               Code = "POSITION_NOT_FOUND"
PositionNameRequired           Code = "POSITION_NAME_REQUIRED"
PositionDepartmentRequired     Code = "POSITION_DEPARTMENT_REQUIRED"
PositionDepartmentInvalid      Code = "POSITION_DEPARTMENT_INVALID"
PositionInvalidChannel         Code = "POSITION_INVALID_CHANNEL"
PositionInvalidStatus          Code = "POSITION_INVALID_STATUS"
PositionDuplicateKeyword       Code = "POSITION_DUPLICATE_KEYWORD"
PositionDuplicateImplicitTag   Code = "POSITION_DUPLICATE_IMPLICIT_TAG"
PositionInvalidImplicitWeight  Code = "POSITION_INVALID_IMPLICIT_WEIGHT"
PositionDeleteHasHistory       Code = "POSITION_DELETE_HAS_HISTORY"
```

Create migration `000006_seed_department_write_permissions.sql` to seed `Department.Create/Update/Delete` only for `__role_super_admin__`.

- [ ] **Step 4: Run green tests and commit**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam ./internal/http/apperror ./test/integration -count=1
git add apps/api/internal/iam apps/api/internal/http/apperror apps/api/migrations apps/api/test/integration
git commit -m "feat(api): add E5 IAM and error foundation"
```

## Task 2: Organization Department Store and Service

**Files:**
- Create: `apps/api/internal/organization/types.go`
- Create: `apps/api/internal/organization/service.go`
- Create: `apps/api/internal/organization/sql_store.go`
- Create: `apps/api/internal/organization/service_test.go`
- Create: `apps/api/internal/organization/sql_store_test.go`

- [ ] **Step 1: Write failing service and store tests**

Cover:

```go
func TestListDepartmentsAppliesScopeAndExcludesSystem(t *testing.T) {}
func TestGetDepartmentDeniesOutOfScope(t *testing.T) {}
func TestCreateDepartmentRejectsEmptyAndDuplicateName(t *testing.T) {}
func TestDeleteDepartmentRejectsRelations(t *testing.T) {}
func TestDepartmentWritesRecordAudit(t *testing.T) {}
```

Use SQLite in-memory GORM, existing migrations, and `iam.ScopePredicate` with department branches.

- [ ] **Step 2: Run red tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/organization -run Department -count=1
```

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement department types/service/store**

Define:

```go
type DepartmentListItem struct {
	ID            string
	Name          string
	PositionCount int
	ResumeCount   int
	UpdatedAt      time.Time
	CanGet         bool
	CanUpdate      bool
	CanDelete      bool
}

type DepartmentDetail struct {
	DepartmentListItem
	Positions []PositionSummary
}
```

Implement `ListDepartments`, `GetDepartment`, `CreateDepartment`, `UpdateDepartment`, and `DeleteDepartment` with scoped SQL. Delete must reject `department_positions`, `department_resumes`, and `user_department_roles` references and protect `__system__`.

- [ ] **Step 4: Run green tests and commit**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/organization -run Department -count=1
git add apps/api/internal/organization
git commit -m "feat(api): manage departments"
```

## Task 3: Organization Position Store and Service

**Files:**
- Modify: `apps/api/internal/organization/types.go`
- Modify: `apps/api/internal/organization/service.go`
- Modify: `apps/api/internal/organization/sql_store.go`
- Modify: `apps/api/internal/organization/service_test.go`
- Modify: `apps/api/internal/organization/sql_store_test.go`

- [ ] **Step 1: Write failing position tests**

Cover:

```go
func TestListPositionsAppliesDepartmentScopeAndExcludesOrphans(t *testing.T) {}
func TestCreatePositionValidatesDuplicatesAndCreatesRelation(t *testing.T) {}
func TestUpdatePositionMovesDepartmentRelation(t *testing.T) {}
func TestUpdatePositionStatusChangesOnOff(t *testing.T) {}
func TestDeletePositionRejectsPositionResumeHistory(t *testing.T) {}
func TestPositionWritesRecordAudit(t *testing.T) {}
```

- [ ] **Step 2: Run red tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/organization -run Position -count=1
```

Expected: FAIL because position service methods are missing.

- [ ] **Step 3: Implement position types/service/store**

Define position inputs and outputs matching `docs/specs/004-position-department-management.md`. Store `duties`, `must`, `keywords`, and `implicitTags` as JSON. Normalize arrays by trimming empty strings. Reject duplicate keywords and implicit tag names case-insensitively. Default missing implicit tag weight to `40`; reject weights outside `0..100`.

Implement create/update in transactions with `department_positions`. Delete must reject any `position_resumes` row.

- [ ] **Step 4: Run green tests and commit**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/organization -count=1
git add apps/api/internal/organization
git commit -m "feat(api): manage positions"
```

## Task 4: Huma Routes, OpenAPI, and Frontend Client Wrappers

**Files:**
- Create: `apps/api/internal/app/organization_routes.go`
- Create: `apps/api/internal/app/organization_routes_test.go`
- Modify: `apps/api/internal/app/server.go`
- Modify: `apps/api/cmd/api/main.go`
- Modify: `apps/web/src/api/client.ts`
- Modify: `apps/web/src/api/client.test.ts`
- Generated: `packages/api-contract/openapi.json`
- Generated: `packages/api-client/src/schema.d.ts`

- [ ] **Step 1: Write failing route and wrapper tests**

Backend tests should cover route auth and representative happy/error paths:

```go
func TestOrganizationRoutesListDepartments(t *testing.T) {}
func TestOrganizationRoutesRejectDepartmentDeleteWithRelations(t *testing.T) {}
func TestOrganizationRoutesCreatePosition(t *testing.T) {}
func TestOrganizationRoutesDenyPositionWriteWithoutPermission(t *testing.T) {}
```

Frontend wrapper tests should cover:

```ts
it("lists departments", async () => {});
it("creates and deletes departments", async () => {});
it("lists positions with filters", async () => {});
it("creates updates and deletes positions", async () => {});
```

- [ ] **Step 2: Run red tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app -run Organization -count=1
CI=true pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts
```

Expected: FAIL because routes and wrappers are missing.

- [ ] **Step 3: Implement routes and wrappers**

Register:

```text
GET /departments
GET /departments/{departmentId}
POST /departments
PATCH /departments/{departmentId}
DELETE /departments/{departmentId}
GET /positions
GET /positions/{positionId}
POST /positions
PATCH /positions/{positionId}
DELETE /positions/{positionId}
```

Wire `OrganizationService` into `app.Options` and `cmd/api/main.go`.

Add frontend wrappers in `apps/web/src/api/client.ts` using generated paths.

- [ ] **Step 4: Regenerate and verify**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-generate
make client-generate
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app ./internal/organization -count=1
CI=true pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts
make openapi-check
make client-check
git add apps/api apps/web/src/api packages/api-contract/openapi.json packages/api-client/src/schema.d.ts
git commit -m "feat(api): expose department position endpoints"
```

## Task 5: Frontend Read-Only Department and Position Page

**Files:**
- Create: `apps/web/src/department-position/DepartmentPositionPage.tsx`
- Create: `apps/web/src/department-position/DepartmentPositionPage.test.tsx`
- Create: `apps/web/src/department-position/types.ts`
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing frontend tests**

Cover:

```ts
it("renders department-position page from IAM route", async () => {});
it("shows department columns and read-only view action", async () => {});
it("opens department detail without staff lists", async () => {});
it("shows position filters, list, and JD detail", async () => {});
it("hides write actions for non-super-admin users", async () => {});
```

- [ ] **Step 2: Run red tests**

Run:

```bash
CI=true pnpm --filter @talentpilot/web test -- --run src/app/App.test.tsx src/department-position/DepartmentPositionPage.test.tsx
```

Expected: FAIL because page does not exist and route renders only a heading.

- [ ] **Step 3: Implement read-only page**

Render tabs, department table/detail, position filters/list/detail, and empty states. Use existing `Button`, `Input`, `Select`, `Field`, and semantic table markup. Keep all data from API wrappers.

- [ ] **Step 4: Run green tests and commit**

Run:

```bash
CI=true pnpm --filter @talentpilot/web test -- --run src/app/App.test.tsx src/department-position/DepartmentPositionPage.test.tsx
make typecheck
git add apps/web/src/app apps/web/src/department-position apps/web/src/i18n
git commit -m "feat(web): render department position management"
```

## Task 6: Frontend Super-Admin Write Workflows

**Files:**
- Modify: `apps/web/src/department-position/DepartmentPositionPage.tsx`
- Modify: `apps/web/src/department-position/DepartmentPositionPage.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing workflow tests**

Cover:

```ts
it("creates and updates a department as super admin", async () => {});
it("shows translated department delete protection errors", async () => {});
it("creates and updates a position with department relation", async () => {});
it("rejects duplicate keywords and implicit tags before submit", async () => {});
it("toggles position on and off", async () => {});
it("deletes a safe position and refreshes the list", async () => {});
```

- [ ] **Step 2: Run red tests**

Run:

```bash
CI=true pnpm --filter @talentpilot/web test -- --run src/department-position/DepartmentPositionPage.test.tsx
```

Expected: FAIL because write workflows are missing.

- [ ] **Step 3: Implement write workflows**

Use controlled forms. Use generated client wrappers only. Hide write actions unless `session.permissions` includes the required permissions. Refresh relevant lists after successful mutations and show SPEC toast text.

- [ ] **Step 4: Run green tests and commit**

Run:

```bash
CI=true pnpm --filter @talentpilot/web test -- --run src/department-position/DepartmentPositionPage.test.tsx
make typecheck
git add apps/web/src/department-position apps/web/src/i18n
git commit -m "feat(web): support department position management workflows"
```

## Task 7: Final Verification and Documentation Status

**Files:**
- Modify: `AGENTS.md`
- Modify: `docs/project-status.md`

- [ ] **Step 1: Run full verification**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH CI=true make ci
git diff --check
```

Expected: PASS.

- [ ] **Step 2: Update status**

Set E5 to `Done` only after verification passes. Evidence must mention `make ci`, OpenAPI/client checks, and that `make test-e2e` remains reserved until Playwright is installed.

- [ ] **Step 3: Commit docs**

Run:

```bash
git add AGENTS.md docs/project-status.md
git commit -m "docs: mark E5 implementation complete"
```

## Scope Guardrails

- Do not implement E2 resume parsing workspace behavior.
- Do not implement E3 recommendation or notification behavior.
- Do not implement E6 user/role assignment.
- Do not display HRBP/manager/trainee staff lists in E5 pages.
- Do not filter permissions only in frontend state.
- Do not expose GORM models directly in DTOs.
- Do not rely on GORM AutoMigrate for schema or seed changes.
