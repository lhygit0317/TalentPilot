# E6 User Role Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build E6 user management and user-role binding management without role definition editing.

**Architecture:** Add a focused `apps/api/internal/useradmin` package that reuses IAM scope predicates, validators, cache invalidation, and audit recording. Expose Huma routes for users, assignable roles, batch role binding creation, and binding deletion; regenerate OpenAPI/client artifacts; then render a `用户管理` page under the existing `users` page access. Role, Permission, and RoleRelation editing stays out of scope for E8.

**Tech Stack:** Go, Echo, Huma, GORM, goose schema, React, TypeScript, Vite, Testing Library, shadcn/project UI wrappers, generated OpenAPI client.

---

## File Structure

- Create: `apps/api/internal/useradmin/types.go` for DTOs, command/query types, and domain errors.
- Create: `apps/api/internal/useradmin/service.go` for scope-aware orchestration, validation, audit, and IAM cache invalidation.
- Create: `apps/api/internal/useradmin/sql_store.go` for SQL projections, role options, binding insertion/deletion, and guest fallback.
- Create: `apps/api/internal/useradmin/service_test.go`.
- Create: `apps/api/internal/useradmin/sql_store_test.go`.
- Create: `apps/api/internal/app/useradmin_routes.go`.
- Create: `apps/api/internal/app/useradmin_routes_test.go`.
- Modify: `apps/api/internal/app/server.go` to register `UserAdminService`.
- Modify: `apps/api/internal/http/apperror/error.go` for E6 stable error codes.
- Modify: `packages/api-contract/openapi.json` after route generation.
- Modify: `packages/api-client/src/schema.d.ts` after client generation.
- Modify: `apps/web/src/api/client.ts` and `apps/web/src/api/client.test.ts` for E6 wrappers.
- Create: `apps/web/src/users/types.ts`.
- Create: `apps/web/src/users/UsersPage.tsx`.
- Create: `apps/web/src/users/UsersPage.test.tsx`.
- Modify: `apps/web/src/app/App.tsx` and `apps/web/src/app/App.test.tsx` to render `users`.
- Modify: `apps/web/src/i18n/zh-CN.ts` and `apps/web/src/i18n/en-US.ts` for user management messages.
- Modify: `docs/project-status.md` and `AGENTS.md` after verification.

## Task 1: UserAdmin Service Contract

**Files:**
- Create: `apps/api/internal/useradmin/types.go`
- Create: `apps/api/internal/useradmin/service.go`
- Test: `apps/api/internal/useradmin/service_test.go`

- [ ] **Step 1: Write the failing service tests**

Create `apps/api/internal/useradmin/service_test.go` with tests that lock validation before SQL exists:

```go
package useradmin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/useradmin"
)

func TestServiceCreateRoleBindingsRejectsDuplicateRequest(t *testing.T) {
	service := useradmin.NewService(&fakeStore{}, &fakeIAM{}, audit.NopRecorder{})

	_, err := service.CreateRoleBindings(context.Background(), useradmin.CreateRoleBindingsInput{
		ActorUserID: "admin",
		UserID:      "user_1",
		CreateScope: allUserDepartmentRoleScope(iam.ActionCreate),
		Bindings: []useradmin.RoleBindingRequest{
			{DepartmentID: "dept_a", RoleID: iam.RoleHRBP},
			{DepartmentID: "dept_a", RoleID: iam.RoleHRBP},
		},
	})

	if !errors.Is(err, useradmin.ErrDuplicateBinding) {
		t.Fatalf("expected duplicate binding error, got %v", err)
	}
}

func TestServiceDeleteRoleBindingRejectsGuestBinding(t *testing.T) {
	store := &fakeStore{binding: useradmin.RoleBindingDetail{
		ID: "udr_guest", UserID: "user_1", Guest: true,
		Department: useradmin.DepartmentSummary{ID: iam.SystemDepartmentID, Name: "system", System: true},
		Role:       useradmin.RoleSummary{ID: iam.RoleGuest, Label: "游客", IsSystem: true, Enabled: true},
	}}
	service := useradmin.NewService(store, &fakeIAM{}, audit.NopRecorder{})

	_, err := service.DeleteRoleBinding(context.Background(), useradmin.DeleteRoleBindingInput{
		ActorUserID: "admin", UserID: "user_1", BindingID: "udr_guest", DeleteScope: allUserDepartmentRoleScope(iam.ActionDelete),
	})

	if !errors.Is(err, useradmin.ErrGuestBindingProtected) {
		t.Fatalf("expected guest protected error, got %v", err)
	}
}

type fakeStore struct {
	binding useradmin.RoleBindingDetail
}

func (f *fakeStore) ListUsers(context.Context, useradmin.ListUsersQuery) (useradmin.UserListResult, error) {
	return useradmin.UserListResult{}, nil
}
func (f *fakeStore) GetUser(context.Context, string, iam.ScopePredicate) (useradmin.UserDetail, error) {
	return useradmin.UserDetail{}, nil
}
func (f *fakeStore) ListAssignableRoles(context.Context) ([]useradmin.AssignableRole, error) {
	return nil, nil
}
func (f *fakeStore) CreateRoleBindings(context.Context, useradmin.CreateRoleBindingsCommand) ([]useradmin.RoleBindingDetail, error) {
	return nil, nil
}
func (f *fakeStore) GetRoleBinding(context.Context, string) (useradmin.RoleBindingDetail, error) {
	return f.binding, nil
}
func (f *fakeStore) DeleteRoleBinding(context.Context, string) (useradmin.RoleBindingDetail, error) {
	return f.binding, nil
}
func (f *fakeStore) CountNonGuestBindings(context.Context, string) (int, error) {
	return 1, nil
}
func (f *fakeStore) EnsureGuestBinding(context.Context, string, string) error {
	return nil
}
func (f *fakeStore) WithTransaction(ctx context.Context, fn func(useradmin.Store) error) error {
	return fn(f)
}

type fakeIAM struct {
	invalidated []string
}

func (f *fakeIAM) InvalidateUser(userID string) {
	f.invalidated = append(f.invalidated, userID)
}

func allUserDepartmentRoleScope(action iam.Action) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourceUserDepartmentRole,
		Action:   action,
		Branches: []iam.ScopeBranch{{AllDepartments: true}},
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd apps/api && go test ./internal/useradmin -run TestService -count=1
```

Expected: FAIL because `apps/api/internal/useradmin` does not exist.

- [ ] **Step 3: Implement domain types and minimal service**

Create `types.go` with these exported names:

```go
package useradmin

import (
	"context"
	"errors"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

var (
	ErrUserNotFound             = errors.New("user not found")
	ErrBindingNotFound          = errors.New("user role binding not found")
	ErrDuplicateBinding         = errors.New("user role binding duplicate")
	ErrBatchEmpty               = errors.New("user role binding batch empty")
	ErrBatchTooLarge            = errors.New("user role binding batch too large")
	ErrGuestBindingProtected    = errors.New("guest binding protected")
	ErrSelfLockout              = errors.New("user role binding self lockout")
	ErrRoleDisabled             = errors.New("user role binding role disabled")
	ErrRoleNotFound             = errors.New("role not found")
	ErrDepartmentNotFound       = errors.New("department not found")
)

type Store interface {
	ListUsers(context.Context, ListUsersQuery) (UserListResult, error)
	GetUser(context.Context, string, iam.ScopePredicate) (UserDetail, error)
	ListAssignableRoles(context.Context) ([]AssignableRole, error)
	CreateRoleBindings(context.Context, CreateRoleBindingsCommand) ([]RoleBindingDetail, error)
	GetRoleBinding(context.Context, string) (RoleBindingDetail, error)
	DeleteRoleBinding(context.Context, string) (RoleBindingDetail, error)
	CountNonGuestBindings(context.Context, string) (int, error)
	EnsureGuestBinding(context.Context, string, string) error
	WithTransaction(context.Context, func(Store) error) error
}

type IAMCache interface {
	InvalidateUser(string)
}

type DepartmentSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	System bool   `json:"system"`
}

type RoleSummary struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	IsSystem bool   `json:"isSystem"`
	Enabled  bool   `json:"enabled"`
}

type RoleBindingDetail struct {
	ID         string            `json:"id"`
	UserID     string            `json:"-"`
	Role       RoleSummary       `json:"role"`
	Department DepartmentSummary `json:"department"`
	Guest      bool              `json:"guest"`
	CreatedAt  string            `json:"createdAt"`
	CreatedBy  string            `json:"createdBy"`
	CanDelete  bool              `json:"canDelete"`
}

type UserSummary struct {
	ID           string              `json:"id"`
	EmployeeID   string              `json:"employeeId"`
	Name         string              `json:"name"`
	Departments  []DepartmentSummary `json:"departments" nullable:"false"`
	RoleBindings []RoleBindingDetail `json:"roleBindings" nullable:"false"`
	RoleSummary  string              `json:"roleSummary"`
	GuestOnly    bool                `json:"guestOnly"`
	CanAssign    bool                `json:"canAssign"`
}

type UserListResult struct {
	Items            []UserSummary `json:"items" nullable:"false"`
	NextCursor       string        `json:"nextCursor"`
	DataScopeSummary  string        `json:"dataScopeSummary"`
	CanAssignRoles   bool          `json:"canAssignRoles"`
}

type UserDetail = UserSummary

type AssignableRole struct {
	ID                        string `json:"id"`
	Label                     string `json:"label"`
	Description               string `json:"description"`
	IsSystem                  bool   `json:"isSystem"`
	SupportsSystemDepartment  bool   `json:"supportsSystemDepartment"`
	AttributeConditionSummary string `json:"attributeConditionSummary"`
}

type AssignableRoleListResult struct {
	Items []AssignableRole `json:"items" nullable:"false"`
}

type ListUsersQuery struct {
	Search      string
	Limit       int
	Cursor      string
	ListScope   iam.ScopePredicate
	DeleteScope iam.ScopePredicate
	CanAssign   bool
}

type RoleBindingRequest struct {
	DepartmentID string `json:"departmentId" required:"true"`
	RoleID       string `json:"roleId" required:"true"`
}

type CreateRoleBindingsInput struct {
	ActorUserID string
	UserID      string
	CreateScope iam.ScopePredicate
	Bindings    []RoleBindingRequest
}

type CreateRoleBindingsCommand struct {
	ActorUserID string
	UserID      string
	Bindings    []RoleBindingRequest
}

type CreateRoleBindingsResult struct {
	User    UserIdentity        `json:"user"`
	Created []RoleBindingDetail `json:"created" nullable:"false"`
	Message string              `json:"message"`
}

type UserIdentity struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
}

type DeleteRoleBindingInput struct {
	ActorUserID string
	UserID      string
	BindingID   string
	DeleteScope iam.ScopePredicate
}

type DeleteRoleBindingResult struct {
	DeletedBindingID string `json:"deletedBindingId"`
	UserID           string `json:"userId"`
	Message          string `json:"message"`
}
```

