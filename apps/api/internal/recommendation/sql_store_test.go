package recommendation_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/recommendation"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLStoreGetResumeAppliesScopeAndLoadsRoutingFields(t *testing.T) {
	db := newRecommendationMigratedSQLiteGormDB(t)
	seedRecommendationRoutingFixture(t, db)
	store := recommendation.NewSQLStore(db)

	resume, err := store.GetResume(context.Background(), "resume_source", recommendationResumeScope(iam.ScopeBranch{
		DepartmentIDs: []string{"dept_source"},
		Channels:      []string{"social"},
	}))
	if err != nil {
		t.Fatalf("get resume: %v", err)
	}
	if resume.ID != "resume_source" || resume.NormalizedName != "zhangsan" || resume.Department.ID != "dept_source" {
		t.Fatalf("unexpected resume context: %#v", resume)
	}
	if resume.ExpBase != 88 || len(resume.Keywords) != 1 || len(resume.Traits) != 1 {
		t.Fatalf("expected routing fields, got %#v", resume)
	}

	_, err = store.GetResume(context.Background(), "resume_source", recommendationResumeScope(iam.ScopeBranch{
		DepartmentIDs: []string{"dept_other"},
		Channels:      []string{"social"},
	}))
	if !errors.Is(err, recommendation.ErrResumeNotFound) {
		t.Fatalf("expected out-of-scope resume to be hidden, got %v", err)
	}
}

func TestSQLStoreListsScopedSameChannelOnShelfPositions(t *testing.T) {
	db := newRecommendationMigratedSQLiteGormDB(t)
	seedRecommendationRoutingFixture(t, db)
	store := recommendation.NewSQLStore(db)

	positions, err := store.ListRoutePositions(context.Background(), recommendation.RoutePositionQuery{
		Channel: "social",
		Scope:   iam.ScopePredicate{Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}}}},
	})
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}
	if len(positions) != 1 || positions[0].ID != "position_social_on" {
		t.Fatalf("expected only scoped social on-shelf position, got %#v", positions)
	}
}

func TestSQLStoreListsDepartmentContactsByPresetRole(t *testing.T) {
	db := newRecommendationMigratedSQLiteGormDB(t)
	seedRecommendationRoutingFixture(t, db)
	store := recommendation.NewSQLStore(db)

	contacts, err := store.ListDepartmentContacts(context.Background(), []string{"dept_target"})
	if err != nil {
		t.Fatalf("contacts: %v", err)
	}
	if got := strings.Join(contacts["dept_target"].HRBPs, "、"); got != "李四" {
		t.Fatalf("unexpected HRBP contacts: %q", got)
	}
	if got := strings.Join(contacts["dept_target"].Managers, "、"); got != "王五" {
		t.Fatalf("unexpected manager contacts: %q", got)
	}
	if got := strings.Join(contacts["dept_target"].Trainees, "、"); got != "赵六" {
		t.Fatalf("unexpected trainee contacts: %q", got)
	}
}

