package integration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
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
	assertGuestRoleSeeded(t, database)
	assertPresetIAMSeeded(t, database)
	assertAuthSessionConstraints(t, database)
	assertKeyUniqueConstraints(t, database)
	assertForeignKeysAreEnforced(t, database)
	assertResumeDeleteCascadesOnlyResumeRelations(t, database)
	assertReferencedRoleDeleteIsRestricted(t, database)
	assertGuestBindingDoesNotBlockE1Rollback(t, database)

	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("goose down to zero: %v", err)
	}
	assertFoundationTablesDropped(t, database)
}

func TestE1MigrationDownRemovesGuestSeedAfterBindings(t *testing.T) {
	ctx := context.Background()
	database := openSQLite(t)
	provider := newMigrationProvider(t, database)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	insertUser(t, database, "user_guest_down")
	mustExec(t, database, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES ('udr_guest_down', 'user_guest_down', '__system__', '__role_guest__', CURRENT_TIMESTAMP, 'migration_test')
	`)

	if _, err := provider.DownTo(ctx, 1); err != nil {
		t.Fatalf("goose down to foundation: %v", err)
	}

	assertCount(t, database, "user_department_roles", "role_id = '__role_guest__'", 0)
	assertCount(t, database, "permissions", "role_id = '__role_guest__'", 0)
	assertCount(t, database, "roles", "id = '__role_guest__'", 0)
}

func TestE1MigrationFailsOnGuestRoleLabelConflict(t *testing.T) {
	ctx := context.Background()
	database := openSQLite(t)
	provider := newMigrationProvider(t, database)

	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatalf("goose up to foundation: %v", err)
	}

	mustExec(t, database, `
		INSERT INTO roles (id, label, description, is_system, enabled, created_at, created_by, updated_at)
		VALUES ('custom_guest', '游客', 'conflicting custom guest label', FALSE, TRUE, CURRENT_TIMESTAMP, 'migration_test', CURRENT_TIMESTAMP)
	`)

	if _, err := provider.Up(ctx); err == nil {
		t.Fatalf("expected E1 migration to fail on conflicting guest role label")
	}
}

func TestIAMSeedMigrationDownRemovesPresetSeed(t *testing.T) {
	ctx := context.Background()
	database := openSQLite(t)
	provider := newMigrationProvider(t, database)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	if _, err := provider.DownTo(ctx, 2); err != nil {
		t.Fatalf("goose down to auth seed: %v", err)
	}

	assertCount(t, database, "role_relations", "parent_role_id IN ('__role_hrd__','__role_super_admin__')", 0)
	assertCount(t, database, "permissions", "role_id IN ('__role_hrbp__','__role_hrd__','__role_manager__','__role_trainee__','__role_social_owner__','__role_campus_owner__','__role_super_admin__')", 0)
	assertCount(t, database, "roles", "id IN ('__role_hrbp__','__role_hrd__','__role_manager__','__role_trainee__','__role_social_owner__','__role_campus_owner__','__role_super_admin__')", 0)
	assertCount(t, database, "roles", "id = '__role_guest__'", 1)
}

func TestResumeImportJobMigrationAddsOwnershipAndResultMetadata(t *testing.T) {
	ctx := context.Background()
	database := openSQLite(t)
	provider := newMigrationProvider(t, database)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	assertColumnExists(t, database, "jobs", "created_by_user_id")
	assertColumnExists(t, database, "jobs", "result_json")
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
		"auth_sessions",
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

func assertGuestRoleSeeded(t *testing.T, database *sql.DB) {
	t.Helper()

	var id string
	var isSystem bool
	var enabled bool
	err := database.QueryRow("SELECT id, is_system, enabled FROM roles WHERE label = ?", "游客").Scan(&id, &isSystem, &enabled)
	if err != nil {
		t.Fatalf("query guest role seed: %v", err)
	}
	if id != "__role_guest__" || !isSystem || !enabled {
		t.Fatalf("expected enabled system guest role, got id=%q is_system=%t enabled=%t", id, isSystem, enabled)
	}

	assertCount(t, database, "permissions", "role_id = '__role_guest__' AND resource = 'Department' AND action = 'List'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_guest__' AND resource = 'User' AND action = 'Get'", 1)
}

func assertPresetIAMSeeded(t *testing.T, database *sql.DB) {
	t.Helper()

	assertCount(t, database, "roles", "id IN ('__role_hrbp__','__role_hrd__','__role_manager__','__role_trainee__','__role_social_owner__','__role_campus_owner__','__role_super_admin__')", 7)
	assertCount(t, database, "role_relations", "parent_role_id = '__role_hrd__'", 3)
	assertCount(t, database, "role_relations", "parent_role_id = '__role_super_admin__'", 3)
	assertCount(t, database, "permissions", "role_id = '__role_social_owner__' AND resource = 'Resume' AND action = 'List' AND attribute_conditions = '{\"chan\":[\"social\"]}'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_campus_owner__' AND resource = 'Resume' AND action = 'List' AND attribute_conditions = '{\"chan\":[\"campus\"]}'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_hrbp__' AND resource = 'DepartmentResume' AND action = 'Delete'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_social_owner__' AND resource = 'DepartmentResume' AND action = 'Delete'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_campus_owner__' AND resource = 'DepartmentResume' AND action = 'Delete'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_super_admin__' AND resource = 'Position' AND action = 'Delete'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_super_admin__' AND resource = 'Department' AND action = 'Create'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_super_admin__' AND resource = 'Department' AND action = 'Update'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_super_admin__' AND resource = 'Department' AND action = 'Delete'", 1)
	assertCount(t, database, "permissions", "role_id != '__role_super_admin__' AND resource = 'Department' AND action IN ('Create','Update','Delete')", 0)
}

func assertAuthSessionConstraints(t *testing.T, database *sql.DB) {
	t.Helper()

	insertUser(t, database, "session_user")
	mustExec(t, database, `
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ('session_1', 'session_user', 'token_hash_1', 'csrf_hash_1', datetime('now', '+1 hour'), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	mustFail(t, database, `
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ('session_2', 'session_user', 'token_hash_1', 'csrf_hash_2', datetime('now', '+1 hour'), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	mustFail(t, database, `
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ('session_missing_user', 'missing_user', 'token_hash_missing', 'csrf_hash_missing', datetime('now', '+1 hour'), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
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

func assertGuestBindingDoesNotBlockE1Rollback(t *testing.T, database *sql.DB) {
	t.Helper()

	insertUser(t, database, "user_guest_rollback")
	mustExec(t, database, `
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES ('udr_guest_rollback', 'user_guest_rollback', '__system__', '__role_guest__', CURRENT_TIMESTAMP, 'migration_test')
	`)
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
					'auth_sessions',
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

func assertColumnExists(t *testing.T, database *sql.DB, table string, column string) {
	t.Helper()

	rows, err := database.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("inspect columns for %s: %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan column for %s: %v", table, err)
		}
		if name == column {
			return
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate columns for %s: %v", table, err)
	}
	t.Fatalf("expected column %s.%s to exist", table, column)
}

func TestMigrationProviderDoesNotRequireGlobalGooseDialect(t *testing.T) {
	database := openSQLite(t)
	provider := newMigrationProvider(t, database)

	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("provider up without global dialect: %v", err)
	}
}

func TestFoundationMigrationsRunOnPostgresWhenConfigured(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close postgres: %v", err)
		}
	})

	resetPostgresSchema(t, database)
	t.Cleanup(func() {
		resetPostgresSchema(t, database)
	})

	provider := newPostgresMigrationProvider(t, database)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("postgres goose up: %v", err)
	}

	assertPostgresFoundationTablesExist(t, database)
	assertPostgresSystemDepartmentSeeded(t, database)
	assertPostgresGuestRoleSeeded(t, database)
	assertPostgresAuthSessionConstraints(t, database)

	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("postgres goose down to zero: %v", err)
	}
	assertPostgresFoundationTablesDropped(t, database)
}

