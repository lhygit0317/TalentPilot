# E8 Custom Role Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build E8 custom role management with an aggregated backend RoleAdmin API and a frontend role-management page using the complete IAM permission whitelist matrix.

**Architecture:** Add a focused `apps/api/internal/roleadmin` package that owns role-definition read models and atomic role mutations. Expose Huma routes under `/roles`, regenerate OpenAPI/client artifacts, add frontend wrappers, and render `apps/web/src/roles/RoleManagementPage.tsx` behind the existing `roles` page access.

**Tech Stack:** Go, Echo, Huma, GORM, SQLite/PostgreSQL-compatible SQL, React, TypeScript, Vite, Testing Library, lucide-react, shadcn/project UI wrappers, generated OpenAPI client.

---

## File Structure

- Create: `apps/api/internal/roleadmin/types.go` for DTOs, inputs, and domain errors.
- Create: `apps/api/internal/roleadmin/service.go` for validation, mutation orchestration, audit, and IAM cache invalidation calls.
- Create: `apps/api/internal/roleadmin/sql_store.go` for role list/detail/options and transactional writes.
- Create: `apps/api/internal/roleadmin/service_test.go`.
- Create: `apps/api/internal/roleadmin/sql_store_test.go`.
- Create: `apps/api/internal/app/roleadmin_routes.go`.
- Create: `apps/api/internal/app/roleadmin_routes_test.go`.
- Modify: `apps/api/internal/app/server.go` to add `RoleAdminService` and register routes.
- Modify: `apps/api/cmd/api/main.go` to wire the RoleAdmin SQL store and service.
- Modify: `apps/api/internal/http/apperror/error.go` for E8 stable error codes.
- Modify after generation: `packages/api-contract/openapi.json`.
- Modify after generation: `packages/api-client/src/schema.d.ts`.
- Modify: `apps/web/src/api/client.ts` and `apps/web/src/api/client.test.ts` for role-management wrappers.
- Create: `apps/web/src/roles/types.ts`.
- Create: `apps/web/src/roles/RoleManagementPage.tsx`.
- Create: `apps/web/src/roles/RoleManagementPage.test.tsx`.
- Modify: `apps/web/src/app/App.tsx` and `apps/web/src/app/App.test.tsx` to route `roles` to the new page.
- Modify: `apps/web/src/i18n/zh-CN.ts` and `apps/web/src/i18n/en-US.ts` for role-management copy.
- Modify: `AGENTS.md` and `docs/project-status.md` after verification.

Generated OpenAPI and TypeScript schema changes are generated-artifact steps. They use generation and drift checks instead of behavior-first tests.

## Task 1: RoleAdmin Read Models and Permission Options

**Files:**
- Create: `apps/api/internal/roleadmin/types.go`
- Create: `apps/api/internal/roleadmin/sql_store.go`
- Create: `apps/api/internal/roleadmin/sql_store_test.go`

- [ ] **Step 1: Write failing SQL store tests**

Create tests:

```go
func TestSQLStoreListRolesReturnsCountsAndCapabilities(t *testing.T)
func TestSQLStoreGetRoleReturnsDirectDefinition(t *testing.T)
func TestSQLStorePermissionOptionsComeFromIAMWhitelist(t *testing.T)
```

The first test should seed one custom role with two direct permissions, one direct child role, and zero user references, then assert:

```go
if item.PermissionCount != 2 || item.ChildRoleCount != 1 || item.ReferenceCount != 0 {
	t.Fatalf("unexpected counts: %#v", item)
}
if !item.CanEdit || !item.CanDelete || !item.CanToggleEnabled {
	t.Fatalf("expected editable unused custom role: %#v", item)
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/roleadmin -run 'TestSQLStore' -count=1
```

Expected: FAIL because `apps/api/internal/roleadmin` does not exist.

- [ ] **Step 3: Implement read DTOs and SQL read methods**

Create `types.go` with these exported contracts:

