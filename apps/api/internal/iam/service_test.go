package iam_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

func TestCreateUserDepartmentRoleInvalidatesUserAndAudits(t *testing.T) {
	cache := &spyPrincipalCache{}
	recorder := &spyAuditRecorder{}
	store := &fakeMutationStore{closureUsers: []string{"u_direct"}}
	service := iam.NewService(store, iam.WithCache(cache), iam.WithAudit(recorder))

	input := iam.UserDepartmentRoleInput{
		ID:           "bind_1",
		UserID:       "u_direct",
		DepartmentID: "dept_a",
		RoleID:       iam.RoleHRBP,
		ActorUserID:  "u_actor",
	}
	if err := service.CreateUserDepartmentRole(context.Background(), input); err != nil {
		t.Fatalf("create user department role: %v", err)
	}

	if !store.createdUserDepartmentRole {
		t.Fatalf("expected store create user department role")
	}
	if !slices.Equal(cache.deleted, []string{"u_direct"}) || cache.cleared {
		t.Fatalf("expected only target user invalidated, deleted=%#v cleared=%v", cache.deleted, cache.cleared)
	}
	assertAuditRecorded(t, recorder.events, audit.EventUserDepartmentRoleCreated, "u_actor", "UserDepartmentRole", "Create", "bind_1")
}

func TestDeleteUserDepartmentRoleInvalidatesUserAndAudits(t *testing.T) {
	cache := &spyPrincipalCache{}
	recorder := &spyAuditRecorder{}
	store := &fakeMutationStore{
		deletedBinding: iam.RoleBinding{ID: "bind_1", UserID: "u_direct", DepartmentID: "dept_a", RoleID: iam.RoleHRBP},
	}
	service := iam.NewService(store, iam.WithCache(cache), iam.WithAudit(recorder))

	if err := service.DeleteUserDepartmentRole(context.Background(), "bind_1"); err != nil {
		t.Fatalf("delete user department role: %v", err)
	}

	if !store.deletedUserDepartmentRole {
		t.Fatalf("expected store delete user department role")
	}
	if !slices.Equal(cache.deleted, []string{"u_direct"}) || cache.cleared {
		t.Fatalf("expected target user invalidated, deleted=%#v cleared=%v", cache.deleted, cache.cleared)
	}
	assertAuditRecorded(t, recorder.events, audit.EventUserDepartmentRoleDeleted, "", "UserDepartmentRole", "Delete", "bind_1")
}

func TestReplaceRolePermissionsInvalidatesDirectAndAncestorUsers(t *testing.T) {
	cache := &spyPrincipalCache{}
	recorder := &spyAuditRecorder{}
	store := &fakeMutationStore{closureUsers: []string{"u_direct", "u_ancestor"}}
	service := iam.NewService(store, iam.WithCache(cache), iam.WithAudit(recorder))

	grants := []iam.PermissionGrant{{Resource: iam.ResourceResume, Action: iam.ActionList}}
	if err := service.ReplaceRolePermissions(context.Background(), iam.RoleHRBP, grants); err != nil {
		t.Fatalf("replace role permissions: %v", err)
	}

	if !store.replacedPermissions {
		t.Fatalf("expected store replace permissions")
	}
	if !slices.Equal(cache.deleted, []string{"u_ancestor", "u_direct"}) || cache.cleared {
		t.Fatalf("expected direct and ancestor users invalidated, deleted=%#v cleared=%v", cache.deleted, cache.cleared)
	}
	assertAuditRecorded(t, recorder.events, audit.EventPermissionsReplaced, "", "Permission", "Update", iam.RoleHRBP)

	cache = &spyPrincipalCache{}
	store = &fakeMutationStore{closureErr: errors.New("unsafe closure")}
	service = iam.NewService(store, iam.WithCache(cache), iam.WithAudit(&spyAuditRecorder{}))
	if err := service.ReplaceRolePermissions(context.Background(), iam.RoleHRBP, grants); err != nil {
		t.Fatalf("replace role permissions with unsafe closure: %v", err)
	}
	if !cache.cleared {
		t.Fatalf("expected unsafe role closure to clear cache")
	}
}

func TestCreateRoleRelationRejectsCycleInvalidatesClosureAndAudits(t *testing.T) {
	cyclicStore := &fakeMutationStore{
		relations: []iam.RoleRelation{{ID: "existing", ParentRoleID: iam.RoleHRBP, ChildRoleID: iam.RoleHRD}},
	}
	cyclicService := iam.NewService(cyclicStore, iam.WithCache(&spyPrincipalCache{}), iam.WithAudit(&spyAuditRecorder{}))
	err := cyclicService.CreateRoleRelation(context.Background(), iam.RoleRelation{ID: "cycle", ParentRoleID: iam.RoleHRD, ChildRoleID: iam.RoleHRBP})
	if !errors.Is(err, iam.ErrRoleRelationCycle) {
		t.Fatalf("expected cycle error, got %v", err)
	}
	if cyclicStore.createdRoleRelation {
		t.Fatalf("expected cycle to be rejected before store write")
	}

	cache := &spyPrincipalCache{}
	recorder := &spyAuditRecorder{}
	store := &fakeMutationStore{closureUsers: []string{"u_direct", "u_ancestor"}}
	service := iam.NewService(store, iam.WithCache(cache), iam.WithAudit(recorder))
	relation := iam.RoleRelation{ID: "rel_1", ParentRoleID: iam.RoleHRD, ChildRoleID: iam.RoleHRBP}
	if err := service.CreateRoleRelation(context.Background(), relation); err != nil {
		t.Fatalf("create role relation: %v", err)
	}

	if !store.createdRoleRelation {
		t.Fatalf("expected store create role relation")
	}
	if !slices.Equal(store.closureRoleIDs, []string{iam.RoleHRBP, iam.RoleHRD}) {
		t.Fatalf("expected closure for both endpoint roles, got %#v", store.closureRoleIDs)
	}
	if !slices.Equal(cache.deleted, []string{"u_ancestor", "u_direct"}) || cache.cleared {
		t.Fatalf("expected endpoint and ancestor users invalidated, deleted=%#v cleared=%v", cache.deleted, cache.cleared)
	}
	assertAuditRecorded(t, recorder.events, audit.EventRoleRelationCreated, "", "RoleRelation", "Create", "rel_1")
}

