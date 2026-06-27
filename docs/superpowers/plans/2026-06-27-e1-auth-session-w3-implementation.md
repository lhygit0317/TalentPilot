# E1 Auth Session W3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build E1 login and identity authentication with company account/password W3 authentication, token Cookie sessions, CSRF protection, first-login guest binding, single-device login, `/me`, logout, and the matching frontend login experience.

**Architecture:** Keep auth in a focused backend module with injected W3 and session dependencies. Persist users, guest bindings, and session token hashes in the database; expose the contract through Huma routes and consume it from the frontend through the generated client wrapper. Frontend page access is UX only; backend auth/session checks remain authoritative.

**Tech Stack:** Go, Echo, Huma, GORM, goose, SQLite/PostgreSQL migrations, React, TypeScript, Vite, Testing Library, Vitest, openapi-typescript.

---

## Scope Check

This plan implements only `docs/specs/001-auth-session-w3.md`. It does not implement full IAM permission expansion, resume parsing, recommendation business rules, or role administration.

## File Structure

- Create `apps/api/migrations/000002_create_auth_sessions_and_guest_seed.sql`: auth session table plus guest role and minimal guest permissions.
- Modify `apps/api/test/integration/migrations_test.go`: expect `auth_sessions`, guest seed data, and session constraints.
- Create `apps/api/internal/auth/types.go`: auth DTOs, errors, W3 adapter, store interfaces.
- Create `apps/api/internal/auth/token.go`: token generation and hashing.
- Create `apps/api/internal/auth/service.go`: login, current-user, logout, CSRF orchestration.
- Create `apps/api/internal/auth/service_test.go`: pure auth service TDD with fake W3 and fake store.
- Create `apps/api/internal/auth/sql_store.go`: GORM-backed user/session store.
- Create `apps/api/internal/auth/sql_store_test.go`: SQLite migration-backed store tests.
- Modify `apps/api/internal/http/apperror/error.go`: add E1 auth error codes and messages.
- Create `apps/api/internal/app/auth_routes.go`: Huma route DTOs and handlers for `/auth/csrf`, `/auth/w3/login`, `/me`, `/auth/logout`.
- Create `apps/api/internal/app/auth_routes_test.go`: HTTP route, Cookie, and CSRF tests.
- Modify `apps/api/internal/app/server.go`: add `NewServerWithOptions` while keeping `NewServer`.
- Modify `apps/api/internal/app/openapi_test.go`: assert auth paths exist in OpenAPI.
- Modify `apps/api/internal/config/config.go` and `apps/api/cmd/api/main.go`: wire DB, W3 mode, frontend origin, and secure Cookie settings.
- Regenerate `packages/api-contract/openapi.json` and `packages/api-client/src/schema.d.ts`.
- Create `apps/web/src/api/client.ts`: generated client wrapper with CSRF header support.
- Create `apps/web/src/components/ui/input.tsx`: project-wrapped input component.
- Replace `apps/web/src/app/App.tsx`: login/session shell with guest navigation.
- Modify `apps/web/src/app/App.test.tsx`: login, guest navigation, logout tests.
- Modify `apps/web/src/i18n/zh-CN.ts` and `apps/web/src/i18n/en-US.ts`: E1 text.
- Modify `docs/project-status.md`: mark E1 implementation progress and evidence.

## Task 0: Execution Setup

**Files:**
- Read: `AGENTS.md`
- Read: `docs/specs/001-auth-session-w3.md`
- Read: `docs/project-status.md`

- [ ] **Step 1: Enter an isolated workspace**

Use `superpowers:using-git-worktrees` before implementation. If already in a linked worktree, continue there. If working in place by user preference, record that in the task notes.

- [ ] **Step 2: Verify baseline**

Run:

```bash
make test-api
make test-web
```

Expected: both commands pass before E1 edits. If Go is unavailable locally, record `go: command not found` and continue only after the user accepts that verification is blocked.

## Task 1: Schema and Seed Data

**Files:**
- Modify: `apps/api/test/integration/migrations_test.go`
- Create: `apps/api/migrations/000002_create_auth_sessions_and_guest_seed.sql`

- [ ] **Step 1: Write the failing migration test**

In `assertFoundationTablesExist`, `foundationTableNames`, `assertPostgresFoundationTablesExist`, and `postgresFoundationTableNames`, add `auth_sessions` to the expected table lists. Add these test helpers and call them after `assertSystemDepartmentSeeded(t, database)`:

```go
assertGuestRoleSeeded(t, database)
assertAuthSessionConstraints(t, database)
```

Add the helpers:

