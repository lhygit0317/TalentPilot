package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/roleadmin"
)

type roleListInput struct {
	Search  string `query:"search"`
	System  string `query:"system" enum:"true,false"`
	Enabled string `query:"enabled" enum:"true,false"`
	Limit   int    `query:"limit" minimum:"1" maximum:"200"`
}

type roleIDInput struct {
	RoleID string `path:"roleId"`
}

type createRoleInput struct {
	Body roleadmin.RoleDefinitionInput `json:"body"`
}

type updateRoleInput struct {
	RoleID string                        `path:"roleId"`
	Body   roleadmin.RoleDefinitionInput `json:"body"`
}

type toggleRoleEnabledInput struct {
	RoleID string                       `path:"roleId"`
	Body   roleadmin.ToggleEnabledInput `json:"body"`
}

type roleListOutput struct {
	Body roleadmin.RoleListResult `json:"body"`
}

type roleDetailOutput struct {
	Body roleadmin.RoleDetail `json:"body"`
}

type rolePermissionOptionsOutput struct {
	Body roleadmin.PermissionOptionsResult `json:"body"`
}

func registerRoleAdminRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{
		OperationID: "get-roles",
		Method:      http.MethodGet,
		Path:        "/roles",
		Summary:     "List role definitions",
		Tags:        []string{"role-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *roleListInput) (*roleListOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceRole, iam.ActionList)
		if err != nil {
			return nil, err
		}
		service, err := requireRoleAdminService(options.RoleAdminService)
		if err != nil {
			return nil, err
		}
		system, err := optionalBoolQuery(input.System)
		if err != nil {
			return nil, apperror.NewProblem(apperror.ValidationFailed, "", "", map[string]any{"field": "system"})
		}
		enabled, err := optionalBoolQuery(input.Enabled)
		if err != nil {
			return nil, apperror.NewProblem(apperror.ValidationFailed, "", "", map[string]any{"field": "enabled"})
		}
		result, err := service.ListRoles(ctx, roleadmin.RoleListQuery{
			ActorCanCreate: canCreateRoleDefinition(ctx, options, principal),
			ActorCanEdit:   canUsePermission(ctx, options, principal, iam.ResourceRole, iam.ActionUpdate),
			ActorCanDelete: canUsePermission(ctx, options, principal, iam.ResourceRole, iam.ActionDelete),
			ActorCanToggle: canUsePermission(ctx, options, principal, iam.ResourceRole, iam.ActionUpdate),
			Search:         input.Search,
			System:         system,
			Enabled:        enabled,
			Limit:          input.Limit,
		})
		if err != nil {
			return nil, mapRoleAdminError(err)
		}
		return &roleListOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-role-permission-options",
		Method:      http.MethodGet,
		Path:        "/roles/permission-options",
		Summary:     "List role permission options",
		Tags:        []string{"role-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *struct{}) (*rolePermissionOptionsOutput, error) {
		if _, _, err := authorizeRequest(ctx, options, iam.ResourcePermission, iam.ActionList); err != nil {
			return nil, err
		}
		service, err := requireRoleAdminService(options.RoleAdminService)
		if err != nil {
			return nil, err
		}
		result, err := service.PermissionOptions(ctx)
		if err != nil {
			return nil, mapRoleAdminError(err)
		}
		return &rolePermissionOptionsOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-role",
		Method:      http.MethodGet,
		Path:        "/roles/{roleId}",
		Summary:     "Get role definition",
		Tags:        []string{"role-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
	}, func(ctx context.Context, input *roleIDInput) (*roleDetailOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceRole, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		service, err := requireRoleAdminService(options.RoleAdminService)
		if err != nil {
			return nil, err
		}
		result, err := service.GetRole(ctx, input.RoleID, roleadmin.RoleCapabilityQuery{
			ActorCanEdit:   canUsePermission(ctx, options, principal, iam.ResourceRole, iam.ActionUpdate),
			ActorCanDelete: canUsePermission(ctx, options, principal, iam.ResourceRole, iam.ActionDelete),
			ActorCanToggle: canUsePermission(ctx, options, principal, iam.ResourceRole, iam.ActionUpdate),
		})
		if err != nil {
			return nil, mapRoleAdminError(err)
		}
		return &roleDetailOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-role",
		Method:      http.MethodPost,
		Path:        "/roles",
		Summary:     "Create a custom role",
		Tags:        []string{"role-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *createRoleInput) (*roleDetailOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceRole, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		if err := requireRoleDefinitionCreatePermissions(ctx, options, principal); err != nil {
			return nil, err
		}
		service, err := requireRoleAdminService(options.RoleAdminService)
		if err != nil {
			return nil, err
		}
		body := input.Body
		body.ActorUserID = principal.User.ID
		result, err := service.CreateRole(ctx, body)
		if err != nil {
			return nil, mapRoleAdminError(err)
		}
		return &roleDetailOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "patch-role",
		Method:      http.MethodPatch,
		Path:        "/roles/{roleId}",
		Summary:     "Update a role definition",
		Tags:        []string{"role-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *updateRoleInput) (*roleDetailOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceRole, iam.ActionUpdate)
		if err != nil {
			return nil, err
		}
		if err := requireRoleDefinitionUpdatePermissions(ctx, options, principal); err != nil {
			return nil, err
		}
		service, err := requireRoleAdminService(options.RoleAdminService)
		if err != nil {
			return nil, err
		}
		body := input.Body
		body.ActorUserID = principal.User.ID
		result, err := service.UpdateRole(ctx, input.RoleID, body)
		if err != nil {
			return nil, mapRoleAdminError(err)
		}
		return &roleDetailOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "patch-role-enabled",
		Method:      http.MethodPatch,
		Path:        "/roles/{roleId}/enabled",
		Summary:     "Toggle role enabled state",
		Tags:        []string{"role-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *toggleRoleEnabledInput) (*roleDetailOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceRole, iam.ActionUpdate)
		if err != nil {
			return nil, err
		}
		service, err := requireRoleAdminService(options.RoleAdminService)
		if err != nil {
			return nil, err
		}
		body := input.Body
		body.ActorUserID = principal.User.ID
		result, err := service.ToggleEnabled(ctx, input.RoleID, body)
		if err != nil {
			return nil, mapRoleAdminError(err)
		}
		return &roleDetailOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-role",
		Method:      http.MethodDelete,
		Path:        "/roles/{roleId}",
		Summary:     "Delete an unused custom role",
		Tags:        []string{"role-admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *roleIDInput) (*struct{}, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceRole, iam.ActionDelete)
		if err != nil {
			return nil, err
		}
		service, err := requireRoleAdminService(options.RoleAdminService)
		if err != nil {
			return nil, err
		}
		if err := service.DeleteRole(ctx, input.RoleID, principal.User.ID); err != nil {
			return nil, mapRoleAdminError(err)
		}
		return &struct{}{}, nil
	})
}

func optionalBoolQuery(value string) (*bool, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func requireRoleAdminService(service RoleAdminService) (RoleAdminService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.Internal, "角色管理服务未配置", "", nil)
	}
	return service, nil
}

func requireRoleDefinitionCreatePermissions(ctx context.Context, options Options, principal iam.Principal) error {
	if !canUsePermission(ctx, options, principal, iam.ResourcePermission, iam.ActionCreate) {
		return apperror.NewProblem(apperror.PermissionDenied, "", "", map[string]any{"resource": iam.ResourcePermission, "action": iam.ActionCreate})
	}
	if !canUsePermission(ctx, options, principal, iam.ResourceRoleRelation, iam.ActionCreate) {
		return apperror.NewProblem(apperror.PermissionDenied, "", "", map[string]any{"resource": iam.ResourceRoleRelation, "action": iam.ActionCreate})
	}
	return nil
}

func requireRoleDefinitionUpdatePermissions(ctx context.Context, options Options, principal iam.Principal) error {
	checks := []struct {
		resource iam.Resource
		action   iam.Action
	}{
		{iam.ResourcePermission, iam.ActionDelete},
		{iam.ResourcePermission, iam.ActionCreate},
		{iam.ResourceRoleRelation, iam.ActionDelete},
		{iam.ResourceRoleRelation, iam.ActionCreate},
	}
	for _, check := range checks {
		if !canUsePermission(ctx, options, principal, check.resource, check.action) {
			return apperror.NewProblem(apperror.PermissionDenied, "", "", map[string]any{"resource": check.resource, "action": check.action})
		}
	}
	return nil
}

func canCreateRoleDefinition(ctx context.Context, options Options, principal iam.Principal) bool {
	return canUsePermission(ctx, options, principal, iam.ResourceRole, iam.ActionCreate) &&
		canUsePermission(ctx, options, principal, iam.ResourcePermission, iam.ActionCreate) &&
		canUsePermission(ctx, options, principal, iam.ResourceRoleRelation, iam.ActionCreate)
}

func canUsePermission(ctx context.Context, options Options, principal iam.Principal, resource iam.Resource, action iam.Action) bool {
	if options.IAMService == nil {
		return false
	}
	return options.IAMService.Can(ctx, principal, resource, action, iam.Target{}).Allowed
}

func mapRoleAdminError(err error) error {
	switch {
	case errors.Is(err, roleadmin.ErrRoleNotFound):
		return apperror.NewProblem(apperror.RoleNotFound, "", "", nil)
	case errors.Is(err, roleadmin.ErrLabelInvalid):
		return apperror.NewProblem(apperror.RoleLabelInvalid, "", "", nil)
	case errors.Is(err, roleadmin.ErrLabelDuplicate):
		return apperror.NewProblem(apperror.RoleLabelDuplicate, "", "", nil)
	case errors.Is(err, roleadmin.ErrSystemRoleProtected):
		return apperror.NewProblem(apperror.RoleSystemProtected, "", "", nil)
	case errors.Is(err, roleadmin.ErrRoleInUse):
		return apperror.NewProblem(apperror.RoleInUse, "", "", nil)
	case errors.Is(err, roleadmin.ErrPermissionInvalid):
		return apperror.NewProblem(apperror.RolePermissionInvalid, "", "", nil)
	case errors.Is(err, roleadmin.ErrPermissionDuplicate):
		return apperror.NewProblem(apperror.RolePermissionDuplicate, "", "", nil)
	case errors.Is(err, roleadmin.ErrRelationInvalid):
		return apperror.NewProblem(apperror.RoleRelationInvalid, "", "", nil)
	case errors.Is(err, iam.ErrRoleRelationCycle):
		return apperror.NewProblem(apperror.IAMRoleRelationCycle, "", "", nil)
	case errors.Is(err, iam.ErrRoleRelationDepthExceeded):
		return apperror.NewProblem(apperror.IAMRoleRelationDepthExceeded, "", "", nil)
	default:
		return apperror.NewProblem(apperror.Internal, "", "", nil)
	}
}
