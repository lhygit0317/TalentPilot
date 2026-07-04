package iam_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLStoreLoadsPrincipalSnapshot(t *testing.T) {
	database := newIAMMigratedSQLiteGormDB(t)
	seedIAMUserWithBinding(t, database, "u_hrd", "dept_a", iam.SystemDepartmentID, iam.RoleHRD)
	store := iam.NewSQLStore(database)

	snapshot, err := store.LoadSnapshot(context.Background(), "u_hrd")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}

	if snapshot.User.ID != "u_hrd" || snapshot.User.EmployeeID != "u_hrd_employee" {
		t.Fatalf("unexpected user: %#v", snapshot.User)
	}
	if len(snapshot.RoleBindings) != 1 || snapshot.RoleBindings[0].RoleID != iam.RoleHRD {
		t.Fatalf("expected HRD binding, got %#v", snapshot.RoleBindings)
	}
	if len(snapshot.Roles) < len(iam.PresetRoles()) || len(snapshot.Permissions) == 0 || len(snapshot.RoleRelations) == 0 {
		t.Fatalf("expected seeded IAM rows, roles=%d permissions=%d relations=%d", len(snapshot.Roles), len(snapshot.Permissions), len(snapshot.RoleRelations))
	}
}

func TestServiceRoleSummaryIncludesPermissionsAndDataScope(t *testing.T) {
	database := newIAMMigratedSQLiteGormDB(t)
	seedIAMUserWithBinding(t, database, "u_hrd", "dept_a", iam.SystemDepartmentID, iam.RoleHRD)
	service := iam.NewService(iam.NewSQLStore(database))

	summary, err := service.RoleSummary(context.Background(), "u_hrd")
	if err != nil {
		t.Fatalf("role summary: %v", err)
	}

	if !slices.Contains(summary.Permissions, "Resume.List") {
		t.Fatalf("expected inherited Resume.List, got %#v", summary.Permissions)
	}
	if !summary.DataScope.AllDepartments {
		t.Fatalf("expected HRD system binding to have all department scope: %#v", summary.DataScope)
	}
	for _, page := range []string{"resume-parse", "users"} {
		if !slices.Contains(summary.PageAccess, page) {
			t.Fatalf("expected page %s in %#v", page, summary.PageAccess)
		}
	}
}

func TestServiceInvalidatesCacheForPermissionAncestorClosure(t *testing.T) {
	cache := &spyPrincipalCache{}
	store := &fakeClosureStore{users: []string{"u_direct", "u_ancestor"}}
	service := iam.NewService(store, iam.WithCache(cache))

	if err := service.InvalidateRoleClosure(context.Background(), []string{iam.RoleHRBP}); err != nil {
		t.Fatalf("invalidate role closure: %v", err)
	}

	if !slices.Equal(cache.deleted, []string{"u_ancestor", "u_direct"}) {
		t.Fatalf("expected direct and ancestor users invalidated, got %#v", cache.deleted)
	}
	if cache.cleared {
		t.Fatalf("did not expect clear when closure is known")
	}
}

func TestServiceInvalidatesCacheForRoleRelationAncestorClosure(t *testing.T) {
	cache := &spyPrincipalCache{}
	store := &fakeClosureStore{err: errors.New("unsafe closure")}
	service := iam.NewService(store, iam.WithCache(cache))

	if err := service.InvalidateRoleClosure(context.Background(), []string{iam.RoleHRD, iam.RoleHRBP}); err != nil {
		t.Fatalf("invalidate role closure with unsafe closure: %v", err)
	}

	if !cache.cleared {
		t.Fatalf("expected unsafe closure to clear entire cache")
	}
}

type fakeClosureStore struct {
	users []string
	err   error
}

func (f *fakeClosureStore) LoadSnapshot(context.Context, string) (iam.Snapshot, error) {
	return iam.Snapshot{}, nil
}

func (f *fakeClosureStore) UsersForRoleClosure(context.Context, []string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.users, nil
}

type spyPrincipalCache struct {
	deleted []string
	cleared bool
}

func (s *spyPrincipalCache) Get(string) (iam.Principal, bool) {
	return iam.Principal{}, false
}

func (s *spyPrincipalCache) Set(string, iam.Principal) {}

func (s *spyPrincipalCache) Delete(userID string) {
	s.deleted = append(s.deleted, userID)
	slices.Sort(s.deleted)
}

func (s *spyPrincipalCache) Clear() {
	s.cleared = true
}

func newIAMMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared&_foreign_keys=on"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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

func seedIAMUserWithBinding(t *testing.T, database *gorm.DB, userID string, concreteDepartmentID string, bindingDepartmentID string, roleID string) {
	t.Helper()

	if err := database.Exec(`
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, userID+"_employee", userID+"_name").Error; err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := database.Exec(`
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, concreteDepartmentID, concreteDepartmentID+"_name").Error; err != nil {
		t.Fatalf("insert department: %v", err)
	}
	if err := database.Exec(`
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, 'test')
	`, "bind_"+userID+"_"+roleID, userID, bindingDepartmentID, roleID).Error; err != nil {
		t.Fatalf("insert user department role: %v", err)
	}
}
