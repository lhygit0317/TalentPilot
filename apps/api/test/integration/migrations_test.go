package integration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestFoundationMigrationsCreateExpectedSchema(t *testing.T) {
	ctx := context.Background()
	database := openSQLite(t)
	provider := newMigrationProvider(t, database)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	assertFoundationTablesExist(t, database)
	assertSystemDepartmentSeeded(t, database)
	assertKeyUniqueConstraints(t, database)
	assertForeignKeysAreEnforced(t, database)
	assertResumeDeleteCascadesOnlyResumeRelations(t, database)
	assertReferencedRoleDeleteIsRestricted(t, database)

	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("goose down: %v", err)
	}
	assertFoundationTablesDropped(t, database)
}

func openSQLite(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite3", "file::memory:?cache=shared&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	return database
}

func newMigrationProvider(t *testing.T, database *sql.DB) *goose.Provider {
	t.Helper()

	migrationDir := filepath.Join("..", "..", "migrations")
	provider, err := goose.NewProvider(goose.DialectSQLite3, database, os.DirFS(migrationDir))
	if err != nil {
		t.Fatalf("new migration provider: %v", err)
	}
	return provider
}

func assertFoundationTablesExist(t *testing.T, database *sql.DB) {
	t.Helper()

	expected := []string{
		"audit_logs",
		"department_positions",
		"department_resumes",
		"departments",
		"jobs",
		"notifications",
		"permissions",
		"position_resumes",
		"positions",
		"resumes",
		"role_relations",
		"roles",
		"user_department_roles",
		"users",
	}

	if names := foundationTableNames(t, database); !slices.Equal(names, expected) {
		t.Fatalf("expected foundation tables %v, got %v", expected, names)
	}
}

func assertSystemDepartmentSeeded(t *testing.T, database *sql.DB) {
	t.Helper()

	var name string
	err := database.QueryRow("SELECT name FROM departments WHERE id = ?", "__system__").Scan(&name)
	if err != nil {
		t.Fatalf("query system department seed: %v", err)
	}
	if name != "system" {
		t.Fatalf("expected system department name, got %q", name)
	}
}

func assertKeyUniqueConstraints(t *testing.T, database *sql.DB) {
	t.Helper()

	insertUser(t, database, "user_unique")
	insertDepartment(t, database, "department_unique")
	insertRole(t, database, "role_unique")
	insertPosition(t, database, "position_unique")
	insertResume(t, database, "resume_unique")

	mustExec(t, database, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES ('udr_unique_1', 'user_unique', 'department_unique', 'role_unique', CURRENT_TIMESTAMP, 'tester')
	`)
	mustFail(t, database, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES ('udr_unique_2', 'user_unique', 'department_unique', 'role_unique', CURRENT_TIMESTAMP, 'tester')
	`)

	mustExec(t, database, `
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES ('department_resume_unique_1', 'department_unique', 'resume_unique', CURRENT_TIMESTAMP, 'user_unique')
	`)
	mustFail(t, database, `
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES ('department_resume_unique_2', '__system__', 'resume_unique', CURRENT_TIMESTAMP, 'user_unique')
	`)

	mustExec(t, database, `
		INSERT INTO position_resumes (id, position_id, resume_id, kind, match_score, created_at, by_user_id)
		VALUES ('position_resume_unique_1', 'position_unique', 'resume_unique', 'parsed', 80, CURRENT_TIMESTAMP, 'user_unique')
	`)
	mustFail(t, database, `
		INSERT INTO position_resumes (id, position_id, resume_id, kind, match_score, created_at, by_user_id)
		VALUES ('position_resume_unique_2', 'position_unique', 'resume_unique', 'parsed', 81, CURRENT_TIMESTAMP, 'user_unique')
	`)
}

func assertForeignKeysAreEnforced(t *testing.T, database *sql.DB) {
	t.Helper()

	mustFail(t, database, `
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES ('department_resume_missing_fk', 'missing_department', 'missing_resume', CURRENT_TIMESTAMP, 'missing_user')
	`)
}

