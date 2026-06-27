package integration

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestFoundationMigrationsCreateCoreTables(t *testing.T) {
	database, err := sql.Open("sqlite3", "file::memory:?cache=shared&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}

	migrationDir := filepath.Join("..", "..", "migrations")
	if err := goose.Up(database, migrationDir); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	rows, err := database.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name IN ('users', 'roles', 'resumes', 'audit_logs') ORDER BY name`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
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

	expected := []string{"audit_logs", "resumes", "roles", "users"}
	if len(names) != len(expected) {
		t.Fatalf("expected tables %v, got %v", expected, names)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("expected tables %v, got %v", expected, names)
		}
	}

	if err := goose.Down(database, migrationDir); err != nil {
		t.Fatalf("goose down: %v", err)
	}
}
