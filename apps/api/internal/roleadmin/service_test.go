package roleadmin_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/roleadmin"
)

func TestServiceCreateRoleRejectsDuplicatePermission(t *testing.T) {
	store := newFakeRoleAdminStore()
	service := roleadmin.NewService(store, &fakeRoleInvalidator{}, audit.NopRecorder{})

	_, err := service.CreateRole(context.Background(), roleadmin.RoleDefinitionInput{
		ActorUserID: "admin",
		Label:       "高级评审者",
		Enabled:     true,
		Permissions: []roleadmin.PermissionInput{
			{Resource: iam.ResourceResume, Action: iam.ActionList},
			{Resource: iam.ResourceResume, Action: iam.ActionList},
		},
	})
	if !errors.Is(err, roleadmin.ErrPermissionDuplicate) {
		t.Fatalf("expected duplicate permission error, got %v", err)
	}
	if store.createdRole {
		t.Fatalf("duplicate permissions should be rejected before create")
	}
}

func TestServiceCreateRoleRejectsInvalidPermissionCondition(t *testing.T) {
	store := newFakeRoleAdminStore()
	service := roleadmin.NewService(store, &fakeRoleInvalidator{}, audit.NopRecorder{})

	_, err := service.CreateRole(context.Background(), roleadmin.RoleDefinitionInput{
		ActorUserID: "admin",
		Label:       "高级评审者",
		Enabled:     true,
		Permissions: []roleadmin.PermissionInput{
			{Resource: iam.ResourceDepartment, Action: iam.ActionList, AttributeConditions: iam.AttributeConditions{Channels: []string{"social"}}},
		},
	})
	if !errors.Is(err, roleadmin.ErrPermissionInvalid) {
		t.Fatalf("expected invalid permission error, got %v", err)
	}
}

func TestServiceUpdateSystemRoleRejectsLabelChange(t *testing.T) {
	store := newFakeRoleAdminStore()
	store.roles[iam.RoleHRBP] = roleadmin.RoleRecord{ID: iam.RoleHRBP, Label: "HRBP", IsSystem: true, Enabled: true}
	service := roleadmin.NewService(store, &fakeRoleInvalidator{}, audit.NopRecorder{})

	_, err := service.UpdateRole(context.Background(), iam.RoleHRBP, roleadmin.RoleDefinitionInput{
		ActorUserID: "admin",
		Label:       "HRBP 新名称",
		Enabled:     true,
	})
	if !errors.Is(err, roleadmin.ErrSystemRoleProtected) {
		t.Fatalf("expected system role protected error, got %v", err)
	}
	if store.updatedRoleID != "" {
		t.Fatalf("system label change should be rejected before update")
	}
}

func TestServiceUpdateRoleRejectsRelationCycle(t *testing.T) {
	store := newFakeRoleAdminStore()
	store.roles["role_parent"] = roleadmin.RoleRecord{ID: "role_parent", Label: "父角色", Enabled: true}
	store.roles["role_child"] = roleadmin.RoleRecord{ID: "role_child", Label: "子角色", Enabled: true}
	store.relations = []iam.RoleRelation{{ID: "rel_existing", ParentRoleID: "role_child", ChildRoleID: "role_parent"}}
	service := roleadmin.NewService(store, &fakeRoleInvalidator{}, audit.NopRecorder{})

	_, err := service.UpdateRole(context.Background(), "role_parent", roleadmin.RoleDefinitionInput{
		ActorUserID:  "admin",
		Label:        "父角色",
		Enabled:      true,
		ChildRoleIDs: []string{"role_child"},
		Permissions:  []roleadmin.PermissionInput{{Resource: iam.ResourceUser, Action: iam.ActionList}},
	})
	if !errors.Is(err, iam.ErrRoleRelationCycle) {
		t.Fatalf("expected cycle error, got %v", err)
	}
	if store.updatedRoleID != "" {
		t.Fatalf("cycle should be rejected before update")
	}
}

func TestServiceDeleteRoleRejectsSystemRole(t *testing.T) {
	store := newFakeRoleAdminStore()
	store.roles[iam.RoleHRBP] = roleadmin.RoleRecord{ID: iam.RoleHRBP, Label: "HRBP", IsSystem: true, Enabled: true}
	service := roleadmin.NewService(store, &fakeRoleInvalidator{}, audit.NopRecorder{})

	err := service.DeleteRole(context.Background(), iam.RoleHRBP, "admin")
	if !errors.Is(err, roleadmin.ErrSystemRoleProtected) {
		t.Fatalf("expected system role protected error, got %v", err)
	}
	if store.deletedRoleID != "" {
		t.Fatalf("system role should not be deleted")
	}
}

func TestServiceDeleteRoleRejectsReferencedCustomRole(t *testing.T) {
	store := newFakeRoleAdminStore()
	store.roles["role_custom"] = roleadmin.RoleRecord{ID: "role_custom", Label: "自定义", Enabled: true, ReferenceCount: 1}
	service := roleadmin.NewService(store, &fakeRoleInvalidator{}, audit.NopRecorder{})

	err := service.DeleteRole(context.Background(), "role_custom", "admin")
	if !errors.Is(err, roleadmin.ErrRoleInUse) {
		t.Fatalf("expected role in use error, got %v", err)
	}
	if store.deletedRoleID != "" {
		t.Fatalf("referenced role should not be deleted")
	}
}

