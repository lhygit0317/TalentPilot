package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/recommendation"
)

func TestRecommendationSendRouteRequiresNotificationCreate(t *testing.T) {
	service := &fakeRecommendationHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceResume, iam.ActionGet):              {Allowed: true},
				iam.PermissionKey(iam.ResourceResume, iam.ActionCreate):           {Allowed: true},
				iam.PermissionKey(iam.ResourceDepartmentResume, iam.ActionCreate): {Allowed: true},
				iam.PermissionKey(iam.ResourcePositionResume, iam.ActionCreate):   {Allowed: true},
				iam.PermissionKey(iam.ResourceNotification, iam.ActionCreate):     {Allowed: false},
			},
		},
		RecommendationService: service,
	})
	req := recommendationJSONRequest(http.MethodPost, "/recommendations/send", `{"resumeId":"resume_1","departmentId":"dept_a","positionId":"position_a"}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with %s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
	if service.sendCalls != 0 {
		t.Fatalf("service must not be called when permission is missing")
	}
}

func TestRecommendationRoutePassesScopesToService(t *testing.T) {
	service := &fakeRecommendationHTTPService{routeResult: recommendation.RouteResult{
		Resume: recommendation.RecommendationResumeSummary{ID: "resume_1", Name: "张三", Channel: "social"},
		Routes: []recommendation.RouteRow{{Department: recommendation.RecommendationDepartmentSummary{ID: "dept_a", Name: "智算调度部"}, Best: true}},
	}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scopes: map[string]iam.ScopePredicate{
				iam.PermissionKey(iam.ResourceResume, iam.ActionGet): {
					Resource: iam.ResourceResume,
					Action:   iam.ActionGet,
					Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_source"}, Channels: []string{"social"}}},
				},
				iam.PermissionKey(iam.ResourcePosition, iam.ActionList): {
					Resource: iam.ResourcePosition,
					Action:   iam.ActionList,
					Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}},
				},
			},
			principal: iam.Principal{User: iam.User{ID: "w3_1", Name: "张三"}},
		},
		RecommendationService: service,
	})
	req := recommendationJSONRequest(http.MethodPost, "/recommendations/route", `{"resumeId":"resume_1"}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.routeCalls != 1 || service.routeInput.ActorUserID != "w3_1" {
		t.Fatalf("expected route service call with actor, calls=%d input=%#v", service.routeCalls, service.routeInput)
	}
	if service.routeInput.ResumeScope.Branches[0].Channels[0] != "social" {
		t.Fatalf("expected resume scope passed, got %#v", service.routeInput.ResumeScope)
	}
	if service.routeInput.PositionScope.Branches[0].DepartmentIDs[0] != "dept_a" {
		t.Fatalf("expected position scope passed, got %#v", service.routeInput.PositionScope)
	}
}

func TestRecommendationSendRouteMapsOfflinePositionError(t *testing.T) {
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scope:    iam.ScopePredicate{Branches: []iam.ScopeBranch{{AllDepartments: true}}},
		},
		RecommendationService: &fakeRecommendationHTTPService{sendErr: recommendation.ErrTargetPositionOffline},
	})
	req := recommendationJSONRequest(http.MethodPost, "/recommendations/send", `{"resumeId":"resume_1","departmentId":"dept_a","positionId":"position_a"}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "RECOMMENDATION_TARGET_POSITION_OFFLINE")
}

func TestOpenAPIDocumentIncludesRecommendationEndpoints(t *testing.T) {
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

	assertOperation(t, doc.Paths, "/recommendations/route", "post", "post-recommendation-route")
	assertOperation(t, doc.Paths, "/recommendations/send", "post", "post-recommendation-send")
}

type fakeRecommendationHTTPService struct {
	routeCalls  int
	routeInput  recommendation.RouteInput
	routeResult recommendation.RouteResult
	routeErr    error
	sendCalls   int
	sendInput   recommendation.SendInput
	sendResult  recommendation.SendResult
	sendErr     error
}

func (f *fakeRecommendationHTTPService) Route(ctx context.Context, input recommendation.RouteInput) (recommendation.RouteResult, error) {
	f.routeCalls++
	f.routeInput = input
	if f.routeErr != nil {
		return recommendation.RouteResult{}, f.routeErr
	}
	return f.routeResult, nil
}

func (f *fakeRecommendationHTTPService) Send(ctx context.Context, input recommendation.SendInput) (recommendation.SendResult, error) {
	f.sendCalls++
	f.sendInput = input
	if f.sendErr != nil {
		return recommendation.SendResult{}, f.sendErr
	}
	if f.sendResult.ResumeID != "" {
		return f.sendResult, nil
	}
	return recommendation.SendResult{ResumeID: "resume_copy", Message: "已推荐到「智算调度部」· 已通知 1 位相关人员"}, nil
}

func recommendationJSONRequest(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	return req
}