```go
func assertGuestRoleSeeded(t *testing.T, database *sql.DB) {
	t.Helper()

	var label string
	err := database.QueryRow("SELECT label FROM roles WHERE id = ?", "__role_guest__").Scan(&label)
	if err != nil {
		t.Fatalf("query guest role seed: %v", err)
	}
	if label != "游客" {
		t.Fatalf("expected guest role label 游客, got %q", label)
	}

	assertCount(t, database, "permissions", "role_id = '__role_guest__' AND resource = 'Department' AND action = 'List'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_guest__' AND resource = 'User' AND action = 'Get'", 1)
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
```

- [ ] **Step 2: Run the migration test and verify red**

Run:

```bash
cd apps/api && go test ./test/integration -run TestFoundationMigrationsCreateExpectedSchema -count=1
```

Expected: FAIL because `auth_sessions` does not exist and the guest role seed is missing.

- [ ] **Step 3: Add the migration**

Create `apps/api/migrations/000002_create_auth_sessions_and_guest_seed.sql`:

```sql
-- +goose Up
CREATE TABLE auth_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  csrf_token_hash TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  revoked_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  last_seen_at TIMESTAMP NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_auth_sessions_user_active ON auth_sessions(user_id, revoked_at, expires_at);

INSERT INTO roles (id, label, description, is_system, enabled, created_at, created_by, updated_at)
VALUES ('__role_guest__', '游客', 'W3 首次登录默认角色', TRUE, TRUE, CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP);

INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
VALUES
  ('__permission_guest_department_list__', '__role_guest__', 'Department', 'List', '{}', CURRENT_TIMESTAMP),
  ('__permission_guest_user_get__', '__role_guest__', 'User', 'Get', '{}', CURRENT_TIMESTAMP);

-- +goose Down
DROP TABLE auth_sessions;
DELETE FROM permissions WHERE role_id = '__role_guest__';
DELETE FROM roles WHERE id = '__role_guest__';
```

- [ ] **Step 4: Run the migration test and verify green**

Run:

```bash
cd apps/api && go test ./test/integration -run TestFoundationMigrationsCreateExpectedSchema -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/migrations/000002_create_auth_sessions_and_guest_seed.sql apps/api/test/integration/migrations_test.go
git commit -m "feat(api): add auth session schema"
```

## Task 2: Auth Service Core

**Files:**
- Create: `apps/api/internal/auth/types.go`
- Create: `apps/api/internal/auth/token.go`
- Create: `apps/api/internal/auth/service.go`
- Create: `apps/api/internal/auth/service_test.go`

- [ ] **Step 1: Write failing service tests**

Create `apps/api/internal/auth/service_test.go` with these first tests:

```go
package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLoginCreatesUserGuestBindingAndSession(t *testing.T) {
	store := newFakeStore()
	w3 := &fakeW3{identity: W3Identity{ID: "w3_1", Name: "张三", EmployeeID: "A123"}}
	service := NewService(ServiceConfig{W3: w3, Store: store, TokenSource: fixedTokenSource("auth_raw", "csrf_raw"), Now: fixedNow})

	result, err := service.Login(context.Background(), LoginInput{Account: "zhangsan", Password: "secret"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if result.User.ID != "w3_1" || result.User.Name != "张三" || result.User.EmployeeID != "A123" {
		t.Fatalf("unexpected user: %#v", result.User)
	}
	if result.AuthToken != "auth_raw" || result.CSRFToken != "csrf_raw" {
		t.Fatalf("expected raw tokens in result, got auth=%q csrf=%q", result.AuthToken, result.CSRFToken)
	}
	if store.upserted.ID != "w3_1" || !store.createdGuestBinding {
		t.Fatalf("expected user upsert with guest binding, got upserted=%#v guest=%v", store.upserted, store.createdGuestBinding)
	}
	if len(store.revokedForUser) != 1 || store.revokedForUser[0] != "w3_1" {
		t.Fatalf("expected old sessions revoked for w3_1, got %v", store.revokedForUser)
	}
}

func TestLoginRetriesW3TimeoutOnce(t *testing.T) {
	store := newFakeStore()
	w3 := &fakeW3{errors: []error{ErrW3Timeout}, identity: W3Identity{ID: "w3_2", Name: "李四", EmployeeID: "B456"}}
	service := NewService(ServiceConfig{W3: w3, Store: store, TokenSource: fixedTokenSource("auth_2", "csrf_2"), Now: fixedNow})

	if _, err := service.Login(context.Background(), LoginInput{Account: "lisi", Password: "secret"}); err != nil {
		t.Fatalf("login after retry: %v", err)
	}
	if w3.calls != 2 {
		t.Fatalf("expected two W3 calls, got %d", w3.calls)
	}
}

func TestInvalidCredentialsDoNotCreateUser(t *testing.T) {
	store := newFakeStore()
	w3 := &fakeW3{errors: []error{ErrInvalidCredentials}}
	service := NewService(ServiceConfig{W3: w3, Store: store, TokenSource: fixedTokenSource("auth", "csrf"), Now: fixedNow})

	_, err := service.Login(context.Background(), LoginInput{Account: "bad", Password: "wrong"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if store.upsertCalled {
		t.Fatalf("expected no user upsert for invalid credentials")
	}
}

func fixedNow() time.Time { return time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC) }
```

