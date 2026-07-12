package notification_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/talentpilot/talentpilot/apps/api/internal/notification"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLStoreListUnreadFiltersCurrentUserAndSortsNewestFirst(t *testing.T) {
	db := newNotificationMigratedSQLiteGormDB(t)
	seedNotificationFixture(t, db)
	store := notification.NewSQLStore(db)

	items, err := store.ListUnread(context.Background(), notification.ListQuery{
		UserID:               "user_receiver",
		Limit:                20,
		CanOpenResumeLibrary: true,
	})
	if err != nil {
		t.Fatalf("list unread: %v", err)
	}

	if len(items) != 2 || items[0].ID != "notification_new" || items[1].ID != "notification_old" {
		t.Fatalf("expected current-user unread sorted newest first, got %#v", items)
	}
	if !items[0].CanOpenResumeLibrary || items[0].Department.Name != "智算调度部" || items[0].Position == nil {
		t.Fatalf("expected enriched notification item, got %#v", items[0])
	}
}

func TestSQLStoreListUnreadUsesSafeFallbacks(t *testing.T) {
	db := newNotificationMigratedSQLiteGormDB(t)
	seedNotificationFixture(t, db)
	store := notification.NewSQLStore(db)

	items, err := store.ListUnread(context.Background(), notification.ListQuery{UserID: "user_receiver", Limit: 20})
	if err != nil {
		t.Fatalf("list unread: %v", err)
	}

	if items[1].Recommender.Name != "missing_recommender" || items[1].Position != nil {
		t.Fatalf("expected recommender fallback and missing position omission, got %#v", items[1])
	}
}

func TestSQLStoreMarkAllReadUpdatesOnlyCurrentUser(t *testing.T) {
	db := newNotificationMigratedSQLiteGormDB(t)
	seedNotificationFixture(t, db)
	store := notification.NewSQLStore(db)

	updated, err := store.MarkAllRead(context.Background(), "user_receiver")
	if err != nil {
		t.Fatalf("mark all read: %v", err)
	}
	unread, err := store.CountUnread(context.Background(), "user_receiver")
	if err != nil {
		t.Fatalf("count unread: %v", err)
	}

	if updated != 2 || unread != 0 {
		t.Fatalf("expected two current-user updates and zero unread, updated=%d unread=%d", updated, unread)
	}
	assertNotificationCount(t, db, "id = 'notification_other_user' AND read = false", 1)
}

func TestSQLStoreMarkReadIsOwnedAndIdempotent(t *testing.T) {
	db := newNotificationMigratedSQLiteGormDB(t)
	seedNotificationFixture(t, db)
	store := notification.NewSQLStore(db)

	item, err := store.MarkRead(context.Background(), notification.MarkReadInput{
		UserID:               "user_receiver",
		NotificationID:       "notification_old",
		CanOpenResumeLibrary: true,
	})
	if err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if item.ID != "notification_old" || !item.Read {
		t.Fatalf("expected read item, got %#v", item)
	}
	if _, err := store.MarkRead(context.Background(), notification.MarkReadInput{
		UserID:         "user_receiver",
		NotificationID: "notification_old",
	}); err != nil {
		t.Fatalf("mark read should be idempotent: %v", err)
	}
	if _, err := store.MarkRead(context.Background(), notification.MarkReadInput{
		UserID:         "user_receiver",
		NotificationID: "notification_other_user",
	}); !errors.Is(err, notification.ErrNotFound) {
		t.Fatalf("expected other user's notification to be hidden, got %v", err)
	}
}

func newNotificationMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
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

func seedNotificationFixture(t *testing.T, db *gorm.DB) {
	t.Helper()

	execNotificationSQL(t, db, `
		INSERT INTO users (id, employee_id, name, created_at, updated_at)
		VALUES
			('user_receiver', 'A10001', '接收人', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('user_other', 'A10002', '其他人', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('user_recommender', 'A10003', '推荐人', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execNotificationSQL(t, db, `
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES ('dept_target', '智算调度部', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execNotificationSQL(t, db, `
		INSERT INTO positions (id, name, chan, status, created_at, updated_at)
		VALUES ('position_target', '平台工程师', 'social', 'on', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execNotificationSQL(t, db, `
		INSERT INTO resumes (id, normalized_name, name, source, chan, created_at, updated_at)
		VALUES ('resume_copy', 'zhangsan', '张三', '推荐', 'social', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	execNotificationSQL(t, db, `
		INSERT INTO notifications (id, to_user_id, resume_id, department_id, position_id, name, by_user_id, chan, time, read)
		VALUES
			('notification_old', 'user_receiver', 'resume_copy', 'dept_target', NULL, '张三', 'missing_recommender', 'social', '2026-07-12T08:00:00Z', FALSE),
			('notification_new', 'user_receiver', 'resume_copy', 'dept_target', 'position_target', '李四', 'user_recommender', 'social', '2026-07-12T09:00:00Z', FALSE),
			('notification_read', 'user_receiver', 'resume_copy', 'dept_target', 'position_target', '王五', 'user_recommender', 'social', '2026-07-12T10:00:00Z', TRUE),
			('notification_other_user', 'user_other', 'resume_copy', 'dept_target', 'position_target', '赵六', 'user_recommender', 'social', '2026-07-12T11:00:00Z', FALSE)
	`)
}

func execNotificationSQL(t *testing.T, db *gorm.DB, query string) {
	t.Helper()
	if err := db.Exec(query).Error; err != nil {
		t.Fatalf("exec fixture sql: %v\n%s", err, query)
	}
}

func assertNotificationCount(t *testing.T, db *gorm.DB, where string, expected int) {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM notifications WHERE " + where).Scan(&count).Error; err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != int64(expected) {
		t.Fatalf("expected notification count %d for %s, got %d", expected, where, count)
	}
}
