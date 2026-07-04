package organization_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/organization"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListDepartmentsAppliesScopeAndExcludesSystem(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	store := organization.NewSQLStore(database)

	result, err := store.ListDepartments(context.Background(), organization.DepartmentListQuery{
		Scope:       departmentScope(iam.ActionList, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}),
		GetScope:    departmentScope(iam.ActionGet, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}),
		UpdateScope: departmentScope(iam.ActionUpdate, iam.ScopeBranch{}),
		DeleteScope: departmentScope(iam.ActionDelete, iam.ScopeBranch{}),
	})
	if err != nil {
		t.Fatalf("list departments: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected one scoped department, got %#v", result.Items)
	}
	item := result.Items[0]
	if item.ID != "dept_a" || item.Name != "算力训练平台部" {
		t.Fatalf("unexpected department item: %#v", item)
	}
	if item.PositionCount != 1 || item.ResumeCount != 1 {
		t.Fatalf("expected relation counts, got positions=%d resumes=%d", item.PositionCount, item.ResumeCount)
	}
	if !item.CanGet || item.CanUpdate || item.CanDelete {
		t.Fatalf("unexpected capabilities: %#v", item)
	}
}

func TestGetDepartmentDeniesOutOfScope(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	store := organization.NewSQLStore(database)

	_, err := store.GetDepartment(context.Background(), "dept_b", departmentScope(iam.ActionGet, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}))
	if !errors.Is(err, organization.ErrDepartmentNotFound) {
		t.Fatalf("expected ErrDepartmentNotFound, got %v", err)
	}
}

func TestCreateDepartmentRejectsEmptyAndDuplicateName(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	service := organization.NewService(organization.NewSQLStore(database), audit.NopRecorder{})

	_, err := service.CreateDepartment(context.Background(), organization.DepartmentInput{ActorUserID: "user_admin", Name: " "})
	if !errors.Is(err, organization.ErrDepartmentNameRequired) {
		t.Fatalf("expected name required, got %v", err)
	}

	_, err = service.CreateDepartment(context.Background(), organization.DepartmentInput{ActorUserID: "user_admin", Name: "算力训练平台部"})
	if !errors.Is(err, organization.ErrDepartmentNameDuplicate) {
		t.Fatalf("expected duplicate name, got %v", err)
	}
}

func TestDeleteDepartmentRejectsRelations(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	service := organization.NewService(organization.NewSQLStore(database), audit.NopRecorder{})

	err := service.DeleteDepartment(context.Background(), "dept_a", departmentScope(iam.ActionDelete, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}), "user_admin")
	if !errors.Is(err, organization.ErrDepartmentDeleteHasRelations) {
		t.Fatalf("expected relation protection, got %v", err)
	}
}

func TestUpdateDepartmentRejectsDuplicateAndSystemDepartment(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	service := organization.NewService(organization.NewSQLStore(database), audit.NopRecorder{})

	scope := departmentScope(iam.ActionUpdate, iam.ScopeBranch{AllDepartments: true})
	_, err := service.UpdateDepartment(context.Background(), "dept_b", organization.DepartmentInput{ActorUserID: "user_admin", Name: "算力训练平台部"}, scope)
	if !errors.Is(err, organization.ErrDepartmentNameDuplicate) {
		t.Fatalf("expected duplicate name, got %v", err)
	}

	_, err = service.UpdateDepartment(context.Background(), iam.SystemDepartmentID, organization.DepartmentInput{ActorUserID: "user_admin", Name: "系统部门"}, scope)
	if !errors.Is(err, organization.ErrDepartmentSystemProtected) {
		t.Fatalf("expected system department protection, got %v", err)
	}
}

func TestDepartmentWritesRecordAudit(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	recorder := &recordingAudit{}
	service := organization.NewService(organization.NewSQLStore(database), recorder)

	created, err := service.CreateDepartment(context.Background(), organization.DepartmentInput{ActorUserID: "user_admin", Name: "硬件加速部"})
	if err != nil {
		t.Fatalf("create department: %v", err)
	}

	if len(recorder.events) != 1 {
		t.Fatalf("expected one audit event, got %#v", recorder.events)
	}
	event := recorder.events[0]
	if event.Type != audit.EventDepartmentCreated || event.Resource != string(iam.ResourceDepartment) || event.TargetID != created.ID {
		t.Fatalf("unexpected audit event: %#v", event)
	}
	if event.After["name"] != "硬件加速部" {
		t.Fatalf("expected safe department name in audit event, got %#v", event.After)
	}
}

type recordingAudit struct {
	events []audit.Event
}

func (r *recordingAudit) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func departmentScope(action iam.Action, branch iam.ScopeBranch) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourceDepartment,
		Action:   action,
		Branches: []iam.ScopeBranch{branch},
	}
}

func newOrganizationMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
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

func seedOrganizationDepartmentFixture(t *testing.T, database *gorm.DB) {
	t.Helper()

	execOrganizationSQL(t, database, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES ('user_admin', 'E100', '管理员', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execOrganizationSQL(t, database, `
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES
			('dept_a', '算力训练平台部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('dept_b', '智算调度部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execOrganizationSQL(t, database, `
		INSERT INTO positions (id, name, chan, level, status, duties, must, keywords, implicit_tags, created_at, updated_at)
		VALUES ('position_a', '平台工程师', 'social', 'P6', 'on', '[]', '[]', '["Go"]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execOrganizationSQL(t, database, `
		INSERT INTO department_positions (id, department_id, position_id, created_at)
		VALUES ('dp_a', 'dept_a', 'position_a', CURRENT_TIMESTAMP)
	`)
	execOrganizationSQL(t, database, `
		INSERT INTO resumes (id, normalized_name, name, source, chan, keywords, profile, created_at, updated_at)
		VALUES ('resume_a', 'zhangsan', '张三', '导入', 'social', '[]', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execOrganizationSQL(t, database, `
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES ('dr_a', 'dept_a', 'resume_a', CURRENT_TIMESTAMP, 'user_admin')
	`)
}

func execOrganizationSQL(t *testing.T, database *gorm.DB, statement string) {
	t.Helper()
	if err := database.Exec(statement).Error; err != nil {
		t.Fatalf("exec fixture sql: %v\n%s", err, statement)
	}
}