Create `service.go` with validation methods that satisfy the tests:

```go
package useradmin

import (
	"context"
	"fmt"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

const maxBindingBatchSize = 20

type Service struct {
	store Store
	iam   IAMCache
	audit audit.Recorder
}

func NewService(store Store, iamCache IAMCache, recorder audit.Recorder) *Service {
	if recorder == nil {
		recorder = audit.NopRecorder{}
	}
	return &Service{store: store, iam: iamCache, audit: recorder}
}

func (s *Service) ListUsers(ctx context.Context, query ListUsersQuery) (UserListResult, error) {
	return s.store.ListUsers(ctx, query)
}

func (s *Service) GetUser(ctx context.Context, userID string, scope iam.ScopePredicate) (UserDetail, error) {
	return s.store.GetUser(ctx, userID, scope)
}

func (s *Service) ListAssignableRoles(ctx context.Context) (AssignableRoleListResult, error) {
	roles, err := s.store.ListAssignableRoles(ctx)
	if err != nil {
		return AssignableRoleListResult{}, err
	}
	return AssignableRoleListResult{Items: roles}, nil
}

func (s *Service) CreateRoleBindings(ctx context.Context, input CreateRoleBindingsInput) (CreateRoleBindingsResult, error) {
	if len(input.Bindings) == 0 {
		return CreateRoleBindingsResult{}, ErrBatchEmpty
	}
	if len(input.Bindings) > maxBindingBatchSize {
		return CreateRoleBindingsResult{}, ErrBatchTooLarge
	}
	if err := rejectDuplicateRequests(input.Bindings); err != nil {
		return CreateRoleBindingsResult{}, err
	}
	var created []RoleBindingDetail
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		var err error
		created, err = store.CreateRoleBindings(ctx, CreateRoleBindingsCommand{
			ActorUserID: input.ActorUserID,
			UserID:      input.UserID,
			Bindings:    input.Bindings,
		})
		return err
	}); err != nil {
		return CreateRoleBindingsResult{}, err
	}
	if s.iam != nil {
		s.iam.InvalidateUser(input.UserID)
	}
	for _, binding := range created {
		s.recordAudit(ctx, audit.Event{Type: audit.EventUserDepartmentRoleCreated, UserID: input.ActorUserID, Resource: string(iam.ResourceUserDepartmentRole), Action: string(iam.ActionCreate), TargetID: binding.ID, Result: "succeeded"})
	}
	return CreateRoleBindingsResult{Created: created, Message: fmt.Sprintf("已为用户分配 %d 个角色绑定", len(created))}, nil
}

func (s *Service) DeleteRoleBinding(ctx context.Context, input DeleteRoleBindingInput) (DeleteRoleBindingResult, error) {
	binding, err := s.store.GetRoleBinding(ctx, input.BindingID)
	if err != nil {
		return DeleteRoleBindingResult{}, err
	}
	if binding.UserID != input.UserID {
		return DeleteRoleBindingResult{}, ErrBindingNotFound
	}
	if binding.Guest {
		return DeleteRoleBindingResult{}, ErrGuestBindingProtected
	}
	if input.ActorUserID == input.UserID {
		count, err := s.store.CountNonGuestBindings(ctx, input.UserID)
		if err != nil {
			return DeleteRoleBindingResult{}, err
		}
		if count <= 1 {
			return DeleteRoleBindingResult{}, ErrSelfLockout
		}
	}
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		_, err := store.DeleteRoleBinding(ctx, input.BindingID)
		if err != nil {
			return err
		}
		remaining, err := store.CountNonGuestBindings(ctx, input.UserID)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return store.EnsureGuestBinding(ctx, input.UserID, input.ActorUserID)
		}
		return nil
	}); err != nil {
		return DeleteRoleBindingResult{}, err
	}
	if s.iam != nil {
		s.iam.InvalidateUser(input.UserID)
	}
	s.recordAudit(ctx, audit.Event{Type: audit.EventUserDepartmentRoleDeleted, UserID: input.ActorUserID, Resource: string(iam.ResourceUserDepartmentRole), Action: string(iam.ActionDelete), TargetID: input.BindingID, Result: "succeeded"})
	return DeleteRoleBindingResult{DeletedBindingID: input.BindingID, UserID: input.UserID, Message: "已解除 " + binding.Role.Label + "(部门:" + binding.Department.Name + ")"}, nil
}

func rejectDuplicateRequests(bindings []RoleBindingRequest) error {
	seen := map[string]bool{}
	for _, binding := range bindings {
		key := binding.DepartmentID + "\x00" + binding.RoleID
		if seen[key] {
			return ErrDuplicateBinding
		}
		seen[key] = true
	}
	return nil
}

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	if event.At.IsZero() {
		event.At = time.Now()
	}
	_ = s.audit.Record(ctx, event)
}
```

- [ ] **Step 4: Run service tests**

Run:

```bash
cd apps/api && go test ./internal/useradmin -run TestService -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/useradmin/types.go apps/api/internal/useradmin/service.go apps/api/internal/useradmin/service_test.go
git commit -m "feat(api): add user role management service"
```

## Task 2: SQL Store Listing and Assignable Roles

**Files:**
- Modify: `apps/api/internal/useradmin/sql_store.go`
- Test: `apps/api/internal/useradmin/sql_store_test.go`

- [ ] **Step 1: Write failing SQL listing tests**

Create `sql_store_test.go` with a migrated SQLite fixture:

```go
package useradmin_test

import (
	"context"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/platform/db"
	"github.com/talentpilot/talentpilot/apps/api/internal/useradmin"
	"gorm.io/gorm"
)

func TestSQLStoreListUsersAppliesBindingScope(t *testing.T) {
	gdb := newMigratedDB(t)
	seedUserAdminFixture(t, gdb)
	store := useradmin.NewSQLStore(gdb)

	result, err := store.ListUsers(context.Background(), useradmin.ListUsersQuery{
		ListScope: iam.ScopePredicate{Resource: iam.ResourceUserDepartmentRole, Action: iam.ActionList, Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}}},
		DeleteScope: iam.ScopePredicate{Resource: iam.ResourceUserDepartmentRole, Action: iam.ActionDelete, Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}}},
		CanAssign: true,
	})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "user_a" {
		t.Fatalf("expected only user_a in dept_a scope, got %#v", result.Items)
	}
	if !result.Items[0].RoleBindings[0].CanDelete {
		t.Fatalf("expected binding canDelete from delete scope: %#v", result.Items[0].RoleBindings)
	}
}

func TestSQLStoreListAssignableRolesExcludesDisabledRoles(t *testing.T) {
	gdb := newMigratedDB(t)
	seedUserAdminFixture(t, gdb)
	store := useradmin.NewSQLStore(gdb)

	roles, err := store.ListAssignableRoles(context.Background())
	if err != nil {
		t.Fatalf("assignable roles: %v", err)
	}
	for _, role := range roles {
		if role.ID == "role_disabled" {
			t.Fatalf("disabled role should be excluded: %#v", roles)
		}
	}
	if len(roles) == 0 || roles[0].ID == "" {
		t.Fatalf("expected assignable roles, got %#v", roles)
	}
}

func newMigratedDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := db.Open(db.Config{Driver: "sqlite", DSN: "file:useradmin?mode=memory&cache=shared"})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(gdb, "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return gdb
}
```