Add fake types in the same test file:

```go
type fakeW3 struct {
	identity W3Identity
	errors   []error
	calls    int
}

func (f *fakeW3) Authenticate(ctx context.Context, input W3Credentials) (W3Identity, error) {
	f.calls++
	if len(f.errors) > 0 {
		err := f.errors[0]
		f.errors = f.errors[1:]
		return W3Identity{}, err
	}
	return f.identity, nil
}

type fakeStore struct {
	upsertCalled        bool
	upserted            W3Identity
	createdGuestBinding bool
	revokedForUser      []string
}

func newFakeStore() *fakeStore { return &fakeStore{} }

func (s *fakeStore) UpsertUserWithGuestBinding(ctx context.Context, identity W3Identity) (UserSummary, []RoleBinding, error) {
	s.upsertCalled = true
	s.upserted = identity
	s.createdGuestBinding = true
	return UserSummary{ID: identity.ID, Name: identity.Name, EmployeeID: identity.EmployeeID}, []RoleBinding{{RoleLabel: "游客", DepartmentID: "__system__", DepartmentName: "system"}}, nil
}

func (s *fakeStore) RevokeOtherSessions(ctx context.Context, userID string, keepSessionID string, now time.Time) error {
	s.revokedForUser = append(s.revokedForUser, userID)
	return nil
}
```

- [ ] **Step 2: Run the service tests and verify red**

Run:

```bash
cd apps/api && go test ./internal/auth -run TestLogin -count=1
```

Expected: FAIL because `internal/auth` types and service do not exist.

- [ ] **Step 3: Implement minimal service code**

Create `types.go`:

```go
package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("w3 invalid credentials")
	ErrW3Timeout          = errors.New("w3 timeout")
	ErrW3Unavailable      = errors.New("w3 unavailable")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrCSRFInvalid        = errors.New("csrf invalid")
)

type W3Credentials struct {
	Account  string
	Password string
}

type W3Identity struct {
	ID         string
	Name       string
	EmployeeID string
}

type W3Adapter interface {
	Authenticate(context.Context, W3Credentials) (W3Identity, error)
}

type Store interface {
	UpsertUserWithGuestBinding(context.Context, W3Identity) (UserSummary, []RoleBinding, error)
	RevokeOtherSessions(context.Context, string, string, time.Time) error
}

type UserSummary struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
}

type RoleBinding struct {
	RoleLabel      string `json:"roleLabel"`
	DepartmentID   string `json:"departmentId"`
	DepartmentName string `json:"departmentName"`
}

type LoginInput struct {
	Account  string
	Password string
}

type LoginResult struct {
	User         UserSummary
	RoleBindings []RoleBinding
	RoleLabels   []string
	PageAccess   []string
	DefaultRoute  string
	AuthToken     string
	CSRFToken     string
}

type TokenSource func() (authToken string, csrfToken string, err error)
```

Create `token.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func NewRandomTokenSource() TokenSource {
	return func() (string, string, error) {
		authToken, err := randomToken()
		if err != nil {
			return "", "", err
		}
		csrfToken, err := randomToken()
		if err != nil {
			return "", "", err
		}
		return authToken, csrfToken, nil
	}
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
```

Create `service.go`:

