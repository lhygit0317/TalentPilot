package apperror

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

func TestNewProblemBuildsStableErrorEnvelope(t *testing.T) {
	details := map[string]any{"roleLabel": "高级评审者"}

	problem := NewProblem(IAMRoleRelationCycle, "角色包含关系不能形成循环", "req_123", details)

	if problem.Code != IAMRoleRelationCycle {
		t.Fatalf("Code = %q, want %q", problem.Code, IAMRoleRelationCycle)
	}

	if problem.Message != "角色包含关系不能形成循环" {
		t.Fatalf("Message = %q, want %q", problem.Message, "角色包含关系不能形成循环")
	}

	if problem.RequestID != "req_123" {
		t.Fatalf("RequestID = %q, want %q", problem.RequestID, "req_123")
	}

	if !reflect.DeepEqual(problem.Details, details) {
		t.Fatalf("Details = %#v, want %#v", problem.Details, details)
	}
}

func TestNewProblemUsesDefaultMessageWhenEmpty(t *testing.T) {
	problem := NewProblem(AuthCSRFInvalid, "", "req_456", nil)

	if problem.Message != "登录校验已失效，请刷新后重试" {
		t.Fatalf("Message = %q, want default CSRF message", problem.Message)
	}
	if problem.Code != AuthCSRFInvalid {
		t.Fatalf("Code = %q, want %q", problem.Code, AuthCSRFInvalid)
	}
}

func TestDefaultMessageIncludesIAMCodes(t *testing.T) {
	cases := []struct {
		code   Code
		status int
	}{
		{IAMPermissionNotFound, http.StatusNotFound},
		{IAMInvalidResource, http.StatusUnprocessableEntity},
		{IAMInvalidAction, http.StatusUnprocessableEntity},
		{IAMInvalidAttributeCondition, http.StatusUnprocessableEntity},
		{IAMRoleRelationDepthExceeded, http.StatusUnprocessableEntity},
		{IAMPrincipalNotFound, http.StatusNotFound},
		{IAMScopeUnsupported, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		problem := NewProblem(tc.code, "", "req_iam", nil)
		if problem.GetStatus() != tc.status {
			t.Fatalf("%s status=%d want %d", tc.code, problem.GetStatus(), tc.status)
		}
		if problem.Message == "" {
			t.Fatalf("expected default message for %s", tc.code)
		}
	}
}

func TestResumeLibraryErrorCodesUseStableMessages(t *testing.T) {
	cases := []struct {
		code   Code
		status int
	}{
		{ResumeNotFound, http.StatusNotFound},
		{ResumeImportFileTooLarge, http.StatusUnprocessableEntity},
		{ResumeImportUnsupportedType, http.StatusUnprocessableEntity},
		{ResumeImportTargetDepartmentRequired, http.StatusUnprocessableEntity},
		{ResumeImportTargetDepartmentInvalid, http.StatusUnprocessableEntity},
		{ResumeImportParseFailed, http.StatusUnprocessableEntity},
		{ResumeImportEmptyFile, http.StatusUnprocessableEntity},
		{ResumeDeleteDenied, http.StatusForbidden},
		{JobNotFound, http.StatusNotFound},
		{JobAccessDenied, http.StatusForbidden},
	}
	for _, tc := range cases {
		problem := NewProblem(tc.code, "", "req_1", nil)
		if problem.GetStatus() != tc.status {
			t.Fatalf("%s status=%d want %d", tc.code, problem.GetStatus(), tc.status)
		}
		if problem.Message == "" || problem.RequestID != "req_1" {
			t.Fatalf("expected message and request id for %s: %#v", tc.code, problem)
		}
	}
}

func TestOrganizationErrorCodesUseStableMessages(t *testing.T) {
	cases := []struct {
		code   Code
		status int
	}{
		{DepartmentNotFound, http.StatusNotFound},
		{DepartmentNameRequired, http.StatusUnprocessableEntity},
		{DepartmentNameDuplicate, http.StatusUnprocessableEntity},
		{DepartmentDeleteHasRelations, http.StatusUnprocessableEntity},
		{DepartmentSystemProtected, http.StatusUnprocessableEntity},
		{PositionNotFound, http.StatusNotFound},
		{PositionNameRequired, http.StatusUnprocessableEntity},
		{PositionDepartmentRequired, http.StatusUnprocessableEntity},
		{PositionDepartmentInvalid, http.StatusUnprocessableEntity},
		{PositionInvalidChannel, http.StatusUnprocessableEntity},
		{PositionInvalidStatus, http.StatusUnprocessableEntity},
		{PositionDuplicateKeyword, http.StatusUnprocessableEntity},
		{PositionDuplicateImplicitTag, http.StatusUnprocessableEntity},
		{PositionInvalidImplicitWeight, http.StatusUnprocessableEntity},
		{PositionDeleteHasHistory, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		problem := NewProblem(tc.code, "", "req_org", nil)
		if problem.GetStatus() != tc.status {
			t.Fatalf("%s status=%d want %d", tc.code, problem.GetStatus(), tc.status)
		}
		if problem.Message == "" || problem.RequestID != "req_org" {
			t.Fatalf("expected message and request id for %s: %#v", tc.code, problem)
		}
	}
}

func TestDetailsFromErrorsDoesNotExposeInvalidValues(t *testing.T) {
	detail := &huma.ErrorDetail{
		Message:  "expected string",
		Location: "body.password",
		Value:    "secret-password",
	}

	details := detailsFromErrors([]error{detail})

	if reflect.DeepEqual(details, map[string]any{}) {
		t.Fatalf("expected validation details")
	}
	if containsValue(details, "secret-password") {
		t.Fatalf("details leaked invalid value: %#v", details)
	}
}

func containsValue(value any, target string) bool {
	switch typed := value.(type) {
	case string:
		return typed == target
	case map[string]any:
		for _, item := range typed {
			if containsValue(item, target) {
				return true
			}
		}
	case []map[string]any:
		for _, item := range typed {
			if containsValue(item, target) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsValue(item, target) {
				return true
			}
		}
	}
	return false
}
