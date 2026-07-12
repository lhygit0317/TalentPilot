package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/useradmin"
)

type userListInput struct {
	Search string `query:"search"`
	Limit  int    `query:"limit" minimum:"1" maximum:"100"`
	Cursor string `query:"cursor"`
}

type userIDInput struct {
	UserID string `path:"userId"`
}

type createUserRoleBindingsInput struct {
	UserID string                     `path:"userId"`
	Body   createUserRoleBindingsBody `json:"body"`
}

type createUserRoleBindingsBody struct {
	Bindings []useradmin.RoleBindingRequest `json:"bindings" nullable:"false"`
}

type deleteUserRoleBindingInput struct {
	UserID    string `path:"userId"`
	BindingID string `path:"bindingId"`
}

type userListOutput struct {
	Body useradmin.UserListResult `json:"body"`
}

type userDetailOutput struct {
	Body useradmin.UserDetail `json:"body"`
}

type assignableRoleListOutput struct {
	Body useradmin.AssignableRoleListResult `json:"body"`
}

type createUserRoleBindingsOutput struct {
	Body useradmin.CreateRoleBindingsResult `json:"body"`
}

type deleteUserRoleBindingOutput struct {
	Body useradmin.DeleteRoleBindingResult `json:"body"`
}

func registerUserAdminRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{
		OperationID: "get-users",
		Method:      http.MethodGet,
		Path:        "/users",
		Summary:     "List users and role bindings",
		Tags:        []string{"user-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *userListInput) (*userListOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceUser, iam.ActionList)
		if err != nil {
			return nil, err
		}
		_, listScope, err := authorizeRequest(ctx, options, iam.ResourceUserDepartmentRole, iam.ActionList)
		if err != nil {
			return nil, err
		}
		service, err := requireUserAdminService(options.UserAdminService)
		if err != nil {
			return nil, err
		}
		deleteScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourceUserDepartmentRole, iam.ActionDelete)
		if err != nil {
			deleteScope = iam.ScopePredicate{}
		}
		_, err = scopeForPrincipal(ctx, options, principal, iam.ResourceUserDepartmentRole, iam.ActionCreate)
		canAssign := err == nil
		result, err := service.ListUsers(ctx, useradmin.ListUsersQuery{
			ActorUserID: principal.User.ID,
			Search:      input.Search,
			Limit:       input.Limit,
			Cursor:      input.Cursor,
			ListScope:   listScope,
			DeleteScope: deleteScope,
			CanAssign:   canAssign,
		})
		if err != nil {
			return nil, mapUserAdminError(err)
		}
		return &userListOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/users/{userId}",
		Summary:     "Get user role bindings",
		Tags:        []string{"user-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
	}, func(ctx context.Context, input *userIDInput) (*userDetailOutput, error) {
		_, _, err := authorizeRequest(ctx, options, iam.ResourceUser, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		_, listScope, err := authorizeRequest(ctx, options, iam.ResourceUserDepartmentRole, iam.ActionList)
		if err != nil {
			return nil, err
		}
		service, err := requireUserAdminService(options.UserAdminService)
		if err != nil {
			return nil, err
		}
		user, err := service.GetUser(ctx, input.UserID, listScope)
		if err != nil {
			return nil, mapUserAdminError(err)
		}
		return &userDetailOutput{Body: user}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-assignable-roles",
		Method:      http.MethodGet,
		Path:        "/roles/assignable",
		Summary:     "List roles assignable to users",
		Tags:        []string{"user-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *struct{}) (*assignableRoleListOutput, error) {
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceUserDepartmentRole, iam.ActionCreate); err != nil {
			return nil, err
		}
		service, err := requireUserAdminService(options.UserAdminService)
		if err != nil {
			return nil, err
		}
		roles, err := service.ListAssignableRoles(ctx)
		if err != nil {
			return nil, mapUserAdminError(err)
		}
		return &assignableRoleListOutput{Body: roles}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-user-role-bindings",
		Method:      http.MethodPost,
		Path:        "/users/{userId}/role-bindings",
		Summary:     "Assign user role bindings",
		Tags:        []string{"user-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *createUserRoleBindingsInput) (*createUserRoleBindingsOutput, error) {
		principal, createScope, err := authorizeRequest(ctx, options, iam.ResourceUserDepartmentRole, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		service, err := requireUserAdminService(options.UserAdminService)
		if err != nil {
			return nil, err
		}
		result, err := service.CreateRoleBindings(ctx, useradmin.CreateRoleBindingsInput{
			ActorUserID: principal.User.ID,
			UserID:      input.UserID,
			CreateScope: createScope,
			Bindings:    input.Body.Bindings,
		})
		if err != nil {
			return nil, mapUserAdminError(err)
		}
		return &createUserRoleBindingsOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-user-role-binding",
		Method:      http.MethodDelete,
		Path:        "/users/{userId}/role-bindings/{bindingId}",
		Summary:     "Delete a user role binding",
		Tags:        []string{"user-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *deleteUserRoleBindingInput) (*deleteUserRoleBindingOutput, error) {
		principal, deleteScope, err := authorizeRequest(ctx, options, iam.ResourceUserDepartmentRole, iam.ActionDelete)
		if err != nil {
			return nil, err
		}
		service, err := requireUserAdminService(options.UserAdminService)
		if err != nil {
			return nil, err
		}
		result, err := service.DeleteRoleBinding(ctx, useradmin.DeleteRoleBindingInput{
			ActorUserID: principal.User.ID,
			UserID:      input.UserID,
			BindingID:   input.BindingID,
			DeleteScope: deleteScope,
		})
		if err != nil {
			return nil, mapUserAdminError(err)
		}
		return &deleteUserRoleBindingOutput{Body: result}, nil
	})
}

func requireUserAdminService(service UserAdminService) (UserAdminService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.Internal, "用户管理服务未配置", "", nil)
	}
	return service, nil
}

func mapUserAdminError(err error) error {
	switch {
	case errors.Is(err, useradmin.ErrUserNotFound):
		return apperror.NewProblem(apperror.UserNotFound, "", "", nil)
	case errors.Is(err, useradmin.ErrBindingNotFound):
		return apperror.NewProblem(apperror.UserRoleBindingNotFound, "", "", nil)
	case errors.Is(err, useradmin.ErrDuplicateBinding):
		return apperror.NewProblem(apperror.UserRoleBindingDuplicate, "", "", nil)
	case errors.Is(err, useradmin.ErrBatchEmpty):
		return apperror.NewProblem(apperror.UserRoleBindingBatchEmpty, "", "", nil)
	case errors.Is(err, useradmin.ErrBatchTooLarge):
		return apperror.NewProblem(apperror.UserRoleBindingBatchTooLarge, "", "", nil)
	case errors.Is(err, useradmin.ErrGuestBindingProtected):
		return apperror.NewProblem(apperror.UserRoleBindingGuestProtected, "", "", nil)
	case errors.Is(err, useradmin.ErrSelfLockout):
		return apperror.NewProblem(apperror.UserRoleBindingSelfLockout, "", "", nil)
	case errors.Is(err, useradmin.ErrRoleDisabled):
		return apperror.NewProblem(apperror.UserRoleBindingRoleDisabled, "", "", nil)
	case errors.Is(err, iam.ErrScopeUnsupported):
		return apperror.NewProblem(apperror.IAMScopeUnsupported, "", "", nil)
	case errors.Is(err, useradmin.ErrPermissionDenied):
		return apperror.NewProblem(apperror.PermissionDenied, "", "", nil)
	default:
		return apperror.NewProblem(apperror.Internal, "", "", nil)
	}
}