Add fixture helpers in the same file:

```go
func seedUserAdminFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	execSQL(t, db, `INSERT INTO departments (id, name, created_at, updated_at) VALUES ('dept_a','算力训练平台部',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),('dept_b','智算调度部',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`)
	execSQL(t, db, `INSERT INTO users (id, employee_id, name, created_at, updated_at) VALUES ('user_a','A10001','张敏',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),('user_b','A10002','李四',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`)
	execSQL(t, db, `INSERT INTO roles (id, label, description, is_system, enabled, created_at, created_by, updated_at) VALUES ('role_disabled','停用角色','停用',FALSE,FALSE,CURRENT_TIMESTAMP,'system',CURRENT_TIMESTAMP)`)
	execSQL(t, db, `INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by) VALUES ('udr_a','user_a','dept_a','__role_hrbp__',CURRENT_TIMESTAMP,'admin'),('udr_b','user_b','dept_b','__role_manager__',CURRENT_TIMESTAMP,'admin')`)
}

func execSQL(t *testing.T, db *gorm.DB, query string) {
	t.Helper()
	if err := db.Exec(query).Error; err != nil {
		t.Fatalf("exec fixture sql: %v\n%s", err, query)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd apps/api && go test ./internal/useradmin -run TestSQLStoreList -count=1
```

Expected: FAIL because `NewSQLStore`, `ListUsers`, and `ListAssignableRoles` do not exist.

- [ ] **Step 3: Implement SQL list projections**

Create `sql_store.go`:

```go
package useradmin

import (
	"context"
	"sort"
	"strings"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"gorm.io/gorm"
)

type SQLStore struct {
	db *gorm.DB
}

func NewSQLStore(db *gorm.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) WithTransaction(ctx context.Context, fn func(Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&SQLStore{db: tx})
	})
}

func (s *SQLStore) ListUsers(ctx context.Context, query ListUsersQuery) (UserListResult, error) {
	rows, err := s.loadVisibleBindingRows(ctx, query.ListScope, query.Search)
	if err != nil {
		return UserListResult{}, err
	}
	users := map[string]*UserSummary{}
	for _, row := range rows {
		user := users[row.UserID]
		if user == nil {
			user = &UserSummary{ID: row.UserID, EmployeeID: row.EmployeeID, Name: row.UserName, CanAssign: query.CanAssign}
			users[row.UserID] = user
		}
		department := DepartmentSummary{ID: row.DepartmentID, Name: row.DepartmentName, System: row.DepartmentID == iam.SystemDepartmentID}
		role := RoleSummary{ID: row.RoleID, Label: row.RoleLabel, IsSystem: row.RoleIsSystem, Enabled: row.RoleEnabled}
		guest := row.RoleID == iam.RoleGuest
		user.RoleBindings = append(user.RoleBindings, RoleBindingDetail{ID: row.ID, UserID: row.UserID, Department: department, Role: role, Guest: guest, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy, CanDelete: !guest && scopeAllowsDepartment(query.DeleteScope, row.DepartmentID)})
		if !guest {
			user.Departments = appendUniqueDepartment(user.Departments, department)
		}
	}
	items := make([]UserSummary, 0, len(users))
	for _, user := range users {
		user.GuestOnly = hasOnlyGuest(user.RoleBindings)
		user.RoleSummary = formatRoleSummary(user.RoleBindings)
		items = append(items, *user)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].EmployeeID < items[j].EmployeeID
		}
		return items[i].Name < items[j].Name
	})
	return UserListResult{Items: items, DataScopeSummary: formatDataScopeSummary(query.ListScope), CanAssignRoles: query.CanAssign}, nil
}

func (s *SQLStore) ListAssignableRoles(ctx context.Context) ([]AssignableRole, error) {
	var rows []struct {
		ID          string
		Label       string
		Description string
		IsSystem    bool `gorm:"column:is_system"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, label, description, is_system
		FROM roles
		WHERE enabled = TRUE
		ORDER BY is_system DESC, label ASC
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	roles := make([]AssignableRole, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, AssignableRole{ID: row.ID, Label: row.Label, Description: row.Description, IsSystem: row.IsSystem, SupportsSystemDepartment: row.ID == iam.RoleGuest || iam.RoleSupportsGlobalScope(row.ID), AttributeConditionSummary: roleConditionSummary(row.ID)})
	}
	return roles, nil
}

type bindingRow struct {
	ID             string
	UserID         string `gorm:"column:user_id"`
	EmployeeID     string `gorm:"column:employee_id"`
	UserName       string `gorm:"column:user_name"`
	DepartmentID   string `gorm:"column:department_id"`
	DepartmentName string `gorm:"column:department_name"`
	RoleID         string `gorm:"column:role_id"`
	RoleLabel      string `gorm:"column:role_label"`
	RoleIsSystem   bool   `gorm:"column:role_is_system"`
	RoleEnabled    bool   `gorm:"column:role_enabled"`
	CreatedAt      string `gorm:"column:created_at"`
	CreatedBy      string `gorm:"column:created_by"`
}
```

Add helper functions in the same file:

```go
func (s *SQLStore) loadVisibleBindingRows(ctx context.Context, scope iam.ScopePredicate, search string) ([]bindingRow, error) {
	departmentIDs, allDepartments := scopeDepartments(scope)
	query := `
		SELECT udr.id, users.id AS user_id, users.employee_id, users.name AS user_name,
			departments.id AS department_id, departments.name AS department_name,
			roles.id AS role_id, roles.label AS role_label, roles.is_system AS role_is_system,
			roles.enabled AS role_enabled, CAST(udr.created_at AS TEXT) AS created_at, udr.created_by
		FROM user_department_roles udr
		JOIN users ON users.id = udr.user_id
		JOIN departments ON departments.id = udr.department_id
		JOIN roles ON roles.id = udr.role_id
		WHERE (? = TRUE OR udr.department_id IN ?)
	`
	args := []any{allDepartments, departmentIDs}
	trimmed := strings.TrimSpace(search)
	if trimmed != "" {
		like := "%" + escapeLike(trimmed) + "%"
		query += ` AND (users.name LIKE ? ESCAPE '\' OR users.employee_id LIKE ? ESCAPE '\' OR departments.name LIKE ? ESCAPE '\' OR roles.label LIKE ? ESCAPE '\')`
		args = append(args, like, like, like, like)
	}
	query += ` ORDER BY users.name ASC, users.employee_id ASC, roles.label ASC`
	var rows []bindingRow
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func scopeDepartments(scope iam.ScopePredicate) ([]string, bool) {
	ids := map[string]bool{}
	for _, branch := range scope.Branches {
		if branch.AllDepartments {
			return []string{iam.SystemDepartmentID}, true
		}
		for _, id := range branch.DepartmentIDs {
			ids[id] = true
		}
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = []string{"__none__"}
	}
	return out, false
}

func scopeAllowsDepartment(scope iam.ScopePredicate, departmentID string) bool {
	for _, branch := range scope.Branches {
		if branch.AllDepartments {
			return true
		}
		for _, id := range branch.DepartmentIDs {
			if id == departmentID {
				return true
			}
		}
	}
	return false
}

func appendUniqueDepartment(items []DepartmentSummary, next DepartmentSummary) []DepartmentSummary {
	for _, item := range items {
		if item.ID == next.ID {
			return items
		}
	}
	return append(items, next)
}

func hasOnlyGuest(bindings []RoleBindingDetail) bool {
	if len(bindings) == 0 {
		return true
	}
	for _, binding := range bindings {
		if !binding.Guest {
			return false
		}
	}
	return true
}
```

Finish summary helpers:

```go
func formatRoleSummary(bindings []RoleBindingDetail) string {
	if hasOnlyGuest(bindings) {
		return "游客"
	}
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Guest {
			continue
		}
		parts = append(parts, binding.Role.Label+"(部门:"+binding.Department.Name+")")
	}
	sort.Strings(parts)
	return strings.Join(parts, " | ")
}

func formatDataScopeSummary(scope iam.ScopePredicate) string {
	departmentIDs, all := scopeDepartments(scope)
	if all {
		return "当前数据权限:全部部门"
	}
	return "当前数据权限:" + strings.Join(departmentIDs, "、")
}

