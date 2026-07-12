package useradmin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/useradmin"
)

func TestServiceCreateRoleBindingsRejectsDuplicateRequest(t *testing.T) {
	service := useradmin.NewService(&fakeStore{}, &fakeIAMCache{}, audit.NopRecorder{})

	_, err := service.CreateRoleBindings(context.Background(), useradmin.CreateRoleBindingsInput{
		ActorUserID: "admin",
		UserID:      "user_1",
		CreateScope: allUserDepartmentRoleScope(iam.ActionCreate),
		Bindings: []useradmin.RoleBindingRequest{
			{DepartmentID: "dept_a", RoleID: iam.RoleHRBP},
			{DepartmentID: "dept_a", RoleID: iam.RoleHRBP},
		},
	})

	if !errors.Is(err, useradmin.ErrDuplicateBinding) {
		t.Fatalf("expected duplicate binding error, got %v", err)
	}
}

func TestServiceCreateRoleBindingsRejectsEmptyBatch(t *testing.T) {
	service := useradmin.NewService(&fakeStore{}, &fakeIAMCache{}, audit.NopRecorder{})

	_, err := service.CreateRoleBindings(context.Background(), useradmin.CreateRoleBindingsInput{
		ActorUserID: "admin",
		UserID:      "user_1",
		CreateScope: allUserDepartmentRoleScope(iam.ActionCreate),
	})

	if !errors.Is(err, useradmin.ErrBatchEmpty) {
		t.Fatalf("expected empty batch error, got %v", err)
	}
}

func TestServiceCreateRoleBindingsRejectsOversizedBatch(t *testing.T) {
	service := useradmin.NewService(&fakeStore{}, &fakeIAMCache{}, audit.NopRecorder{})
	bindings := make([]useradmin.RoleBindingRequest, 21)
	for index := range bindings {
		bindings[index] = useradmin.RoleBindingRequest{DepartmentID: "dept_a", RoleID: iam.RoleHRBP}
	}

	_, err := service.CreateRoleBindings(context.Background(), useradmin.CreateRoleBindingsInput{
		ActorUserID: "admin",
		UserID:      "user_1",
		CreateScope: allUserDepartmentRoleScope(iam.ActionCreate),
		Bindings:    bindings,
	})

	if !errors.Is(err, useradmin.ErrBatchTooLarge) {
		t.Fatalf("expected oversized batch error, got %v", err)
	}
}

func TestServiceDeleteRoleBindingRejectsGuestBinding(t *testing.T) {
	store := &fakeStore{binding: useradmin.RoleBindingDetail{
		ID:     "udr_guest",
		UserID: "user_1",
		Guest:  true,
		Department: useradmin.DepartmentSummary{
			ID:     iam.SystemDepartmentID,
			Name:   "system",
			System: true,
		},
		Role: useradmin.RoleSummary{
			ID:       iam.RoleGuest,
			Label:    "游客",
			IsSystem: true,
			Enabled:  true,
		},
	}}
	service := useradmin.NewService(store, &fakeIAMCache{}, audit.NopRecorder{})

	_, err := service.DeleteRoleBinding(context.Background(), useradmin.DeleteRoleBindingInput{
		ActorUserID: "admin",
		UserID:      "user_1",
		BindingID:   "udr_guest",
		DeleteScope: allUserDepartmentRoleScope(iam.ActionDelete),
	})

	if !errors.Is(err, useradmin.ErrGuestBindingProtected) {
		t.Fatalf("expected guest protected error, got %v", err)
	}
}

func TestServiceDeleteRoleBindingRejectsSelfLockout(t *testing.T) {
	store := &fakeStore{
		binding: useradmin.RoleBindingDetail{
			ID:     "udr_hrbp",
			UserID: "user_1",
			Department: useradmin.DepartmentSummary{
				ID:   "dept_a",
				Name: "算力训练平台部",
			},
			Role: useradmin.RoleSummary{
				ID:       iam.RoleHRBP,
				Label:    "HRBP",
				IsSystem: true,
				Enabled:  true,
			},
		},
		nonGuestCount: 1,
	}
	service := useradmin.NewService(store, &fakeIAMCache{}, audit.NopRecorder{})

	_, err := service.DeleteRoleBinding(context.Background(), useradmin.DeleteRoleBindingInput{
		ActorUserID: "user_1",
		UserID:      "user_1",
		BindingID:   "udr_hrbp",
		DeleteScope: allUserDepartmentRoleScope(iam.ActionDelete),
	})

	if !errors.Is(err, useradmin.ErrSelfLockout) {
		t.Fatalf("expected self lockout error, got %v", err)
	}
}

