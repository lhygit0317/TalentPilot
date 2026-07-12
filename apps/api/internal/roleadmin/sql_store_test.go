package roleadmin_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/roleadmin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLStoreListRolesReturnsCountsAndCapabilities(t *testing.T) {
	db := newRoleAdminMigratedSQLiteGormDB(t)
	seedRoleAdminFixture(t, db)
	store := roleadmin.NewSQLStore(db)

	result, err := store.ListRoles(context.Background(), roleadmin.RoleListQuery{
		ActorCanCreate: true,
		ActorCanEdit:   true,
		ActorCanDelete: true,
		ActorCanToggle: true,
	})
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}

	var item roleadmin.RoleListItem
	for _, candidate := range result.Items {
		if candidate.ID == "role_custom_reviewer" {
			item = candidate
			break
		}
	}
	if item.ID == "" {
		t.Fatalf("expected custom reviewer role in list, got %#v", result.Items)
	}
	if item.PermissionCount != 2 || item.ChildRoleCount != 1 || item.ReferenceCount != 0 {
		t.Fatalf("unexpected counts: %#v", item)
	}
	if !item.CanEdit || !item.CanDelete || !item.CanToggleEnabled {
		t.Fatalf("expected editable unused custom role: %#v", item)
	}
	if item.ConditionSummary != "社招" {
		t.Fatalf("expected social condition summary, got %q", item.ConditionSummary)
	}

	var systemRole roleadmin.RoleListItem
	for _, candidate := range result.Items {
		if candidate.ID == iam.RoleHRBP {
			systemRole = candidate
			break
		}
	}
	if systemRole.ID == "" || systemRole.CanDelete {
		t.Fatalf("expected system role to be present but not deletable, got %#v", systemRole)
	}
}

func TestSQLStoreGetRoleReturnsDirectDefinition(t *testing.T) {
	db := newRoleAdminMigratedSQLiteGormDB(t)
	seedRoleAdminFixture(t, db)
	store := roleadmin.NewSQLStore(db)

	detail, err := store.GetRole(context.Background(), "role_custom_reviewer", roleadmin.RoleCapabilityQuery{
		ActorCanEdit:   true,
		ActorCanDelete: true,
		ActorCanToggle: true,
	})
	if err != nil {
		t.Fatalf("get role: %v", err)
	}

	if detail.Label != "高级评审者" || detail.IsSystem || !detail.Enabled {
		t.Fatalf("unexpected role metadata: %#v", detail)
	}
	if detail.ReferenceCount != 0 || !detail.CanDelete {
		t.Fatalf("expected unused custom role to be deletable: %#v", detail)
	}
	if !slices.Contains(detail.ChildRoleIDs, iam.RoleTrainee) {
		t.Fatalf("expected direct trainee child role, got %#v", detail.ChildRoleIDs)
	}
	if len(detail.Permissions) != 2 {
		t.Fatalf("expected two direct permissions, got %#v", detail.Permissions)
	}
	if !hasRolePermission(detail.Permissions, iam.ResourceResume, iam.ActionList, []string{"social"}) {
		t.Fatalf("expected direct social Resume.List permission, got %#v", detail.Permissions)
	}
}

func TestSQLStorePermissionOptionsComeFromIAMWhitelist(t *testing.T) {
	db := newRoleAdminMigratedSQLiteGormDB(t)
	store := roleadmin.NewSQLStore(db)

	options, err := store.PermissionOptions(context.Background())
	if err != nil {
		t.Fatalf("permission options: %v", err)
	}

	resumeList, ok := findPermissionOption(options, iam.ResourceResume, iam.ActionList)
	if !ok {
		t.Fatalf("expected Resume.List option, got %#v", options.Resources)
	}
	if !resumeList.SupportsConditions.Channels || !resumeList.SupportsConditions.Expired || resumeList.SupportsConditions.Self {
		t.Fatalf("unexpected Resume.List condition support: %#v", resumeList.SupportsConditions)
	}

	userGet, ok := findPermissionOption(options, iam.ResourceUser, iam.ActionGet)
	if !ok {
		t.Fatalf("expected User.Get option, got %#v", options.Resources)
	}
	if !userGet.SupportsConditions.Self || userGet.SupportsConditions.Channels || userGet.SupportsConditions.Expired {
		t.Fatalf("unexpected User.Get condition support: %#v", userGet.SupportsConditions)
	}
	if !slices.Contains(options.ConditionOptions.Channels, "social") || !slices.Contains(options.ConditionOptions.Channels, "campus") {
		t.Fatalf("expected channel options, got %#v", options.ConditionOptions)
	}
}

func hasRolePermission(permissions []roleadmin.PermissionInput, resource iam.Resource, action iam.Action, channels []string) bool {
	for _, permission := range permissions {
		if permission.Resource != resource || permission.Action != action {
			continue
		}
		if slices.Equal(permission.AttributeConditions.Channels, channels) {
			return true
		}
	}
	return false
}

func findPermissionOption(options roleadmin.PermissionOptionsResult, resource iam.Resource, action iam.Action) (roleadmin.PermissionActionOption, bool) {
	for _, group := range options.Resources {
		if group.Resource != resource {
			continue
		}
		for _, candidate := range group.Actions {
			if candidate.Action == action {
				return candidate, true
			}
		}
	}
	return roleadmin.PermissionActionOption{}, false
}

func newRoleAdminMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
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

func seedRoleAdminFixture(t *testing.T, db *gorm.DB) {
	t.Helper()

	execRoleAdminSQL(t, db, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES ('user_a', 'A10001', '张敏', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execRoleAdminSQL(t, db, `
		INSERT INTO roles (id, label, description, is_system, enabled, created_at, created_by, updated_at)
		VALUES ('role_custom_reviewer', '高级评审者', '可查看社招简历', FALSE, TRUE, CURRENT_TIMESTAMP, 'user_a', CURRENT_TIMESTAMP)
	`)
	execRoleAdminSQL(t, db, `
		INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
		VALUES
			('perm_custom_user_list', 'role_custom_reviewer', 'User', 'List', '{}', CURRENT_TIMESTAMP),
			('perm_custom_resume_list', 'role_custom_reviewer', 'Resume', 'List', '{"chan":["social"]}', CURRENT_TIMESTAMP)
	`)
	execRoleAdminSQL(t, db, `
		INSERT INTO role_relations (id, parent_role_id, child_role_id, created_at)
		VALUES ('rel_custom_trainee', 'role_custom_reviewer', '__role_trainee__', CURRENT_TIMESTAMP)
	`)
	execRoleAdminSQL(t, db, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES ('udr_hrbp', 'user_a', '__system__', '__role_hrbp__', CURRENT_TIMESTAMP, 'system')
	`)
}

func execRoleAdminSQL(t *testing.T, db *gorm.DB, query string) {
	t.Helper()
	if err := db.Exec(query).Error; err != nil {
		t.Fatalf("exec fixture sql: %v\n%s", err, query)
	}
}
