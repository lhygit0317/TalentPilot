package apperror

import (
	"reflect"
	"testing"
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