func TestServiceDeleteRoleBindingAuditsGuestFallbackCreation(t *testing.T) {
	store := &fakeStore{
		binding: useradmin.RoleBindingDetail{
			ID:     "udr_manager",
			UserID: "user_1",
			Department: useradmin.DepartmentSummary{
				ID:   "dept_a",
				Name: "算力训练平台部",
			},
			Role: useradmin.RoleSummary{
				ID:       iam.RoleManager,
				Label:    "主管",
				IsSystem: true,
				Enabled:  true,
			},
		},
		nonGuestCount: 0,
	}
	recorder := &recordingUserAdminAudit{}
	service := useradmin.NewService(store, &fakeIAMCache{}, recorder)

	_, err := service.DeleteRoleBinding(context.Background(), useradmin.DeleteRoleBindingInput{
		ActorUserID: "admin",
		UserID:      "user_1",
		BindingID:   "udr_manager",
		DeleteScope: allUserDepartmentRoleScope(iam.ActionDelete),
	})
	if err != nil {
		t.Fatalf("delete binding: %v", err)
	}

	if len(recorder.events) != 2 {
		t.Fatalf("expected delete and fallback create audit events, got %#v", recorder.events)
	}
	if recorder.events[1].Type != audit.EventUserDepartmentRoleCreated || recorder.events[1].Details["roleId"] != iam.RoleGuest {
		t.Fatalf("expected guest fallback creation audit, got %#v", recorder.events[1])
	}
}

type fakeStore struct {
	binding       useradmin.RoleBindingDetail
	nonGuestCount int
}

func (f *fakeStore) ListUsers(context.Context, useradmin.ListUsersQuery) (useradmin.UserListResult, error) {
	return useradmin.UserListResult{}, nil
}

func (f *fakeStore) GetUser(context.Context, string, iam.ScopePredicate) (useradmin.UserDetail, error) {
	return useradmin.UserDetail{}, nil
}

func (f *fakeStore) GetUserIdentity(context.Context, string) (useradmin.UserIdentity, error) {
	return useradmin.UserIdentity{ID: "user_1", EmployeeID: "A10001", Name: "张敏"}, nil
}

func (f *fakeStore) ListAssignableRoles(context.Context) ([]useradmin.AssignableRole, error) {
	return nil, nil
}

func (f *fakeStore) CreateRoleBindings(context.Context, useradmin.CreateRoleBindingsCommand) ([]useradmin.RoleBindingDetail, error) {
	return nil, nil
}

func (f *fakeStore) GetRoleBinding(context.Context, string) (useradmin.RoleBindingDetail, error) {
	return f.binding, nil
}

func (f *fakeStore) DeleteRoleBinding(context.Context, string) (useradmin.RoleBindingDetail, error) {
	return f.binding, nil
}

func (f *fakeStore) CountNonGuestBindings(context.Context, string) (int, error) {
	return f.nonGuestCount, nil
}

func (f *fakeStore) EnsureGuestBinding(context.Context, string, string) (useradmin.RoleBindingDetail, bool, error) {
	return useradmin.RoleBindingDetail{
		ID:     "udr_guest_fallback",
		UserID: "user_1",
		Department: useradmin.DepartmentSummary{
			ID:     iam.SystemDepartmentID,
			Name:   "system",
			System: true,
		},
		Role: useradmin.RoleSummary{
			ID:       iam.RoleGuest,
			Label:    "游客",
			IsSystem: true,
			Enabled:  true,
		},
		Guest: true,
	}, true, nil
}

func (f *fakeStore) WithTransaction(ctx context.Context, fn func(useradmin.Store) error) error {
	return fn(f)
}

type fakeIAMCache struct {
	invalidated []string
}

func (f *fakeIAMCache) InvalidateUser(userID string) {
	f.invalidated = append(f.invalidated, userID)
}

type recordingUserAdminAudit struct {
	events []audit.Event
}

func (r *recordingUserAdminAudit) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func allUserDepartmentRoleScope(action iam.Action) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourceUserDepartmentRole,
		Action:   action,
		Branches: []iam.ScopeBranch{{AllDepartments: true}},
	}
}
