package iam_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

func TestResolvePrincipalExpandsRecursiveRoleRelations(t *testing.T) {
	principal, err := iam.ResolvePrincipalFromSnapshot(baseSnapshot(
		iam.RoleBinding{ID: "bind_hrd", UserID: "u1", DepartmentID: iam.SystemDepartmentID, RoleID: iam.RoleHRD},
	))
	if err != nil {
		t.Fatalf("resolve principal: %v", err)
	}

	for _, roleID := range []string{iam.RoleHRD, iam.RoleHRBP, iam.RoleManager, iam.RoleTrainee} {
		if !slices.Contains(principal.ExpandedRoleIDs, roleID) {
			t.Fatalf("expected expanded role %s in %#v", roleID, principal.ExpandedRoleIDs)
		}
	}
	assertPermission(t, principal.Permissions, iam.ResourceResume, iam.ActionDelete)
}

func TestResolvePrincipalRejectsRoleRelationCycle(t *testing.T) {
	snapshot := baseSnapshot(iam.RoleBinding{ID: "bind_hrd", UserID: "u1", DepartmentID: iam.SystemDepartmentID, RoleID: iam.RoleHRD})
	snapshot.RoleRelations = append(snapshot.RoleRelations, iam.RoleRelation{ID: "cycle", ParentRoleID: iam.RoleHRBP, ChildRoleID: iam.RoleHRD})

	if _, err := iam.ResolvePrincipalFromSnapshot(snapshot); !errors.Is(err, iam.ErrRoleRelationCycle) {
		t.Fatalf("expected ErrRoleRelationCycle, got %v", err)
	}
}

func TestResolvePrincipalRejectsDepthExceeded(t *testing.T) {
	var roles []iam.Role
	var relations []iam.RoleRelation
	for i := 0; i < 18; i++ {
		roleID := roleIDForDepth(i)
		roles = append(roles, iam.Role{ID: roleID, Label: roleID, IsSystem: false, Enabled: true})
		if i > 0 {
			relations = append(relations, iam.RoleRelation{ID: roleIDForDepth(i-1) + "_" + roleID, ParentRoleID: roleIDForDepth(i - 1), ChildRoleID: roleID})
		}
	}
	snapshot := baseSnapshot(iam.RoleBinding{ID: "bind_depth", UserID: "u1", DepartmentID: "dept_a", RoleID: roleIDForDepth(0)})
	snapshot.Roles = append(snapshot.Roles, roles...)
	snapshot.RoleRelations = relations

	if _, err := iam.ResolvePrincipalFromSnapshot(snapshot); !errors.Is(err, iam.ErrRoleRelationDepthExceeded) {
		t.Fatalf("expected ErrRoleRelationDepthExceeded, got %v", err)
	}
}

func TestResolvePrincipalSkipsDisabledChildRoleReachedThroughRelation(t *testing.T) {
	snapshot := baseSnapshot(iam.RoleBinding{ID: "bind_hrd", UserID: "u1", DepartmentID: iam.SystemDepartmentID, RoleID: iam.RoleHRD})
	for i := range snapshot.Roles {
		if snapshot.Roles[i].ID == iam.RoleHRBP {
			snapshot.Roles[i].Enabled = false
		}
	}

	principal, err := iam.ResolvePrincipalFromSnapshot(snapshot)
	if err != nil {
		t.Fatalf("resolve principal: %v", err)
	}
	if slices.Contains(principal.ExpandedRoleIDs, iam.RoleHRBP) {
		t.Fatalf("disabled child role must not be expanded: %#v", principal.ExpandedRoleIDs)
	}
	assertNoPermission(t, principal.Permissions, iam.ResourceResume, iam.ActionDelete)
}