func roleConditionSummary(roleID string) string {
	switch roleID {
	case iam.RoleSocialOwner:
		return "社招"
	case iam.RoleCampusOwner:
		return "校招"
	default:
		return ""
	}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
```

- [ ] **Step 4: Run SQL listing tests**

Run:

```bash
cd apps/api && go test ./internal/useradmin -run 'TestSQLStoreList(User|Assignable)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/useradmin/sql_store.go apps/api/internal/useradmin/sql_store_test.go
git commit -m "feat(api): list user role management data"
```

## Task 3: SQL Store Mutations

**Files:**
- Modify: `apps/api/internal/useradmin/sql_store.go`
- Modify: `apps/api/internal/useradmin/sql_store_test.go`

- [ ] **Step 1: Write failing mutation tests**

Append these tests:

```go
func TestSQLStoreCreateRoleBindingsRejectsStoredDuplicate(t *testing.T) {
	gdb := newMigratedDB(t)
	seedUserAdminFixture(t, gdb)
	store := useradmin.NewSQLStore(gdb)

	_, err := store.CreateRoleBindings(context.Background(), useradmin.CreateRoleBindingsCommand{
		ActorUserID: "admin",
		UserID:      "user_a",
		Bindings:    []useradmin.RoleBindingRequest{{DepartmentID: "dept_a", RoleID: "__role_hrbp__"}},
	})
	if err == nil {
		t.Fatalf("expected duplicate insert to fail")
	}
}

func TestSQLStoreDeleteRoleBindingAndEnsureGuestFallback(t *testing.T) {
	gdb := newMigratedDB(t)
	seedUserAdminFixture(t, gdb)
	store := useradmin.NewSQLStore(gdb)

	deleted, err := store.DeleteRoleBinding(context.Background(), "udr_a")
	if err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	if deleted.ID != "udr_a" || deleted.UserID != "user_a" {
		t.Fatalf("unexpected deleted binding: %#v", deleted)
	}
	if err := store.EnsureGuestBinding(context.Background(), "user_a", "admin"); err != nil {
		t.Fatalf("ensure guest: %v", err)
	}
	count := countRows(t, gdb, "user_department_roles", "user_id = 'user_a' AND role_id = '__role_guest__'")
	if count != 1 {
		t.Fatalf("expected guest fallback, got %d", count)
	}
}
```

Add `countRows` helper:

```go
func countRows(t *testing.T, db *gorm.DB, table string, where string) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Where(where).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}
```

- [ ] **Step 2: Run mutation tests and verify RED**

Run:

```bash
cd apps/api && go test ./internal/useradmin -run 'TestSQLStore(CreateRoleBindings|DeleteRoleBinding)' -count=1
```

Expected: FAIL because mutation methods are not implemented.

- [ ] **Step 3: Implement SQL mutation methods**

Append to `sql_store.go`:

```go
func (s *SQLStore) CreateRoleBindings(ctx context.Context, command CreateRoleBindingsCommand) ([]RoleBindingDetail, error) {
	created := make([]RoleBindingDetail, 0, len(command.Bindings))
	for _, binding := range command.Bindings {
		id := stableBindingID(command.UserID, binding.DepartmentID, binding.RoleID)
		if err := s.db.WithContext(ctx).Exec(`
			INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
		`, id, command.UserID, binding.DepartmentID, binding.RoleID, command.ActorUserID).Error; err != nil {
			return nil, ErrDuplicateBinding
		}
		detail, err := s.GetRoleBinding(ctx, id)
		if err != nil {
			return nil, err
		}
		created = append(created, detail)
	}
	return created, nil
}

func (s *SQLStore) GetRoleBinding(ctx context.Context, bindingID string) (RoleBindingDetail, error) {
	rows, err := s.loadBindingRowsByID(ctx, bindingID)
	if err != nil {
		return RoleBindingDetail{}, err
	}
	if len(rows) == 0 {
		return RoleBindingDetail{}, ErrBindingNotFound
	}
	row := rows[0]
	return RoleBindingDetail{
		ID: row.ID, UserID: row.UserID, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy,
		Department: DepartmentSummary{ID: row.DepartmentID, Name: row.DepartmentName, System: row.DepartmentID == iam.SystemDepartmentID},
		Role:       RoleSummary{ID: row.RoleID, Label: row.RoleLabel, IsSystem: row.RoleIsSystem, Enabled: row.RoleEnabled},
		Guest:      row.RoleID == iam.RoleGuest,
		CanDelete:  row.RoleID != iam.RoleGuest,
	}, nil
}

func (s *SQLStore) DeleteRoleBinding(ctx context.Context, bindingID string) (RoleBindingDetail, error) {
	binding, err := s.GetRoleBinding(ctx, bindingID)
	if err != nil {
		return RoleBindingDetail{}, err
	}
	if err := s.db.WithContext(ctx).Exec(`DELETE FROM user_department_roles WHERE id = ?`, bindingID).Error; err != nil {
		return RoleBindingDetail{}, err
	}
	return binding, nil
}
```

Add count/fallback helpers:

```go
func (s *SQLStore) CountNonGuestBindings(ctx context.Context, userID string) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).Table("user_department_roles").Where("user_id = ? AND role_id <> ?", userID, iam.RoleGuest).Count(&count).Error
	return int(count), err
}

func (s *SQLStore) EnsureGuestBinding(ctx context.Context, userID string, actorUserID string) error {
	id := stableBindingID(userID, iam.SystemDepartmentID, iam.RoleGuest)
	return s.db.WithContext(ctx).Exec(`
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(user_id, department_id, role_id) DO NOTHING
	`, id, userID, iam.SystemDepartmentID, iam.RoleGuest, actorUserID).Error
}

