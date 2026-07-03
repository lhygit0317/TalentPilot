# IAM Permission Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the IAM runtime foundation from `docs/specs/002-iam-permission-model.md`: preset role seeds, permission expansion, data scopes, route guard support, `/me` IAM fields, audit hooks, and frontend shell consumption.

**Architecture:** Add a focused `apps/api/internal/iam` package that owns resource/action enums, whitelist validation, preset role definitions, role expansion, principal resolution, decisions, and structured scope predicates. Keep persistence behind an IAM SQL store and expose only small app/auth integration points. Do not implement E6/E8 management pages or E2/E4/E5 business workflows in this plan.

**Tech Stack:** Go 1.26, Echo, Huma, GORM, goose, SQLite/PostgreSQL-compatible SQL, Vitest/Testing Library, generated OpenAPI/client flow.

---

## File Structure

- Create `apps/api/internal/iam/types.go`: resource/action enums, errors, principal and decision DTOs.
- Create `apps/api/internal/iam/whitelist.go`: permission whitelist and attribute-condition validation.
- Create `apps/api/internal/iam/presets.go`: preset roles, direct permission matrix, role relations, page-access mapping, global-scope role set.
- Create `apps/api/internal/iam/resolve.go`: pure expansion, scope merge, cycle/depth guard, page-access calculation.
- Create `apps/api/internal/iam/service.go`: service facade, store interface, cache hooks, mutation methods.
- Create `apps/api/internal/iam/sql_store.go`: GORM-backed IAM snapshot and mutation persistence.
- Create `apps/api/internal/iam/*_test.go`: pure and SQL tests.
- Create `apps/api/migrations/000003_seed_preset_iam_roles.sql`: idempotent preset IAM seed.
- Modify `apps/api/test/integration/migrations_test.go`: seed assertions and rollback assertions.
- Modify `apps/api/internal/http/apperror/error.go`: add missing IAM stable errors from SPEC 002.
- Modify `apps/api/internal/auth/types.go`, `service.go`, and `sql_store.go`: enrich current-user result via IAM role summary.
- Modify `apps/api/internal/app/server.go` and `auth_routes.go`: wire IAM service, route guard helper, `/me` response schema.
- Modify `apps/api/cmd/api/main.go`: create IAM SQL store/service and pass it into app options.
- Modify `apps/web/src/app/App.tsx`, `App.test.tsx`, `i18n/zh-CN.ts`, `i18n/en-US.ts`: consume `permissions` and `dataScope`.
- Modify generated files through commands only: `packages/api-contract/openapi.json`, `packages/api-client/src/schema.d.ts`.
- Modify `docs/project-status.md` and `AGENTS.md`: mark IAM implementation status after completion.

## Task 1: IAM Enums, Whitelist, and Preset Matrix

**Files:**
- Create: `apps/api/internal/iam/types.go`
- Create: `apps/api/internal/iam/whitelist.go`
- Create: `apps/api/internal/iam/presets.go`
- Test: `apps/api/internal/iam/whitelist_test.go`
- Test: `apps/api/internal/iam/presets_test.go`

- [ ] **Step 1: Write failing whitelist and preset tests**

Create tests with these names and assertions:

