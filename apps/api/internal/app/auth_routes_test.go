package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/talentpilot/talentpilot/apps/api/internal/auth"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

func TestW3LoginRequiresCSRF(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc})
	req := httptest.NewRequest(http.MethodPost, "/auth/w3/login", strings.NewReader(`{"account":"a","password":"p"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if authSvc.loginCalls != 0 {
		t.Fatalf("expected invalid CSRF not to call login, got %d calls", authSvc.loginCalls)
	}
	if strings.Contains(rec.Body.String(), "p") {
		t.Fatalf("response leaked password: %s", rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "AUTH_CSRF_INVALID")
}

func TestGetAuthCSRFSetsCSRFCookie(t *testing.T) {
	server := NewServerWithOptions(Options{AuthService: newFakeHTTPAuthService()})
	req := httptest.NewRequest(http.MethodGet, "/auth/csrf", nil)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertCookie(t, rec.Result().Cookies(), "tp_csrf", "csrf_issued", false, 0)
}

func TestW3LoginRejectsInvalidOrigin(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc, FrontendOrigin: "https://talentpilot.example"})
	req := httptest.NewRequest(http.MethodPost, "/auth/w3/login", strings.NewReader(`{"account":"a","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if authSvc.loginCalls != 0 {
		t.Fatalf("expected invalid origin not to call login, got %d calls", authSvc.loginCalls)
	}
	assertErrorCode(t, rec.Body.String(), "AUTH_CSRF_INVALID")
}

func TestCORSAllowsConfiguredFrontendCredentialedRequests(t *testing.T) {
	server := NewServerWithOptions(Options{AuthService: newFakeHTTPAuthService(), FrontendOrigin: "http://localhost:5173"})
	req := httptest.NewRequest(http.MethodOptions, "/auth/w3/login", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "X-CSRF-Token, Content-Type")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected CORS preflight 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("unexpected allow origin %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("expected credentialed CORS")
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "X-CSRF-Token") {
		t.Fatalf("expected X-CSRF-Token in allowed headers, got %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestProductionW3LoginRequiresHTTPSBeforeCallingService(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc, FrontendOrigin: "https://talentpilot.example", RequireHTTPS: true})
	req := httptest.NewRequest(http.MethodPost, "/auth/w3/login", strings.NewReader(`{"account":"zhangsan","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://talentpilot.example")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if authSvc.loginCalls != 0 {
		t.Fatalf("expected insecure login not to call service, got %d calls", authSvc.loginCalls)
	}
	assertErrorCode(t, rec.Body.String(), "AUTH_HTTPS_REQUIRED")
}

func TestProductionW3LoginAcceptsTrustedForwardedHTTPS(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc, FrontendOrigin: "https://talentpilot.example", RequireHTTPS: true, TrustForwardedProto: true})
	req := httptest.NewRequest(http.MethodPost, "/auth/w3/login", strings.NewReader(`{"account":"zhangsan","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://talentpilot.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if authSvc.loginCalls != 1 {
		t.Fatalf("expected trusted HTTPS login to call service once, got %d", authSvc.loginCalls)
	}
}

func TestProductionW3LoginRejectsUntrustedForwardedHTTPS(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc, FrontendOrigin: "https://talentpilot.example", RequireHTTPS: true})
	req := httptest.NewRequest(http.MethodPost, "/auth/w3/login", strings.NewReader(`{"account":"zhangsan","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://talentpilot.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if authSvc.loginCalls != 0 {
		t.Fatalf("expected untrusted forwarded proto not to call service, got %d", authSvc.loginCalls)
	}
	assertErrorCode(t, rec.Body.String(), "AUTH_HTTPS_REQUIRED")
}

func TestW3LoginSetsAuthAndCSRFCookies(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc, FrontendOrigin: "https://talentpilot.example"})
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
	assertCookie(t, rec.Result().Cookies(), "tp_auth", "auth_after", true, 0)
	assertCookie(t, rec.Result().Cookies(), "tp_csrf", "csrf_after", false, 0)
	assertAuthResponse(t, rec.Body.String())
	if strings.Contains(rec.Body.String(), "auth_after") || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("response leaked sensitive value: %s", rec.Body.String())
	}
}

func TestMeUsesAuthCookie(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc})
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if authSvc.currentUserToken != "auth_cookie" {
		t.Fatalf("expected auth cookie token passed to service, got %q", authSvc.currentUserToken)
	}
	assertAuthResponse(t, rec.Body.String())
}

func TestMeIncludesIAMPermissionsAndDataScope(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	iamSvc := &fakeIAMService{
		roleSummary: iam.RoleSummary{
			Permissions: []string{"Resume.List", "Position.List"},
			DataScope: iam.DataScope{
				Departments: []iam.DepartmentScope{{ID: "dept_a", Name: "算力训练平台部"}},
				Channels:    []string{"social", "campus"},
			},
			PageAccess:   []string{"resume-parse", "resume-library"},
			DefaultRoute: "/resume-parse",
		},
	}
	server := NewServerWithOptions(Options{AuthService: authSvc, IAMService: iamSvc})
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Permissions []string `json:"permissions"`
		DataScope   struct {
			Departments    []struct{ ID, Name string } `json:"departments"`
			AllDepartments bool                        `json:"allDepartments"`
			Channels       []string                    `json:"channels"`
		} `json:"dataScope"`
		PageAccess []string `json:"pageAccess"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal /me body: %v", err)
	}
	if !containsString(body.Permissions, "Resume.List") || !containsString(body.PageAccess, "resume-library") {
		t.Fatalf("expected IAM permissions and page access, got %#v", body)
	}
	if len(body.DataScope.Departments) != 1 || body.DataScope.Departments[0].Name != "算力训练平台部" || !containsString(body.DataScope.Channels, "campus") {
		t.Fatalf("expected IAM data scope, got %#v", body.DataScope)
	}
}

func TestLogoutRevokesSessionAndExpiresCookies(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc})
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	if authSvc.logoutToken != "auth_cookie" {
		t.Fatalf("expected logout token auth_cookie, got %q", authSvc.logoutToken)
	}
	if authSvc.logoutCSRF != "csrf_before" {
		t.Fatalf("expected logout CSRF csrf_before, got %q", authSvc.logoutCSRF)
	}
	assertCookie(t, rec.Result().Cookies(), "tp_auth", "", true, -1)
	assertCookie(t, rec.Result().Cookies(), "tp_csrf", "", false, -1)
}

func TestAuthenticatedMutationMiddlewareRequiresCSRFForFutureRoutes(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	server := NewServerWithOptions(Options{AuthService: authSvc, FrontendOrigin: "https://talentpilot.example"})
	server.Echo.POST("/future/mutation", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/future/mutation", nil)
	req.Header.Set("Origin", "https://talentpilot.example")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "AUTH_CSRF_INVALID")
}

func TestAuthenticatedMiddlewareRejectsInvalidSessionForFutureRoutes(t *testing.T) {
	authSvc := newFakeHTTPAuthService()
	authSvc.currentUserErr = auth.ErrUnauthenticated
	server := NewServerWithOptions(Options{AuthService: authSvc, FrontendOrigin: "https://talentpilot.example"})
	server.Echo.GET("/future/session", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/future/session", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "AUTH_UNAUTHENTICATED")
}

func TestIAMGuardRejectsAuthenticatedUnauthorizedFutureRoute(t *testing.T) {
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService:  &fakeIAMService{decision: iam.Decision{Allowed: false}},
	})
	server.Echo.GET("/future/iam-denied", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, RequirePermission(iam.ResourceResume, iam.ActionList))

	req := httptest.NewRequest(http.MethodGet, "/future/iam-denied", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
}

func TestIAMGuardAttachesScopePredicateForListRoute(t *testing.T) {
	expectedScope := iam.ScopePredicate{
		Resource: iam.ResourceResume,
		Action:   iam.ActionList,
		Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}},
	}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService:  &fakeIAMService{decision: iam.Decision{Allowed: true}, scope: expectedScope},
	})
	server.Echo.GET("/future/iam-allowed", func(c echo.Context) error {
		scope, ok := ScopePredicateFromContext(c.Request().Context())
		if !ok || len(scope.Branches) != 1 || scope.Branches[0].DepartmentIDs[0] != "dept_a" {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "missing scope"})
		}
		return c.NoContent(http.StatusNoContent)
	}, RequirePermission(iam.ResourceResume, iam.ActionList))

	req := httptest.NewRequest(http.MethodGet, "/future/iam-allowed", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
}

