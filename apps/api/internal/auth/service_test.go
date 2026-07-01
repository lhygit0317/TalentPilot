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
	upsertCalled        bool
	upserted            W3Identity
	createdGuestBinding bool
	revokedForUser      []string
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

func (s *fakeStore) RevokeOtherSessions(ctx context.Context, userID string, keepSessionID string, now time.Time) error {
	s.revokedForUser = append(s.revokedForUser, userID)
	return nil
}