func TestDeleteRoleRelationInvalidatesClosureAndAudits(t *testing.T) {
	cache := &spyPrincipalCache{}
	recorder := &spyAuditRecorder{}
	store := &fakeMutationStore{
		closureUsers:    []string{"u_direct", "u_ancestor"},
		deletedRelation: iam.RoleRelation{ID: "rel_1", ParentRoleID: iam.RoleHRD, ChildRoleID: iam.RoleHRBP},
	}
	service := iam.NewService(store, iam.WithCache(cache), iam.WithAudit(recorder))

	if err := service.DeleteRoleRelation(context.Background(), "rel_1"); err != nil {
		t.Fatalf("delete role relation: %v", err)
	}

	if !store.deletedRoleRelation {
		t.Fatalf("expected store delete role relation")
	}
	if !slices.Equal(store.closureRoleIDs, []string{iam.RoleHRBP, iam.RoleHRD}) {
		t.Fatalf("expected closure for both endpoint roles, got %#v", store.closureRoleIDs)
	}
	if !slices.Equal(cache.deleted, []string{"u_ancestor", "u_direct"}) || cache.cleared {
		t.Fatalf("expected endpoint and ancestor users invalidated, deleted=%#v cleared=%v", cache.deleted, cache.cleared)
	}
	assertAuditRecorded(t, recorder.events, audit.EventRoleRelationDeleted, "", "RoleRelation", "Delete", "rel_1")
}

type fakeMutationStore struct {
	closureUsers []string
	closureErr   error

	relations       []iam.RoleRelation
	deletedBinding  iam.RoleBinding
	deletedRelation iam.RoleRelation

	createdUserDepartmentRole bool
	deletedUserDepartmentRole bool
	replacedPermissions       bool
	createdRoleRelation       bool
	deletedRoleRelation       bool
	closureRoleIDs            []string
}

func (f *fakeMutationStore) LoadSnapshot(context.Context, string) (iam.Snapshot, error) {
	return iam.Snapshot{}, nil
}

func (f *fakeMutationStore) UsersForRoleClosure(ctx context.Context, roleIDs []string) ([]string, error) {
	f.closureRoleIDs = append([]string(nil), roleIDs...)
	slices.Sort(f.closureRoleIDs)
	if f.closureErr != nil {
		return nil, f.closureErr
	}
	return append([]string(nil), f.closureUsers...), nil
}

func (f *fakeMutationStore) LoadRoleRelations(context.Context) ([]iam.RoleRelation, error) {
	return append([]iam.RoleRelation(nil), f.relations...), nil
}

func (f *fakeMutationStore) CreateUserDepartmentRole(context.Context, iam.UserDepartmentRoleInput) error {
	f.createdUserDepartmentRole = true
	return nil
}

func (f *fakeMutationStore) DeleteUserDepartmentRole(context.Context, string) (iam.RoleBinding, error) {
	f.deletedUserDepartmentRole = true
	return f.deletedBinding, nil
}

func (f *fakeMutationStore) ReplaceRolePermissions(context.Context, string, []iam.PermissionGrant) error {
	f.replacedPermissions = true
	return nil
}

func (f *fakeMutationStore) CreateRoleRelation(context.Context, iam.RoleRelation) error {
	f.createdRoleRelation = true
	return nil
}

func (f *fakeMutationStore) DeleteRoleRelation(context.Context, string) (iam.RoleRelation, error) {
	f.deletedRoleRelation = true
	return f.deletedRelation, nil
}

func (f *fakeMutationStore) WithTransaction(ctx context.Context, fn func(iam.Store) error) error {
	return fn(f)
}

type spyAuditRecorder struct {
	events []audit.Event
}

func (s *spyAuditRecorder) Record(ctx context.Context, event audit.Event) error {
	s.events = append(s.events, event)
	return nil
}

func assertAuditRecorded(t *testing.T, events []audit.Event, eventType audit.EventType, userID string, resource string, action string, targetID string) {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType && event.UserID == userID && event.Resource == resource && event.Action == action && event.TargetID == targetID {
			return
		}
	}
	t.Fatalf("expected audit event type=%s user=%s resource=%s action=%s target=%s, got %#v", eventType, userID, resource, action, targetID, events)
}