```go
type RoleListQuery struct {
	ActorCanCreate bool
	ActorCanEdit bool
	ActorCanDelete bool
	ActorCanToggle bool
	Search string
	System *bool
	Enabled *bool
	Limit int
}

type RoleListItem struct {
	ID string `json:"id"`
	Label string `json:"label"`
	Description string `json:"description"`
	IsSystem bool `json:"isSystem"`
	Enabled bool `json:"enabled"`
	PermissionCount int `json:"permissionCount"`
	ChildRoleCount int `json:"childRoleCount"`
	ReferenceCount int `json:"referenceCount"`
	ConditionSummary string `json:"conditionSummary"`
	CanEdit bool `json:"canEdit"`
	CanDelete bool `json:"canDelete"`
	CanToggleEnabled bool `json:"canToggleEnabled"`
}

type RoleListResult struct {
	Items []RoleListItem `json:"items" nullable:"false"`
	Total int `json:"total"`
	CanCreate bool `json:"canCreate"`
}
```

Implement `SQLStore.ListRoles`, `SQLStore.GetRole`, and `SQLStore.PermissionOptions`. Use aggregate subqueries for counts, order by `is_system DESC, label ASC`, and cap list limit to 200.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/roleadmin -run 'TestSQLStore' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/roleadmin
git commit -m "feat(api): add role admin read models"
```

## Task 2: RoleAdmin Mutations, Validation, Audit, and Cache Invalidation

**Files:**
- Modify: `apps/api/internal/roleadmin/types.go`
- Create: `apps/api/internal/roleadmin/service.go`
- Modify: `apps/api/internal/roleadmin/sql_store.go`
- Create: `apps/api/internal/roleadmin/service_test.go`
- Modify: `apps/api/internal/http/apperror/error.go`

- [ ] **Step 1: Write failing service tests**

Create tests:

```go
func TestServiceCreateRoleRejectsDuplicatePermission(t *testing.T)
func TestServiceCreateRoleRejectsInvalidPermissionCondition(t *testing.T)
func TestServiceUpdateSystemRoleRejectsLabelChange(t *testing.T)
func TestServiceUpdateRoleRejectsRelationCycle(t *testing.T)
func TestServiceDeleteRoleRejectsSystemRole(t *testing.T)
func TestServiceDeleteRoleRejectsReferencedCustomRole(t *testing.T)
func TestServiceToggleEnabledInvalidatesRoleClosure(t *testing.T)
```

Use a fake store that records `updatedRoleID`, `replacedPermissions`, `replacedChildRoleIDs`, and `invalidatedRoleIDs`. Assert cycle errors use `iam.ErrRoleRelationCycle`.

- [ ] **Step 2: Run service tests and verify RED**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/roleadmin -run 'TestService' -count=1
```

Expected: FAIL because mutation service methods are missing.

- [ ] **Step 3: Implement mutation inputs and errors**

Add domain errors:

```go
var (
	ErrRoleNotFound = errors.New("role not found")
	ErrLabelInvalid = errors.New("role label invalid")
	ErrLabelDuplicate = errors.New("role label duplicate")
	ErrSystemRoleProtected = errors.New("system role protected")
	ErrRoleInUse = errors.New("role in use")
	ErrPermissionInvalid = errors.New("role permission invalid")
	ErrPermissionDuplicate = errors.New("role permission duplicate")
	ErrRelationInvalid = errors.New("role relation invalid")
)
```

Add mutation DTOs:

```go
type PermissionInput struct {
	Resource iam.Resource `json:"resource"`
	Action iam.Action `json:"action"`
	AttributeConditions iam.AttributeConditions `json:"attributeConditions,omitempty"`
}

type RoleDefinitionInput struct {
	ActorUserID string
	Label string `json:"label"`
	Description string `json:"description"`
	Enabled bool `json:"enabled"`
	Permissions []PermissionInput `json:"permissions" nullable:"false"`
	ChildRoleIDs []string `json:"childRoleIds" nullable:"false"`
}

type ToggleEnabledInput struct {
	ActorUserID string
	Enabled bool `json:"enabled"`
}
```

- [ ] **Step 4: Implement service validation and store writes**

Implement `CreateRole`, `UpdateRole`, `ToggleEnabled`, and `DeleteRole`:

- trim `label` and require 2-20 runes for custom roles;
- trim `description` and require at most 200 runes;
- call `iam.ValidatePermissionGrant` for every direct permission;
- reject duplicate canonical permission keys;
- reject missing child roles and self-relations;
- validate the full relation graph before writing;
- use one store transaction for role metadata, permission replacement, and child-role replacement;
- call `iam.InvalidateRoleClosure(ctx, []string{roleID})` after success;
- record audit events through the existing audit recorder.

