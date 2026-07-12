package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/useradmin"
)

func TestUserAdminRoutesListUsersPassesBindingScope(t *testing.T) {
	service := &fakeUserAdminHTTPService{listResult: useradmin.UserListResult{
		Items: []useradmin.UserSummary{{
			ID:         "user_a",
			EmployeeID: "A10001",
			Name:       "张敏",
			RoleBindings: []useradmin.RoleBindingDetail{{
				ID:         "udr_a",
				Role:       useradmin.RoleSummary{ID: iam.RoleHRBP, Label: "HRBP", IsSystem: true, Enabled: true},
				Department: useradmin.DepartmentSummary{ID: "dept_a", Name: "算力训练平台部"},
				CanDelete:  true,
			}},
			CanAssign: true,
		}},
		DataScopeSummary: "负责部门:算力训练平台部",
		CanAssignRoles:   true,
	}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scopes: map[string]iam.ScopePredicate{
				iam.PermissionKey(iam.ResourceUser, iam.ActionList):               allScope(iam.ResourceUser, iam.ActionList),
				iam.PermissionKey(iam.ResourceUserDepartmentRole, iam.ActionList): scopedUserDepartmentRole(iam.ActionList, "dept_a"),
				iam.PermissionKey(iam.ResourceUserDepartmentRole, iam.ActionCreate): {
					Resource: iam.ResourceUserDepartmentRole,
					Action:   iam.ActionCreate,
					Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}},
				},
				iam.PermissionKey(iam.ResourceUserDepartmentRole, iam.ActionDelete): scopedUserDepartmentRole(iam.ActionDelete, "dept_a"),
			},
		},
		UserAdminService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/users?search=张&limit=25", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.listCalls != 1 || service.listQuery.Search != "张" || service.listQuery.Limit != 25 {
		t.Fatalf("expected list query forwarded, calls=%d query=%#v", service.listCalls, service.listQuery)
	}
	if service.listQuery.ListScope.Branches[0].DepartmentIDs[0] != "dept_a" || !service.listQuery.CanAssign {
		t.Fatalf("expected binding scope and canAssign, got %#v", service.listQuery)
	}
	if !strings.Contains(rec.Body.String(), "user_a") {
		t.Fatalf("expected user list response, got %s", rec.Body.String())
	}
}

func TestUserAdminRoutesDenyListWithoutBindingListPermission(t *testing.T) {
	service := &fakeUserAdminHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceUser, iam.ActionList):               {Allowed: true},
				iam.PermissionKey(iam.ResourceUserDepartmentRole, iam.ActionList): {Allowed: false},
			},
		},
		UserAdminService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
	if service.listCalls != 0 {
		t.Fatalf("service must not be called when binding list permission is missing")
	}
}

func TestUserAdminRoutesAssignableRolesRequiresCreatePermission(t *testing.T) {
	service := &fakeUserAdminHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceUserDepartmentRole, iam.ActionCreate): {Allowed: false},
			},
		},
		UserAdminService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/roles/assignable", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
	if service.assignableCalls != 0 {
		t.Fatalf("service must not be called without create permission")
	}
}

