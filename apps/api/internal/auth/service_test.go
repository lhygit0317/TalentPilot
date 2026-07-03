package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
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
	if len(result.RoleLabels) != 1 || result.RoleLabels[0] != "游客" {
		t.Fatalf("expected guest role label, got %v", result.RoleLabels)
	}
	if len(result.PageAccess) != 2 || result.PageAccess[0] != "resume-parse" || result.PageAccess[1] != "resume-recommend" {
		t.Fatalf("expected guest page access, got %v", result.PageAccess)
	}
	if result.DefaultRoute != "/resume-parse" {
		t.Fatalf("expected default route /resume-parse, got %q", result.DefaultRoute)
	}
	if store.upserted.ID != "w3_1" || !store.createdGuestBinding {
		t.Fatalf("expected user upsert with guest binding, got upserted=%#v guest=%v", store.upserted, store.createdGuestBinding)
	}
	if store.createdSession.UserID != "w3_1" || store.createdSession.TokenHash != HashToken("auth_raw") || store.createdSession.CSRFTokenHash != HashToken("csrf_raw") {
		t.Fatalf("expected hashed session creation, got %#v", store.createdSession)
	}
	if !store.createdSession.ExpiresAt.Equal(fixedNow().Add(12 * time.Hour)) {
		t.Fatalf("expected 12h session expiry, got %s", store.createdSession.ExpiresAt)
	}
	if len(store.revokedForUser) != 1 || store.revokedForUser[0] != "w3_1" {
		t.Fatalf("expected old sessions revoked for w3_1, got %v", store.revokedForUser)
	}
	if store.keepSessionID != "session_fake" {
		t.Fatalf("expected new session retained while revoking others, got %q", store.keepSessionID)
	}
}

func TestLoginUsesAtomicSessionRotation(t *testing.T) {
	store := newFakeStore()
	store.rejectSeparateSessionOps = true
	w3 := &fakeW3{identity: W3Identity{ID: "w3_atomic", Name: "孙八", EmployeeID: "F001"}}
	service := NewService(ServiceConfig{W3: w3, Store: store, TokenSource: fixedTokenSource("auth_atomic", "csrf_atomic"), Now: fixedNow})

	if _, err := service.Login(context.Background(), LoginInput{Account: "sunba", Password: "secret"}); err != nil {
		t.Fatalf("login: %v", err)
	}
	if !store.rotateCalled {
		t.Fatalf("expected login to use atomic session rotation")
	}
}

func TestCurrentUserLoadsSessionByHashedToken(t *testing.T) {
	store := newFakeStore()
	store.session = SessionSummary{
		ID:            "session_current",
		TokenHash:     HashToken("auth_current"),
		CSRFTokenHash: HashToken("csrf_current"),
		User:          UserSummary{ID: "w3_current", Name: "当前用户", EmployeeID: "H001"},
		RoleBindings:  []RoleBinding{{RoleLabel: "游客", DepartmentID: "__system__", DepartmentName: "system"}},
		ExpiresAt:     fixedNow().Add(time.Hour),
	}
	service := NewService(ServiceConfig{Store: store, TokenSource: fixedTokenSource("unused", "csrf"), Now: fixedNow})

	result, err := service.CurrentUser(context.Background(), "auth_current")
	if err != nil {
		t.Fatalf("current user: %v", err)
	}
	if store.findTokenHash != HashToken("auth_current") {
		t.Fatalf("expected hashed token lookup, got %q", store.findTokenHash)
	}
	if result.User.ID != "w3_current" || len(result.RoleLabels) != 1 || result.DefaultRoute != "/resume-parse" {
		t.Fatalf("unexpected current user result: %#v", result)
	}
}

func TestLogoutValidatesSessionCSRFAndRevokesCurrentSession(t *testing.T) {
	store := newFakeStore()
	store.session = SessionSummary{
		ID:            "session_logout",
		TokenHash:     HashToken("auth_logout"),
		CSRFTokenHash: HashToken("csrf_logout"),
		User:          UserSummary{ID: "w3_logout", Name: "退出用户", EmployeeID: "H002"},
	}
	service := NewService(ServiceConfig{Store: store, TokenSource: fixedTokenSource("unused", "csrf"), Now: fixedNow})

	if err := service.Logout(context.Background(), "auth_logout", "wrong_csrf"); !errors.Is(err, ErrCSRFInvalid) {
		t.Fatalf("expected CSRF error, got %v", err)
	}
	if store.revokedSessionID != "" {
		t.Fatalf("expected CSRF failure not to revoke session, got %q", store.revokedSessionID)
	}

	if err := service.Logout(context.Background(), "auth_logout", "csrf_logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if store.revokedSessionID != "session_logout" {
		t.Fatalf("expected current session revoked, got %q", store.revokedSessionID)
	}
}