type fakeHTTPAuthService struct {
	loginCalls       int
	currentUserToken string
	logoutToken      string
	logoutCSRF       string
	loginErr         error
	currentUserErr   error
	logoutErr        error
}

type fakeIAMService struct {
	roleSummary iam.RoleSummary
	decision    iam.Decision
	scope       iam.ScopePredicate
}

func (f *fakeIAMService) RoleSummary(ctx context.Context, userID string) (iam.RoleSummary, error) {
	return f.roleSummary, nil
}

func (f *fakeIAMService) Can(ctx context.Context, principal iam.Principal, resource iam.Resource, action iam.Action, target iam.Target) iam.Decision {
	return f.decision
}

func (f *fakeIAMService) Scope(ctx context.Context, principal iam.Principal, resource iam.Resource, action iam.Action) (iam.ScopePredicate, error) {
	return f.scope, nil
}

func (f *fakeIAMService) ResolvePrincipal(ctx context.Context, userID string) (iam.Principal, error) {
	return iam.Principal{User: iam.User{ID: userID}}, nil
}

func newFakeHTTPAuthService() *fakeHTTPAuthService {
	return &fakeHTTPAuthService{}
}

func (f *fakeHTTPAuthService) IssueCSRF(ctx context.Context) (string, error) {
	return "csrf_issued", nil
}