```go
func TestPermissionWhitelistRejectsUnknownResourceAction(t *testing.T) {
	if err := iam.ValidatePermissionGrant(iam.PermissionGrant{Resource: "Unknown", Action: iam.ActionList}); !errors.Is(err, iam.ErrInvalidResource) {
		t.Fatalf("expected ErrInvalidResource, got %v", err)
	}
	if err := iam.ValidatePermissionGrant(iam.PermissionGrant{Resource: iam.ResourceResume, Action: "Export"}); !errors.Is(err, iam.ErrInvalidAction) {
		t.Fatalf("expected ErrInvalidAction, got %v", err)
	}
}

func TestAttributeConditionValidation(t *testing.T) {
	valid := iam.PermissionGrant{Resource: iam.ResourceResume, Action: iam.ActionList, AttributeConditions: iam.AttributeConditions{Channels: []string{"social"}}}
	if err := iam.ValidatePermissionGrant(valid); err != nil {
		t.Fatalf("valid resume channel condition: %v", err)
	}
	invalid := iam.PermissionGrant{Resource: iam.ResourceDepartment, Action: iam.ActionList, AttributeConditions: iam.AttributeConditions{Channels: []string{"social"}}}
	if err := iam.ValidatePermissionGrant(invalid); !errors.Is(err, iam.ErrInvalidAttributeCondition) {
		t.Fatalf("expected invalid attribute condition, got %v", err)
	}
}

func TestPresetRoleMatrixMatchesSpec(t *testing.T) {
	matrix := iam.PresetRolePermissions()
	assertGrant(t, matrix, iam.RoleGuest, iam.ResourceUser, iam.ActionGet, iam.AttributeConditions{Self: true})
	assertGrant(t, matrix, iam.RoleHRBP, iam.ResourceResume, iam.ActionDelete, iam.AttributeConditions{})
	assertGrant(t, matrix, iam.RoleSocialOwner, iam.ResourceResume, iam.ActionList, iam.AttributeConditions{Channels: []string{"social"}})
	assertGrant(t, matrix, iam.RoleCampusOwner, iam.ResourceResume, iam.ActionList, iam.AttributeConditions{Channels: []string{"campus"}})
	assertGrant(t, matrix, iam.RoleSuperAdmin, iam.ResourcePosition, iam.ActionDelete, iam.AttributeConditions{})
}

func TestGlobalScopeRoleSet(t *testing.T) {
	if iam.RoleSupportsGlobalScope(iam.RoleHRBP) {
		t.Fatalf("HRBP must not support __system__ all-department scope")
	}
	for _, roleID := range []string{iam.RoleHRD, iam.RoleSocialOwner, iam.RoleCampusOwner, iam.RoleSuperAdmin} {
		if !iam.RoleSupportsGlobalScope(roleID) {
			t.Fatalf("expected %s to support global scope", roleID)
		}
	}
}
```

- [ ] **Step 2: Run the focused tests and verify red**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam -run 'TestPermissionWhitelist|TestAttributeCondition|TestPreset|TestGlobal' -count=1
```

Expected: FAIL because `apps/api/internal/iam` does not exist.

- [ ] **Step 3: Implement enum, whitelist, and preset definitions**

Implement these public names exactly so later tasks can depend on them:

```go
type Resource string
type Action string
type AttributeConditions struct {
	Channels []string
	Expired  []bool
	Self     bool
}
type PermissionGrant struct {
	RoleID              string
	Resource            Resource
	Action              Action
	AttributeConditions AttributeConditions
}

const (
	ResourceUser Resource = "User"
	ResourceDepartment Resource = "Department"
	ResourcePosition Resource = "Position"
	ResourceResume Resource = "Resume"
	ResourceRole Resource = "Role"
	ResourcePermission Resource = "Permission"
	ResourceUserDepartmentRole Resource = "UserDepartmentRole"
	ResourceDepartmentPosition Resource = "DepartmentPosition"
	ResourceDepartmentResume Resource = "DepartmentResume"
	ResourcePositionResume Resource = "PositionResume"
	ResourceRoleRelation Resource = "RoleRelation"
	ResourceNotification Resource = "Notification"
	ResourceAuditLog Resource = "AuditLog"
	ResourceJob Resource = "Job"

	ActionList Action = "List"
	ActionGet Action = "Get"
	ActionCreate Action = "Create"
	ActionUpdate Action = "Update"
	ActionDelete Action = "Delete"

	RoleGuest = "__role_guest__"
	RoleHRBP = "__role_hrbp__"
	RoleHRD = "__role_hrd__"
	RoleManager = "__role_manager__"
	RoleTrainee = "__role_trainee__"
	RoleSocialOwner = "__role_social_owner__"
	RoleCampusOwner = "__role_campus_owner__"
	RoleSuperAdmin = "__role_super_admin__"
	SystemDepartmentID = "__system__"
)
```

Functions required in this task:

```go
func PermissionKey(resource Resource, action Action) string
func ValidatePermissionGrant(grant PermissionGrant) error
func PermissionWhitelist() map[Resource]map[Action]ConditionAllowance
func PresetRoles() []PresetRole
func PresetRolePermissions() map[string][]PermissionGrant
func PresetRoleRelations() []RoleRelation
func RoleSupportsGlobalScope(roleID string) bool
```

- [ ] **Step 4: Run tests and verify green**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam -run 'TestPermissionWhitelist|TestAttributeCondition|TestPreset|TestGlobal' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/iam
git commit -m "feat(api): add IAM permission definitions"
```