```go
package auth

import (
	"context"
	"errors"
	"time"
)

type Service struct {
	w3          W3Adapter
	store       Store
	tokenSource TokenSource
	now         func() time.Time
}

type ServiceConfig struct {
	W3          W3Adapter
	Store       Store
	TokenSource TokenSource
	Now         func() time.Time
}

func NewService(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	tokenSource := cfg.TokenSource
	if tokenSource == nil {
		tokenSource = NewRandomTokenSource()
	}
	return &Service{w3: cfg.W3, store: cfg.Store, tokenSource: tokenSource, now: now}
}

func (s *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	identity, err := s.authenticateWithRetry(ctx, W3Credentials{Account: input.Account, Password: input.Password})
	if err != nil {
		return LoginResult{}, err
	}
	user, bindings, err := s.store.UpsertUserWithGuestBinding(ctx, identity)
	if err != nil {
		return LoginResult{}, err
	}
	authToken, csrfToken, err := s.tokenSource()
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.store.RevokeOtherSessions(ctx, user.ID, "", s.now()); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		User: user, RoleBindings: bindings, RoleLabels: roleLabels(bindings),
		PageAccess: []string{"resume-parse", "resume-recommend"}, DefaultRoute: "/resume-parse",
		AuthToken: authToken, CSRFToken: csrfToken,
	}, nil
}

func (s *Service) authenticateWithRetry(ctx context.Context, creds W3Credentials) (W3Identity, error) {
	identity, err := s.w3.Authenticate(ctx, creds)
	if errors.Is(err, ErrW3Timeout) {
		return s.w3.Authenticate(ctx, creds)
	}
	return identity, err
}

func roleLabels(bindings []RoleBinding) []string {
	seen := map[string]bool{}
	labels := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if !seen[binding.RoleLabel] {
			seen[binding.RoleLabel] = true
			labels = append(labels, binding.RoleLabel)
		}
	}
	return labels
}
```

- [ ] **Step 4: Run service tests and verify green**

Run:

```bash
cd apps/api && go test ./internal/auth -run TestLogin -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/auth
git commit -m "feat(api): add auth service core"
```

## Task 3: Persistent Auth Store and Session Validation

**Files:**
- Modify: `apps/api/internal/auth/types.go`
- Modify: `apps/api/internal/auth/service.go`
- Create: `apps/api/internal/auth/sql_store.go`
- Create: `apps/api/internal/auth/sql_store_test.go`

- [ ] **Step 1: Write failing SQL store tests**

Create `sql_store_test.go` with tests for user upsert, permanent guest binding, session creation, token lookup, and logout:

```go
func TestSQLStoreUpsertsUserAndCreatesGuestBinding(t *testing.T) {
	database := newMigratedSQLiteGormDB(t)
	store := NewSQLStore(database)

	user, bindings, err := store.UpsertUserWithGuestBinding(context.Background(), W3Identity{ID: "w3_sql", Name: "王五", EmployeeID: "C789"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if user.ID != "w3_sql" || len(bindings) != 1 || bindings[0].RoleLabel != "游客" {
		t.Fatalf("unexpected summary user=%#v bindings=%#v", user, bindings)
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
	if session, err := store.FindSessionByTokenHash(ctx, second.TokenHash, now); err != nil || session.User.ID != "w3_session" {
		t.Fatalf("expected second session active, session=%#v err=%v", session, err)
	}
}
```

Add the test database helper in the same file:

```go
func newMigratedSQLiteGormDB(t *testing.T) *gorm.DB {
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
	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("new migration provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	return gdb
}
```

- [ ] **Step 2: Run SQL store tests and verify red**

Run:

```bash
cd apps/api && go test ./internal/auth -run TestSQLStore -count=1
```

Expected: FAIL because `SQLStore`, `CreateSession`, and session lookup do not exist.

- [ ] **Step 3: Extend auth interfaces**

Add to `types.go`:

```go
type CreateSessionInput struct {
	UserID        string
	TokenHash     string
	CSRFTokenHash string
	ExpiresAt     time.Time
	Now           time.Time
}

type SessionSummary struct {
	ID            string
	TokenHash     string
	CSRFTokenHash string
	User          UserSummary
	RoleBindings  []RoleBinding
	ExpiresAt     time.Time
}

type Store interface {
	UpsertUserWithGuestBinding(context.Context, W3Identity) (UserSummary, []RoleBinding, error)
	CreateSession(context.Context, CreateSessionInput) (SessionSummary, error)
	FindSessionByTokenHash(context.Context, string, time.Time) (SessionSummary, error)
	RevokeSession(context.Context, string, time.Time) error
	RevokeOtherSessions(context.Context, string, string, time.Time) error
}
```

Update the fake store in `service_test.go` with no-op implementations for the new methods.

- [ ] **Step 4: Implement SQL store**

Create `sql_store.go`:

```go
package auth

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

const systemDepartmentID = "__system__"
const guestRoleID = "__role_guest__"

type SQLStore struct{ db *gorm.DB }

func NewSQLStore(db *gorm.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) UpsertUserWithGuestBinding(ctx context.Context, identity W3Identity) (UserSummary, []RoleBinding, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO users (id, employee_id, name, created_at, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET employee_id = excluded.employee_id, name = excluded.name, updated_at = CURRENT_TIMESTAMP
		`, identity.ID, identity.EmployeeID, identity.Name).Error; err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, 'system')
			ON CONFLICT(user_id, department_id, role_id) DO NOTHING
		`, "udr_"+identity.ID+"_guest", identity.ID, systemDepartmentID, guestRoleID).Error
	})
	if err != nil {
		return UserSummary{}, nil, err
	}
	bindings, err := s.roleBindings(ctx, identity.ID)
	if err != nil {
		return UserSummary{}, nil, err
	}
	return UserSummary{ID: identity.ID, EmployeeID: identity.EmployeeID, Name: identity.Name}, bindings, nil
}

func (s *SQLStore) CreateSession(ctx context.Context, input CreateSessionInput) (SessionSummary, error) {
	id := "session_" + input.TokenHash[:16]
	if err := s.db.WithContext(ctx).Exec(`
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, input.UserID, input.TokenHash, input.CSRFTokenHash, input.ExpiresAt, input.Now, input.Now).Error; err != nil {
		return SessionSummary{}, err
	}
	session, err := s.FindSessionByTokenHash(ctx, input.TokenHash, input.Now)
	if err != nil {
		return SessionSummary{}, err
	}
	return session, nil
}

func (s *SQLStore) FindSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (SessionSummary, error) {
	var row struct{ ID, UserID, EmployeeID, Name, CSRFTokenHash string; ExpiresAt time.Time }
	err := s.db.WithContext(ctx).Raw(`
		SELECT s.id, s.user_id, s.csrf_token_hash, s.expires_at, u.employee_id, u.name
		FROM auth_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ?
	`, tokenHash, now).Scan(&row).Error
	if err != nil {
		return SessionSummary{}, err
	}
	if row.ID == "" {
		return SessionSummary{}, ErrUnauthenticated
	}
	bindings, err := s.roleBindings(ctx, row.UserID)
	if err != nil {
		return SessionSummary{}, err
	}
	return SessionSummary{ID: row.ID, TokenHash: tokenHash, CSRFTokenHash: row.CSRFTokenHash, ExpiresAt: row.ExpiresAt, User: UserSummary{ID: row.UserID, EmployeeID: row.EmployeeID, Name: row.Name}, RoleBindings: bindings}, nil
}

func (s *SQLStore) RevokeSession(ctx context.Context, sessionID string, now time.Time) error {
	return s.db.WithContext(ctx).Exec("UPDATE auth_sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL", now, sessionID).Error
}

func (s *SQLStore) RevokeOtherSessions(ctx context.Context, userID string, keepSessionID string, now time.Time) error {
	return s.db.WithContext(ctx).Exec("UPDATE auth_sessions SET revoked_at = ? WHERE user_id = ? AND id <> ? AND revoked_at IS NULL", now, userID, keepSessionID).Error
}