func newPostgresMigrationProvider(t *testing.T, database *sql.DB) *goose.Provider {
	t.Helper()

	migrationDir := filepath.Join("..", "..", "migrations")
	provider, err := goose.NewProvider(goose.DialectPostgres, database, os.DirFS(migrationDir))
	if err != nil {
		t.Fatalf("new postgres migration provider: %v", err)
	}
	return provider
}

func resetPostgresSchema(t *testing.T, database *sql.DB) {
	t.Helper()

	if _, err := database.Exec("DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("reset postgres schema: %v", err)
	}
}

func assertPostgresFoundationTablesExist(t *testing.T, database *sql.DB) {
	t.Helper()

	expected := []string{
		"audit_logs",
		"auth_sessions",
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

	if names := postgresFoundationTableNames(t, database); !slices.Equal(names, expected) {
		t.Fatalf("expected postgres foundation tables %v, got %v", expected, names)
	}
}

func assertPostgresGuestRoleSeeded(t *testing.T, database *sql.DB) {
	t.Helper()

	var id string
	var isSystem bool
	var enabled bool
	err := database.QueryRow("SELECT id, is_system, enabled FROM roles WHERE label = $1", "游客").Scan(&id, &isSystem, &enabled)
	if err != nil {
		t.Fatalf("query postgres guest role seed: %v", err)
	}
	if id != "__role_guest__" || !isSystem || !enabled {
		t.Fatalf("expected postgres enabled system guest role, got id=%q is_system=%t enabled=%t", id, isSystem, enabled)
	}

	assertCount(t, database, "permissions", "role_id = '__role_guest__' AND resource = 'Department' AND action = 'List'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_guest__' AND resource = 'User' AND action = 'Get'", 1)
}

func assertPostgresAuthSessionConstraints(t *testing.T, database *sql.DB) {
	t.Helper()

	mustExec(t, database, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES ('session_user', 'session_user_employee', 'session_user_name', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	mustExec(t, database, `
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ('session_1', 'session_user', 'token_hash_1', 'csrf_hash_1', CURRENT_TIMESTAMP + INTERVAL '1 hour', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	mustFail(t, database, `
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ('session_2', 'session_user', 'token_hash_1', 'csrf_hash_2', CURRENT_TIMESTAMP + INTERVAL '1 hour', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	mustFail(t, database, `
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ('session_missing_user', 'missing_user', 'token_hash_missing', 'csrf_hash_missing', CURRENT_TIMESTAMP + INTERVAL '1 hour', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
}

func assertPostgresSystemDepartmentSeeded(t *testing.T, database *sql.DB) {
	t.Helper()

	var name string
	err := database.QueryRow("SELECT name FROM departments WHERE id = $1", "__system__").Scan(&name)
	if err != nil {
		t.Fatalf("query postgres system department seed: %v", err)
	}
	if name != "system" {
		t.Fatalf("expected postgres system department name, got %q", name)
	}
}

func assertPostgresFoundationTablesDropped(t *testing.T, database *sql.DB) {
	t.Helper()

	if names := postgresFoundationTableNames(t, database); len(names) != 0 {
		t.Fatalf("expected postgres foundation tables to be dropped, got %v", names)
	}
}

func postgresFoundationTableNames(t *testing.T, database *sql.DB) []string {
	t.Helper()

	rows, err := database.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
			AND table_type = 'BASE TABLE'
			AND table_name IN (
					'audit_logs',
					'auth_sessions',
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
		ORDER BY table_name
	`)
	if err != nil {
		t.Fatalf("query postgres foundation tables: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan postgres table: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate postgres tables: %v", err)
	}
	return names
}