func TestLoginRecordsAuditWithoutPassword(t *testing.T) {
	auditLog := &fakeAuditRecorder{}
	failingService := NewService(ServiceConfig{
		W3:          &fakeW3{errors: []error{ErrInvalidCredentials}},
		Store:       newFakeStore(),
		TokenSource: fixedTokenSource("auth_fail", "csrf_fail"),
		Now:         fixedNow,
		Audit:       auditLog,
	})

	_, err := failingService.Login(context.Background(), LoginInput{Account: "bad", Password: "secret-password"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	assertAuditEvent(t, auditLog.events, audit.EventLoginFailed, "", "secret-password")

	auditLog.events = nil
	successService := NewService(ServiceConfig{
		W3:          &fakeW3{identity: W3Identity{ID: "w3_audit", Name: "审计用户", EmployeeID: "A001"}},
		Store:       newFakeStore(),
		TokenSource: fixedTokenSource("auth_success", "csrf_success"),
		Now:         fixedNow,
		Audit:       auditLog,
	})

	if _, err := successService.Login(context.Background(), LoginInput{Account: "audited", Password: "secret-password"}); err != nil {
		t.Fatalf("login: %v", err)
	}
	assertAuditEvent(t, auditLog.events, audit.EventLoginSucceeded, "w3_audit", "secret-password")
}

func TestLogoutRecordsAuditWithoutTokens(t *testing.T) {
	auditLog := &fakeAuditRecorder{}
	store := newFakeStore()
	store.session = SessionSummary{
		ID:            "session_logout",
		TokenHash:     HashToken("auth_logout"),
		CSRFTokenHash: HashToken("csrf_logout"),
		User:          UserSummary{ID: "w3_logout", Name: "退出用户", EmployeeID: "H002"},
	}
	service := NewService(ServiceConfig{Store: store, TokenSource: fixedTokenSource("unused", "csrf"), Now: fixedNow, Audit: auditLog})

	if err := service.Logout(context.Background(), "auth_logout", "csrf_logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	assertAuditEvent(t, auditLog.events, audit.EventLogoutSucceeded, "w3_logout", "auth_logout")
	assertAuditEvent(t, auditLog.events, audit.EventLogoutSucceeded, "w3_logout", "csrf_logout")
}

func TestIssueCSRFReturnsRawCSRFToken(t *testing.T) {
	service := NewService(ServiceConfig{Store: newFakeStore(), TokenSource: fixedTokenSource("unused_auth", "csrf_issued"), Now: fixedNow})

	token, err := service.IssueCSRF(context.Background())
	if err != nil {
		t.Fatalf("issue csrf: %v", err)
	}
	if token != "csrf_issued" {
		t.Fatalf("expected raw CSRF token, got %q", token)
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

func fixedNow() time.Time {
	return time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)
}

func fixedTokenSource(authToken string, csrfToken string) TokenSource {
	return func() (string, string, error) {
		return authToken, csrfToken, nil
	}
}

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
	upsertCalled             bool
	upserted                 W3Identity
	createdGuestBinding      bool
	createdSession           CreateSessionInput
	keepSessionID            string
	rotateCalled             bool
	rejectSeparateSessionOps bool
	session                  SessionSummary
	findTokenHash            string
	revokedSessionID         string
	revokedForUser           []string
}

type fakeAuditRecorder struct {
	events []audit.Event
}

func (r *fakeAuditRecorder) Record(ctx context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func assertAuditEvent(t *testing.T, events []audit.Event, expectedType audit.EventType, expectedUserID string, forbidden string) {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("expected one audit event, got %#v", events)
	}
	event := events[0]
	if event.Type != expectedType {
		t.Fatalf("expected audit type %q, got %#v", expectedType, event)
	}
	if expectedUserID != "" && event.UserID != expectedUserID {
		t.Fatalf("expected audit user %q, got %#v", expectedUserID, event)
	}
	if strings.Contains(fmt.Sprintf("%#v", event), forbidden) {
		t.Fatalf("audit event leaked forbidden value %q: %#v", forbidden, event)
	}
}

func newFakeStore() *fakeStore {
	return &fakeStore{}
}

func (s *fakeStore) UpsertUserWithGuestBinding(ctx context.Context, identity W3Identity) (UserSummary, []RoleBinding, error) {
	s.upsertCalled = true
	s.upserted = identity
	s.createdGuestBinding = true
	return UserSummary{ID: identity.ID, Name: identity.Name, EmployeeID: identity.EmployeeID}, []RoleBinding{{RoleLabel: "游客", DepartmentID: "__system__", DepartmentName: "system"}}, nil
}

func (s *fakeStore) CreateSession(ctx context.Context, input CreateSessionInput) (SessionSummary, error) {
	if s.rejectSeparateSessionOps {
		return SessionSummary{}, errors.New("separate session creation is not atomic")
	}
	s.createdSession = input
	return SessionSummary{ID: "session_fake", TokenHash: input.TokenHash, CSRFTokenHash: input.CSRFTokenHash, ExpiresAt: input.ExpiresAt}, nil
}

func (s *fakeStore) RotateSession(ctx context.Context, input CreateSessionInput) (SessionSummary, error) {
	s.rotateCalled = true
	s.createdSession = input
	s.keepSessionID = "session_fake"
	s.revokedForUser = append(s.revokedForUser, input.UserID)
	return SessionSummary{ID: "session_fake", TokenHash: input.TokenHash, CSRFTokenHash: input.CSRFTokenHash, ExpiresAt: input.ExpiresAt}, nil
}

func (s *fakeStore) FindSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (SessionSummary, error) {
	s.findTokenHash = tokenHash
	if s.session.ID == "" || s.session.TokenHash != tokenHash {
		return SessionSummary{}, ErrUnauthenticated
	}
	return s.session, nil
}

func (s *fakeStore) RevokeSession(ctx context.Context, sessionID string, now time.Time) error {
	s.revokedSessionID = sessionID
	return nil
}

func (s *fakeStore) RevokeOtherSessions(ctx context.Context, userID string, keepSessionID string, now time.Time) error {
	if s.rejectSeparateSessionOps {
		return errors.New("separate session revocation is not atomic")
	}
	s.revokedForUser = append(s.revokedForUser, userID)
	s.keepSessionID = keepSessionID
	return nil
}