func (s *SQLStore) roleBindings(ctx context.Context, userID string) ([]RoleBinding, error) {
	var rows []struct {
		RoleLabel      string
		DepartmentID   string
		DepartmentName string
	}
	err := s.db.WithContext(ctx).Raw(`
		SELECT r.label AS role_label, d.id AS department_id, d.name AS department_name
		FROM user_department_roles udr
		JOIN roles r ON r.id = udr.role_id
		JOIN departments d ON d.id = udr.department_id
		WHERE udr.user_id = ?
		ORDER BY r.label, d.name
	`, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	bindings := make([]RoleBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, RoleBinding{RoleLabel: row.RoleLabel, DepartmentID: row.DepartmentID, DepartmentName: row.DepartmentName})
	}
	return bindings, nil
}
```

- [ ] **Step 5: Update service to persist sessions**

In `Login`, after generating tokens, create the session and keep that session while revoking others:

```go
session, err := s.store.CreateSession(ctx, CreateSessionInput{
	UserID: user.ID, TokenHash: HashToken(authToken), CSRFTokenHash: HashToken(csrfToken),
	ExpiresAt: s.now().Add(12 * time.Hour), Now: s.now(),
})
if err != nil {
	return LoginResult{}, err
}
if err := s.store.RevokeOtherSessions(ctx, user.ID, session.ID, s.now()); err != nil {
	return LoginResult{}, err
}
```

- [ ] **Step 6: Run SQL store and service tests**

Run:

```bash
cd apps/api && go test ./internal/auth -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/auth
git commit -m "feat(api): persist auth sessions"
```

## Task 4: HTTP Auth Routes, Cookies, and CSRF

**Files:**
- Modify: `apps/api/internal/http/apperror/error.go`
- Modify: `apps/api/internal/app/server.go`
- Create: `apps/api/internal/app/auth_routes.go`
- Create: `apps/api/internal/app/auth_routes_test.go`

- [ ] **Step 1: Write failing route tests**

Create route tests for login and CSRF:

```go
func TestW3LoginRequiresCSRF(t *testing.T) {
	server := NewServerWithOptions(Options{AuthService: newFakeHTTPAuthService()})
	req := httptest.NewRequest(http.MethodPost, "/auth/w3/login", strings.NewReader(`{"account":"a","password":"p"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "p") {
		t.Fatalf("response leaked password: %s", rec.Body.String())
	}
}

func TestW3LoginSetsAuthAndCSRFCookies(t *testing.T) {
	auth := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: auth, FrontendOrigin: "https://talentpilot.example"})
	req := httptest.NewRequest(http.MethodPost, "/auth/w3/login", strings.NewReader(`{"account":"zhangsan","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://talentpilot.example")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertCookie(t, rec.Result().Cookies(), "tp_auth", true)
	assertCookie(t, rec.Result().Cookies(), "tp_csrf", false)
}
```

Add route test helpers:

```go
type fakeHTTPAuthService struct{}

func newFakeHTTPAuthService() *fakeHTTPAuthService { return &fakeHTTPAuthService{} }

func (f *fakeHTTPAuthService) IssueCSRF(ctx context.Context) (string, error) {
	return "csrf_issued", nil
}

func (f *fakeHTTPAuthService) Login(ctx context.Context, input auth.LoginInput) (auth.LoginResult, error) {
	return auth.LoginResult{
		User: auth.UserSummary{ID: "w3_1", EmployeeID: "A123", Name: "张三"},
		RoleBindings: []auth.RoleBinding{{RoleLabel: "游客", DepartmentID: "__system__", DepartmentName: "system"}},
		RoleLabels: []string{"游客"},
		PageAccess: []string{"resume-parse", "resume-recommend"},
		DefaultRoute: "/resume-parse",
		AuthToken: "auth_after",
		CSRFToken: "csrf_after",
	}, nil
}

func (f *fakeHTTPAuthService) CurrentUser(ctx context.Context, token string) (auth.LoginResult, error) {
	return auth.LoginResult{User: auth.UserSummary{ID: "w3_1", EmployeeID: "A123", Name: "张三"}, RoleLabels: []string{"游客"}, PageAccess: []string{"resume-parse", "resume-recommend"}, DefaultRoute: "/resume-parse"}, nil
}

func (f *fakeHTTPAuthService) Logout(ctx context.Context, token string) error { return nil }

func assertCookie(t *testing.T, cookies []*http.Cookie, name string, httpOnly bool) {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			if cookie.HttpOnly != httpOnly {
				t.Fatalf("expected %s HttpOnly=%v, got %v", name, httpOnly, cookie.HttpOnly)
			}
			if cookie.Value == "" {
				t.Fatalf("expected %s cookie value", name)
			}
			return
		}
	}
	t.Fatalf("expected cookie %s", name)
}
```

- [ ] **Step 2: Run route tests and verify red**

Run:

```bash
cd apps/api && go test ./internal/app -run TestW3Login -count=1
```

Expected: FAIL because `NewServerWithOptions`, auth routes, and CSRF checks do not exist.

- [ ] **Step 3: Add app options and route registration**

Modify `server.go`:

```go
type Options struct {
	AuthService    AuthService
	FrontendOrigin string
	SecureCookies  bool
}

type AuthService interface {
	IssueCSRF(context.Context) (string, error)
	Login(context.Context, auth.LoginInput) (auth.LoginResult, error)
	CurrentUser(context.Context, string) (auth.LoginResult, error)
	Logout(context.Context, string) error
}

func NewServer() *Server {
	return NewServerWithOptions(Options{})
}

func NewServerWithOptions(options Options) *Server {
	apperror.InstallHumaErrorFactory()
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	cfg := huma.DefaultConfig("TalentPilot API", "0.1.0")
	cfg.CreateHooks = nil
	cfg.Transformers = append(cfg.Transformers, apperror.RequestIDTransformer)
	api := humaecho.New(e, cfg)
	registerHealth(api)
	registerAuthRoutes(api, options)
	return &Server{Echo: e, API: api}
}
```

- [ ] **Step 4: Implement auth route DTOs**

Create `auth_routes.go` with DTOs:

```go
type loginInput struct {
	Body struct {
		Account  string `json:"account" minLength:"1"`
		Password string `json:"password" minLength:"1"`
	}
	CSRFHeader string `header:"X-CSRF-Token"`
	CSRFCookie string `cookie:"tp_csrf"`
	Origin     string `header:"Origin"`
	Referer    string `header:"Referer"`
}

