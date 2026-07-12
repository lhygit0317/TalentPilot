package useradmin_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/useradmin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLStoreListUsersAppliesBindingScope(t *testing.T) {
	database := newUserAdminMigratedSQLiteGormDB(t)
	seedUserAdminFixture(t, database)
	store := useradmin.NewSQLStore(database)

	result, err := store.ListUsers(context.Background(), useradmin.ListUsersQuery{
		ListScope:   userDepartmentRoleScope(iam.ActionList, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}),
		DeleteScope: userDepartmentRoleScope(iam.ActionDelete, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}),
		CanAssign:   true,
	})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected two users in dept_a scope, got %#v", result.Items)
	}
	for _, item := range result.Items {
		if item.ID == "user_b" {
			t.Fatalf("expected dept_b user to be filtered out, got %#v", result.Items)
		}
	}
	if !result.Items[0].RoleBindings[0].CanDelete {
		t.Fatalf("expected binding canDelete from delete scope: %#v", result.Items[0].RoleBindings)
	}
	if !result.CanAssignRoles {
		t.Fatalf("expected canAssignRoles from query flag")
	}
}

func TestSQLStoreSearchesUserDepartmentAndRole(t *testing.T) {
	database := newUserAdminMigratedSQLiteGormDB(t)
	seedUserAdminFixture(t, database)
	store := useradmin.NewSQLStore(database)

	result, err := store.ListUsers(context.Background(), useradmin.ListUsersQuery{
		Search:    "HRBP",
		ListScope: userDepartmentRoleScope(iam.ActionList, iam.ScopeBranch{AllDepartments: true}),
	})
	if err != nil {
		t.Fatalf("search users: %v", err)
	}

	if len(result.Items) != 1 || result.Items[0].ID != "user_a" {
		t.Fatalf("expected role-label search to find user_a, got %#v", result.Items)
	}
}

func TestSQLStoreListAssignableRolesExcludesDisabledRoles(t *testing.T) {
	database := newUserAdminMigratedSQLiteGormDB(t)
	seedUserAdminFixture(t, database)
	store := useradmin.NewSQLStore(database)

	roles, err := store.ListAssignableRoles(context.Background())
	if err != nil {
		t.Fatalf("assignable roles: %v", err)
	}
	for _, role := range roles {
		if role.ID == "role_disabled" {
			t.Fatalf("disabled role should be excluded: %#v", roles)
		}
	}
	var sawSocial bool
	for _, role := range roles {
		if role.ID == iam.RoleSocialOwner && role.AttributeConditionSummary == "社招" {
			sawSocial = true
		}
	}
	if !sawSocial {
		t.Fatalf("expected social owner attribute summary, got %#v", roles)
	}
}

func TestSQLStoreCreateRoleBindingsRejectsExistingDuplicateAtomically(t *testing.T) {
	database := newUserAdminMigratedSQLiteGormDB(t)
	seedUserAdminFixture(t, database)
	service := useradmin.NewService(useradmin.NewSQLStore(database), &fakeIAMCache{}, audit.NopRecorder{})

	_, err := service.CreateRoleBindings(context.Background(), useradmin.CreateRoleBindingsInput{
		ActorUserID: "admin",
		UserID:      "user_a",
		CreateScope: userDepartmentRoleScope(iam.ActionCreate, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}, AllDepartments: false}),
		Bindings: []useradmin.RoleBindingRequest{
			{DepartmentID: "dept_a", RoleID: iam.RoleManager},
			{DepartmentID: "dept_a", RoleID: iam.RoleHRBP},
		},
	})
	if !errors.Is(err, useradmin.ErrDuplicateBinding) {
		t.Fatalf("expected duplicate binding error, got %v", err)
	}
	assertUserAdminBindingCount(t, database, "user_id = 'user_a' AND role_id = '__role_manager__'", 0)
}

