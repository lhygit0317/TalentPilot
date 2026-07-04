package audit_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLRecorderWritesResumeAuditRows(t *testing.T) {
	database := newAuditMigratedSQLiteGormDB(t)
	recorder := audit.NewSQLRecorder(database)

	err := recorder.Record(context.Background(), audit.Event{
		Type:             audit.EventResumeImportSucceeded,
		RequestID:        "req_audit",
		ActorUserID:      "user_owner",
		ActorEmployeeID:  "E001",
		ActorRoleSummary: "HRBP",
		Resource:         "Resume",
		Action:           "Create",
		TargetID:         "resume_audit",
		Result:           "succeeded",
		After: map[string]any{
			"resumeId": "resume_audit",
			"chan":     "social",
			"profile":  map[string]any{"phone": "secret-phone"},
		},
		At: time.Date(2026, 7, 4, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("record audit: %v", err)
	}

	var row struct {
		RequestID string `gorm:"column:request_id"`
		Resource  string
		Action    string
		After     string `gorm:"column:after_value"`
	}
	if err := database.Raw(`
		SELECT request_id, resource, action, after_value
		FROM audit_logs
		WHERE target_id = 'resume_audit'
	`).Scan(&row).Error; err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if row.RequestID != "req_audit" || row.Resource != "Resume" || row.Action != "Create" {
		t.Fatalf("unexpected audit row: %#v", row)
	}
	if strings.Contains(row.After, "profile") || strings.Contains(row.After, "secret-phone") {
		t.Fatalf("audit row leaked sensitive fields: %s", row.After)
	}
}

func newAuditMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
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
	runAuditMigrations(t, sqlDB)
	return gdb
}

func runAuditMigrations(t *testing.T, database *sql.DB) {
	t.Helper()

	provider, err := goose.NewProvider(goose.DialectSQLite3, database, os.DirFS(filepath.Join("..", "..", "migrations")))
	if err != nil {
		t.Fatalf("new migration provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("goose up: %v", err)
	}
}