type meInput struct {
	AuthCookie string `cookie:"tp_auth"`
}

type authOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      authResponse `json:"body"`
}

type authResponse struct {
	User         auth.UserSummary  `json:"user"`
	RoleBindings []auth.RoleBinding `json:"roleBindings"`
	RoleLabels   []string           `json:"roleLabels"`
	PageAccess   []string           `json:"pageAccess"`
	DefaultRoute  string             `json:"defaultRoute"`
}
```

Register these operations: `get-auth-csrf`, `post-auth-w3-login`, `get-me`, `post-auth-logout`. Map auth errors to `apperror.Problem` codes from Task 5.

- [ ] **Step 5: Run route tests and surrounding app tests**

Run:

```bash
cd apps/api && go test ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/app apps/api/internal/http/apperror/error.go
git commit -m "feat(api): expose auth session routes"
```

## Task 5: App Wiring, OpenAPI, and Generated Client

**Files:**
- Modify: `apps/api/internal/config/config.go`
- Modify: `apps/api/cmd/api/main.go`
- Modify: `apps/api/cmd/openapi/main.go`
- Modify: `apps/api/internal/app/openapi_test.go`
- Modify: `packages/api-contract/openapi.json`
- Modify: `packages/api-client/src/schema.d.ts`

- [ ] **Step 1: Write failing OpenAPI test**

Add to `openapi_test.go`:

```go
func TestOpenAPIDocumentIncludesAuthEndpoints(t *testing.T) {
	server := NewServer()
	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal openapi: %v", err)
	}
	assertOperation(t, doc.Paths, "/auth/csrf", "get", "get-auth-csrf")
	assertOperation(t, doc.Paths, "/auth/w3/login", "post", "post-auth-w3-login")
	assertOperation(t, doc.Paths, "/me", "get", "get-me")
	assertOperation(t, doc.Paths, "/auth/logout", "post", "post-auth-logout")
}
```

Add the helper:

```go
func assertOperation(t *testing.T, paths map[string]map[string]struct{ OperationID string `json:"operationId"` }, path string, method string, operationID string) {
	t.Helper()
	pathItem, ok := paths[path]
	if !ok {
		t.Fatalf("expected OpenAPI path %s", path)
	}
	operation, ok := pathItem[method]
	if !ok {
		t.Fatalf("expected OpenAPI %s %s", method, path)
	}
	if operation.OperationID != operationID {
		t.Fatalf("expected operationId %s, got %s", operationID, operation.OperationID)
	}
}
```

- [ ] **Step 2: Run OpenAPI test and verify red**

Run:

```bash
cd apps/api && go test ./internal/app -run TestOpenAPIDocumentIncludesAuthEndpoints -count=1
```

Expected: FAIL until routes are in `NewServer()`.

- [ ] **Step 3: Wire config and real server startup**

Extend `Config`:

```go
type Config struct {
	Env            string
	APIAddr        string
	DatabaseDriver string
	DatabaseDSN    string
	FrontendOrigin string
	SecureCookies  bool
	W3Mode         string
}
```

In `main.go`, open DB with `platform/db.Open`, create `auth.NewSQLStore`, choose W3 mock only outside production, create `auth.NewService`, then call `app.NewServerWithOptions`.

- [ ] **Step 4: Regenerate OpenAPI and client**

Run:

```bash
make openapi-generate
make client-generate
make openapi-check
make client-check
```

Expected: generation succeeds and checks pass after generated files are committed.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/config/config.go apps/api/cmd/api/main.go apps/api/cmd/openapi/main.go apps/api/internal/app/openapi_test.go packages/api-contract/openapi.json packages/api-client/src/schema.d.ts
git commit -m "feat(api): generate auth contract"
```

## Task 6: Frontend Login and Guest Navigation

**Files:**
- Create: `apps/web/src/api/client.ts`
- Create: `apps/web/src/components/ui/input.tsx`
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing frontend tests**

Replace `App.test.tsx` assertions with login/session behavior:

```tsx
it("renders the company account login form for unauthenticated users", () => {
  render(<App />);

  expect(screen.getByLabelText("公司账号")).toBeInTheDocument();
  expect(screen.getByLabelText("公司密码")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "登录" })).toBeInTheDocument();
});

it("shows only guest navigation after login succeeds", async () => {
  const user = userEvent.setup();
  render(<App />);

  await user.type(screen.getByLabelText("公司账号"), "zhangsan");
  await user.type(screen.getByLabelText("公司密码"), "secret");
  await user.click(screen.getByRole("button", { name: "登录" }));

  expect(await screen.findByRole("link", { name: "简历解析" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "简历推荐" })).toBeInTheDocument();
  expect(screen.queryByText("您当前为游客身份")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "简历库" })).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run frontend tests and verify red**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/app/App.test.tsx
```