func (s *SQLStore) loadBindingRowsByID(ctx context.Context, bindingID string) ([]bindingRow, error) {
	var rows []bindingRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT udr.id, users.id AS user_id, users.employee_id, users.name AS user_name,
			departments.id AS department_id, departments.name AS department_name,
			roles.id AS role_id, roles.label AS role_label, roles.is_system AS role_is_system,
			roles.enabled AS role_enabled, CAST(udr.created_at AS TEXT) AS created_at, udr.created_by
		FROM user_department_roles udr
		JOIN users ON users.id = udr.user_id
		JOIN departments ON departments.id = udr.department_id
		JOIN roles ON roles.id = udr.role_id
		WHERE udr.id = ?
	`, bindingID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
```

Add stable ID helper:

```go
func stableBindingID(parts ...string) string {
	joined := strings.Join(parts, "_")
	joined = strings.ReplaceAll(joined, "-", "_")
	return "udr_" + joined
}
```

- [ ] **Step 4: Run mutation and package tests**

Run:

```bash
cd apps/api && go test ./internal/useradmin -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/useradmin
git commit -m "feat(api): mutate user role bindings"
```

## Task 4: HTTP Routes and Error Mapping

**Files:**
- Create: `apps/api/internal/app/useradmin_routes.go`
- Create: `apps/api/internal/app/useradmin_routes_test.go`
- Modify: `apps/api/internal/app/server.go`
- Modify: `apps/api/internal/http/apperror/error.go`

- [ ] **Step 1: Write failing route tests**

Create `useradmin_routes_test.go`:

```go
package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/useradmin"
)

func TestUserAdminRoutesListUsersRequiresBindingList(t *testing.T) {
	service := &fakeUserAdminService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{decisions: map[string]iam.Decision{
			iam.PermissionKey(iam.ResourceUser, iam.ActionList):               {Allowed: true},
			iam.PermissionKey(iam.ResourceUserDepartmentRole, iam.ActionList): {Allowed: false},
		}},
		UserAdminService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.listCalls != 0 {
		t.Fatalf("expected denial before service call")
	}
}

func TestUserAdminRoutesCreateBindingsForwardsActorAndScope(t *testing.T) {
	service := &fakeUserAdminService{createResult: useradmin.CreateRoleBindingsResult{
		Created: []useradmin.RoleBindingDetail{{ID: "udr_new"}},
		Message: "已为 张敏 分配 1 个角色绑定",
	}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scope:    allScope(iam.ResourceUserDepartmentRole, iam.ActionCreate),
		},
		UserAdminService: service,
	})
	req := httptest.NewRequest(http.MethodPost, "/users/user_1/role-bindings", strings.NewReader(`{"bindings":[{"departmentId":"dept_a","roleId":"__role_hrbp__"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.createInput.ActorUserID != "w3_1" || service.createInput.UserID != "user_1" {
		t.Fatalf("unexpected create input: %#v", service.createInput)
	}
}
```

Add fake service:

```go
type fakeUserAdminService struct {
	listCalls int
	createInput useradmin.CreateRoleBindingsInput
	createResult useradmin.CreateRoleBindingsResult
}

func (f *fakeUserAdminService) ListUsers(ctx context.Context, query useradmin.ListUsersQuery) (useradmin.UserListResult, error) {
	f.listCalls++
	return useradmin.UserListResult{}, nil
}
func (f *fakeUserAdminService) GetUser(ctx context.Context, userID string, scope iam.ScopePredicate) (useradmin.UserDetail, error) {
	return useradmin.UserDetail{ID: userID, Name: "张敏"}, nil
}
func (f *fakeUserAdminService) ListAssignableRoles(ctx context.Context) (useradmin.AssignableRoleListResult, error) {
	return useradmin.AssignableRoleListResult{}, nil
}
func (f *fakeUserAdminService) CreateRoleBindings(ctx context.Context, input useradmin.CreateRoleBindingsInput) (useradmin.CreateRoleBindingsResult, error) {
	f.createInput = input
	return f.createResult, nil
}
func (f *fakeUserAdminService) DeleteRoleBinding(ctx context.Context, input useradmin.DeleteRoleBindingInput) (useradmin.DeleteRoleBindingResult, error) {
	return useradmin.DeleteRoleBindingResult{DeletedBindingID: input.BindingID, UserID: input.UserID}, nil
}
```

- [ ] **Step 2: Add OpenAPI route test**

Add:

```go
func TestOpenAPIDocumentIncludesUserAdminEndpoints(t *testing.T) {
	server := NewServer()
	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal openapi: %v", err)
	}
	assertOperation(t, doc.Paths, "/users", "get", "get-users")
	assertOperation(t, doc.Paths, "/users/{userId}", "get", "get-user")
	assertOperation(t, doc.Paths, "/roles/assignable", "get", "get-assignable-roles")
	assertOperation(t, doc.Paths, "/users/{userId}/role-bindings", "post", "post-user-role-bindings")
	assertOperation(t, doc.Paths, "/users/{userId}/role-bindings/{bindingId}", "delete", "delete-user-role-binding")
}
```

- [ ] **Step 3: Run route tests and verify RED**

Run:

```bash
cd apps/api && go test ./internal/app -run 'TestUserAdmin|TestOpenAPIDocumentIncludesUserAdmin' -count=1
```

Expected: FAIL because route/service wiring does not exist.

- [ ] **Step 4: Add server interface and route registration**

Modify `server.go`:

```go
import "github.com/talentpilot/talentpilot/apps/api/internal/useradmin"

type Options struct {
	AuthService           AuthService
	FrontendOrigin        string
	RequireHTTPS          bool
	SecureCookies         bool
	TrustForwardedProto   bool
	IAMService            IAMService
	ResumeService         ResumeService
	OrganizationService   OrganizationService
	MatchingService       MatchingService
	RecommendationService RecommendationService
	UserAdminService UserAdminService
}

type UserAdminService interface {
	ListUsers(context.Context, useradmin.ListUsersQuery) (useradmin.UserListResult, error)
	GetUser(context.Context, string, iam.ScopePredicate) (useradmin.UserDetail, error)
	ListAssignableRoles(context.Context) (useradmin.AssignableRoleListResult, error)
	CreateRoleBindings(context.Context, useradmin.CreateRoleBindingsInput) (useradmin.CreateRoleBindingsResult, error)
	DeleteRoleBinding(context.Context, useradmin.DeleteRoleBindingInput) (useradmin.DeleteRoleBindingResult, error)
}

func NewServerWithOptions(options Options) *Server {
	registerHealth(api)
	registerAuthRoutes(api, options)
	registerResumeRoutes(api, options)
	registerOrganizationRoutes(api, options)
	registerMatchingRoutes(api, options)
	registerRecommendationRoutes(api, options)
	registerUserAdminRoutes(api, options)
	return &Server{Echo: e, API: api}
}
```

- [ ] **Step 5: Implement `useradmin_routes.go`**

Create route file with all endpoints:

```go
package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/useradmin"
)

type userListInput struct {
	Search string `query:"search"`
	Limit  int    `query:"limit" minimum:"1" maximum:"100"`
	Cursor string `query:"cursor"`
}

type userIDInput struct {
	UserID string `path:"userId"`
}

type createUserRoleBindingsInput struct {
	UserID string `path:"userId"`
	Body struct {
		Bindings []useradmin.RoleBindingRequest `json:"bindings" nullable:"false"`
	} `json:"body"`
}

type deleteUserRoleBindingInput struct {
	UserID    string `path:"userId"`
	BindingID string `path:"bindingId"`
}

type userListOutput struct { Body useradmin.UserListResult `json:"body"` }
type userDetailOutput struct { Body useradmin.UserDetail `json:"body"` }
type assignableRolesOutput struct { Body useradmin.AssignableRoleListResult `json:"body"` }
type createUserRoleBindingsOutput struct { Body useradmin.CreateRoleBindingsResult `json:"body"` }
type deleteUserRoleBindingOutput struct { Body useradmin.DeleteRoleBindingResult `json:"body"` }
```

Register list/detail/options/mutation routes:

```go
func registerUserAdminRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{OperationID: "get-users", Method: http.MethodGet, Path: "/users", Tags: []string{"users"}}, func(ctx context.Context, input *userListInput) (*userListOutput, error) {
		principal, listScope, err := authorizeRequest(ctx, options, iam.ResourceUser, iam.ActionList)
		if err != nil { return nil, err }
		if err := requireDecision(ctx, options, principal, iam.ResourceUserDepartmentRole, iam.ActionList); err != nil { return nil, err }
		bindingScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourceUserDepartmentRole, iam.ActionList)
		if err != nil { return nil, err }
		deleteScope, _ := scopeForPrincipal(ctx, options, principal, iam.ResourceUserDepartmentRole, iam.ActionDelete)
		canAssign := options.IAMService.Can(ctx, principal, iam.ResourceUserDepartmentRole, iam.ActionCreate, iam.Target{}).Allowed
		service, err := requireUserAdminService(options.UserAdminService)
		if err != nil { return nil, err }
		result, err := service.ListUsers(ctx, useradmin.ListUsersQuery{Search: input.Search, Limit: input.Limit, Cursor: input.Cursor, ListScope: bindingScope, DeleteScope: deleteScope, CanAssign: canAssign})
		_ = listScope
		if err != nil { return nil, mapUserAdminError(err) }
		return &userListOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "get-assignable-roles", Method: http.MethodGet, Path: "/roles/assignable", Tags: []string{"users"}}, func(ctx context.Context, input *struct{}) (*assignableRolesOutput, error) {
		_, _, err := authorizeRequest(ctx, options, iam.ResourceUserDepartmentRole, iam.ActionCreate)
		if err != nil { return nil, err }
		service, err := requireUserAdminService(options.UserAdminService)
		if err != nil { return nil, err }
		result, err := service.ListAssignableRoles(ctx)
		if err != nil { return nil, mapUserAdminError(err) }
		return &assignableRolesOutput{Body: result}, nil
	})
}
```

Add the remaining routes in the same file:

```go
func requireUserAdminService(service UserAdminService) (UserAdminService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.Internal, "User admin service is not configured", "", nil)
	}
	return service, nil
}

func requireDecision(ctx context.Context, options Options, principal iam.Principal, resource iam.Resource, action iam.Action) error {
	if decision := options.IAMService.Can(ctx, principal, resource, action, iam.Target{}); !decision.Allowed {
		return apperror.NewProblem(apperror.PermissionDenied, "", "", map[string]any{"resource": resource, "action": action})
	}
	return nil
}

func mapUserAdminError(err error) error {
	switch {
	case errors.Is(err, useradmin.ErrUserNotFound):
		return apperror.NewProblem(apperror.UserNotFound, "", "", nil)
	case errors.Is(err, useradmin.ErrBindingNotFound):
		return apperror.NewProblem(apperror.UserRoleBindingNotFound, "", "", nil)
	case errors.Is(err, useradmin.ErrDuplicateBinding):
		return apperror.NewProblem(apperror.UserRoleBindingDuplicate, "", "", nil)
	case errors.Is(err, useradmin.ErrGuestBindingProtected):
		return apperror.NewProblem(apperror.UserRoleBindingGuestProtected, "", "", nil)
	case errors.Is(err, useradmin.ErrSelfLockout):
		return apperror.NewProblem(apperror.UserRoleBindingSelfLockout, "", "", nil)
	case errors.Is(err, useradmin.ErrRoleDisabled):
		return apperror.NewProblem(apperror.UserRoleBindingRoleDisabled, "", "", nil)
	default:
		return apperror.NewProblem(apperror.Internal, "", "", nil)
	}
}
```

- [ ] **Step 6: Add error codes**

Modify `apps/api/internal/http/apperror/error.go`:

```go
const (
	UserNotFound                    Code = "USER_NOT_FOUND"
	UserRoleBindingNotFound         Code = "USER_ROLE_BINDING_NOT_FOUND"
	UserRoleBindingDuplicate        Code = "USER_ROLE_BINDING_DUPLICATE"
	UserRoleBindingBatchEmpty       Code = "USER_ROLE_BINDING_BATCH_EMPTY"
	UserRoleBindingBatchTooLarge    Code = "USER_ROLE_BINDING_BATCH_TOO_LARGE"
	UserRoleBindingGuestProtected   Code = "USER_ROLE_BINDING_GUEST_PROTECTED"
	UserRoleBindingSelfLockout      Code = "USER_ROLE_BINDING_SELF_LOCKOUT"
	UserRoleBindingRoleDisabled     Code = "USER_ROLE_BINDING_ROLE_DISABLED"
)
```

Map statuses:

```go
case UserNotFound, UserRoleBindingNotFound:
	return http.StatusNotFound
