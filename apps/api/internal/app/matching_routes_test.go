package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
)

func TestMatchingParseRouteRequiresPositionResumeCreate(t *testing.T) {
	service := &fakeMatchingHTTPService{}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			decisions: map[string]iam.Decision{
				iam.PermissionKey(iam.ResourceResume, iam.ActionGet):              {Allowed: true},
				iam.PermissionKey(iam.ResourcePosition, iam.ActionGet):            {Allowed: true},
				iam.PermissionKey(iam.ResourceDepartmentPosition, iam.ActionList): {Allowed: true},
				iam.PermissionKey(iam.ResourcePositionResume, iam.ActionCreate):   {Allowed: false},
			},
		},
		MatchingService: service,
	})
	req := matchingJSONRequest(http.MethodPost, "/matching/parse", `{"resumeId":"resume_1","positionId":"position_1"}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
	if service.parseCalls != 0 {
		t.Fatalf("service must not be called when permission is missing")
	}
}

func TestMatchingParseRoutePassesScopesToService(t *testing.T) {
	service := &fakeMatchingHTTPService{parseResult: matching.ParseResult{
		ID: "position_resume_1",
		Score: matching.Score{
			Total:     76,
			Judgement: "建议进入面试",
		},
		CreatedAt: time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC),
	}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scopes: map[string]iam.ScopePredicate{
				iam.PermissionKey(iam.ResourceResume, iam.ActionGet): {
					Resource: iam.ResourceResume,
					Action:   iam.ActionGet,
					Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}, Channels: []string{"social"}}},
				},
				iam.PermissionKey(iam.ResourcePosition, iam.ActionGet): {
					Resource: iam.ResourcePosition,
					Action:   iam.ActionGet,
					Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}},
				},
				iam.PermissionKey(iam.ResourcePositionResume, iam.ActionCreate): {
					Resource: iam.ResourcePositionResume,
					Action:   iam.ActionCreate,
					Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}},
				},
			},
			principal: iam.Principal{User: iam.User{ID: "w3_1", Name: "张三"}},
		},
		MatchingService: service,
	})
	req := matchingJSONRequest(http.MethodPost, "/matching/parse", `{"resumeId":"resume_1","positionId":"position_1"}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.parseCalls != 1 || service.parseInput.ActorUserID != "w3_1" {
		t.Fatalf("expected parse service call with actor, calls=%d input=%#v", service.parseCalls, service.parseInput)
	}
	if service.parseInput.ResumeScope.Branches[0].Channels[0] != "social" {
		t.Fatalf("expected resume scope passed, got %#v", service.parseInput.ResumeScope)
	}
	if service.parseInput.PositionResumeCreateScope.Branches[0].DepartmentIDs[0] != "dept_a" {
		t.Fatalf("expected create scope passed, got %#v", service.parseInput.PositionResumeCreateScope)
	}
	if !strings.Contains(rec.Body.String(), "建议进入面试") {
		t.Fatalf("expected parse response body, got %s", rec.Body.String())
	}
}

func TestMatchingParseRouteMapsOfflinePositionError(t *testing.T) {
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scope:    iam.ScopePredicate{Branches: []iam.ScopeBranch{{AllDepartments: true}}},
		},
		MatchingService: &fakeMatchingHTTPService{parseErr: matching.ErrPositionOffline},
	})
	req := matchingJSONRequest(http.MethodPost, "/matching/parse", `{"resumeId":"resume_1","positionId":"position_1"}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "MATCHING_POSITION_OFFLINE")
}

func TestMatchingInterviewQuestionRouteReturnsGroups(t *testing.T) {
	service := &fakeMatchingHTTPService{questions: matching.InterviewQuestionResult{Groups: []matching.InterviewQuestionGroup{
		{Type: "professional", Label: "专业面试", Questions: []matching.InterviewQuestion{{Order: 1, Question: "请介绍 Go 项目", Why: "验证经验", Difficulty: "核心"}}},
	}}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scope:    iam.ScopePredicate{Branches: []iam.ScopeBranch{{AllDepartments: true}}},
		},
		MatchingService: service,
	})
	req := matchingJSONRequest(http.MethodPost, "/matching/interview-questions", `{"resumeId":"resume_1","positionId":"position_1","matchScore":76}`)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if service.questionCalls != 1 || service.questionInput.MatchScore == nil || *service.questionInput.MatchScore != 76 {
		t.Fatalf("expected question service call with match score, got calls=%d input=%#v", service.questionCalls, service.questionInput)
	}
	if !strings.Contains(rec.Body.String(), "专业面试") {
		t.Fatalf("expected question response body, got %s", rec.Body.String())
	}
}

func TestOpenAPIDocumentIncludesMatchingEndpoints(t *testing.T) {
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

	assertOperation(t, doc.Paths, "/matching/parse", "post", "post-matching-parse")
	assertOperation(t, doc.Paths, "/matching/interview-questions", "post", "post-matching-interview-questions")
}

type fakeMatchingHTTPService struct {
	parseCalls    int
	parseInput    matching.ParseInput
	parseResult   matching.ParseResult
	parseErr      error
	questionCalls int
	questionInput matching.InterviewQuestionInput
	questions     matching.InterviewQuestionResult
	questionErr   error
}

func (f *fakeMatchingHTTPService) Parse(ctx context.Context, input matching.ParseInput) (matching.ParseResult, error) {
	f.parseCalls++
	f.parseInput = input
	if f.parseErr != nil {
		return matching.ParseResult{}, f.parseErr
	}
	if f.parseResult.ID != "" {
		return f.parseResult, nil
	}
	return matching.ParseResult{ID: "position_resume_1", Score: matching.Score{Total: 76, Judgement: "建议进入面试"}}, nil
}

func (f *fakeMatchingHTTPService) GenerateInterviewQuestions(ctx context.Context, input matching.InterviewQuestionInput) (matching.InterviewQuestionResult, error) {
	f.questionCalls++
	f.questionInput = input
	if f.questionErr != nil {
		return matching.InterviewQuestionResult{}, f.questionErr
	}
	if len(f.questions.Groups) > 0 {
		return f.questions, nil
	}
	return matching.InterviewQuestionResult{Groups: []matching.InterviewQuestionGroup{}}, nil
}

func matchingJSONRequest(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	return req
}