## Task 2: Preset IAM Goose Seed

**Files:**
- Create: `apps/api/migrations/000003_seed_preset_iam_roles.sql`
- Modify: `apps/api/test/integration/migrations_test.go`

- [ ] **Step 1: Write failing migration assertions**

Add helper `assertPresetIAMSeeded` and call it from `TestFoundationMigrationsCreateExpectedSchema` after `assertGuestRoleSeeded`. Assert:

```go
assertCount(t, database, "roles", "id IN ('__role_hrbp__','__role_hrd__','__role_manager__','__role_trainee__','__role_social_owner__','__role_campus_owner__','__role_super_admin__')", 7)
assertCount(t, database, "role_relations", "parent_role_id = '__role_hrd__'", 3)
assertCount(t, database, "role_relations", "parent_role_id = '__role_super_admin__'", 3)
assertCount(t, database, "permissions", "role_id = '__role_social_owner__' AND resource = 'Resume' AND action = 'List' AND attribute_conditions = '{\"chan\":[\"social\"]}'", 1)
assertCount(t, database, "permissions", "role_id = '__role_campus_owner__' AND resource = 'Resume' AND action = 'List' AND attribute_conditions = '{\"chan\":[\"campus\"]}'", 1)
assertCount(t, database, "permissions", "role_id = '__role_super_admin__' AND resource = 'Position' AND action = 'Delete'", 1)
```

Add `TestIAMSeedMigrationDownRemovesPresetSeed` that runs all migrations, rolls down to version 2, and asserts non-guest preset roles, permissions, and relations are gone while `__role_guest__` remains.

- [ ] **Step 2: Run migration tests and verify red**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./test/integration -run 'TestFoundationMigrationsCreateExpectedSchema|TestIAMSeedMigrationDownRemovesPresetSeed' -count=1
```

Expected: FAIL because migration `000003` does not exist.

- [ ] **Step 3: Add idempotent seed migration**

Create `000003_seed_preset_iam_roles.sql` with:

- `INSERT INTO roles ... ON CONFLICT(id) DO UPDATE` for preset roles except guest.
- `INSERT INTO permissions ... ON CONFLICT(role_id, resource, action, attribute_conditions) DO NOTHING`.
- `INSERT INTO role_relations ... ON CONFLICT(parent_role_id, child_role_id) DO NOTHING`.
- `Down` deletes role relations first, then permissions for non-guest preset roles, then non-guest preset roles.

Use deterministic IDs such as:

```text
__permission_<role_slug>_<resource_slug>_<action_slug>_<condition_slug>__
__role_relation_hrd_hrbp__
```

Use compact JSON strings exactly as tested, for example `{"chan":["social"]}`.

- [ ] **Step 4: Run migration tests and verify green**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./test/integration -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/migrations/000003_seed_preset_iam_roles.sql apps/api/test/integration/migrations_test.go
git commit -m "feat(api): seed preset IAM roles"
```

## Task 3: Pure IAM Principal Resolution

**Files:**
- Create: `apps/api/internal/iam/resolve.go`
- Test: `apps/api/internal/iam/resolve_test.go`

- [ ] **Step 1: Write failing pure resolver tests**

Create table-driven tests for:

```go
func TestResolvePrincipalExpandsRecursiveRoleRelations(t *testing.T)
func TestResolvePrincipalRejectsRoleRelationCycle(t *testing.T)
func TestResolvePrincipalRejectsDepthExceeded(t *testing.T)
func TestResolvePrincipalSkipsDisabledChildRoleReachedThroughRelation(t *testing.T)
func TestResolvePrincipalMergesDepartmentAndChannelScopes(t *testing.T)
func TestResolvePrincipalRejectsUnsupportedSystemScope(t *testing.T)
func TestPageAccessFromEffectivePermissions(t *testing.T)
```

Use an in-memory snapshot:

```go
snapshot := iam.Snapshot{
	User: iam.User{ID: "u1", EmployeeID: "E001", Name: "张三"},
	Departments: []iam.Department{{ID: "dept_a", Name: "算力训练平台部"}, {ID: "dept_b", Name: "智算调度部"}},
	RoleBindings: []iam.RoleBinding{{ID: "bind_1", UserID: "u1", DepartmentID: "dept_a", RoleID: iam.RoleHRBP}},
	Roles: iam.PresetRoles(),
	Permissions: flatten(iam.PresetRolePermissions()),
	RoleRelations: iam.PresetRoleRelations(),
}
```

Assert:

- HRD inherits HRBP, 主管, 锻炼干部.
- a direct cycle returns `ErrRoleRelationCycle`.
- a chain of 17 relations returns `ErrRoleRelationDepthExceeded`.
- a disabled child role reached through RoleRelation contributes no permission.
- HRBP concrete department + social owner system binding yields department `dept_a` OR all-department social channel.
- HRBP bound to `__system__` returns `ErrScopeUnsupported`.
- HRBP gets `resume-parse`, `resume-recommend`, `resume-library`, `departments-positions`, `notifications`.

- [ ] **Step 2: Run resolver tests and verify red**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam -run 'TestResolvePrincipal|TestPageAccess' -count=1
```

Expected: FAIL because `ResolvePrincipalFromSnapshot` and related types do not exist.

- [ ] **Step 3: Implement resolver and structured scope types**

Implement:

```go
type Snapshot struct { User User; Departments []Department; RoleBindings []RoleBinding; Roles []Role; Permissions []PermissionGrant; RoleRelations []RoleRelation }
type Principal struct { User User; Bindings []RoleBinding; ExpandedRoleIDs []string; Permissions []PermissionGrant; DataScope DataScope; PageAccess []string; DefaultRoute string }
type DataScope struct { Departments []DepartmentScope; AllDepartments bool; Channels []string }
type ScopePredicate struct { Resource Resource; Action Action; DepartmentIDs []string; AllDepartments bool; Channels []string; Expired []bool; SelfUserID string }
type Decision struct { Allowed bool; Code string; MatchedBindingIDs []string }

func ResolvePrincipalFromSnapshot(snapshot Snapshot) (Principal, error)
func Can(principal Principal, resource Resource, action Action, target Target) Decision
func Scope(principal Principal, resource Resource, action Action) (ScopePredicate, error)
```

Keep the resolver pure. It must not import GORM, Echo, Huma, or app packages.

- [ ] **Step 4: Run resolver tests and verify green**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/iam/resolve.go apps/api/internal/iam/resolve_test.go
git commit -m "feat(api): resolve IAM principals"
```

## Task 4: IAM SQL Store and Role Summary

**Files:**
- Create: `apps/api/internal/iam/service.go`
- Create: `apps/api/internal/iam/sql_store.go`
- Test: `apps/api/internal/iam/sql_store_test.go`

- [ ] **Step 1: Write failing SQL store tests**

Create tests:

```go
func TestSQLStoreLoadsPrincipalSnapshot(t *testing.T)
func TestServiceRoleSummaryIncludesPermissionsAndDataScope(t *testing.T)
func TestServiceInvalidatesCacheForPermissionAncestorClosure(t *testing.T)
func TestServiceInvalidatesCacheForRoleRelationAncestorClosure(t *testing.T)
```

Use the existing migrated SQLite pattern from `apps/api/internal/auth/sql_store_test.go`. Seed:

- user `u_hrd`;
- department `dept_a`;
- `UserDepartmentRole(u_hrd, __system__, __role_hrd__)`;
- use migration-seeded roles, permissions, and role relations.

Assert `RoleSummary` contains inherited `Resume.List`, all-department scope for HRD system binding, and both `resume-parse` and `users` page access when permissions allow it.

- [ ] **Step 2: Run SQL store tests and verify red**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam -run 'TestSQLStore|TestService' -count=1
```

Expected: FAIL because SQL store/service are missing.

- [ ] **Step 3: Implement IAM service facade and SQL store**

Implement:

```go
type Store interface {
	LoadSnapshot(ctx context.Context, userID string) (Snapshot, error)
	UsersForRoleClosure(ctx context.Context, roleIDs []string) ([]string, error)
}
type Service struct { store Store; cache Cache; audit audit.Recorder }
type RoleSummary struct { Permissions []string; DataScope DataScope; PageAccess []string; DefaultRoute string }