func TestSQLStoreSendRecommendationCreatesCopyAndNotifications(t *testing.T) {
	db := newRecommendationMigratedSQLiteGormDB(t)
	seedRecommendationRoutingFixture(t, db)
	seedRecommendationSendFixture(t, db)
	store := recommendation.NewSQLStore(db)

	result, err := store.SendRecommendation(context.Background(), recommendation.SendCommand{
		ActorUserID:                 "user_recommender",
		ActorName:                   "推荐人",
		ResumeID:                    "resume_source",
		DepartmentID:                "dept_target",
		PositionID:                  "position_social_on",
		ResumeGetScope:              recommendationResumeScope(iam.ScopeBranch{DepartmentIDs: []string{"dept_source"}, Channels: []string{"social"}}),
		ResumeCreateScope:           iam.ScopePredicate{Resource: iam.ResourceResume, Action: iam.ActionCreate, Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}, Channels: []string{"social"}}}},
		DepartmentResumeCreateScope: iam.ScopePredicate{Resource: iam.ResourceDepartmentResume, Action: iam.ActionCreate, Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}}}},
		PositionResumeCreateScope:   iam.ScopePredicate{Resource: iam.ResourcePositionResume, Action: iam.ActionCreate, Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}}}},
		NotificationCreateScope:     iam.ScopePredicate{Resource: iam.ResourceNotification, Action: iam.ActionCreate, Branches: []iam.ScopeBranch{{AllDepartments: true}}},
	})
	if err != nil {
		t.Fatalf("send recommendation: %v", err)
	}
	if result.ResumeID == "" || result.ResumeID == "resume_source" || result.NotifiedCount != 4 {
		t.Fatalf("unexpected send result: %#v", result)
	}
	if result.Department.ID != "dept_target" || result.Position.ID != "position_social_on" || result.Message != "已推荐到「智算调度部」· 已通知 4 位相关人员" {
		t.Fatalf("unexpected response summary: %#v", result)
	}
	assertRecommendationCount(t, db, "resumes", "id = '"+result.ResumeID+"' AND source = '推荐' AND source_by = '推荐人' AND chan = 'social'", 1)
	assertRecommendationCount(t, db, "department_resumes", "resume_id = '"+result.ResumeID+"' AND department_id = 'dept_target'", 1)
	assertRecommendationCount(t, db, "position_resumes", "resume_id = '"+result.ResumeID+"' AND position_id = 'position_social_on' AND kind = 'recommended'", 1)
	assertRecommendationCount(t, db, "notifications", "resume_id = '"+result.ResumeID+"' AND to_user_id = 'user_hrbp' AND read = false", 1)
	assertRecommendationCount(t, db, "notifications", "resume_id = '"+result.ResumeID+"' AND to_user_id = 'user_manager' AND read = false", 1)
	assertRecommendationCount(t, db, "notifications", "resume_id = '"+result.ResumeID+"' AND to_user_id = 'user_trainee' AND read = false", 1)
	assertRecommendationCount(t, db, "notifications", "resume_id = '"+result.ResumeID+"' AND to_user_id = 'user_recommender' AND read = true", 1)
}

func TestSQLStoreSendRecommendationReusesExistingDepartmentCopy(t *testing.T) {
	db := newRecommendationMigratedSQLiteGormDB(t)
	seedRecommendationRoutingFixture(t, db)
	seedRecommendationSendFixture(t, db)
	store := recommendation.NewSQLStore(db)
	command := recommendation.SendCommand{
		ActorUserID:                 "user_recommender",
		ActorName:                   "推荐人",
		ResumeID:                    "resume_source",
		DepartmentID:                "dept_target",
		PositionID:                  "position_social_on",
		ResumeGetScope:              recommendationResumeScope(iam.ScopeBranch{DepartmentIDs: []string{"dept_source"}, Channels: []string{"social"}}),
		ResumeCreateScope:           iam.ScopePredicate{Resource: iam.ResourceResume, Action: iam.ActionCreate, Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}, Channels: []string{"social"}}}},
		DepartmentResumeCreateScope: iam.ScopePredicate{Resource: iam.ResourceDepartmentResume, Action: iam.ActionCreate, Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}}}},
		PositionResumeCreateScope:   iam.ScopePredicate{Resource: iam.ResourcePositionResume, Action: iam.ActionCreate, Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}}}},
		NotificationCreateScope:     iam.ScopePredicate{Resource: iam.ResourceNotification, Action: iam.ActionCreate, Branches: []iam.ScopeBranch{{AllDepartments: true}}},
	}

	first, err := store.SendRecommendation(context.Background(), command)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	second, err := store.SendRecommendation(context.Background(), command)
	if err != nil {
		t.Fatalf("second send: %v", err)
	}
	if first.ResumeID != second.ResumeID || !second.ReusedExistingCopy {
		t.Fatalf("expected second send to reuse copy, first=%#v second=%#v", first, second)
	}
	assertRecommendationCount(t, db, "resumes", "normalized_name = 'zhangsan'", 2)
	assertRecommendationCount(t, db, "position_resumes", "resume_id = '"+first.ResumeID+"' AND position_id = 'position_social_on' AND kind = 'recommended'", 1)
}

