package iam_test

import (
	"errors"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

func TestPermissionWhitelistRejectsUnknownResourceAction(t *testing.T) {
	if err := iam.ValidatePermissionGrant(iam.PermissionGrant{Resource: "Unknown", Action: iam.ActionList}); !errors.Is(err, iam.ErrInvalidResource) {
		t.Fatalf("expected ErrInvalidResource, got %v", err)
	}
	if err := iam.ValidatePermissionGrant(iam.PermissionGrant{Resource: iam.ResourceResume, Action: "Export"}); !errors.Is(err, iam.ErrInvalidAction) {
		t.Fatalf("expected ErrInvalidAction, got %v", err)
	}
}

func TestAttributeConditionValidation(t *testing.T) {
	valid := iam.PermissionGrant{Resource: iam.ResourceResume, Action: iam.ActionList, AttributeConditions: iam.AttributeConditions{Channels: []string{"social"}}}
	if err := iam.ValidatePermissionGrant(valid); err != nil {
		t.Fatalf("valid resume channel condition: %v", err)
	}

	invalid := iam.PermissionGrant{Resource: iam.ResourceDepartment, Action: iam.ActionList, AttributeConditions: iam.AttributeConditions{Channels: []string{"social"}}}
	if err := iam.ValidatePermissionGrant(invalid); !errors.Is(err, iam.ErrInvalidAttributeCondition) {
		t.Fatalf("expected invalid attribute condition, got %v", err)
	}
}