func NewService(store Store, options ...Option) *Service
func (s *Service) ResolvePrincipal(ctx context.Context, userID string) (Principal, error)
func (s *Service) RoleSummary(ctx context.Context, userID string) (RoleSummary, error)
func (s *Service) InvalidateUser(userID string)
func (s *Service) InvalidateAll()
```

`SQLStore.LoadSnapshot` must load users, direct bindings, departments, enabled/disabled roles, permissions, and role relations. `UsersForRoleClosure` must include users bound to requested roles and users bound to ancestor roles. If the query cannot compute closure safely, service code must call `InvalidateAll`.

- [ ] **Step 4: Run IAM SQL tests and verify green**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/iam/service.go apps/api/internal/iam/sql_store.go apps/api/internal/iam/sql_store_test.go
git commit -m "feat(api): load IAM principals from SQL"
```

## Task 5: Error Codes, `/me` Contract, and Backend Guard

**Files:**
- Modify: `apps/api/internal/http/apperror/error.go`
- Modify: `apps/api/internal/auth/types.go`
- Modify: `apps/api/internal/auth/service.go`
- Modify: `apps/api/internal/auth/sql_store.go`
- Modify: `apps/api/internal/app/server.go`
- Modify: `apps/api/internal/app/auth_routes.go`
- Modify: `apps/api/cmd/api/main.go`
- Test: `apps/api/internal/app/auth_routes_test.go`
- Test: `apps/api/internal/app/server_test.go`
- Test: `apps/api/internal/app/openapi_test.go`

- [ ] **Step 1: Write failing API tests**

Add tests:

```go
func TestMeIncludesIAMPermissionsAndDataScope(t *testing.T)
func TestIAMGuardRejectsAuthenticatedUnauthorizedFutureRoute(t *testing.T)
func TestIAMGuardAttachesScopePredicateForListRoute(t *testing.T)
func TestOpenAPIAuthResponseIncludesIAMFields(t *testing.T)
```

For route guard tests, register a test route requiring `Resume.List` and use a fake IAM service that returns either denied or a known `ScopePredicate`. Assert:

- denied response status is 403;
- response code is `IAM_PERMISSION_DENIED`;
- allowed List route can read the scope predicate from Echo context or request context.

- [ ] **Step 2: Run API tests and verify red**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app ./internal/http/apperror -run 'TestMeIncludesIAM|TestIAMGuard|TestOpenAPIAuthResponseIncludesIAMFields|TestDefaultMessage' -count=1
```

Expected: FAIL because IAM fields and guard are not wired.

- [ ] **Step 3: Implement error codes and app interfaces**

Add constants from SPEC 002:

```go
IAMPermissionNotFound Code = "IAM_PERMISSION_NOT_FOUND"
IAMInvalidResource Code = "IAM_INVALID_RESOURCE"
IAMInvalidAction Code = "IAM_INVALID_ACTION"
IAMInvalidAttributeCondition Code = "IAM_INVALID_ATTRIBUTE_CONDITION"
IAMRoleRelationDepthExceeded Code = "IAM_ROLE_RELATION_DEPTH_EXCEEDED"
IAMPrincipalNotFound Code = "IAM_PRINCIPAL_NOT_FOUND"
IAMScopeUnsupported Code = "IAM_SCOPE_UNSUPPORTED"
```

Keep `PermissionDenied Code = "IAM_PERMISSION_DENIED"` as the 403 code.

Extend `app.Options`:

```go
IAMService IAMService
```

Add app interface:

```go
type IAMService interface {
	RoleSummary(context.Context, string) (iam.RoleSummary, error)
	Can(context.Context, iam.Principal, iam.Resource, iam.Action, iam.Target) iam.Decision
	Scope(context.Context, iam.Principal, iam.Resource, iam.Action) (iam.ScopePredicate, error)
	ResolvePrincipal(context.Context, string) (iam.Principal, error)
}
```

- [ ] **Step 4: Enrich `/me` through IAM**

Extend auth response DTO with:

```go
Permissions []string `json:"permissions" nullable:"false"`
DataScope iam.DataScope `json:"dataScope"`
```

Keep old fields `roleBindings`, `roleLabels`, `pageAccess`, `defaultRoute` intact for frontend compatibility. Current user loading should call IAM by user ID after session validation. If IAM is not configured in tests, fallback to the E1 role-label behavior only for auth bootstrap tests; production `main.go` must wire IAM.

- [ ] **Step 5: Implement route guard helper**

Expose a helper in app package:

```go
func RequirePermission(resource iam.Resource, action iam.Action) echo.MiddlewareFunc
func ScopePredicateFromContext(ctx context.Context) (iam.ScopePredicate, bool)
```

The middleware must require an authenticated principal, call IAM, return 401 for missing auth, return `IAM_PERMISSION_DENIED` for denied decisions, and attach scope for List routes.

- [ ] **Step 6: Wire main**

In `apps/api/cmd/api/main.go`, create:

```go
iamStore := iam.NewSQLStore(database)
iamService := iam.NewService(iamStore)
```

Pass `IAMService: iamService` to `app.NewServerWithOptions`.

- [ ] **Step 7: Run API tests and generated contract checks**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app ./internal/auth ./internal/http/apperror -count=1
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-check
```