func TestUserAdminRoutesCreateBindings(t *testing.T) {
	service := &fakeUserAdminHTTPService{createResult: useradmin.CreateRoleBindingsResult{
		User:    useradmin.UserIdentity{ID: "user_a", EmployeeID: "A10001", Name: "张敏"},
		Created: []useradmin.RoleBindingDetail{{ID: "udr_new", Role: useradmin.RoleSummary{ID: iam.RoleManager, Label: "主管"}}},
		Message: "已为 张敏 分配 1 个角色绑定",
	}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scopes: map[string]iam.ScopePredicate{
				iam.PermissionKey(iam.ResourceUserDepartmentRole, iam.ActionCreate): scopedUserDepartmentRole(iam.ActionCreate, "dept_a"),
			},
			principal: iam.Principal{User: iam.User{ID: "w3_1", Name: "张三"}},
		},
		UserAdminService: service,
	})
	req := userAdminJSONRequest(http.MethodPost, "/users/user_a/role-bindings", `{"bindings":[{"departmentId":"dept_a","roleId":"__role_manager__"}]}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.createCalls != 1 || service.createInput.ActorUserID != "w3_1" || service.createInput.UserID != "user_a" {
		t.Fatalf("expected create input with actor and target, calls=%d input=%#v", service.createCalls, service.createInput)
	}
	if service.createInput.Bindings[0].DepartmentID != "dept_a" || service.createInput.Bindings[0].RoleID != iam.RoleManager {
		t.Fatalf("expected binding request from body, got %#v", service.createInput.Bindings)
	}
}

func TestUserAdminRoutesDeleteMapsGuestProtection(t *testing.T) {
	service := &fakeUserAdminHTTPService{deleteErr: useradmin.ErrGuestBindingProtected}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scope:    scopedUserDepartmentRole(iam.ActionDelete, "dept_a"),
		},
		UserAdminService: service,
	})
	req := userAdminJSONRequest(http.MethodDelete, "/users/user_a/role-bindings/udr_guest", "")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "USER_ROLE_BINDING_GUEST_PROTECTED")
}

func TestOpenAPIDocumentIncludesUserAdminEndpoints(t *testing.T) {
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

	assertOperation(t, doc.Paths, "/users", "get", "get-users")
	assertOperation(t, doc.Paths, "/users/{userId}", "get", "get-user")
	assertOperation(t, doc.Paths, "/roles/assignable", "get", "get-assignable-roles")
	assertOperation(t, doc.Paths, "/users/{userId}/role-bindings", "post", "post-user-role-bindings")
	assertOperation(t, doc.Paths, "/users/{userId}/role-bindings/{bindingId}", "delete", "delete-user-role-binding")
}

type fakeUserAdminHTTPService struct {
	listCalls       int
	listQuery       useradmin.ListUsersQuery
	listResult      useradmin.UserListResult
	assignableCalls int
	assignableRoles useradmin.AssignableRoleListResult
	createCalls     int
	createInput     useradmin.CreateRoleBindingsInput
	createResult    useradmin.CreateRoleBindingsResult
	deleteCalls     int
	deleteInput     useradmin.DeleteRoleBindingInput
	deleteResult    useradmin.DeleteRoleBindingResult
	deleteErr       error
}

func (f *fakeUserAdminHTTPService) ListUsers(ctx context.Context, query useradmin.ListUsersQuery) (useradmin.UserListResult, error) {
	f.listCalls++
	f.listQuery = query
	return f.listResult, nil
}

func (f *fakeUserAdminHTTPService) GetUser(ctx context.Context, userID string, scope iam.ScopePredicate) (useradmin.UserDetail, error) {
	return useradmin.UserDetail{ID: userID, EmployeeID: "A10001", Name: "张敏"}, nil
}

func (f *fakeUserAdminHTTPService) ListAssignableRoles(ctx context.Context) (useradmin.AssignableRoleListResult, error) {
	f.assignableCalls++
	return f.assignableRoles, nil
}

func (f *fakeUserAdminHTTPService) CreateRoleBindings(ctx context.Context, input useradmin.CreateRoleBindingsInput) (useradmin.CreateRoleBindingsResult, error) {
	f.createCalls++
	f.createInput = input
	return f.createResult, nil
}

func (f *fakeUserAdminHTTPService) DeleteRoleBinding(ctx context.Context, input useradmin.DeleteRoleBindingInput) (useradmin.DeleteRoleBindingResult, error) {
	f.deleteCalls++
	f.deleteInput = input
	if f.deleteErr != nil {
		return useradmin.DeleteRoleBindingResult{}, f.deleteErr
	}
	return f.deleteResult, nil
}

func scopedUserDepartmentRole(action iam.Action, departmentID string) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourceUserDepartmentRole,
		Action:   action,
		Branches: []iam.ScopeBranch{{DepartmentIDs: []string{departmentID}}},
	}
}

func userAdminJSONRequest(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	return req
}