func (f *fakeHTTPAuthService) Login(ctx context.Context, input auth.LoginInput) (auth.LoginResult, error) {
	f.loginCalls++
	if f.loginErr != nil {
		return auth.LoginResult{}, f.loginErr
	}
	return fakeAuthResult(), nil
}

func (f *fakeHTTPAuthService) CurrentUser(ctx context.Context, token string) (auth.LoginResult, error) {
	f.currentUserToken = token
	if f.currentUserErr != nil {
		return auth.LoginResult{}, f.currentUserErr
	}
	if token == "" {
		return auth.LoginResult{}, auth.ErrUnauthenticated
	}
	return fakeAuthResult(), nil
}

func (f *fakeHTTPAuthService) Logout(ctx context.Context, token string, csrfToken string) error {
	f.logoutToken = token
	f.logoutCSRF = csrfToken
	if f.logoutErr != nil {
		return f.logoutErr
	}
	if token == "" {
		return auth.ErrUnauthenticated
	}
	return nil
}

func fakeAuthResult() auth.LoginResult {
	return auth.LoginResult{
		User:         auth.UserSummary{ID: "w3_1", EmployeeID: "A123", Name: "张三"},
		RoleBindings: []auth.RoleBinding{{RoleLabel: "游客", DepartmentID: "__system__", DepartmentName: "system"}},
		RoleLabels:   []string{"游客"},
		PageAccess:   []string{"resume-parse", "resume-recommend"},
		DefaultRoute: "/resume-parse",
		AuthToken:    "auth_after",
		CSRFToken:    "csrf_after",
	}
}

func assertCookie(t *testing.T, cookies []*http.Cookie, name string, value string, httpOnly bool, maxAge int) {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name != name {
			continue
		}
		if cookie.HttpOnly != httpOnly {
			t.Fatalf("expected %s HttpOnly=%v, got %v", name, httpOnly, cookie.HttpOnly)
		}
		if cookie.Value != value {
			t.Fatalf("expected %s value %q, got %q", name, value, cookie.Value)
		}
		if cookie.MaxAge != maxAge {
			t.Fatalf("expected %s MaxAge=%d, got %d", name, maxAge, cookie.MaxAge)
		}
		return
	}
	t.Fatalf("expected cookie %s", name)
}

func assertAuthResponse(t *testing.T, raw string) {
	t.Helper()
	var body struct {
		User struct {
			ID         string `json:"id"`
			EmployeeID string `json:"employeeId"`
			Name       string `json:"name"`
		} `json:"user"`
		RoleLabels   []string `json:"roleLabels"`
		PageAccess   []string `json:"pageAccess"`
		DefaultRoute string   `json:"defaultRoute"`
		AuthToken    string   `json:"authToken"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("unmarshal auth response: %v body=%s", err, raw)
	}
	if body.User.ID != "w3_1" || len(body.RoleLabels) != 1 || body.RoleLabels[0] != "游客" || body.DefaultRoute != "/resume-parse" {
		t.Fatalf("unexpected auth response: %#v", body)
	}
	if body.AuthToken != "" {
		t.Fatalf("auth response must not include raw auth token")
	}
}

func assertErrorCode(t *testing.T, raw string, expected string) {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("unmarshal error response: %v body=%s", err, raw)
	}
	if body.Code != expected {
		t.Fatalf("expected error code %s, got %s body=%s", expected, body.Code, raw)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