case UserRoleBindingDuplicate, UserRoleBindingBatchEmpty, UserRoleBindingBatchTooLarge, UserRoleBindingGuestProtected, UserRoleBindingSelfLockout, UserRoleBindingRoleDisabled:
	return http.StatusUnprocessableEntity
```

Add default messages:

```go
case UserNotFound:
	return "用户不存在"
case UserRoleBindingNotFound:
	return "角色绑定不存在"
case UserRoleBindingDuplicate:
	return "该用户已存在相同角色绑定"
case UserRoleBindingBatchEmpty:
	return "请至少添加一条角色绑定"
case UserRoleBindingBatchTooLarge:
	return "一次最多添加 20 条角色绑定"
case UserRoleBindingGuestProtected:
	return "游客身份不可解除"
case UserRoleBindingSelfLockout:
	return "不能解除自己的最后一个业务角色"
case UserRoleBindingRoleDisabled:
	return "该角色已禁用，不能分配"
```

- [ ] **Step 7: Run route tests**

Run:

```bash
cd apps/api && go test ./internal/app -run 'TestUserAdmin|TestOpenAPIDocumentIncludesUserAdmin' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/app/useradmin_routes.go apps/api/internal/app/useradmin_routes_test.go apps/api/internal/app/server.go apps/api/internal/http/apperror/error.go
git commit -m "feat(api): expose user role management endpoints"
```

## Task 5: OpenAPI, Client Wrappers, and Frontend Types

**Files:**
- Modify generated: `packages/api-contract/openapi.json`
- Modify generated: `packages/api-client/src/schema.d.ts`
- Modify: `apps/web/src/api/client.ts`
- Modify: `apps/web/src/api/client.test.ts`
- Create: `apps/web/src/users/types.ts`

- [ ] **Step 1: Regenerate OpenAPI and client**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-generate
make client-generate
```

Expected: `packages/api-contract/openapi.json` and `packages/api-client/src/schema.d.ts` include `/users`, `/roles/assignable`, and `/users/{userId}/role-bindings`.

- [ ] **Step 2: Write failing API wrapper test**

Append to `apps/web/src/api/client.test.ts`:

```ts
it("wraps user role management endpoints", async () => {
  const { listUsers, getUser, listAssignableRoles, createUserRoleBindings, deleteUserRoleBinding } = await import("./client");

  await listUsers({ search: "张敏", limit: 25 });
  expect(apiClient.GET).toHaveBeenCalledWith("/users", { params: { query: { search: "张敏", limit: 25 } } });

  await getUser("user_1");
  expect(apiClient.GET).toHaveBeenCalledWith("/users/{userId}", { params: { path: { userId: "user_1" } } });

  await listAssignableRoles();
  expect(apiClient.GET).toHaveBeenCalledWith("/roles/assignable");

  await createUserRoleBindings("user_1", { bindings: [{ departmentId: "dept_a", roleId: "__role_hrbp__" }] });
  expect(apiClient.POST).toHaveBeenCalledWith("/users/{userId}/role-bindings", {
    params: { path: { userId: "user_1" } },
    body: { bindings: [{ departmentId: "dept_a", roleId: "__role_hrbp__" }] },
  });

  await deleteUserRoleBinding("user_1", "udr_1");
  expect(apiClient.DELETE).toHaveBeenCalledWith("/users/{userId}/role-bindings/{bindingId}", {
    params: { path: { userId: "user_1", bindingId: "udr_1" } },
  });
});
```

- [ ] **Step 3: Run wrapper test and verify RED**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts
```

Expected: FAIL because wrapper functions do not exist.

- [ ] **Step 4: Add client wrappers**

Modify `apps/web/src/api/client.ts`:

```ts
export function listUsers(query: { search?: string; limit?: number; cursor?: string } = {}) {
  return apiClient.GET("/users", { params: { query } });
}

export function getUser(userId: string) {
  return apiClient.GET("/users/{userId}", { params: { path: { userId } } });
}

export function listAssignableRoles() {
  return apiClient.GET("/roles/assignable");
}

export function createUserRoleBindings(userId: string, body: { bindings: Array<{ departmentId: string; roleId: string }> }) {
  return apiClient.POST("/users/{userId}/role-bindings", { params: { path: { userId } }, body });
}

export function deleteUserRoleBinding(userId: string, bindingId: string) {
  return apiClient.DELETE("/users/{userId}/role-bindings/{bindingId}", { params: { path: { userId, bindingId } } });
}
```

- [ ] **Step 5: Add frontend user types**

Create `apps/web/src/users/types.ts`:

```ts
export type DepartmentSummary = {
  id: string;
  name: string;
  system: boolean;
};

export type RoleSummary = {
  enabled: boolean;
  id: string;
  isSystem: boolean;
  label: string;
};

export type RoleBindingDetail = {
  canDelete: boolean;
  createdAt: string;
  createdBy: string;
  department: DepartmentSummary;
  guest: boolean;
  id: string;
  role: RoleSummary;
};

export type UserListItem = {
  canAssign: boolean;
  departments: DepartmentSummary[];
  employeeId: string;
  guestOnly: boolean;
  id: string;
  name: string;
  roleBindings: RoleBindingDetail[];
  roleSummary: string;
};

export type UserListResponse = {
  canAssignRoles: boolean;
  dataScopeSummary: string;
  items: UserListItem[];
  nextCursor: string;
};

export type AssignableRole = {
  attributeConditionSummary: string;
  description: string;
  id: string;
  isSystem: boolean;
  label: string;
  supportsSystemDepartment: boolean;
};

export type AssignableRoleListResponse = {
  items: AssignableRole[];
};

export type UsersSession = {
  dataScope: {
    allDepartments: boolean;
    channels: string[];
    departments: Array<{ id: string; name: string }>;
  };
  permissions: string[];
};
```

- [ ] **Step 6: Run wrapper test and drift checks**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-check
make client-check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add packages/api-contract/openapi.json packages/api-client/src/schema.d.ts apps/web/src/api/client.ts apps/web/src/api/client.test.ts apps/web/src/users/types.ts
git commit -m "feat(web): add user management client wrappers"
```

## Task 6: Users Page List and Read-Only Mode

**Files:**
- Create: `apps/web/src/users/UsersPage.tsx`
- Create: `apps/web/src/users/UsersPage.test.tsx`
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing users page list test**

Create `UsersPage.test.tsx`:

```tsx
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { UsersPage } from "./UsersPage";

const apiMocks = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getUser: vi.fn(),
  listAssignableRoles: vi.fn(),
  listDepartments: vi.fn(),
  createUserRoleBindings: vi.fn(),
  deleteUserRoleBinding: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const session = {
  dataScope: { allDepartments: false, channels: ["social"], departments: [{ id: "dept_a", name: "算力训练平台部" }] },
  permissions: ["User.List", "UserDepartmentRole.List"],
};

describe("UsersPage", () => {
  beforeEach(() => {
    apiMocks.listUsers.mockResolvedValue({
      data: {
        dataScopeSummary: "当前数据权限:算力训练平台部",
        canAssignRoles: false,
        nextCursor: "",
        items: [{
          id: "user_a",
          employeeId: "A10001",
          name: "张敏",
          departments: [{ id: "dept_a", name: "算力训练平台部", system: false }],
          roleSummary: "HRBP(部门:算力训练平台部)",
          guestOnly: false,
          canAssign: false,
          roleBindings: [{ id: "udr_a", role: { id: "__role_hrbp__", label: "HRBP", isSystem: true, enabled: true }, department: { id: "dept_a", name: "算力训练平台部", system: false }, guest: false, createdAt: "2026-07-12T08:00:00Z", createdBy: "admin", canDelete: false }],
        }],
      },
      error: undefined,
    });
  });

  afterEach(() => cleanup());

  it("renders scoped users in read-only mode", async () => {
    render(<UsersPage session={session} />);

    expect(await screen.findByRole("heading", { name: "用户管理" })).toBeInTheDocument();
    expect(screen.getByText("当前数据权限:算力训练平台部")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "姓名" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "工号" })).toBeInTheDocument();
    const row = await screen.findByRole("row", { name: /张敏/ });
    expect(within(row).getByText("A10001")).toBeInTheDocument();
    expect(within(row).getByText("HRBP(部门:算力训练平台部)")).toBeInTheDocument();
    expect(screen.getByText("只读模式")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "分配角色" })).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run page test and verify RED**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/users/UsersPage.test.tsx
```