func assertResumeDeleteCascadesOnlyResumeRelations(t *testing.T, database *sql.DB) {
	t.Helper()

	insertUser(t, database, "user_cascade")
	insertDepartment(t, database, "department_cascade")
	insertPosition(t, database, "position_cascade")
	insertResume(t, database, "resume_cascade")

	mustExec(t, database, `
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES ('department_resume_cascade', 'department_cascade', 'resume_cascade', CURRENT_TIMESTAMP, 'user_cascade')
	`)
	mustExec(t, database, `
		INSERT INTO position_resumes (id, position_id, resume_id, kind, match_score, created_at, by_user_id)
		VALUES ('position_resume_cascade', 'position_cascade', 'resume_cascade', 'manual', 70, CURRENT_TIMESTAMP, 'user_cascade')
	`)
	mustExec(t, database, `
		INSERT INTO notifications (id, to_user_id, resume_id, department_id, position_id, name, by_user_id, chan, time, read)
		VALUES ('notification_cascade', 'user_cascade', 'resume_cascade', 'department_cascade', 'position_cascade', '候选人', 'user_cascade', 'social', CURRENT_TIMESTAMP, FALSE)
	`)

	mustExec(t, database, "DELETE FROM resumes WHERE id = 'resume_cascade'")

	assertCount(t, database, "department_resumes", "resume_id = 'resume_cascade'", 0)
	assertCount(t, database, "position_resumes", "resume_id = 'resume_cascade'", 0)
	assertCount(t, database, "notifications", "id = 'notification_cascade'", 1)
}

func assertReferencedRoleDeleteIsRestricted(t *testing.T, database *sql.DB) {
	t.Helper()

	insertUser(t, database, "user_restrict")
	insertDepartment(t, database, "department_restrict")
	insertRole(t, database, "role_restrict")

	mustExec(t, database, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES ('udr_restrict', 'user_restrict', 'department_restrict', 'role_restrict', CURRENT_TIMESTAMP, 'tester')
	`)
	mustFail(t, database, "DELETE FROM roles WHERE id = 'role_restrict'")
	assertCount(t, database, "roles", "id = 'role_restrict'", 1)
}

func assertFoundationTablesDropped(t *testing.T, database *sql.DB) {
	t.Helper()

	if names := foundationTableNames(t, database); len(names) != 0 {
		t.Fatalf("expected foundation tables to be dropped, got %v", names)
	}
}

func foundationTableNames(t *testing.T, database *sql.DB) []string {
	t.Helper()

	rows, err := database.Query(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
			AND name IN (
				'audit_logs',
				'department_positions',
				'department_resumes',
				'departments',
				'jobs',
				'notifications',
				'permissions',
				'position_resumes',
				'positions',
				'resumes',
				'role_relations',
				'roles',
				'user_department_roles',
				'users'
			)
		ORDER BY name
	`)
	if err != nil {
		t.Fatalf("query foundation tables: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}
	return names
}

func insertUser(t *testing.T, database *sql.DB, id string) {
	t.Helper()

	mustExec(t, database, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, id+"_employee", id+"_name")
}

func insertDepartment(t *testing.T, database *sql.DB, id string) {
	t.Helper()

	mustExec(t, database, `
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, id+"_name")
}

func insertRole(t *testing.T, database *sql.DB, id string) {
	t.Helper()

	mustExec(t, database, `
		INSERT INTO roles (id, label, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, id+"_label")
}

func insertPosition(t *testing.T, database *sql.DB, id string) {
	t.Helper()

	mustExec(t, database, `
		INSERT INTO positions (id, name, chan, status, created_at, updated_at)
		VALUES (?, ?, 'social', 'on', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, id+"_name")
}

func insertResume(t *testing.T, database *sql.DB, id string) {
	t.Helper()

	mustExec(t, database, `
		INSERT INTO resumes (id, normalized_name, name, source, chan, created_at, updated_at)
		VALUES (?, ?, ?, '导入', 'social', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, id+"_normalized", id+"_name")
}

func mustExec(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()

	if _, err := database.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func mustFail(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()

	if _, err := database.Exec(query, args...); err == nil {
		t.Fatalf("expected exec %q to fail", query)
	}
}

func assertCount(t *testing.T, database *sql.DB, table string, where string, expected int) {
	t.Helper()

	var count int
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + where
	if err := database.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("count %s where %s: %v", table, where, err)
	}
	if count != expected {
		t.Fatalf("expected %s where %s count %d, got %d", table, where, expected, count)
	}
}

func TestMigrationProviderDoesNotRequireGlobalGooseDialect(t *testing.T) {
	database := openSQLite(t)
	provider := newMigrationProvider(t, database)

	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("provider up without global dialect: %v", err)
	}
}
