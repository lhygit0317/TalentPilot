package resume_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/resume"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLStoreListAppliesDepartmentScopeAndChannelCounts(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	store := resume.NewSQLStore(database)

	result, err := store.List(context.Background(), resume.ListQuery{
		Channel: resume.ChannelSocial,
		Limit:   50,
		Scope: scopeFor(iam.ActionList, iam.ScopeBranch{
			DepartmentIDs: []string{"dept_a"},
			Channels:      []string{"social"},
		}),
		GetScope: scopeFor(iam.ActionGet, iam.ScopeBranch{
			DepartmentIDs: []string{"dept_a"},
			Channels:      []string{"social"},
		}),
		DeleteScope: scopeFor(iam.ActionDelete, iam.ScopeBranch{
			DepartmentIDs: []string{"dept_a"},
			Channels:      []string{"social"},
		}),
	})
	if err != nil {
		t.Fatalf("list resumes: %v", err)
	}

	if len(result.Items) != 1 || result.Items[0].ID != "resume_social_a" {
		t.Fatalf("expected only dept_a social resume, got %#v", result.Items)
	}
	if result.ChannelCounts[resume.ChannelSocial] != 1 || result.ChannelCounts[resume.ChannelCampus] != 0 {
		t.Fatalf("expected authorized channel counts, got %#v", result.ChannelCounts)
	}
	if len(result.AvailableChannels) != 1 || result.AvailableChannels[0] != resume.ChannelSocial {
		t.Fatalf("expected only social channel available, got %#v", result.AvailableChannels)
	}
	if !result.Items[0].CanGet || !result.Items[0].CanDelete {
		t.Fatalf("expected row capabilities from get/delete scope, got %#v", result.Items[0])
	}
}

func TestSQLStoreListAppliesChannelAttributeScope(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	store := resume.NewSQLStore(database)

	result, err := store.List(context.Background(), resume.ListQuery{
		Channel: resume.ChannelSocial,
		Limit:   50,
		Scope: scopeFor(iam.ActionList, iam.ScopeBranch{
			AllDepartments: true,
			Channels:       []string{"social"},
		}),
	})
	if err != nil {
		t.Fatalf("list social resumes: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected two social resumes, got %#v", result.Items)
	}
	for _, item := range result.Items {
		if item.Channel != resume.ChannelSocial {
			t.Fatalf("expected social-only scope, got item %#v", item)
		}
	}
}

func TestSQLStoreSearchIsLiteralCaseInsensitiveAndScoped(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	store := resume.NewSQLStore(database)

	result, err := store.List(context.Background(), resume.ListQuery{
		Channel: resume.ChannelSocial,
		Search:  "c++",
		Limit:   50,
		Scope: scopeFor(iam.ActionList, iam.ScopeBranch{
			DepartmentIDs: []string{"dept_a"},
			Channels:      []string{"social", "campus"},
		}),
	})
	if err != nil {
		t.Fatalf("search resumes: %v", err)
	}

	if len(result.Items) != 1 || result.Items[0].ID != "resume_social_a" {
		t.Fatalf("expected literal scoped C++ match, got %#v", result.Items)
	}
}

func TestSQLStoreGetRejectsOutOfScopeResume(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	store := resume.NewSQLStore(database)

	_, err := store.Get(context.Background(), "resume_social_b", scopeFor(iam.ActionGet, iam.ScopeBranch{
		DepartmentIDs: []string{"dept_a"},
		Channels:      []string{"social"},
	}))
	if !errors.Is(err, resume.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for out-of-scope resume, got %v", err)
	}
}

func TestSQLStoreDeleteCascadesResumeRelationsButPreservesNotifications(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	store := resume.NewSQLStore(database)

	err := store.Delete(context.Background(), "resume_social_a", scopeFor(iam.ActionDelete, iam.ScopeBranch{
		DepartmentIDs: []string{"dept_a"},
		Channels:      []string{"social"},
	}))
	if err != nil {
		t.Fatalf("delete resume: %v", err)
	}

	assertResumeCount(t, database, "resumes", "id = 'resume_social_a'", 0)
	assertResumeCount(t, database, "department_resumes", "resume_id = 'resume_social_a'", 0)
	assertResumeCount(t, database, "position_resumes", "resume_id = 'resume_social_a'", 0)
	assertResumeCount(t, database, "notifications", "id = 'notification_social_a'", 1)
}

func scopeFor(action iam.Action, branch iam.ScopeBranch) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourceResume,
		Action:   action,
		Branches: []iam.ScopeBranch{branch},
	}
}

func newResumeMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
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

func seedResumeFixture(t *testing.T, database *gorm.DB) {
	t.Helper()

	execResumeSQL(t, database, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES ('user_owner', 'E001', '李四', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execResumeSQL(t, database, `
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES
			('dept_a', '算力训练平台部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('dept_b', '智算调度部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execResumeSQL(t, database, `
		INSERT INTO positions (id, name, chan, status, created_at, updated_at)
		VALUES ('position_a', '算法工程师', 'social', 'on', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execResumeSQL(t, database, `
		INSERT INTO resumes (id, normalized_name, name, age, school, years_exp, pos, source, source_by, chan, keywords, profile, created_at, updated_at)
		VALUES
			('resume_social_a', 'zhangsan', '张三', 29, '浙江大学', 6, 'Go C++ 工程师', '导入', '李四', 'social', '["Go","C++"]', '{"education":[]}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('resume_campus_a', 'lisi', '李四', 22, '南京大学', 0, '后端工程师', '导入', '李四', 'campus', '["Java"]', '{"education":[]}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('resume_social_b', 'wangwu', '王五', 31, '上海交通大学', 8, '平台工程师', '导入', '王五', 'social', '["Kubernetes"]', '{"education":[]}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execResumeSQL(t, database, `
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES
			('dr_social_a', 'dept_a', 'resume_social_a', CURRENT_TIMESTAMP, 'user_owner'),
			('dr_campus_a', 'dept_a', 'resume_campus_a', CURRENT_TIMESTAMP, 'user_owner'),
			('dr_social_b', 'dept_b', 'resume_social_b', CURRENT_TIMESTAMP, 'user_owner')
	`)
	execResumeSQL(t, database, `
		INSERT INTO position_resumes (id, position_id, resume_id, kind, match_score, created_at, by_user_id)
		VALUES ('pr_social_a', 'position_a', 'resume_social_a', 'manual', 80, CURRENT_TIMESTAMP, 'user_owner')
	`)
	execResumeSQL(t, database, `
		INSERT INTO notifications (id, to_user_id, resume_id, department_id, position_id, name, by_user_id, chan, time, read)
		VALUES ('notification_social_a', 'user_owner', 'resume_social_a', 'dept_a', 'position_a', '张三', 'user_owner', 'social', CURRENT_TIMESTAMP, FALSE)
	`)
}

func execResumeSQL(t *testing.T, database *gorm.DB, query string) {
	t.Helper()

	if err := database.Exec(query).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func assertResumeCount(t *testing.T, database *gorm.DB, table string, where string, expected int) {
	t.Helper()

	var count int64
	if err := database.Raw("SELECT COUNT(*) FROM " + table + " WHERE " + where).Scan(&count).Error; err != nil {
		t.Fatalf("count %s where %s: %v", table, where, err)
	}
	if count != int64(expected) {
		t.Fatalf("expected %s where %s count %d, got %d", table, where, expected, count)
	}
}