Expected: Go tests PASS. `make openapi-check` FAILS with expected diff because `/me` schema changed.

- [ ] **Step 8: Regenerate OpenAPI**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-generate
```

Expected: `packages/api-contract/openapi.json` changes only for IAM response fields and error enums.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/http/apperror/error.go apps/api/internal/auth apps/api/internal/app apps/api/cmd/api/main.go packages/api-contract/openapi.json
git commit -m "feat(api): expose IAM summary and guard"
```

## Task 6: IAM Mutation Hooks, Audit, and Cache Invalidation

**Files:**
- Modify: `apps/api/internal/iam/service.go`
- Modify: `apps/api/internal/iam/sql_store.go`
- Modify: `apps/api/internal/audit/audit.go`
- Test: `apps/api/internal/iam/service_test.go`

- [ ] **Step 1: Write failing service mutation tests**

Create tests:

```go
func TestCreateUserDepartmentRoleInvalidatesUserAndAudits(t *testing.T)
func TestDeleteUserDepartmentRoleInvalidatesUserAndAudits(t *testing.T)
func TestReplaceRolePermissionsInvalidatesDirectAndAncestorUsers(t *testing.T)
func TestCreateRoleRelationRejectsCycleInvalidatesClosureAndAudits(t *testing.T)
func TestDeleteRoleRelationInvalidatesClosureAndAudits(t *testing.T)
```

Use a fake store that records calls and returns affected users:

```go
directUsers := []string{"u_direct"}
ancestorUsers := []string{"u_ancestor"}
```

Assert the service invalidates both users. For unsafe closure errors, assert `InvalidateAll` is called.

- [ ] **Step 2: Run mutation tests and verify red**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam -run 'TestCreateUserDepartmentRole|TestDeleteUserDepartmentRole|TestReplaceRolePermissions|TestCreateRoleRelation|TestDeleteRoleRelation' -count=1
```

Expected: FAIL because mutation methods do not exist.

- [ ] **Step 3: Implement internal mutation service methods**

Implement methods without adding HTTP routes:

```go
func (s *Service) CreateUserDepartmentRole(ctx context.Context, input UserDepartmentRoleInput) error
func (s *Service) DeleteUserDepartmentRole(ctx context.Context, id string) error
func (s *Service) ReplaceRolePermissions(ctx context.Context, roleID string, grants []PermissionGrant) error
func (s *Service) CreateRoleRelation(ctx context.Context, relation RoleRelation) error
func (s *Service) DeleteRoleRelation(ctx context.Context, id string) error
```

Each method must:

- validate whitelist and attribute conditions where relevant;
- reject unsupported `__system__` binding;
- reject RoleRelation cycle/depth violations;
- call SQL store inside a transaction;
- invalidate direct and ancestor users;
- record audit events with role/resource/action summaries only.

- [ ] **Step 4: Run IAM tests and verify green**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/iam ./internal/audit -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/iam apps/api/internal/audit/audit.go
git commit -m "feat(api): add IAM mutation hooks"
```

## Task 7: Frontend Shell Consumes IAM Summary