Expected: FAIL because `UsersPage` does not exist.

- [ ] **Step 3: Add i18n messages**

Add `users` messages to both locale files. Chinese messages:

```ts
users: {
  title: "用户管理",
  subtitle: "查看 W3 已登录用户，并维护用户在部门中的角色绑定。",
  dataScopeLabel: "当前数据权限:",
  readOnly: "只读模式",
  searchLabel: "搜索",
  searchPlaceholder: "搜索姓名、工号、部门或角色",
  columns: { name: "姓名", employeeId: "工号", roles: "当前角色集合", departments: "所属部门", operations: "操作" },
  actions: { assign: "分配角色", save: "保存", cancel: "取消", remove: "移除", addBinding: "添加另一绑定" },
  empty: { users: "暂无可见用户", value: "—", guest: "游客" },
  errors: { list: "用户列表加载失败，请稍后重试。", generic: "操作失败，请稍后重试。" },
}
```

- [ ] **Step 4: Implement minimal `UsersPage` list**

Create `UsersPage.tsx`:

```tsx
import * as React from "react";
import { listUsers } from "../api/client";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { zhCN } from "../i18n/zh-CN";
import type { UserListItem, UserListResponse, UsersSession } from "./types";

const text = zhCN.users;

export function UsersPage({ session }: { session: UsersSession }) {
  const [search, setSearch] = React.useState("");
  const [list, setList] = React.useState<UserListResponse | null>(null);
  const [errorMessage, setErrorMessage] = React.useState("");

  const loadUsers = React.useCallback(async (isCurrent: () => boolean = () => true) => {
    try {
      const { data, error } = await listUsers({ search: search.trim() || undefined });
      if (!isCurrent()) return;
      if (error || !data) {
        setErrorMessage(text.errors.list);
        return;
      }
      setList(data as UserListResponse);
      setErrorMessage("");
    } catch {
      if (isCurrent()) setErrorMessage(text.errors.list);
    }
  }, [search]);

  React.useEffect(() => {
    let isCurrent = true;
    void loadUsers(() => isCurrent);
    return () => { isCurrent = false; };
  }, [loadUsers]);

  const rows = list?.items ?? [];
  const canAssign = Boolean(list?.canAssignRoles && session.permissions.includes("UserDepartmentRole.Create"));

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="grid gap-2">
          <h1 className="text-2xl font-semibold tracking-normal">{text.title}</h1>
          <p className="text-sm text-muted">{text.subtitle}</p>
        </div>
        <div className="border border-accent/40 bg-accent/10 px-3 py-2 text-sm">{list?.dataScopeSummary}</div>
      </div>
      {!canAssign ? <p className="border border-white/10 bg-white/[0.03] px-3 py-2 text-sm text-muted">{text.readOnly}</p> : null}
      {errorMessage ? <p role="alert" className="border border-red-400/30 bg-red-400/10 px-3 py-2 text-sm text-red-200">{errorMessage}</p> : null}
      <Field className="max-w-md" label={text.searchLabel}>
        <Input onChange={(event) => setSearch(event.target.value)} placeholder={text.searchPlaceholder} type="search" value={search} />
      </Field>
      <UserTable canAssign={canAssign} rows={rows} />
    </div>
  );
}
```

Add table helpers:

```tsx
function UserTable({ canAssign, rows }: { canAssign: boolean; rows: UserListItem[] }) {
  return (
    <div className="overflow-x-auto border border-white/10">
      <table className="w-full min-w-[780px] border-collapse text-left text-sm">
        <thead className="bg-white/[0.04] text-muted">
          <tr>
            {Object.values(text.columns).map((label) => (
              <th className="border-b border-white/10 px-3 py-3 font-medium" key={label} scope="col">{label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length > 0 ? rows.map((user) => (
            <tr className="border-b border-white/10 last:border-0" key={user.id}>
              <td className="px-3 py-3 font-medium">{user.name}</td>
              <td className="px-3 py-3">{user.employeeId}</td>
              <td className="px-3 py-3">{user.roleSummary || text.empty.guest}</td>
              <td className="px-3 py-3">{user.departments.map((department) => department.name).join("、") || text.empty.value}</td>
              <td className="px-3 py-3">{canAssign ? <Button type="button">{text.actions.assign}</Button> : <span className="text-muted">{text.readOnly}</span>}</td>
            </tr>
          )) : (
            <tr><td className="px-3 py-8 text-center text-muted" colSpan={5}>{text.empty.users}</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 5: Wire App route**

Modify `App.tsx`:

```tsx
import { UsersPage } from "../users/UsersPage";

{activePage === "users" ? (
  <UsersPage session={session} />
) : activePage === "resume-parse" ? (
  <ResumeParsePage session={session} />
) : activePage === "resume-recommend" ? (
  <ResumeRecommendPage session={session} />
) : activePage === "resume-library" ? (
  <ResumeLibraryPage session={session} />
) : activePage === "departments-positions" ? (
  <DepartmentPositionPage session={session} />
) : (
  <h1 className="text-2xl font-semibold tracking-normal">{routeLabels[activePage] ?? text.nav.resumeParse}</h1>
)}
```

Add an `App.test.tsx` case:

```tsx
it("renders the users page when it is the active IAM route", async () => {
  apiMocks.getCurrentUser.mockResolvedValue({ data: { ...hrbpSession, defaultRoute: "/users", pageAccess: [...hrbpSession.pageAccess, "users"], permissions: [...hrbpSession.permissions, "User.List", "UserDepartmentRole.List"] }, error: undefined });
  apiMocks.listUsers.mockResolvedValue({ data: { items: [], nextCursor: "", dataScopeSummary: "当前数据权限:算力训练平台部", canAssignRoles: false }, error: undefined });

  render(<App />);

  expect(await screen.findByRole("heading", { name: "用户管理" })).toBeInTheDocument();
});
```

- [ ] **Step 6: Run focused frontend tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/users/UsersPage.test.tsx src/app/App.test.tsx
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/users apps/web/src/app/App.tsx apps/web/src/app/App.test.tsx apps/web/src/i18n/zh-CN.ts apps/web/src/i18n/en-US.ts
git commit -m "feat(web): render user management list"
```

## Task 7: Role Binding Modal, Batch Create, and Delete

**Files:**
- Modify: `apps/web/src/users/UsersPage.tsx`
- Modify: `apps/web/src/users/UsersPage.test.tsx`

- [ ] **Step 1: Write failing assignment workflow test**

Append:

```tsx
it("adds multiple pending role bindings and saves them", async () => {
  apiMocks.listUsers.mockResolvedValue({
    data: {
      dataScopeSummary: "当前数据权限:全部部门",
      canAssignRoles: true,
      nextCursor: "",
      items: [{ id: "user_a", employeeId: "A10001", name: "张敏", departments: [], roleSummary: "游客", guestOnly: true, canAssign: true, roleBindings: [{ id: "udr_guest", role: { id: "__role_guest__", label: "游客", isSystem: true, enabled: true }, department: { id: "__system__", name: "system", system: true }, guest: true, createdAt: "", createdBy: "system", canDelete: false }] }],
    },
    error: undefined,
  });
  apiMocks.listAssignableRoles.mockResolvedValue({ data: { items: [{ id: "__role_hrbp__", label: "HRBP", description: "", isSystem: true, supportsSystemDepartment: false, attributeConditionSummary: "" }] }, error: undefined });
  apiMocks.listDepartments.mockResolvedValue({ data: { items: [{ id: "dept_a", name: "算力训练平台部" }] }, error: undefined });
  apiMocks.createUserRoleBindings.mockResolvedValue({ data: { message: "已为 张敏 分配 1 个角色绑定", created: [], user: { id: "user_a", employeeId: "A10001", name: "张敏" } }, error: undefined });
  const user = userEvent.setup();

  render(<UsersPage session={{ ...session, permissions: [...session.permissions, "UserDepartmentRole.Create"] }} />);

  await user.click(await screen.findByRole("button", { name: "分配角色" }));
  await user.selectOptions(await screen.findByLabelText("角色"), "__role_hrbp__");
  await user.selectOptions(screen.getByLabelText("部门"), "dept_a");
  await user.click(screen.getByRole("button", { name: "添加另一绑定" }));
  await user.click(screen.getByRole("button", { name: "保存" }));

  expect(apiMocks.createUserRoleBindings).toHaveBeenCalledWith("user_a", { bindings: [{ roleId: "__role_hrbp__", departmentId: "dept_a" }] });
  expect(await screen.findByRole("status")).toHaveTextContent("已为 张敏 分配 1 个角色绑定");
});
```

