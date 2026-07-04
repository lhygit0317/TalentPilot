package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/organization"
)

func TestOrganizationRoutesListDepartments(t *testing.T) {
	organizationSvc := &fakeOrganizationService{
		departmentListResult: organization.DepartmentListResult{
			Items: []organization.DepartmentListItem{{ID: "dept_a", Name: "算力训练平台部", PositionCount: 1, ResumeCount: 2}},
		},
	}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scopes: map[string]iam.ScopePredicate{
				iam.PermissionKey(iam.ResourceDepartment, iam.ActionList):   scopedDepartment(iam.ActionList, "dept_a"),
				iam.PermissionKey(iam.ResourceDepartment, iam.ActionGet):    scopedDepartment(iam.ActionGet, "dept_a"),
				iam.PermissionKey(iam.ResourceDepartment, iam.ActionUpdate): scopedDepartment(iam.ActionUpdate, "dept_a"),
				iam.PermissionKey(iam.ResourceDepartment, iam.ActionDelete): scopedDepartment(iam.ActionDelete, "dept_a"),
			},
		},
		OrganizationService: organizationSvc,
	})
	req := httptest.NewRequest(http.MethodGet, "/departments?search=算力&limit=25", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if organizationSvc.departmentListQuery.Search != "算力" || organizationSvc.departmentListQuery.Limit != 25 {
		t.Fatalf("expected department query forwarded, got %#v", organizationSvc.departmentListQuery)
	}
	if len(organizationSvc.departmentListQuery.Scope.Branches) != 1 || organizationSvc.departmentListQuery.Scope.Branches[0].DepartmentIDs[0] != "dept_a" {
		t.Fatalf("expected list scope forwarded, got %#v", organizationSvc.departmentListQuery.Scope)
	}
	if !strings.Contains(rec.Body.String(), "dept_a") {
		t.Fatalf("expected department response body, got %s", rec.Body.String())
	}
}

func TestOrganizationRoutesRejectDepartmentDeleteWithRelations(t *testing.T) {
	organizationSvc := &fakeOrganizationService{deleteDepartmentErr: organization.ErrDepartmentDeleteHasRelations}
	server := NewServerWithOptions(Options{
		AuthService:         newFakeHTTPAuthService(),
		IAMService:          &fakeIAMService{decision: iam.Decision{Allowed: true}, scope: allScope(iam.ResourceDepartment, iam.ActionDelete)},
		OrganizationService: organizationSvc,
	})
	req := httptest.NewRequest(http.MethodDelete, "/departments/dept_a", nil)
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "DEPARTMENT_DELETE_HAS_RELATIONS")
}

func TestOrganizationRoutesCreatePosition(t *testing.T) {
	organizationSvc := &fakeOrganizationService{
		createPositionResult: organization.PositionDetail{
			PositionListItem: organization.PositionListItem{
				ID:         "position_new",
				Name:       "平台工程师",
				Department: organization.PositionDepartmentSummary{ID: "dept_a", Name: "算力训练平台部"},
				Chan:       "social",
				Status:     "on",
			},
			Keywords: []string{"Go"},
		},
	}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scopes: map[string]iam.ScopePredicate{
				iam.PermissionKey(iam.ResourcePosition, iam.ActionCreate):           allScope(iam.ResourcePosition, iam.ActionCreate),
				iam.PermissionKey(iam.ResourceDepartmentPosition, iam.ActionCreate): allScope(iam.ResourceDepartmentPosition, iam.ActionCreate),
			},
		},
		OrganizationService: organizationSvc,
	})
	body := `{"name":"平台工程师","departmentId":"dept_a","chan":"social","level":"P6","status":"on","duties":["负责训练平台"],"must":["熟悉 Go"],"keywords":["Go"],"implicitTags":[{"name":"系统设计","w":40}]}`
	req := httptest.NewRequest(http.MethodPost, "/positions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if organizationSvc.createPositionCalls != 1 || organizationSvc.createPositionInput.ActorUserID != "w3_1" {
		t.Fatalf("expected create position service call, got calls=%d input=%#v", organizationSvc.createPositionCalls, organizationSvc.createPositionInput)
	}
	if organizationSvc.createPositionInput.DepartmentID != "dept_a" || organizationSvc.createPositionInput.ImplicitTags[0].Weight == nil || *organizationSvc.createPositionInput.ImplicitTags[0].Weight != 40 {
		t.Fatalf("expected position input from body, got %#v", organizationSvc.createPositionInput)
	}
}

