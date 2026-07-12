package useradmin

import (
	"context"
	"fmt"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

const maxBindingBatchSize = 20

type Service struct {
	store Store
	iam   IAMCache
	audit audit.Recorder
}

func NewService(store Store, iamCache IAMCache, recorder audit.Recorder) *Service {
	if recorder == nil {
		recorder = audit.NopRecorder{}
	}
	return &Service{store: store, iam: iamCache, audit: recorder}
}

func (s *Service) ListUsers(ctx context.Context, query ListUsersQuery) (UserListResult, error) {
	return s.store.ListUsers(ctx, query)
}

func (s *Service) GetUser(ctx context.Context, userID string, scope iam.ScopePredicate) (UserDetail, error) {
	return s.store.GetUser(ctx, userID, scope)
}

func (s *Service) ListAssignableRoles(ctx context.Context) (AssignableRoleListResult, error) {
	roles, err := s.store.ListAssignableRoles(ctx)
	if err != nil {
		return AssignableRoleListResult{}, err
	}
	return AssignableRoleListResult{Items: roles}, nil
}

func (s *Service) CreateRoleBindings(ctx context.Context, input CreateRoleBindingsInput) (CreateRoleBindingsResult, error) {
	if len(input.Bindings) == 0 {
		return CreateRoleBindingsResult{}, ErrBatchEmpty
	}
	if len(input.Bindings) > maxBindingBatchSize {
		return CreateRoleBindingsResult{}, ErrBatchTooLarge
	}
	if err := rejectDuplicateRequests(input.Bindings); err != nil {
		return CreateRoleBindingsResult{}, err
	}

	user, err := s.store.GetUserIdentity(ctx, input.UserID)
	if err != nil {
		return CreateRoleBindingsResult{}, err
	}

	var created []RoleBindingDetail
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		var createErr error
		created, createErr = store.CreateRoleBindings(ctx, CreateRoleBindingsCommand{
			ActorUserID: input.ActorUserID,
			UserID:      input.UserID,
			CreateScope: input.CreateScope,
			Bindings:    input.Bindings,
		})
		return createErr
	}); err != nil {
		return CreateRoleBindingsResult{}, err
	}

	if s.iam != nil {
		s.iam.InvalidateUser(input.UserID)
	}
	for _, binding := range created {
		s.recordAudit(ctx, audit.Event{
			Type:     audit.EventUserDepartmentRoleCreated,
			UserID:   input.ActorUserID,
			Resource: string(iam.ResourceUserDepartmentRole),
			Action:   string(iam.ActionCreate),
			TargetID: binding.ID,
			Result:   "succeeded",
			Details: map[string]any{
				"bindingId":    binding.ID,
				"departmentId": binding.Department.ID,
				"roleId":       binding.Role.ID,
				"userId":       input.UserID,
			},
		})
	}

	return CreateRoleBindingsResult{
		User:    user,
		Created: created,
		Message: fmt.Sprintf("已为 %s 分配 %d 个角色绑定", user.Name, len(created)),
	}, nil
}

func (s *Service) DeleteRoleBinding(ctx context.Context, input DeleteRoleBindingInput) (DeleteRoleBindingResult, error) {
	binding, err := s.store.GetRoleBinding(ctx, input.BindingID)
	if err != nil {
		return DeleteRoleBindingResult{}, err
	}
	if binding.UserID != input.UserID {
		return DeleteRoleBindingResult{}, ErrBindingNotFound
	}
	if binding.Guest {
		return DeleteRoleBindingResult{}, ErrGuestBindingProtected
	}
	if !ScopeAllowsDepartment(input.DeleteScope, binding.Department.ID) {
		return DeleteRoleBindingResult{}, ErrPermissionDenied
	}
	if input.ActorUserID == input.UserID {
		count, err := s.store.CountNonGuestBindings(ctx, input.UserID)
		if err != nil {
			return DeleteRoleBindingResult{}, err
		}
		if count <= 1 {
			return DeleteRoleBindingResult{}, ErrSelfLockout
		}
	}

	var fallbackBinding RoleBindingDetail
	var fallbackCreated bool
	if err := s.store.WithTransaction(ctx, func(store Store) error {
		if _, err := store.DeleteRoleBinding(ctx, input.BindingID); err != nil {
			return err
		}
		remaining, err := store.CountNonGuestBindings(ctx, input.UserID)
		if err != nil {
			return err
		}
		if remaining == 0 {
			var ensureErr error
			fallbackBinding, fallbackCreated, ensureErr = store.EnsureGuestBinding(ctx, input.UserID, input.ActorUserID)
			return ensureErr
		}
		return nil
	}); err != nil {
		return DeleteRoleBindingResult{}, err
	}

	if s.iam != nil {
		s.iam.InvalidateUser(input.UserID)
	}
	s.recordAudit(ctx, audit.Event{
		Type:     audit.EventUserDepartmentRoleDeleted,
		UserID:   input.ActorUserID,
		Resource: string(iam.ResourceUserDepartmentRole),
		Action:   string(iam.ActionDelete),
		TargetID: input.BindingID,
		Result:   "succeeded",
		Details: map[string]any{
			"bindingId":    input.BindingID,
			"departmentId": binding.Department.ID,
			"roleId":       binding.Role.ID,
			"userId":       input.UserID,
		},
	})
	if fallbackCreated {
		s.recordAudit(ctx, audit.Event{
			Type:     audit.EventUserDepartmentRoleCreated,
			UserID:   input.ActorUserID,
			Resource: string(iam.ResourceUserDepartmentRole),
			Action:   string(iam.ActionCreate),
			TargetID: fallbackBinding.ID,
			Result:   "succeeded",
			Details: map[string]any{
				"bindingId":    fallbackBinding.ID,
				"departmentId": fallbackBinding.Department.ID,
				"roleId":       fallbackBinding.Role.ID,
				"userId":       input.UserID,
			},
		})
	}

	return DeleteRoleBindingResult{
		DeletedBindingID: input.BindingID,
		UserID:           input.UserID,
		Message:          "已解除 " + binding.Role.Label + "(部门:" + binding.Department.Name + ")",
	}, nil
}

func rejectDuplicateRequests(bindings []RoleBindingRequest) error {
	seen := map[string]bool{}
	for _, binding := range bindings {
		key := binding.DepartmentID + "\x00" + binding.RoleID
		if seen[key] {
			return ErrDuplicateBinding
		}
		seen[key] = true
	}
	return nil
}

func ScopeAllowsDepartment(scope iam.ScopePredicate, departmentID string) bool {
	if scope.AllDepartments {
		return true
	}
	for _, branch := range scope.Branches {
		if branch.AllDepartments {
			return true
		}
		for _, allowed := range branch.DepartmentIDs {
			if allowed == departmentID {
				return true
			}
		}
	}
	for _, allowed := range scope.DepartmentIDs {
		if allowed == departmentID {
			return true
		}
	}
	return false
}

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	if event.At.IsZero() {
		event.At = time.Now()
	}
	_ = s.audit.Record(ctx, event)
}
