package organization_test

import (
	"context"
	"errors"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/organization"
	"gorm.io/gorm"
)

func TestListPositionsAppliesDepartmentScopeAndExcludesOrphans(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	seedOrganizationPositionFixture(t, database)
	store := organization.NewSQLStore(database)

	result, err := store.ListPositions(context.Background(), organization.PositionListQuery{
		Scope:       positionScope(iam.ActionList, iam.ScopeBranch{AllDepartments: true}),
		GetScope:    positionScope(iam.ActionGet, iam.ScopeBranch{AllDepartments: true}),
		UpdateScope: positionScope(iam.ActionUpdate, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}),
		DeleteScope: positionScope(iam.ActionDelete, iam.ScopeBranch{}),
	})
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}

	byID := map[string]organization.PositionListItem{}
	for _, item := range result.Items {
		byID[item.ID] = item
	}
	if len(byID) != 2 {
		t.Fatalf("expected related positions only, got %#v", result.Items)
	}
	if _, ok := byID["position_orphan"]; ok {
		t.Fatalf("orphan position should be excluded: %#v", result.Items)
	}
	item := byID["position_a"]
	if item.Department.ID != "dept_a" || item.Department.Name != "算力训练平台部" {
		t.Fatalf("unexpected department summary: %#v", item.Department)
	}
	if item.KeywordCount != 1 || item.ImplicitTagCount != 0 {
		t.Fatalf("unexpected counts: %#v", item)
	}
	if !item.CanGet || !item.CanUpdate || item.CanDelete {
		t.Fatalf("unexpected capabilities: %#v", item)
	}

	scoped, err := store.ListPositions(context.Background(), organization.PositionListQuery{
		Scope: positionScope(iam.ActionList, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}),
	})
	if err != nil {
		t.Fatalf("list scoped positions: %v", err)
	}
	if len(scoped.Items) != 1 || scoped.Items[0].ID != "position_a" {
		t.Fatalf("expected only dept_a position, got %#v", scoped.Items)
	}
}

func TestCreatePositionValidatesDuplicatesAndCreatesRelation(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	service := organization.NewService(organization.NewSQLStore(database), audit.NopRecorder{})

	input := validPositionInput("新平台工程师", "dept_b")
	input.Name = " "
	_, err := service.CreatePosition(context.Background(), input)
	if !errors.Is(err, organization.ErrPositionNameRequired) {
		t.Fatalf("expected name required, got %v", err)
	}

	input = validPositionInput("新平台工程师", "")
	_, err = service.CreatePosition(context.Background(), input)
	if !errors.Is(err, organization.ErrPositionDepartmentRequired) {
		t.Fatalf("expected department required, got %v", err)
	}

	input = validPositionInput("新平台工程师", "missing")
	_, err = service.CreatePosition(context.Background(), input)
	if !errors.Is(err, organization.ErrPositionDepartmentInvalid) {
		t.Fatalf("expected department invalid, got %v", err)
	}

	input = validPositionInput("新平台工程师", "dept_b")
	input.Keywords = []string{"Go", " go "}
	_, err = service.CreatePosition(context.Background(), input)
	if !errors.Is(err, organization.ErrPositionDuplicateKeyword) {
		t.Fatalf("expected duplicate keyword, got %v", err)
	}

	input = validPositionInput("新平台工程师", "dept_b")
	input.ImplicitTags = []organization.ImplicitTagInput{
		{Name: "系统设计", Weight: intPtr(40)},
		{Name: " 系统设计 ", Weight: intPtr(50)},
	}
	_, err = service.CreatePosition(context.Background(), input)
	if !errors.Is(err, organization.ErrPositionDuplicateImplicitTag) {
		t.Fatalf("expected duplicate implicit tag, got %v", err)
	}

	input = validPositionInput("新平台工程师", "dept_b")
	input.ImplicitTags = []organization.ImplicitTagInput{{Name: "稳定性", Weight: intPtr(101)}}
	_, err = service.CreatePosition(context.Background(), input)
	if !errors.Is(err, organization.ErrPositionInvalidImplicitWeight) {
		t.Fatalf("expected invalid implicit weight, got %v", err)
	}

	input = validPositionInput("新平台工程师", "dept_b")
	input.Keywords = []string{" Go ", "", "调度"}
	input.ImplicitTags = []organization.ImplicitTagInput{{Name: " 系统设计 "}}
	created, err := service.CreatePosition(context.Background(), input)
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	if created.Department.ID != "dept_b" || len(created.Keywords) != 2 || created.ImplicitTags[0].Weight != 40 {
		t.Fatalf("unexpected created position: %#v", created)
	}
	assertPositionDepartmentRelation(t, database, created.ID, "dept_b")
}

func TestUpdatePositionMovesDepartmentRelation(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	service := organization.NewService(organization.NewSQLStore(database), audit.NopRecorder{})
	store := organization.NewSQLStore(database)

	updated, err := service.UpdatePosition(
		context.Background(),
		"position_a",
		validPositionInput("平台工程师", "dept_b"),
		positionScope(iam.ActionUpdate, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}),
	)
	if err != nil {
		t.Fatalf("update position: %v", err)
	}
	if updated.Department.ID != "dept_b" {
		t.Fatalf("expected moved department relation, got %#v", updated.Department)
	}
	if _, err := store.GetPosition(context.Background(), "position_a", positionScope(iam.ActionGet, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}})); !errors.Is(err, organization.ErrPositionNotFound) {
		t.Fatalf("expected old department scope to lose access, got %v", err)
	}
	if _, err := store.GetPosition(context.Background(), "position_a", positionScope(iam.ActionGet, iam.ScopeBranch{DepartmentIDs: []string{"dept_b"}})); err != nil {
		t.Fatalf("expected new department scope to access position: %v", err)
	}
	assertPositionDepartmentRelation(t, database, "position_a", "dept_b")
}