Expected: FAIL because the app still renders the foundation shell.

- [ ] **Step 3: Add project input component**

Create `input.tsx`:

```tsx
import * as React from "react";
import { cn } from "./cn";

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

export const Input = React.forwardRef<HTMLInputElement, InputProps>(({ className, ...props }, ref) => (
  <input
    ref={ref}
    className={cn(
      "min-h-11 w-full border border-white/15 bg-white/5 px-3 text-sm text-fg outline-none transition placeholder:text-muted focus-visible:border-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
      className,
    )}
    {...props}
  />
));
Input.displayName = "Input";
```

- [ ] **Step 4: Add frontend API wrapper**

Create `api/client.ts`:

```ts
import { createTalentPilotClient } from "@talentpilot/api-client";

export const apiClient = createTalentPilotClient(import.meta.env.VITE_API_BASE_URL ?? "");

export async function loginWithW3(account: string, password: string) {
  await apiClient.GET("/auth/csrf");
  return apiClient.POST("/auth/w3/login", {
    body: { account, password },
    headers: { "X-CSRF-Token": readCookie("tp_csrf") },
  });
}

function readCookie(name: string) {
  const match = document.cookie.split("; ").find((item) => item.startsWith(`${name}=`));
  return match ? decodeURIComponent(match.split("=")[1] ?? "") : "";
}
```

- [ ] **Step 5: Implement login shell**

Replace `App.tsx` with a login form, guest nav, and logout button. Use only project UI wrappers for interactive elements:

```tsx
type SessionView = {
  user: { id: string; employeeId: string; name: string };
  roleLabels: string[];
  pageAccess: string[];
  defaultRoute: string;
};

const guestLinks = ["简历解析", "简历推荐"];

export function App() {
  const [session, setSession] = React.useState<SessionView | null>(null);
  const [account, setAccount] = React.useState("");
  const [password, setPassword] = React.useState("");

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    const { data, error } = await loginWithW3(account, password);
    if (!error && data) setSession(data);
  }

  if (!session) {
    return <LoginForm account={account} password={password} onAccountChange={setAccount} onPasswordChange={setPassword} onSubmit={onSubmit} />;
  }

  return (
    <main aria-label="TalentPilot 工作台" className="min-h-screen bg-bg text-fg">
      <nav aria-label="主导航" className="flex gap-2 border-b border-white/10 px-6 py-4">
        {guestLinks.map((label) => <a key={label} href="#" className="text-sm text-fg">{label}</a>)}
      </nav>
      <section className="px-6 py-8">
        <h1 className="text-2xl font-semibold">简历解析</h1>
      </section>
    </main>
  );
}

function LoginForm(props: {
  account: string;
  password: string;
  onAccountChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onSubmit: (event: React.FormEvent) => void;
}) {
  return (
    <main aria-label="登录" className="min-h-screen bg-bg px-6 py-10 text-fg">
      <form onSubmit={props.onSubmit} className="mx-auto grid max-w-sm gap-4">
        <label className="grid gap-2 text-sm">
          公司账号
          <Input value={props.account} onChange={(event) => props.onAccountChange(event.target.value)} />
        </label>
        <label className="grid gap-2 text-sm">
          公司密码
          <Input type="password" value={props.password} onChange={(event) => props.onPasswordChange(event.target.value)} />
        </label>
        <Button variant="primary" type="submit">登录</Button>
      </form>
    </main>
  );
}
```

- [ ] **Step 6: Run frontend tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/app/App.test.tsx
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src
git commit -m "feat(web): add W3 login shell"
```

## Task 7: Final Verification and Status

**Files:**
- Modify: `docs/project-status.md`

- [ ] **Step 1: Run focused checks**

Run:

```bash
make test-api
make test-web
make openapi-check
make client-check
make typecheck
```

Expected: all pass.

- [ ] **Step 2: Update project status**

Change the E1 row in `docs/project-status.md` to:

```markdown
| E1 | Login and identity authentication | In Progress | E1 implementation has auth schema, W3 login API, token Cookie sessions, CSRF, `/me`, logout, generated client, and login shell tests passing. | Run full `make ci`; then start `002-iam-permission-model.md`. |
```

- [ ] **Step 3: Commit status update**

```bash
git add docs/project-status.md
git commit -m "docs: update E1 implementation status"
```

- [ ] **Step 4: Run final CI gate**

Run:

```bash
make ci
```

Expected: PASS. If Go is unavailable, report the missing runtime and do not claim API verification passed.
