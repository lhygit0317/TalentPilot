package apperror

import (
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