In `error.go`, add the E8 problem codes and Chinese messages from the SPEC.

- [ ] **Step 5: Run service tests and surrounding IAM tests**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/roleadmin ./internal/iam -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/roleadmin apps/api/internal/http/apperror/error.go
git commit -m "feat(api): add role admin mutations"
```

## Task 3: RoleAdmin Huma Routes and App Wiring

**Files:**
- Create: `apps/api/internal/app/roleadmin_routes.go`
- Create: `apps/api/internal/app/roleadmin_routes_test.go`
- Modify: `apps/api/internal/app/server.go`
- Modify: `apps/api/cmd/api/main.go`

- [ ] **Step 1: Write failing route tests**

Create tests:

```go
func TestRoleAdminRoutesListRequiresRoleList(t *testing.T)
func TestRoleAdminRoutesPermissionOptionsRequiresPermissionList(t *testing.T)
func TestRoleAdminRoutesCreateRequiresAllMutationPermissions(t *testing.T)
func TestRoleAdminRoutesUpdateMapsDuplicateLabel(t *testing.T)
func TestRoleAdminRoutesDeleteMapsRoleInUse(t *testing.T)
```

Use existing app route-test fakes. For create, deny `Permission.Create` while allowing `Role.Create` and assert HTTP 403.

- [ ] **Step 2: Run route tests and verify RED**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app -run 'TestRoleAdminRoutes' -count=1
```

Expected: FAIL because routes are not registered.

- [ ] **Step 3: Implement routes**

Register:

```text
GET    /roles
GET    /roles/{roleId}
GET    /roles/permission-options
POST   /roles
PATCH  /roles/{roleId}
PATCH  /roles/{roleId}/enabled
DELETE /roles/{roleId}
```

Add `RoleAdminService` to `app.Options` with methods matching the roleadmin service. In route handlers, use `authorizeRequest` and `scopeForPrincipal` only for permission checks; do not put business validation in handlers.

- [ ] **Step 4: Wire main**

In `apps/api/cmd/api/main.go`, create:

```go
roleAdminStore := roleadmin.NewSQLStore(database)
roleAdminService := roleadmin.NewService(roleAdminStore, iamService, auditRecorder)
```

Pass `RoleAdminService: roleAdminService` to `app.Options`.

- [ ] **Step 5: Run route and API tests**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app ./internal/roleadmin -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/app apps/api/cmd/api/main.go
git commit -m "feat(api): expose role admin routes"
```

## Task 4: OpenAPI, API Client, and Frontend Wrappers

**Files:**
- Modify generated: `packages/api-contract/openapi.json`
- Modify generated: `packages/api-client/src/schema.d.ts`
- Modify: `apps/web/src/api/client.ts`
- Modify: `apps/web/src/api/client.test.ts`

- [ ] **Step 1: Generate OpenAPI and client**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-generate
make client-generate
```

Expected: `openapi.json` and `schema.d.ts` include `/roles` operations.

- [ ] **Step 2: Write frontend client wrapper tests**

In `client.test.ts`, assert wrappers call:

```ts
listRoles({ search: "HR", system: true, enabled: true });
getRole("role_1");
getRolePermissionOptions();
createRoleDefinition(payload);
updateRoleDefinition("role_1", payload);
toggleRoleEnabled("role_1", false);
deleteRoleDefinition("role_1");
```

- [ ] **Step 3: Run wrapper tests and verify RED**

Run:

```bash
CI=true pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts
```

Expected: FAIL because wrappers are missing.

- [ ] **Step 4: Add wrappers**

Add wrappers to `apps/web/src/api/client.ts`:

```ts
export function listRoles(query: { search?: string; system?: boolean; enabled?: boolean; limit?: number } = {}) {
  return apiClient.GET("/roles", { params: { query } });
}

export function getRole(roleId: string) {
  return apiClient.GET("/roles/{roleId}", { params: { path: { roleId } } });
}

export function getRolePermissionOptions() {
  return apiClient.GET("/roles/permission-options");
}
```

Also add `createRoleDefinition`, `updateRoleDefinition`, `toggleRoleEnabled`, and `deleteRoleDefinition` with generated path types.

