package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/roleadmin"
)

func TestRoleAdminRoutesListRequiresRoleList(t *testing.T) {
	service := &fakeRoleAdminHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceRole, iam.ActionList): {Allowed: false},
			},
		},
		RoleAdminService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
	if service.listCalls != 0 {
		t.Fatalf("service must not be called without Role.List")
	}
}

func TestRoleAdminRoutesPermissionOptionsRequiresPermissionList(t *testing.T) {
	service := &fakeRoleAdminHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourcePermission, iam.ActionList): {Allowed: false},
			},
		},
		RoleAdminService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/roles/permission-options", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
	if service.permissionOptionCalls != 0 {
		t.Fatalf("service must not be called without Permission.List")
	}
}

func TestRoleAdminRoutesCreateRequiresAllMutationPermissions(t *testing.T) {
	service := &fakeRoleAdminHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceRole, iam.ActionCreate):         {Allowed: true},
				iam.PermissionKey(iam.ResourcePermission, iam.ActionCreate):   {Allowed: false},
				iam.PermissionKey(iam.ResourceRoleRelation, iam.ActionCreate): {Allowed: true},
			},
		},
		RoleAdminService: service,
	})
	req := roleAdminJSONRequest(http.MethodPost, "/roles", `{"label":"高级评审者","enabled":true,"permissions":[],"childRoleIds":[]}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
	if service.createCalls != 0 {
		t.Fatalf("service must not be called without all create permissions")
	}
}

func TestRoleAdminRoutesCreatePassesActorAndBody(t *testing.T) {
	service := &fakeRoleAdminHTTPService{createResult: roleadmin.RoleDetail{ID: "role_created", Label: "高级评审者", Enabled: true}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision:  iam.Decision{Allowed: true},
			principal: iam.Principal{User: iam.User{ID: "w3_admin", Name: "管理员"}},
		},
		RoleAdminService: service,
	})
	req := roleAdminJSONRequest(http.MethodPost, "/roles", `{"label":"高级评审者","description":"查看社招","enabled":true,"permissions":[{"resource":"Resume","action":"List","attributeConditions":{"chan":["social"]}}],"childRoleIds":["__role_trainee__"]}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.createCalls != 1 || service.createInput.ActorUserID != "w3_admin" || service.createInput.Label != "高级评审者" {
		t.Fatalf("expected create input with actor and body, calls=%d input=%#v", service.createCalls, service.createInput)
	}
	if len(service.createInput.Permissions) != 1 || service.createInput.Permissions[0].AttributeConditions.Channels[0] != "social" {
		t.Fatalf("expected permission conditions from body, got %#v", service.createInput.Permissions)
	}
}

func TestRoleAdminRoutesUpdateMapsDuplicateLabel(t *testing.T) {
	service := &fakeRoleAdminHTTPService{updateErr: roleadmin.ErrLabelDuplicate}
	server := NewServerWithOptions(Options{
		AuthService:      newFakeHTTPAuthService(),
		IAMService:       &fakeIAMService{decision: iam.Decision{Allowed: true}},
		RoleAdminService: service,
	})
	req := roleAdminJSONRequest(http.MethodPatch, "/roles/role_1", `{"label":"重复","enabled":true,"permissions":[],"childRoleIds":[]}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "ROLE_LABEL_DUPLICATE")
}

func TestRoleAdminRoutesDeleteMapsRoleInUse(t *testing.T) {
	service := &fakeRoleAdminHTTPService{deleteErr: roleadmin.ErrRoleInUse}
	server := NewServerWithOptions(Options{
		AuthService:      newFakeHTTPAuthService(),
		IAMService:       &fakeIAMService{decision: iam.Decision{Allowed: true}, principal: iam.Principal{User: iam.User{ID: "w3_admin"}}},
		RoleAdminService: service,
	})
	req := roleAdminJSONRequest(http.MethodDelete, "/roles/role_1", "")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "ROLE_IN_USE")
}

func TestOpenAPIDocumentIncludesRoleAdminEndpoints(t *testing.T) {
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

	assertOperation(t, doc.Paths, "/roles", "get", "get-roles")
	assertOperation(t, doc.Paths, "/roles/{roleId}", "get", "get-role")
	assertOperation(t, doc.Paths, "/roles/permission-options", "get", "get-role-permission-options")
	assertOperation(t, doc.Paths, "/roles", "post", "post-role")
	assertOperation(t, doc.Paths, "/roles/{roleId}", "patch", "patch-role")
	assertOperation(t, doc.Paths, "/roles/{roleId}/enabled", "patch", "patch-role-enabled")
	assertOperation(t, doc.Paths, "/roles/{roleId}", "delete", "delete-role")
}

type fakeRoleAdminHTTPService struct {
	listCalls             int
	listQuery             roleadmin.RoleListQuery
	listResult            roleadmin.RoleListResult
	detailCalls           int
	permissionOptionCalls int
	permissionOptions     roleadmin.PermissionOptionsResult
	createCalls           int
	createInput           roleadmin.RoleDefinitionInput
	createResult          roleadmin.RoleDetail
	updateCalls           int
	updateInput           roleadmin.RoleDefinitionInput
	updateErr             error
	toggleCalls           int
	deleteCalls           int
	deleteErr             error
}

func (f *fakeRoleAdminHTTPService) ListRoles(ctx context.Context, query roleadmin.RoleListQuery) (roleadmin.RoleListResult, error) {
	f.listCalls++
	f.listQuery = query
	return f.listResult, nil
}

func (f *fakeRoleAdminHTTPService) GetRole(ctx context.Context, roleID string, query roleadmin.RoleCapabilityQuery) (roleadmin.RoleDetail, error) {
	f.detailCalls++
	return roleadmin.RoleDetail{ID: roleID, Label: "高级评审者"}, nil
}

func (f *fakeRoleAdminHTTPService) PermissionOptions(ctx context.Context) (roleadmin.PermissionOptionsResult, error) {
	f.permissionOptionCalls++
	return f.permissionOptions, nil
}

func (f *fakeRoleAdminHTTPService) CreateRole(ctx context.Context, input roleadmin.RoleDefinitionInput) (roleadmin.RoleDetail, error) {
	f.createCalls++
	f.createInput = input
	return f.createResult, nil
}

func (f *fakeRoleAdminHTTPService) UpdateRole(ctx context.Context, roleID string, input roleadmin.RoleDefinitionInput) (roleadmin.RoleDetail, error) {
	f.updateCalls++
	f.updateInput = input
	if f.updateErr != nil {
		return roleadmin.RoleDetail{}, f.updateErr
	}
	return roleadmin.RoleDetail{ID: roleID, Label: input.Label, Enabled: input.Enabled}, nil
}

func (f *fakeRoleAdminHTTPService) ToggleEnabled(ctx context.Context, roleID string, input roleadmin.ToggleEnabledInput) (roleadmin.RoleDetail, error) {
	f.toggleCalls++
	return roleadmin.RoleDetail{ID: roleID, Enabled: input.Enabled}, nil
}

func (f *fakeRoleAdminHTTPService) DeleteRole(ctx context.Context, roleID string, actorUserID string) error {
	f.deleteCalls++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

func roleAdminJSONRequest(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	return req
}
