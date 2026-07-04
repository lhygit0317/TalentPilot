package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/resume"
)

func TestResumeListRequiresAuthAndPermission(t *testing.T) {
	server := NewServerWithOptions(Options{
		AuthService:    newFakeHTTPAuthService(),
		IAMService:     &fakeIAMService{decision: iam.Decision{Allowed: false}},
		ResumeService:  &fakeResumeService{},
		FrontendOrigin: "https://talentpilot.example",
	})
	req := httptest.NewRequest(http.MethodGet, "/resumes", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
}

func TestResumeListPassesScopePredicateToService(t *testing.T) {
	resumeSvc := &fakeResumeService{listResult: resume.ListResult{Items: []resume.ListItem{{ID: "resume_1", Name: "张三"}}}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scope: iam.ScopePredicate{
				Resource: iam.ResourceResume,
				Action:   iam.ActionList,
				Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}, Channels: []string{"social"}}},
			},
		},
		ResumeService: resumeSvc,
	})
	req := httptest.NewRequest(http.MethodGet, "/resumes?chan=social&search=Go", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if resumeSvc.listQuery.Channel != resume.ChannelSocial || resumeSvc.listQuery.Search != "Go" {
		t.Fatalf("expected channel/search query, got %#v", resumeSvc.listQuery)
	}
	if len(resumeSvc.listQuery.Scope.Branches) != 1 || resumeSvc.listQuery.Scope.Branches[0].DepartmentIDs[0] != "dept_a" {
		t.Fatalf("expected IAM scope passed to service, got %#v", resumeSvc.listQuery.Scope)
	}
}

func TestResumeDetailMapsUnauthorizedToIAMDenied(t *testing.T) {
	server := NewServerWithOptions(Options{
		AuthService:   newFakeHTTPAuthService(),
		IAMService:    &fakeIAMService{decision: iam.Decision{Allowed: false}},
		ResumeService: &fakeResumeService{},
	})
	req := httptest.NewRequest(http.MethodGet, "/resumes/resume_1", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "IAM_PERMISSION_DENIED")
}

func TestResumeImportRequiresCSRFAndCreatePermissions(t *testing.T) {
	server := NewServerWithOptions(Options{
		AuthService:    newFakeHTTPAuthService(),
		IAMService:     &fakeIAMService{decision: iam.Decision{Allowed: true}},
		ResumeService:  &fakeResumeService{},
		FrontendOrigin: "https://talentpilot.example",
	})
	req := httptest.NewRequest(http.MethodPost, "/resumes/imports", strings.NewReader(""))
	req.Header.Set("Origin", "https://talentpilot.example")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "AUTH_CSRF_INVALID")
}

func TestResumeDeleteMapsStableErrors(t *testing.T) {
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scope:    iam.ScopePredicate{Resource: iam.ResourceResume, Action: iam.ActionDelete, Branches: []iam.ScopeBranch{{AllDepartments: true}}},
		},
		ResumeService: &fakeResumeService{deleteErr: resume.ErrNotFound},
	})
	req := httptest.NewRequest(http.MethodDelete, "/resumes/missing_resume", nil)
	req.Header.Set("X-CSRF-Token", "csrf_before")
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	req.AddCookie(&http.Cookie{Name: "tp_csrf", Value: "csrf_before"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.String(), "RESUME_NOT_FOUND")
}

func TestJobStatusRequiresCurrentUserOwnership(t *testing.T) {
	resumeSvc := &fakeResumeService{jobStatus: resume.JobStatus{ID: "job_1", Type: "resume_import", Status: "succeeded"}}
	server := NewServerWithOptions(Options{
		AuthService: newFakeHTTPAuthService(),
		IAMService: &fakeIAMService{
			decision: iam.Decision{Allowed: true},
			scope:    iam.ScopePredicate{Resource: iam.ResourceJob, Action: iam.ActionGet, Branches: []iam.ScopeBranch{{AllDepartments: true}}},
		},
		ResumeService: resumeSvc,
	})
	req := httptest.NewRequest(http.MethodGet, "/jobs/job_1", nil)
	req.AddCookie(&http.Cookie{Name: "tp_auth", Value: "auth_cookie"})
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if resumeSvc.jobUserID != "w3_1" {
		t.Fatalf("expected current user ownership check with w3_1, got %q", resumeSvc.jobUserID)
	}
}

func TestOpenAPIDocumentIncludesResumeAndJobEndpoints(t *testing.T) {
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

	assertOperation(t, doc.Paths, "/resumes", "get", "get-resumes")
	assertOperation(t, doc.Paths, "/resumes/{resumeId}", "get", "get-resume")
	assertOperation(t, doc.Paths, "/resumes/imports", "post", "post-resume-import")
	assertOperation(t, doc.Paths, "/resumes/batch-imports", "post", "post-resume-batch-import")
	assertOperation(t, doc.Paths, "/resumes/{resumeId}", "delete", "delete-resume")
	assertOperation(t, doc.Paths, "/jobs/{jobId}", "get", "get-job")
}

type fakeResumeService struct {
	listQuery  ListQueryAlias
	listResult resume.ListResult
	getErr     error
	deleteErr  error
	jobStatus  resume.JobStatus
	jobUserID  string
}

type ListQueryAlias = resume.ListQuery

func (f *fakeResumeService) List(ctx context.Context, query resume.ListQuery) (resume.ListResult, error) {
	f.listQuery = query
	return f.listResult, nil
}

func (f *fakeResumeService) Get(ctx context.Context, id string, scope iam.ScopePredicate) (resume.Detail, error) {
	if f.getErr != nil {
		return resume.Detail{}, f.getErr
	}
	return resume.Detail{ListItem: resume.ListItem{ID: id, Name: "张三"}}, nil
}

func (f *fakeResumeService) Delete(ctx context.Context, id string, scope iam.ScopePredicate) error {
	return f.deleteErr
}

func (f *fakeResumeService) ImportOne(ctx context.Context, input resume.ImportInput) (resume.JobStatus, error) {
	return resume.JobStatus{ID: "job_import", Type: "resume_import", Status: "pending"}, nil
}

func (f *fakeResumeService) ImportBatch(ctx context.Context, input resume.BatchImportInput) (resume.JobStatus, error) {
	return resume.JobStatus{ID: "job_batch", Type: "resume_import", Status: "pending"}, nil
}

func (f *fakeResumeService) GetJob(ctx context.Context, jobID string, userID string) (resume.JobStatus, error) {
	f.jobUserID = userID
	if f.jobStatus.ID == "" {
		return resume.JobStatus{ID: jobID, Type: "resume_import", Status: "pending"}, nil
	}
	return f.jobStatus, nil
}