func TestSQLStoreDeleteEnsuresGuestFallback(t *testing.T) {
	database := newUserAdminMigratedSQLiteGormDB(t)
	seedUserAdminFixture(t, database)
	service := useradmin.NewService(useradmin.NewSQLStore(database), &fakeIAMCache{}, audit.NopRecorder{})

	_, err := service.DeleteRoleBinding(context.Background(), useradmin.DeleteRoleBindingInput{
		ActorUserID: "admin",
		UserID:      "user_c",
		BindingID:   "udr_c",
		DeleteScope: userDepartmentRoleScope(iam.ActionDelete, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}),
	})
	if err != nil {
		t.Fatalf("delete binding: %v", err)
	}

	assertUserAdminBindingCount(t, database, "user_id = 'user_c' AND role_id = '__role_guest__' AND department_id = '__system__'", 1)
}

func TestSQLStoreRejectsSystemDepartmentForNonGlobalRole(t *testing.T) {
	database := newUserAdminMigratedSQLiteGormDB(t)
	seedUserAdminFixture(t, database)
	service := useradmin.NewService(useradmin.NewSQLStore(database), &fakeIAMCache{}, audit.NopRecorder{})

	_, err := service.CreateRoleBindings(context.Background(), useradmin.CreateRoleBindingsInput{
		ActorUserID: "admin",
		UserID:      "user_a",
		CreateScope: userDepartmentRoleScope(iam.ActionCreate, iam.ScopeBranch{AllDepartments: true}),
		Bindings: []useradmin.RoleBindingRequest{
			{DepartmentID: iam.SystemDepartmentID, RoleID: iam.RoleHRBP},
		},
	})
	if !errors.Is(err, iam.ErrScopeUnsupported) {
		t.Fatalf("expected scope unsupported, got %v", err)
	}
}

func userDepartmentRoleScope(action iam.Action, branch iam.ScopeBranch) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourceUserDepartmentRole,
		Action:   action,
		Branches: []iam.ScopeBranch{branch},
	}
}

func newUserAdminMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite gorm: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sql db: %v", err)
		}
	})

	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, os.DirFS(filepath.Join("..", "..", "migrations")))
	if err != nil {
		t.Fatalf("new migration provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	return gdb
}

func seedUserAdminFixture(t *testing.T, database *gorm.DB) {
	t.Helper()

	execUserAdminSQL(t, database, `
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES
			('dept_a', '算力训练平台部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('dept_b', '智算调度部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execUserAdminSQL(t, database, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES
			('admin', 'A00000', '管理员', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('user_a', 'A10001', '张敏', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('user_b', 'A10002', '李四', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('user_c', 'A10003', '王五', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execUserAdminSQL(t, database, `
		INSERT INTO roles (id, label, description, is_system, enabled, created_at, created_by, updated_at)
		VALUES ('role_disabled', '停用角色', '停用', FALSE, FALSE, CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP)
	`)
	execUserAdminSQL(t, database, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES
			('udr_a', 'user_a', 'dept_a', '__role_hrbp__', CURRENT_TIMESTAMP, 'admin'),
			('udr_b', 'user_b', 'dept_b', '__role_manager__', CURRENT_TIMESTAMP, 'admin'),
			('udr_c', 'user_c', 'dept_a', '__role_manager__', CURRENT_TIMESTAMP, 'admin')
	`)
}

func execUserAdminSQL(t *testing.T, database *gorm.DB, statement string) {
	t.Helper()
	if err := database.Exec(statement).Error; err != nil {
		t.Fatalf("exec fixture sql: %v\n%s", err, statement)
	}
}

func assertUserAdminBindingCount(t *testing.T, database *gorm.DB, where string, expected int64) {
	t.Helper()
	var count int64
	if err := database.Table("user_department_roles").Where(where).Count(&count).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if count != expected {
		t.Fatalf("expected binding count %d for %s, got %d", expected, where, count)
	}
}