func TestServiceToggleEnabledInvalidatesRoleClosure(t *testing.T) {
	store := newFakeRoleAdminStore()
	store.roles["role_custom"] = roleadmin.RoleRecord{ID: "role_custom", Label: "自定义", Enabled: true}
	invalidator := &fakeRoleInvalidator{}
	service := roleadmin.NewService(store, invalidator, audit.NopRecorder{})

	detail, err := service.ToggleEnabled(context.Background(), "role_custom", roleadmin.ToggleEnabledInput{
		ActorUserID: "admin",
		Enabled:     false,
	})
	if err != nil {
		t.Fatalf("toggle enabled: %v", err)
	}

	if detail.Enabled || !slices.Equal(invalidator.roleIDs, []string{"role_custom"}) {
		t.Fatalf("expected disabled detail and role closure invalidation, detail=%#v invalidated=%#v", detail, invalidator.roleIDs)
	}
	if store.toggledRoleID != "role_custom" {
		t.Fatalf("expected store toggle, got %q", store.toggledRoleID)
	}
}

type fakeRoleAdminStore struct {
	roles     map[string]roleadmin.RoleRecord
	relations []iam.RoleRelation

	createdRole          bool
	updatedRoleID        string
	deletedRoleID        string
	toggledRoleID        string
	replacedPermissions  []roleadmin.PermissionInput
	replacedChildRoleIDs []string
	transactionCallCount int
}

func newFakeRoleAdminStore() *fakeRoleAdminStore {
	return &fakeRoleAdminStore{
		roles: map[string]roleadmin.RoleRecord{
			"role_existing": {ID: "role_existing", Label: "已存在", Enabled: true},
		},
	}
}

func (f *fakeRoleAdminStore) ListRoles(context.Context, roleadmin.RoleListQuery) (roleadmin.RoleListResult, error) {
	return roleadmin.RoleListResult{}, nil
}

func (f *fakeRoleAdminStore) GetRole(ctx context.Context, roleID string, query roleadmin.RoleCapabilityQuery) (roleadmin.RoleDetail, error) {
	record, err := f.GetRoleRecord(ctx, roleID)
	if err != nil {
		return roleadmin.RoleDetail{}, err
	}
	return roleadmin.RoleDetail{
		ID:               record.ID,
		Label:            record.Label,
		Description:      record.Description,
		IsSystem:         record.IsSystem,
		Enabled:          record.Enabled,
		ReferenceCount:   record.ReferenceCount,
		CanEdit:          query.ActorCanEdit,
		CanDelete:        query.ActorCanDelete && !record.IsSystem && record.ReferenceCount == 0,
		CanToggleEnabled: query.ActorCanToggle,
	}, nil
}

func (f *fakeRoleAdminStore) PermissionOptions(context.Context) (roleadmin.PermissionOptionsResult, error) {
	return roleadmin.PermissionOptionsResult{}, nil
}

func (f *fakeRoleAdminStore) GetRoleRecord(ctx context.Context, roleID string) (roleadmin.RoleRecord, error) {
	record, ok := f.roles[roleID]
	if !ok {
		return roleadmin.RoleRecord{}, roleadmin.ErrRoleNotFound
	}
	return record, nil
}

func (f *fakeRoleAdminStore) RoleLabelExists(ctx context.Context, label string, excludeRoleID string) (bool, error) {
	for _, role := range f.roles {
		if role.Label == label && role.ID != excludeRoleID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeRoleAdminStore) ChildRolesExist(ctx context.Context, roleIDs []string) (bool, error) {
	for _, roleID := range roleIDs {
		if _, ok := f.roles[roleID]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func (f *fakeRoleAdminStore) LoadRoleRelations(context.Context) ([]iam.RoleRelation, error) {
	return append([]iam.RoleRelation(nil), f.relations...), nil
}

func (f *fakeRoleAdminStore) CreateRole(ctx context.Context, input roleadmin.RoleDefinitionRecord) (string, error) {
	f.createdRole = true
	roleID := "role_created"
	f.roles[roleID] = roleadmin.RoleRecord{ID: roleID, Label: input.Label, Description: input.Description, Enabled: input.Enabled}
	return roleID, nil
}

func (f *fakeRoleAdminStore) UpdateRole(ctx context.Context, roleID string, input roleadmin.RoleDefinitionRecord) error {
	f.updatedRoleID = roleID
	record := f.roles[roleID]
	record.Label = input.Label
	record.Description = input.Description
	record.Enabled = input.Enabled
	f.roles[roleID] = record
	return nil
}

func (f *fakeRoleAdminStore) ReplaceRolePermissions(ctx context.Context, roleID string, permissions []roleadmin.PermissionInput) error {
	f.replacedPermissions = append([]roleadmin.PermissionInput(nil), permissions...)
	return nil
}

func (f *fakeRoleAdminStore) ReplaceRoleChildren(ctx context.Context, roleID string, childRoleIDs []string) error {
	f.replacedChildRoleIDs = append([]string(nil), childRoleIDs...)
	return nil
}

func (f *fakeRoleAdminStore) ToggleRoleEnabled(ctx context.Context, roleID string, enabled bool) error {
	f.toggledRoleID = roleID
	record := f.roles[roleID]
	record.Enabled = enabled
	f.roles[roleID] = record
	return nil
}

func (f *fakeRoleAdminStore) DeleteRole(ctx context.Context, roleID string) error {
	f.deletedRoleID = roleID
	delete(f.roles, roleID)
	return nil
}

func (f *fakeRoleAdminStore) WithTransaction(ctx context.Context, fn func(roleadmin.Store) error) error {
	f.transactionCallCount++
	return fn(f)
}

type fakeRoleInvalidator struct {
	roleIDs []string
}

func (f *fakeRoleInvalidator) InvalidateRoleClosure(ctx context.Context, roleIDs []string) error {
	f.roleIDs = append([]string(nil), roleIDs...)
	slices.Sort(f.roleIDs)
	return nil
}