func recommendationResumeScope(branch iam.ScopeBranch) iam.ScopePredicate {
	return iam.ScopePredicate{Resource: iam.ResourceResume, Action: iam.ActionGet, Branches: []iam.ScopeBranch{branch}}
}

func newRecommendationMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
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

func seedRecommendationRoutingFixture(t *testing.T, db *gorm.DB) {
	t.Helper()

	execRecommendationSQL(t, db, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES
			('user_recommender', 'E001', '推荐人', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('user_hrbp', 'E002', '李四', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('user_manager', 'E003', '王五', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('user_trainee', 'E004', '赵六', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execRecommendationSQL(t, db, `
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES
			('dept_source', '来源部门', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('dept_target', '智算调度部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('dept_other', '其他部门', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execRecommendationSQL(t, db, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES
			('udr_hrbp', 'user_hrbp', 'dept_target', '__role_hrbp__', CURRENT_TIMESTAMP, 'user_recommender'),
			('udr_manager', 'user_manager', 'dept_target', '__role_manager__', CURRENT_TIMESTAMP, 'user_recommender'),
			('udr_trainee', 'user_trainee', 'dept_target', '__role_trainee__', CURRENT_TIMESTAMP, 'user_recommender')
	`)
	execRecommendationSQL(t, db, `
		INSERT INTO positions (id, name, chan, level, status, duties, must, keywords, implicit_tags, created_at, updated_at)
		VALUES
			('position_social_on', '调度工程师', 'social', 'P6', 'on', '[]', '[]', '["Go"]', '[{"name":"稳定","w":40}]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('position_social_off', '下架岗位', 'social', 'P6', 'off', '[]', '[]', '["Go"]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('position_campus_on', '校招岗位', 'campus', 'P5', 'on', '[]', '[]', '["Go"]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('position_other_on', '其他部门岗位', 'social', 'P6', 'on', '[]', '[]', '["Go"]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execRecommendationSQL(t, db, `
		INSERT INTO department_positions (id, department_id, position_id, created_at)
		VALUES
			('dp_target_on', 'dept_target', 'position_social_on', CURRENT_TIMESTAMP),
			('dp_target_off', 'dept_target', 'position_social_off', CURRENT_TIMESTAMP),
			('dp_target_campus', 'dept_target', 'position_campus_on', CURRENT_TIMESTAMP),
			('dp_other_on', 'dept_other', 'position_other_on', CURRENT_TIMESTAMP)
	`)
	execRecommendationSQL(t, db, `
		INSERT INTO resumes (id, normalized_name, name, age, school, years_exp, pos, source, source_by, chan, keywords, traits, exp_base, profile, created_at, updated_at)
		VALUES
			('resume_source', 'zhangsan', '张三', 29, '浙江大学', 6, '平台工程师', '导入', '推荐人', 'social', '["Go"]', '["稳定"]', 88, '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execRecommendationSQL(t, db, `
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES ('dr_source', 'dept_source', 'resume_source', CURRENT_TIMESTAMP, 'user_recommender')
	`)
}

func seedRecommendationSendFixture(t *testing.T, db *gorm.DB) {
	t.Helper()

	execRecommendationSQL(t, db, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES ('udr_recommender_target', 'user_recommender', 'dept_target', '__role_hrbp__', CURRENT_TIMESTAMP, 'user_recommender')
	`)
}

func execRecommendationSQL(t *testing.T, db *gorm.DB, query string) {
	t.Helper()

	if err := db.Exec(query).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func assertRecommendationCount(t *testing.T, db *gorm.DB, table string, where string, expected int) {
	t.Helper()

	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM " + table + " WHERE " + where).Scan(&count).Error; err != nil {
		t.Fatalf("count %s where %s: %v", table, where, err)
	}
	if count != int64(expected) {
		t.Fatalf("expected %s where %s count %d, got %d", table, where, expected, count)
	}
}