func TestResolvePrincipalMergesDepartmentAndChannelScopes(t *testing.T) {
	principal, err := iam.ResolvePrincipalFromSnapshot(baseSnapshot(
		iam.RoleBinding{ID: "bind_hrbp", UserID: "u1", DepartmentID: "dept_a", RoleID: iam.RoleHRBP},
		iam.RoleBinding{ID: "bind_social", UserID: "u1", DepartmentID: iam.SystemDepartmentID, RoleID: iam.RoleSocialOwner},
	))
	if err != nil {
		t.Fatalf("resolve principal: %v", err)
	}

	scope, err := iam.Scope(principal, iam.ResourceResume, iam.ActionList)
	if err != nil {
		t.Fatalf("scope resume list: %v", err)
	}
	assertScopeBranch(t, scope, false, []string{"dept_a"}, nil)
	assertScopeBranch(t, scope, true, nil, []string{"social"})
}

func TestResolvePrincipalRejectsUnsupportedSystemScope(t *testing.T) {
	_, err := iam.ResolvePrincipalFromSnapshot(baseSnapshot(
		iam.RoleBinding{ID: "bind_hrbp", UserID: "u1", DepartmentID: iam.SystemDepartmentID, RoleID: iam.RoleHRBP},
	))
	if !errors.Is(err, iam.ErrScopeUnsupported) {
		t.Fatalf("expected ErrScopeUnsupported, got %v", err)
	}
}

func TestPageAccessFromEffectivePermissions(t *testing.T) {
	principal, err := iam.ResolvePrincipalFromSnapshot(baseSnapshot(
		iam.RoleBinding{ID: "bind_hrbp", UserID: "u1", DepartmentID: "dept_a", RoleID: iam.RoleHRBP},
	))
	if err != nil {
		t.Fatalf("resolve principal: %v", err)
	}

	for _, page := range []string{"resume-parse", "resume-recommend", "resume-library", "departments-positions", "notifications"} {
		if !slices.Contains(principal.PageAccess, page) {
			t.Fatalf("expected page %s in %#v", page, principal.PageAccess)
		}
	}
}

func baseSnapshot(bindings ...iam.RoleBinding) iam.Snapshot {
	return iam.Snapshot{
		User:          iam.User{ID: "u1", EmployeeID: "E001", Name: "张三"},
		Departments:   []iam.Department{{ID: "dept_a", Name: "算力训练平台部"}, {ID: "dept_b", Name: "智算调度部"}},
		RoleBindings:  bindings,
		Roles:         iam.PresetRoles(),
		Permissions:   flattenPresetPermissions(),
		RoleRelations: iam.PresetRoleRelations(),
	}
}

func flattenPresetPermissions() []iam.PermissionGrant {
	matrix := iam.PresetRolePermissions()
	var grants []iam.PermissionGrant
	for _, roleGrants := range matrix {
		grants = append(grants, roleGrants...)
	}
	return grants
}

func assertPermission(t *testing.T, grants []iam.PermissionGrant, resource iam.Resource, action iam.Action) {
	t.Helper()
	for _, grant := range grants {
		if grant.Resource == resource && grant.Action == action {
			return
		}
	}
	t.Fatalf("expected permission %s.%s in %#v", resource, action, grants)
}

func assertNoPermission(t *testing.T, grants []iam.PermissionGrant, resource iam.Resource, action iam.Action) {
	t.Helper()
	for _, grant := range grants {
		if grant.Resource == resource && grant.Action == action {
			t.Fatalf("did not expect permission %s.%s in %#v", resource, action, grants)
		}
	}
}

func assertScopeBranch(t *testing.T, scope iam.ScopePredicate, allDepartments bool, departmentIDs []string, channels []string) {
	t.Helper()
	for _, branch := range scope.Branches {
		if branch.AllDepartments == allDepartments && slices.Equal(branch.DepartmentIDs, departmentIDs) && slices.Equal(branch.Channels, channels) {
			return
		}
	}
	t.Fatalf("expected branch allDepartments=%t departments=%v channels=%v in %#v", allDepartments, departmentIDs, channels, scope.Branches)
}

func roleIDForDepth(index int) string {
	return "depth_role_" + string(rune('a'+index))
}