func TestUpdatePositionStatusChangesOnOff(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	service := organization.NewService(organization.NewSQLStore(database), audit.NopRecorder{})

	input := validPositionInput("平台工程师", "dept_a")
	input.Status = "off"
	updated, err := service.UpdatePosition(context.Background(), "position_a", input, positionScope(iam.ActionUpdate, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}))
	if err != nil {
		t.Fatalf("update position status: %v", err)
	}
	if updated.Status != "off" {
		t.Fatalf("expected off status, got %#v", updated)
	}
}

func TestDeletePositionRejectsPositionResumeHistory(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	execOrganizationSQL(t, database, `
		INSERT INTO position_resumes (id, position_id, resume_id, kind, created_at, by_user_id)
		VALUES ('pr_a', 'position_a', 'resume_a', 'parsed', CURRENT_TIMESTAMP, 'user_admin')
	`)
	service := organization.NewService(organization.NewSQLStore(database), audit.NopRecorder{})

	err := service.DeletePosition(context.Background(), "position_a", positionScope(iam.ActionDelete, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}}), "user_admin")
	if !errors.Is(err, organization.ErrPositionDeleteHasHistory) {
		t.Fatalf("expected history protection, got %v", err)
	}
}

func TestPositionWritesRecordAudit(t *testing.T) {
	database := newOrganizationMigratedSQLiteGormDB(t)
	seedOrganizationDepartmentFixture(t, database)
	recorder := &recordingAudit{}
	service := organization.NewService(organization.NewSQLStore(database), recorder)

	created, err := service.CreatePosition(context.Background(), validPositionInput("审计岗位", "dept_b"))
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	updateInput := validPositionInput("审计岗位高级", "dept_b")
	updateInput.Status = "off"
	if _, err := service.UpdatePosition(context.Background(), created.ID, updateInput, positionScope(iam.ActionUpdate, iam.ScopeBranch{AllDepartments: true})); err != nil {
		t.Fatalf("update position: %v", err)
	}
	if err := service.DeletePosition(context.Background(), created.ID, positionScope(iam.ActionDelete, iam.ScopeBranch{AllDepartments: true}), "user_admin"); err != nil {
		t.Fatalf("delete position: %v", err)
	}

	if len(recorder.events) != 3 {
		t.Fatalf("expected three audit events, got %#v", recorder.events)
	}
	if recorder.events[0].Type != audit.EventPositionCreated || recorder.events[0].Resource != string(iam.ResourcePosition) || recorder.events[0].TargetID != created.ID {
		t.Fatalf("unexpected create audit event: %#v", recorder.events[0])
	}
	if recorder.events[0].After["name"] != "审计岗位" || recorder.events[0].After["departmentId"] != "dept_b" {
		t.Fatalf("expected safe create payload, got %#v", recorder.events[0].After)
	}
	if recorder.events[1].Type != audit.EventPositionUpdated || recorder.events[1].After["status"] != "off" {
		t.Fatalf("unexpected update audit event: %#v", recorder.events[1])
	}
	if recorder.events[2].Type != audit.EventPositionDeleted || recorder.events[2].TargetID != created.ID {
		t.Fatalf("unexpected delete audit event: %#v", recorder.events[2])
	}
}

func seedOrganizationPositionFixture(t *testing.T, database *gorm.DB) {
	t.Helper()

	execOrganizationSQL(t, database, `
		INSERT INTO positions (id, name, chan, level, status, duties, must, keywords, implicit_tags, created_at, updated_at)
		VALUES
			('position_b', '调度工程师', 'campus', 'P5', 'off', '["负责调度平台"]', '["熟悉 Kubernetes"]', '["Kubernetes", "调度"]', '[{"name":"沟通","w":30}]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('position_orphan', '孤儿岗位', 'social', 'P6', 'on', '[]', '[]', '["Go"]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execOrganizationSQL(t, database, `
		INSERT INTO department_positions (id, department_id, position_id, created_at)
		VALUES ('dp_b', 'dept_b', 'position_b', CURRENT_TIMESTAMP)
	`)
}

func validPositionInput(name string, departmentID string) organization.PositionInput {
	return organization.PositionInput{
		ActorUserID:  "user_admin",
		Name:         name,
		DepartmentID: departmentID,
		Chan:         "social",
		Level:        "P6",
		Status:       "on",
		Duties:       []string{"负责训练平台服务端研发"},
		Must:         []string{"熟悉 Go"},
		Keywords:     []string{"Go", "调度"},
		ImplicitTags: []organization.ImplicitTagInput{{Name: "系统设计", Weight: intPtr(40)}},
	}
}

func positionScope(action iam.Action, branch iam.ScopeBranch) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourcePosition,
		Action:   action,
		Branches: []iam.ScopeBranch{branch},
	}
}

func assertPositionDepartmentRelation(t *testing.T, database *gorm.DB, positionID string, departmentID string) {
	t.Helper()

	var count int64
	if err := database.Table("department_positions").Where("position_id = ? AND department_id = ?", positionID, departmentID).Count(&count).Error; err != nil {
		t.Fatalf("count department position relation: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one department position relation, got %d", count)
	}
}

func intPtr(value int) *int {
	return &value
}