func TestOrganizationRoutesDenyPositionWriteWithoutPermission(t *testing.T) {
	organizationSvc := &fakeOrganizationService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourcePosition, iam.ActionCreate):           {Allowed: true},
				iam.PermissionKey(iam.ResourceDepartmentPosition, iam.ActionCreate): {Allowed: false},
			},
		},
		OrganizationService: organizationSvc,
	})
	body := `{"name":"平台工程师","departmentId":"dept_a","chan":"social","level":"P6","status":"on","duties":[],"must":[],"keywords":[],"implicitTags":[]}`
	req := httptest.NewRequest(http.MethodPost, "/positions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
	if organizationSvc.createPositionCalls != 0 {
		t.Fatalf("expected denial before service call, got calls=%d", organizationSvc.createPositionCalls)
	}
}

func TestOpenAPIDocumentIncludesOrganizationEndpoints(t *testing.T) {
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

	assertOperation(t, doc.Paths, "/departments", "get", "get-departments")
	assertOperation(t, doc.Paths, "/departments/{departmentId}", "get", "get-department")
	assertOperation(t, doc.Paths, "/departments", "post", "post-department")
	assertOperation(t, doc.Paths, "/departments/{departmentId}", "patch", "patch-department")
	assertOperation(t, doc.Paths, "/departments/{departmentId}", "delete", "delete-department")
	assertOperation(t, doc.Paths, "/positions", "get", "get-positions")
	assertOperation(t, doc.Paths, "/positions/{positionId}", "get", "get-position")
	assertOperation(t, doc.Paths, "/positions", "post", "post-position")
	assertOperation(t, doc.Paths, "/positions/{positionId}", "patch", "patch-position")
	assertOperation(t, doc.Paths, "/positions/{positionId}", "delete", "delete-position")
}

type fakeOrganizationService struct {
	departmentListQuery  organization.DepartmentListQuery
	departmentListResult organization.DepartmentListResult
	deleteDepartmentErr  error
	deleteDepartmentID   string
	createPositionCalls  int
	createPositionInput  organization.PositionInput
	createPositionResult organization.PositionDetail
}

func (f *fakeOrganizationService) ListDepartments(ctx context.Context, query organization.DepartmentListQuery) (organization.DepartmentListResult, error) {
	f.departmentListQuery = query
	return f.departmentListResult, nil
}

func (f *fakeOrganizationService) GetDepartment(ctx context.Context, id string, scope iam.ScopePredicate) (organization.DepartmentDetail, error) {
	return organization.DepartmentDetail{DepartmentListItem: organization.DepartmentListItem{ID: id, Name: "算力训练平台部"}}, nil
}

func (f *fakeOrganizationService) CreateDepartment(ctx context.Context, input organization.DepartmentInput) (organization.DepartmentDetail, error) {
	return organization.DepartmentDetail{DepartmentListItem: organization.DepartmentListItem{ID: "dept_new", Name: input.Name}}, nil
}

func (f *fakeOrganizationService) UpdateDepartment(ctx context.Context, id string, input organization.DepartmentInput, scope iam.ScopePredicate) (organization.DepartmentDetail, error) {
	return organization.DepartmentDetail{DepartmentListItem: organization.DepartmentListItem{ID: id, Name: input.Name}}, nil
}

func (f *fakeOrganizationService) DeleteDepartment(ctx context.Context, id string, scope iam.ScopePredicate, actorUserID string) error {
	f.deleteDepartmentID = id
	return f.deleteDepartmentErr
}

func (f *fakeOrganizationService) ListPositions(ctx context.Context, query organization.PositionListQuery) (organization.PositionListResult, error) {
	return organization.PositionListResult{}, nil
}

func (f *fakeOrganizationService) GetPosition(ctx context.Context, id string, scope iam.ScopePredicate) (organization.PositionDetail, error) {
	return organization.PositionDetail{PositionListItem: organization.PositionListItem{ID: id, Name: "平台工程师"}}, nil
}

func (f *fakeOrganizationService) CreatePosition(ctx context.Context, input organization.PositionInput) (organization.PositionDetail, error) {
	f.createPositionCalls++
	f.createPositionInput = input
	if f.createPositionResult.ID != "" {
		return f.createPositionResult, nil
	}
	return organization.PositionDetail{PositionListItem: organization.PositionListItem{ID: "position_new", Name: input.Name}}, nil
}

func (f *fakeOrganizationService) UpdatePosition(ctx context.Context, id string, input organization.PositionInput, scope iam.ScopePredicate) (organization.PositionDetail, error) {
	return organization.PositionDetail{PositionListItem: organization.PositionListItem{ID: id, Name: input.Name}}, nil
}

func (f *fakeOrganizationService) DeletePosition(ctx context.Context, id string, scope iam.ScopePredicate, actorUserID string) error {
	return nil
}

func scopedDepartment(action iam.Action, departmentID string) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourceDepartment,
		Action:   action,
		Branches: []iam.ScopeBranch{{DepartmentIDs: []string{departmentID}}},
	}
}

func allScope(resource iam.Resource, action iam.Action) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: resource,
		Action:   action,
		Branches: []iam.ScopeBranch{{AllDepartments: true}},
	}
}