- [ ] **Step 2: Write failing protected delete test**

Append:

```tsx
it("does not show remove action for guest bindings", async () => {
  apiMocks.listUsers.mockResolvedValue({
    data: { dataScopeSummary: "当前数据权限:全部部门", canAssignRoles: true, nextCursor: "", items: [{ id: "user_a", employeeId: "A10001", name: "张敏", departments: [], roleSummary: "游客", guestOnly: true, canAssign: true, roleBindings: [{ id: "udr_guest", role: { id: "__role_guest__", label: "游客", isSystem: true, enabled: true }, department: { id: "__system__", name: "system", system: true }, guest: true, createdAt: "", createdBy: "system", canDelete: false }] }] },
    error: undefined,
  });
  apiMocks.listAssignableRoles.mockResolvedValue({ data: { items: [] }, error: undefined });
  apiMocks.listDepartments.mockResolvedValue({ data: { items: [] }, error: undefined });
  const user = userEvent.setup();

  render(<UsersPage session={{ ...session, permissions: [...session.permissions, "UserDepartmentRole.Create", "UserDepartmentRole.Delete"] }} />);
  await user.click(await screen.findByRole("button", { name: "分配角色" }));

  expect(screen.getByText("游客")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "移除" })).not.toBeInTheDocument();
});
```

- [ ] **Step 3: Run tests and verify RED**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/users/UsersPage.test.tsx
```

Expected: FAIL because the modal workflow is missing.

- [ ] **Step 4: Implement modal state and loading**

Modify `UsersPage.tsx` imports:

```tsx
import { createUserRoleBindings, deleteUserRoleBinding, listAssignableRoles, listDepartments, listUsers } from "../api/client";
import { Select } from "../components/ui/select";
import type { AssignableRole, DepartmentSummary, RoleBindingDetail, UserListItem } from "./types";
```

Add state:

```tsx
const [selectedUser, setSelectedUser] = React.useState<UserListItem | null>(null);
const [assignableRoles, setAssignableRoles] = React.useState<AssignableRole[]>([]);
const [departments, setDepartments] = React.useState<DepartmentSummary[]>([]);
const [pending, setPending] = React.useState<Array<{ roleId: string; departmentId: string }>>([]);
const [draftRoleId, setDraftRoleId] = React.useState("");
const [draftDepartmentId, setDraftDepartmentId] = React.useState("");
const [successMessage, setSuccessMessage] = React.useState("");
```

Add modal open loader:

```tsx
async function handleOpenAssign(user: UserListItem) {
  setSelectedUser(user);
  setPending([]);
  setDraftRoleId("");
  setDraftDepartmentId("");
  const [rolesResult, departmentsResult] = await Promise.all([listAssignableRoles(), listDepartments({})]);
  if (rolesResult.data) setAssignableRoles((rolesResult.data.items ?? []) as AssignableRole[]);
  if (departmentsResult.data) setDepartments((departmentsResult.data.items ?? []).map((item: { id: string; name: string }) => ({ id: item.id, name: item.name, system: item.id === "__system__" })));
}
```

- [ ] **Step 5: Implement add/save/delete handlers**

Add handlers:

```tsx
function handleAddPendingBinding() {
  if (!draftRoleId || !draftDepartmentId) return;
  if (pending.some((item) => item.roleId === draftRoleId && item.departmentId === draftDepartmentId)) {
    setErrorMessage(text.errors.duplicatePending);
    return;
  }
  setPending([...pending, { roleId: draftRoleId, departmentId: draftDepartmentId }]);
  setErrorMessage("");
}

async function handleSaveBindings() {
  if (!selectedUser || pending.length === 0) return;
  const { data, error } = await createUserRoleBindings(selectedUser.id, { bindings: pending });
  if (error || !data) {
    setErrorMessage(text.errors.generic);
    return;
  }
  setSuccessMessage(data.message);
  setSelectedUser(null);
  await loadUsers();
}

async function handleDeleteBinding(binding: RoleBindingDetail) {
  if (!selectedUser || binding.guest || !binding.canDelete) return;
  const { data, error } = await deleteUserRoleBinding(selectedUser.id, binding.id);
  if (error || !data) {
    setErrorMessage(text.errors.generic);
    return;
  }
  setSuccessMessage(data.message);
  setSelectedUser(null);
  await loadUsers();
}
```

- [ ] **Step 6: Render modal panel**

Add after the table:

```tsx
{selectedUser ? (
  <section aria-label="角色绑定" className="grid gap-4 border border-white/10 bg-white/[0.03] p-4">
    <div className="flex items-start justify-between gap-4">
      <div>
        <h2 className="text-xl font-semibold tracking-normal">角色绑定</h2>
        <p className="text-sm text-muted">{selectedUser.name} · {selectedUser.employeeId}</p>
      </div>
      <Button onClick={() => setSelectedUser(null)} type="button">{text.actions.cancel}</Button>
    </div>
    <div className="grid gap-3 md:grid-cols-3">
      <Field label="角色">
        <Select value={draftRoleId} onChange={(event) => setDraftRoleId(event.target.value)}>
          <option value="">请选择角色</option>
          {assignableRoles.map((role) => <option key={role.id} value={role.id}>{role.label}</option>)}
        </Select>
      </Field>
      <Field label="部门">
        <Select value={draftDepartmentId} onChange={(event) => setDraftDepartmentId(event.target.value)}>
          <option value="">请选择部门</option>
          {departments.map((department) => <option key={department.id} value={department.id}>{department.name}</option>)}
        </Select>
      </Field>
      <Button onClick={handleAddPendingBinding} type="button">{text.actions.addBinding}</Button>
    </div>
    <BindingList bindings={selectedUser.roleBindings} onDelete={handleDeleteBinding} />
    <PendingList items={pending} roles={assignableRoles} departments={departments} />
    <Button disabled={pending.length === 0} onClick={() => void handleSaveBindings()} type="button" variant="primary">{text.actions.save}</Button>
  </section>
) : null}
```

Add helper components:

```tsx
function BindingList({ bindings, onDelete }: { bindings: RoleBindingDetail[]; onDelete: (binding: RoleBindingDetail) => void }) {
  return (
    <div className="grid gap-2">
      {bindings.map((binding) => (
        <div className="flex flex-wrap items-center justify-between gap-3 border border-white/10 p-3" key={binding.id}>
          <span>{binding.role.label}(部门:{binding.department.name})</span>
          {binding.canDelete && !binding.guest ? <Button onClick={() => onDelete(binding)} type="button">{text.actions.remove}</Button> : <span className="text-muted">{text.empty.guest}</span>}
        </div>
      ))}
    </div>
  );
}

function PendingList({ items, roles, departments }: { items: Array<{ roleId: string; departmentId: string }>; roles: AssignableRole[]; departments: DepartmentSummary[] }) {
  if (items.length === 0) return null;
  return (
    <div className="grid gap-2">
      {items.map((item) => (
        <span className="border border-accent/30 bg-accent/10 px-2 py-1 text-sm" key={`${item.roleId}-${item.departmentId}`}>
          {roles.find((role) => role.id === item.roleId)?.label ?? item.roleId}(部门:{departments.find((department) => department.id === item.departmentId)?.name ?? item.departmentId})
        </span>
      ))}
    </div>
  );
}
```

- [ ] **Step 7: Run users page tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/users/UsersPage.test.tsx
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/users/UsersPage.tsx apps/web/src/users/UsersPage.test.tsx apps/web/src/i18n/zh-CN.ts apps/web/src/i18n/en-US.ts
git commit -m "feat(web): manage user role bindings"
```

## Task 8: Final Verification and Status Updates

**Files:**
- Modify: `docs/project-status.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Run backend verification**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make test-api
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-check
```

Expected: PASS.

- [ ] **Step 2: Run frontend verification**

Run:

```bash
make test-web
make client-check
make typecheck
```

Expected: PASS.

- [ ] **Step 3: Run full quality checks**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make lint
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make build
git diff --check
```

Expected: PASS.

- [ ] **Step 4: Update project status and agent guide**

Update `docs/project-status.md`:

- E6 status becomes `Done`;
- Evidence lists `docs/specs/007-user-role-management.md`, this implementation plan, and the verification commands that passed;
- E8 remains `Not Started` and explicitly owns role definition editing.

Update `AGENTS.md`:

- current phase moves to E7 notification center planning unless product priority changes;
- E6 implementation plan remains in the Documentation Index.

- [ ] **Step 5: Commit final docs**

```bash
git add docs/project-status.md AGENTS.md
git commit -m "docs: mark E6 implementation complete"
```

- [ ] **Step 6: Final status**

Run:

```bash
git status --short
```

Expected: only unrelated pre-existing files remain untracked or modified.
