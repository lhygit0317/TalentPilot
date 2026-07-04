package iam_test

import (
	"reflect"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

func TestPresetRoleMatrixMatchesSpec(t *testing.T) {
	matrix := iam.PresetRolePermissions()
	assertGrant(t, matrix, iam.RoleGuest, iam.ResourceUser, iam.ActionGet, iam.AttributeConditions{Self: true})
	assertGrant(t, matrix, iam.RoleHRBP, iam.ResourceResume, iam.ActionDelete, iam.AttributeConditions{})
	assertGrant(t, matrix, iam.RoleHRBP, iam.ResourceDepartmentResume, iam.ActionDelete, iam.AttributeConditions{})
	assertGrant(t, matrix, iam.RoleSocialOwner, iam.ResourceResume, iam.ActionList, iam.AttributeConditions{Channels: []string{"social"}})
	assertGrant(t, matrix, iam.RoleSocialOwner, iam.ResourceDepartmentResume, iam.ActionDelete, iam.AttributeConditions{})
	assertGrant(t, matrix, iam.RoleCampusOwner, iam.ResourceResume, iam.ActionList, iam.AttributeConditions{Channels: []string{"campus"}})
	assertGrant(t, matrix, iam.RoleCampusOwner, iam.ResourceDepartmentResume, iam.ActionDelete, iam.AttributeConditions{})
	assertGrant(t, matrix, iam.RoleSuperAdmin, iam.ResourcePosition, iam.ActionDelete, iam.AttributeConditions{})
}

func TestGlobalScopeRoleSet(t *testing.T) {
	if iam.RoleSupportsGlobalScope(iam.RoleHRBP) {
		t.Fatalf("HRBP must not support __system__ all-department scope")
	}
	for _, roleID := range []string{iam.RoleHRD, iam.RoleSocialOwner, iam.RoleCampusOwner, iam.RoleSuperAdmin} {
		if !iam.RoleSupportsGlobalScope(roleID) {
			t.Fatalf("expected %s to support global scope", roleID)
		}
	}
}

func TestOnlySuperAdminPresetGetsDepartmentWrites(t *testing.T) {
	matrix := iam.PresetRolePermissions()
	for roleID, grants := range matrix {
		hasWrite := hasGrant(grants, iam.ResourceDepartment, iam.ActionCreate) ||
			hasGrant(grants, iam.ResourceDepartment, iam.ActionUpdate) ||
			hasGrant(grants, iam.ResourceDepartment, iam.ActionDelete)
		if roleID == iam.RoleSuperAdmin && !hasWrite {
			t.Fatalf("super admin should have department writes")
		}
		if roleID != iam.RoleSuperAdmin && hasWrite {
			t.Fatalf("%s should not have department writes", roleID)
		}
	}
}

func hasGrant(grants []iam.PermissionGrant, resource iam.Resource, action iam.Action) bool {
	for _, grant := range grants {
		if grant.Resource == resource && grant.Action == action {
			return true
		}
	}
	return false
}

func assertGrant(t *testing.T, matrix map[string][]iam.PermissionGrant, roleID string, resource iam.Resource, action iam.Action, conditions iam.AttributeConditions) {
	t.Helper()
	for _, grant := range matrix[roleID] {
		if grant.Resource == resource && grant.Action == action && reflect.DeepEqual(grant.AttributeConditions, conditions) {
			return
		}
	}
	t.Fatalf("expected grant role=%s resource=%s action=%s conditions=%#v in %#v", roleID, resource, action, conditions, matrix[roleID])
}