**Files:**
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`
- Modify by command: `packages/api-client/src/schema.d.ts`

- [ ] **Step 1: Write failing frontend tests**

Extend `guestSession` and add `hrbpSession` in `App.test.tsx`:

```ts
const hrbpSession = {
  ...guestSession,
  permissions: ["Resume.List", "Resume.Get", "Position.List", "PositionResume.Create"],
  dataScope: { allDepartments: false, channels: ["social", "campus"], departments: [{ id: "dept_a", name: "算力训练平台部" }] },
  pageAccess: ["resume-parse", "resume-recommend", "resume-library", "departments-positions"],
  roleLabels: ["HRBP"],
};
```

Add tests:

```ts
it("shows IAM data scope summary in the workspace header", async () => {
  apiMocks.getCurrentUser.mockResolvedValue({ data: hrbpSession, error: undefined });
  render(<App />);
  expect(await screen.findByText("算力训练平台部")).toBeInTheDocument();
  expect(screen.getByText("社招、校招")).toBeInTheDocument();
});

it("renders navigation from IAM page access", async () => {
  apiMocks.getCurrentUser.mockResolvedValue({ data: hrbpSession, error: undefined });
  render(<App />);
  expect(await screen.findByRole("link", { name: "简历库" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "部门与岗位" })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run frontend tests and verify red**

Run:

```bash
make test-web
```

Expected: FAIL because App does not render IAM data-scope summary and does not know the new page labels.

- [ ] **Step 3: Regenerate client and update frontend types**

Run:

```bash
make client-generate
```

Expected: `packages/api-client/src/schema.d.ts` includes `permissions` and `dataScope`.

Update `SessionView` in `App.tsx` to include:

```ts
permissions: string[];
dataScope: {
  allDepartments: boolean;
  channels: string[];
  departments: Array<{ id: string; name: string }>;
};
```

Add labels for `resume-library`, `departments-positions`, `users`, `roles`, `notifications`, and `audit-logs`. Render a compact data-scope summary in the workspace header using i18n text. Continue using project UI components for interactive controls.

- [ ] **Step 4: Run frontend tests and typecheck**

Run:

```bash
make test-web
make typecheck
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/app/App.tsx apps/web/src/app/App.test.tsx apps/web/src/i18n/zh-CN.ts apps/web/src/i18n/en-US.ts packages/api-client/src/schema.d.ts
git commit -m "feat(web): show IAM session scope"
```

## Task 8: Final Docs, Drift Checks, and Status

**Files:**
- Modify: `docs/project-status.md`
- Modify: `AGENTS.md`
- Modify: `README.md`

- [ ] **Step 1: Update project status**

Set IAM row to `Done` only after all implementation checks pass. Evidence must list:

```text
make test-api
make test-web
make openapi-check
make client-check
make typecheck
make lint
make build
```

Set next action to start E4/E5/E2 planning order based on dependencies.

- [ ] **Step 2: Update Agent memory**

Update `AGENTS.md` current phase to say IAM implemented and ready for the next business SPEC. Keep `docs/specs/002-iam-permission-model.md` and this plan in the documentation index.

- [ ] **Step 3: Run final verification**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make test-api
make test-web
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-check
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make client-check
make typecheck
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make lint
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make build
git diff --check
```

Expected: all commands PASS and generated OpenAPI/client files have no uncommitted drift.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md README.md docs/project-status.md
git commit -m "docs: mark IAM implementation complete"
```

## Implementation Notes

- Keep each task under roughly 1000 lines of patch. Split a task if it starts growing beyond that.
- Do not use raw SQL strings from IAM in handlers. IAM returns structured predicates; repositories translate them.
- Do not implement user-role or role-management HTTP endpoints in this plan.
- Do not add a production cache service. An in-memory cache or no cache is acceptable if invalidation behavior is test-covered.
- Keep all business route authorization behind central guard helpers.
- Run `make openapi-check` before committing generated contract changes; run `make client-check` after client generation.

## Self-Review Checklist

- SPEC coverage: tasks cover preset seeds, whitelist, RoleRelation expansion, attribute conditions, department scope, `/me`, guard, errors, audit hooks, cache invalidation, frontend shell, and docs.
- Scope control: E6/E8 UI and E2/E4/E5 business workflows are excluded.
- Type consistency: `Resource`, `Action`, `PermissionGrant`, `AttributeConditions`, `Principal`, `RoleSummary`, `DataScope`, and `ScopePredicate` are introduced before use in later tasks.
- Verification: each behavior task includes a failing test step, an implementation step, a passing verification step, and a commit step.