- [ ] **Step 5: Verify generated artifacts are stable**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-check
make client-check
CI=true pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/api-contract/openapi.json packages/api-client/src/schema.d.ts apps/web/src/api/client.ts apps/web/src/api/client.test.ts
git commit -m "feat(web): add role admin api client"
```

## Task 5: Role Management Frontend Page

**Files:**
- Create: `apps/web/src/roles/types.ts`
- Create: `apps/web/src/roles/RoleManagementPage.tsx`
- Create: `apps/web/src/roles/RoleManagementPage.test.tsx`
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing page tests**

Create tests:

```ts
it("renders role rows with status, counts, and delete disabled reason", async () => {});
it("builds permission matrix payload when creating a role", async () => {});
it("shows confirmation before disabling a system role", async () => {});
it("renders localized backend errors", async () => {});
```

In `App.test.tsx`, add a session with `pageAccess: ["roles"]` and assert the `roles` route renders the page.

- [ ] **Step 2: Run frontend tests and verify RED**

Run:

```bash
CI=true pnpm --filter @talentpilot/web test -- --run src/roles/RoleManagementPage.test.tsx src/app/App.test.tsx
```

Expected: FAIL because the page does not exist or is not routed.

- [ ] **Step 3: Implement types and page shell**

Create `types.ts` for frontend view models:

```ts
export type RolePermissionInput = {
  resource: string;
  action: string;
  attributeConditions?: {
    chan?: Array<"social" | "campus">;
    expired?: boolean[];
    self?: boolean;
  };
};

export type RoleDefinitionPayload = {
  label: string;
  description: string;
  enabled: boolean;
  permissions: RolePermissionInput[];
  childRoleIds: string[];
};
```

Implement `RoleManagementPage` with list loading, filters, table actions, and editor dialog state. Use project UI wrappers for buttons, inputs, fields, and forms.

- [ ] **Step 4: Implement permission matrix behavior**

Render permission options grouped by resource. Each whitelist action has a checkbox. Resume permissions with condition support render channel and expired controls. Convert checked rows to `RoleDefinitionPayload.permissions` on save.

- [ ] **Step 5: Route the page**

In `App.tsx`, import the page and render it for `selectedPage === "roles"`. Keep the existing page-access filtering so users without `roles` access cannot navigate there.

- [ ] **Step 6: Run frontend tests**

Run:

```bash
CI=true pnpm --filter @talentpilot/web test -- --run src/roles/RoleManagementPage.test.tsx src/app/App.test.tsx
make typecheck
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/roles apps/web/src/app/App.tsx apps/web/src/app/App.test.tsx apps/web/src/i18n/zh-CN.ts apps/web/src/i18n/en-US.ts
git commit -m "feat(web): add role management page"
```

## Task 6: Final Verification and Documentation

**Files:**
- Modify: `AGENTS.md`
- Modify: `docs/project-status.md`

- [ ] **Step 1: Update documentation state**

Update:

- `AGENTS.md` current phase to E8 implementation or post-E8 planning, depending on implementation status.
- `AGENTS.md` documentation index to include `docs/specs/008-custom-role-management.md` and this implementation plan.
- `docs/project-status.md` E8 row evidence and status.

- [ ] **Step 2: Run full verification**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH CI=true make ci
git diff --check
```

Expected: PASS for both commands.

- [ ] **Step 3: Inspect final status**

Run:

```bash
git status --short
git log --oneline -5
```

Expected: only intended files are changed before the final commit, then clean tracked state after committing.

- [ ] **Step 4: Commit documentation**

```bash
git add AGENTS.md docs/project-status.md
git commit -m "docs: mark E8 custom role management implemented"
```

## Execution Notes

- Keep implementation batches under roughly 1000 changed lines per patch.
- Do not edit generated OpenAPI or client files manually.
- Do not weaken IAM authorization in route handlers to satisfy frontend tests.
- Preserve E6 behavior: `GET /roles/assignable` continues to return only `enabled=true` roles.
- If a role mutation changes permissions or role relations, invalidate users bound to ancestor roles, not just users directly bound to the edited role.
- If PostgreSQL-specific SQL is tempting, rewrite it using SQLite-compatible constructs because local automated tests run on SQLite by default.
