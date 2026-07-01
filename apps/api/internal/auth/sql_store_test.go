package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLStoreUpsertsUserAndCreatesGuestBinding(t *testing.T) {
	database := newMigratedSQLiteGormDB(t)
	store := NewSQLStore(database)

	user, bindings, err := store.UpsertUserWithGuestBinding(context.Background(), W3Identity{ID: "w3_sql", Name: "王五", EmployeeID: "C789"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if user.ID != "w3_sql" || user.Name != "王五" || user.EmployeeID != "C789" {
		t.Fatalf("unexpected user summary: %#v", user)
	}
	if len(bindings) != 1 || bindings[0].RoleLabel != "游客" || bindings[0].DepartmentID != "__system__" {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}

	user, bindings, err = store.UpsertUserWithGuestBinding(context.Background(), W3Identity{ID: "w3_sql", Name: "王五新", EmployeeID: "C790"})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if user.Name != "王五新" || user.EmployeeID != "C790" || len(bindings) != 1 {
		t.Fatalf("expected refresh without duplicate guest binding, got user=%#v bindings=%#v", user, bindings)
	}
}

func TestSQLStoreSessionLifecycle(t *testing.T) {
	database := newMigratedSQLiteGormDB(t)
	store := NewSQLStore(database)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)

	_, _, err := store.UpsertUserWithGuestBinding(ctx, W3Identity{ID: "w3_session", Name: "赵六", EmployeeID: "D001"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	first, err := store.CreateSession(ctx, CreateSessionInput{UserID: "w3_session", TokenHash: HashToken("first"), CSRFTokenHash: HashToken("csrf_first"), ExpiresAt: now.Add(time.Hour), Now: now})
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	second, err := store.CreateSession(ctx, CreateSessionInput{UserID: "w3_session", TokenHash: HashToken("second"), CSRFTokenHash: HashToken("csrf_second"), ExpiresAt: now.Add(time.Hour), Now: now})
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	if err := store.RevokeOtherSessions(ctx, "w3_session", second.ID, now); err != nil {
		t.Fatalf("revoke others: %v", err)
	}
	if _, err := store.FindSessionByTokenHash(ctx, first.TokenHash, now); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected first session revoked, got %v", err)
	}
	session, err := store.FindSessionByTokenHash(ctx, second.TokenHash, now)
	if err != nil {
		t.Fatalf("find second session: %v", err)
	}
	if session.ID != second.ID || session.User.ID != "w3_session" || len(session.RoleBindings) != 1 {
		t.Fatalf("expected second session active, got %#v", session)
	}

	if err := store.RevokeSession(ctx, second.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, err := store.FindSessionByTokenHash(ctx, second.TokenHash, now.Add(2*time.Minute)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected second session revoked, got %v", err)
	}
}

func TestSQLStoreRotatesSessionAndUsesIndependentID(t *testing.T) {
	database := newMigratedSQLiteGormDB(t)
	store := NewSQLStore(database)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)

	_, _, err := store.UpsertUserWithGuestBinding(ctx, W3Identity{ID: "w3_rotate", Name: "周九", EmployeeID: "G001"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	first, err := store.CreateSession(ctx, CreateSessionInput{UserID: "w3_rotate", TokenHash: HashToken("rotate_first"), CSRFTokenHash: HashToken("csrf_rotate_first"), ExpiresAt: now.Add(time.Hour), Now: now})
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	rotated, err := store.RotateSession(ctx, CreateSessionInput{UserID: "w3_rotate", TokenHash: HashToken("rotate_second"), CSRFTokenHash: HashToken("csrf_rotate_second"), ExpiresAt: now.Add(time.Hour), Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("rotate session: %v", err)
	}

	if _, err := store.FindSessionByTokenHash(ctx, first.TokenHash, now.Add(time.Minute)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected first session revoked by rotation, got %v", err)
	}
	if session, err := store.FindSessionByTokenHash(ctx, rotated.TokenHash, now.Add(time.Minute)); err != nil || session.ID != rotated.ID {
		t.Fatalf("expected rotated session active, session=%#v err=%v", session, err)
	}
	derivedID := "session_" + rotated.TokenHash[:16]
	if rotated.ID == derivedID {
		t.Fatalf("expected session id independent from token hash prefix, got %q", rotated.ID)
	}
}

func TestSQLStoreRejectsExpiredSession(t *testing.T) {
	database := newMigratedSQLiteGormDB(t)
	store := NewSQLStore(database)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)

	_, _, err := store.UpsertUserWithGuestBinding(ctx, W3Identity{ID: "w3_expired", Name: "钱七", EmployeeID: "E001"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	session, err := store.CreateSession(ctx, CreateSessionInput{UserID: "w3_expired", TokenHash: HashToken("expired"), CSRFTokenHash: HashToken("csrf_expired"), ExpiresAt: now.Add(time.Minute), Now: now})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := store.FindSessionByTokenHash(ctx, session.TokenHash, now.Add(2*time.Minute)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected expired session unauthenticated, got %v", err)
	}
}

func newMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared&_foreign_keys=on"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
