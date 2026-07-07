package matching_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLStoreGetResumeAppliesDepartmentAndChannelScope(t *testing.T) {
	db := newMatchingMigratedSQLiteGormDB(t)
	seedMatchingFixture(t, db)
	store := matching.NewSQLStore(db)

	resume, err := store.GetResume(context.Background(), "resume_social_a", matchingResumeScope(iam.ScopeBranch{
		DepartmentIDs: []string{"dept_a"},
		Channels:      []string{"social"},
	}))
	if err != nil {
		t.Fatalf("get scoped resume: %v", err)
	}

	if resume.ID != "resume_social_a" || resume.Name != "张三" || resume.Channel != "social" {
		t.Fatalf("unexpected resume context: %#v", resume)
	}
	if resume.CurrentDepartment.ID != "dept_a" || resume.CurrentDepartment.Name != "算力训练平台部" {
		t.Fatalf("unexpected department summary: %#v", resume.CurrentDepartment)
	}
	if resume.ExpBase != 82 || len(resume.Keywords) != 2 || len(resume.Traits) != 1 {
		t.Fatalf("expected parsed matching fields, got %#v", resume)
	}

	_, err = store.GetResume(context.Background(), "resume_social_b", matchingResumeScope(iam.ScopeBranch{
		DepartmentIDs: []string{"dept_a"},
		Channels:      []string{"social"},
	}))
	if !errors.Is(err, matching.ErrResumeNotFound) {
		t.Fatalf("expected out-of-scope resume to be hidden, got %v", err)
	}
}

func TestSQLStoreGetPositionAppliesDepartmentScopeAndLoadsJD(t *testing.T) {
	db := newMatchingMigratedSQLiteGormDB(t)
	seedMatchingFixture(t, db)
	store := matching.NewSQLStore(db)

	position, err := store.GetPosition(context.Background(), "position_a", matchingPositionScope(iam.ScopeBranch{
		DepartmentIDs: []string{"dept_a"},
	}))
	if err != nil {
		t.Fatalf("get scoped position: %v", err)
	}

	if position.ID != "position_a" || position.Name != "平台工程师" || position.Status != "on" {
		t.Fatalf("unexpected position context: %#v", position)
	}
	if position.Department.ID != "dept_a" || position.Department.Name != "算力训练平台部" {
		t.Fatalf("unexpected position department: %#v", position.Department)
	}
	if len(position.Keywords) != 2 || position.ImplicitTags[0].Name != "稳定" || position.ImplicitTags[0].Weight != 40 {
		t.Fatalf("expected JD matching fields, got %#v", position)
	}

	_, err = store.GetPosition(context.Background(), "position_b", matchingPositionScope(iam.ScopeBranch{
		DepartmentIDs: []string{"dept_a"},
	}))
	if !errors.Is(err, matching.ErrPositionNotFound) {
		t.Fatalf("expected out-of-scope position to be hidden, got %v", err)
	}
}

func TestSQLStoreUpsertsParsedPositionResume(t *testing.T) {
	db := newMatchingMigratedSQLiteGormDB(t)
	seedMatchingFixture(t, db)
	store := matching.NewSQLStore(db)

	first, err := store.UpsertParsedRelation(context.Background(), matching.ParsedRelationInput{
		ResumeID: "resume_social_a", PositionID: "position_a", MatchScore: 76, ActorUserID: "user_owner",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	second, err := store.UpsertParsedRelation(context.Background(), matching.ParsedRelationInput{
		ResumeID: "resume_social_a", PositionID: "position_a", MatchScore: 88, ActorUserID: "user_owner",
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("expected stable parsed relation id, got first=%#v second=%#v", first, second)
	}
	assertMatchingCount(t, db, "position_resumes", "resume_id = 'resume_social_a' AND position_id = 'position_a' AND kind = 'parsed'", 1)
	assertMatchingCount(t, db, "position_resumes", "resume_id = 'resume_social_a' AND position_id = 'position_a' AND kind = 'parsed' AND match_score = 88 AND by_user_id = 'user_owner'", 1)
}

func matchingResumeScope(branch iam.ScopeBranch) iam.ScopePredicate {
	return iam.ScopePredicate{Resource: iam.ResourceResume, Action: iam.ActionGet, Branches: []iam.ScopeBranch{branch}}
}

func matchingPositionScope(branch iam.ScopeBranch) iam.ScopePredicate {
	return iam.ScopePredicate{Resource: iam.ResourcePosition, Action: iam.ActionGet, Branches: []iam.ScopeBranch{branch}}
}

func newMatchingMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
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

func seedMatchingFixture(t *testing.T, db *gorm.DB) {
	t.Helper()

	execMatchingSQL(t, db, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES ('user_owner', 'E001', '李四', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execMatchingSQL(t, db, `
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES
			('dept_a', '算力训练平台部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('dept_b', '智算调度部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execMatchingSQL(t, db, `
		INSERT INTO positions (id, name, chan, level, status, duties, must, keywords, implicit_tags, created_at, updated_at)
		VALUES
			('position_a', '平台工程师', 'social', 'P6', 'on', '["负责平台研发"]', '["熟悉 Go"]', '["Go","Kubernetes"]', '[{"name":"稳定","w":40}]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('position_b', '调度工程师', 'social', 'P5', 'off', '[]', '[]', '["调度"]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execMatchingSQL(t, db, `
		INSERT INTO department_positions (id, department_id, position_id, created_at)
		VALUES
			('dp_a', 'dept_a', 'position_a', CURRENT_TIMESTAMP),
			('dp_b', 'dept_b', 'position_b', CURRENT_TIMESTAMP)
	`)
	execMatchingSQL(t, db, `
		INSERT INTO resumes (id, normalized_name, name, age, school, years_exp, pos, source, source_by, chan, keywords, traits, exp_base, profile, created_at, updated_at)
		VALUES
			('resume_social_a', 'zhangsan', '张三', 29, '浙江大学', 6, '平台工程师', '导入', '李四', 'social', '["Go","调度"]', '["稳定"]', 82, '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('resume_social_b', 'wangwu', '王五', 31, '上海交通大学', 8, '调度工程师', '导入', '王五', 'social', '["Kubernetes"]', '[]', 75, '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execMatchingSQL(t, db, `
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES
			('dr_a', 'dept_a', 'resume_social_a', CURRENT_TIMESTAMP, 'user_owner'),
			('dr_b', 'dept_b', 'resume_social_b', CURRENT_TIMESTAMP, 'user_owner')
	`)
}

func execMatchingSQL(t *testing.T, db *gorm.DB, query string) {
	t.Helper()

	if err := db.Exec(query).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func assertMatchingCount(t *testing.T, db *gorm.DB, table string, where string, expected int) {
	t.Helper()

	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM " + table + " WHERE " + where).Scan(&count).Error; err != nil {
		t.Fatalf("count %s where %s: %v", table, where, err)
	}
	if count != int64(expected) {
		t.Fatalf("expected %s where %s count %d, got %d", table, where, expected, count)
	}
}
