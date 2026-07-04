package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/organization"
)

type departmentListInput struct {
	Search string `query:"search"`
	Limit  int    `query:"limit" minimum:"1" maximum:"100"`
}

type departmentIDInput struct {
	DepartmentID string `path:"departmentId"`
}

type departmentBody struct {
	Name string `json:"name" required:"true"`
}

type createDepartmentInput struct {
	Body departmentBody `json:"body"`
}

type updateDepartmentInput struct {
	DepartmentID string         `path:"departmentId"`
	Body         departmentBody `json:"body"`
}

type positionListInput struct {
	DepartmentID string `query:"departmentId"`
	Chan         string `query:"chan" enum:"social,campus"`
	Status       string `query:"status" enum:"on,off"`
	Search       string `query:"search"`
	Limit        int    `query:"limit" minimum:"1" maximum:"100"`
}

type positionIDInput struct {
	PositionID string `path:"positionId"`
}

type implicitTagBody struct {
	Name   string `json:"name"`
	Weight *int   `json:"w,omitempty" minimum:"0" maximum:"100"`
}

type positionBody struct {
	Name         string            `json:"name" required:"true"`
	DepartmentID string            `json:"departmentId" required:"true"`
	Chan         string            `json:"chan" enum:"social,campus" required:"true"`
	Level        string            `json:"level"`
	Status       string            `json:"status" enum:"on,off" required:"true"`
	Duties       []string          `json:"duties" nullable:"false"`
	Must         []string          `json:"must" nullable:"false"`
	Keywords     []string          `json:"keywords" nullable:"false"`
	ImplicitTags []implicitTagBody `json:"implicitTags" nullable:"false"`
}

type createPositionInput struct {
	Body positionBody `json:"body"`
}

type updatePositionInput struct {
	PositionID string       `path:"positionId"`
	Body       positionBody `json:"body"`
}

type departmentListOutput struct {
	Body organization.DepartmentListResult `json:"body"`
}

type departmentDetailOutput struct {
	Body organization.DepartmentDetail `json:"body"`
}

type positionListOutput struct {
	Body organization.PositionListResult `json:"body"`
}

type positionDetailOutput struct {
	Body organization.PositionDetail `json:"body"`
}

type deleteOrganizationOutput struct {
	Status int `json:"-"`
}

func registerOrganizationRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{
		OperationID: "get-departments",
		Method:      http.MethodGet,
		Path:        "/departments",
		Summary:     "List departments",
		Tags:        []string{"organization"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *departmentListInput) (*departmentListOutput, error) {
		principal, scope, err := authorizeRequest(ctx, options, iam.ResourceDepartment, iam.ActionList)
		if err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		getScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourceDepartment, iam.ActionGet)
		if err != nil {
			getScope = iam.ScopePredicate{}
		}
		updateScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourceDepartment, iam.ActionUpdate)
		if err != nil {
			updateScope = iam.ScopePredicate{}
		}
		deleteScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourceDepartment, iam.ActionDelete)
		if err != nil {
			deleteScope = iam.ScopePredicate{}
		}
		result, err := service.ListDepartments(ctx, organization.DepartmentListQuery{
			Search:      input.Search,
			Limit:       input.Limit,
			Scope:       scope,
			GetScope:    getScope,
			UpdateScope: updateScope,
			DeleteScope: deleteScope,
		})
		if err != nil {
			return nil, mapOrganizationError(err)
		}
		return &departmentListOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-department",
		Method:      http.MethodGet,
		Path:        "/departments/{departmentId}",
		Summary:     "Get department detail",
		Tags:        []string{"organization"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
	}, func(ctx context.Context, input *departmentIDInput) (*departmentDetailOutput, error) {
		_, scope, err := authorizeRequest(ctx, options, iam.ResourceDepartment, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		detail, err := service.GetDepartment(ctx, input.DepartmentID, scope)
		if err != nil {
			return nil, mapOrganizationError(err)
		}
		return &departmentDetailOutput{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-department",
		Method:      http.MethodPost,
		Path:        "/departments",
		Summary:     "Create department",
		Tags:        []string{"organization"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *createDepartmentInput) (*departmentDetailOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceDepartment, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		detail, err := service.CreateDepartment(ctx, organization.DepartmentInput{ActorUserID: principal.User.ID, Name: input.Body.Name})
		if err != nil {
			return nil, mapOrganizationError(err)
		}
		return &departmentDetailOutput{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "patch-department",
		Method:      http.MethodPatch,
		Path:        "/departments/{departmentId}",
		Summary:     "Update department",
		Tags:        []string{"organization"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *updateDepartmentInput) (*departmentDetailOutput, error) {
		principal, scope, err := authorizeRequest(ctx, options, iam.ResourceDepartment, iam.ActionUpdate)
		if err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		detail, err := service.UpdateDepartment(ctx, input.DepartmentID, organization.DepartmentInput{ActorUserID: principal.User.ID, Name: input.Body.Name}, scope)
		if err != nil {
			return nil, mapOrganizationError(err)
		}
		return &departmentDetailOutput{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-department",
		Method:        http.MethodDelete,
		Path:          "/departments/{departmentId}",
		Summary:       "Delete department",
		Tags:          []string{"organization"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *departmentIDInput) (*deleteOrganizationOutput, error) {
		principal, scope, err := authorizeRequest(ctx, options, iam.ResourceDepartment, iam.ActionDelete)
		if err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		if err := service.DeleteDepartment(ctx, input.DepartmentID, scope, principal.User.ID); err != nil {
			return nil, mapOrganizationError(err)
		}
		return &deleteOrganizationOutput{Status: http.StatusNoContent}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-positions",
		Method:      http.MethodGet,
		Path:        "/positions",
		Summary:     "List positions",
		Tags:        []string{"organization"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *positionListInput) (*positionListOutput, error) {
		principal, scope, err := authorizeRequest(ctx, options, iam.ResourcePosition, iam.ActionList)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionList); err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		getScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourcePosition, iam.ActionGet)
		if err != nil {
			getScope = iam.ScopePredicate{}
		}
		updateScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourcePosition, iam.ActionUpdate)
		if err != nil {
			updateScope = iam.ScopePredicate{}
		}
		deleteScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourcePosition, iam.ActionDelete)
		if err != nil {
			deleteScope = iam.ScopePredicate{}
		}
		result, err := service.ListPositions(ctx, organization.PositionListQuery{
			DepartmentID: input.DepartmentID,
			Chan:         input.Chan,
			Status:       input.Status,
			Search:       input.Search,
			Limit:        input.Limit,
			Scope:        scope,
			GetScope:     getScope,
			UpdateScope:  updateScope,
			DeleteScope:  deleteScope,
		})
		if err != nil {
			return nil, mapOrganizationError(err)
		}
		return &positionListOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-position",
		Method:      http.MethodGet,
		Path:        "/positions/{positionId}",
		Summary:     "Get position detail",
		Tags:        []string{"organization"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
	}, func(ctx context.Context, input *positionIDInput) (*positionDetailOutput, error) {
		_, scope, err := authorizeRequest(ctx, options, iam.ResourcePosition, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionList); err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		detail, err := service.GetPosition(ctx, input.PositionID, scope)
		if err != nil {
			return nil, mapOrganizationError(err)
		}
		return &positionDetailOutput{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-position",
		Method:      http.MethodPost,
		Path:        "/positions",
		Summary:     "Create position",
		Tags:        []string{"organization"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *createPositionInput) (*positionDetailOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourcePosition, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionCreate); err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		detail, err := service.CreatePosition(ctx, positionInputFromBody(principal.User.ID, input.Body))
		if err != nil {
			return nil, mapOrganizationError(err)
		}
		return &positionDetailOutput{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "patch-position",
		Method:      http.MethodPatch,
		Path:        "/positions/{positionId}",
		Summary:     "Update position",
		Tags:        []string{"organization"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *updatePositionInput) (*positionDetailOutput, error) {
		principal, scope, err := authorizeRequest(ctx, options, iam.ResourcePosition, iam.ActionUpdate)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionCreate); err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionDelete); err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		detail, err := service.UpdatePosition(ctx, input.PositionID, positionInputFromBody(principal.User.ID, input.Body), scope)
		if err != nil {
			return nil, mapOrganizationError(err)
		}
		return &positionDetailOutput{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-position",
		Method:        http.MethodDelete,
		Path:          "/positions/{positionId}",
		Summary:       "Delete position",
		Tags:          []string{"organization"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *positionIDInput) (*deleteOrganizationOutput, error) {
		principal, scope, err := authorizeRequest(ctx, options, iam.ResourcePosition, iam.ActionDelete)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionDelete); err != nil {
			return nil, err
		}
		service, err := requireOrganizationService(options.OrganizationService)
		if err != nil {
			return nil, err
		}
		if err := service.DeletePosition(ctx, input.PositionID, scope, principal.User.ID); err != nil {
			return nil, mapOrganizationError(err)
		}
		return &deleteOrganizationOutput{Status: http.StatusNoContent}, nil
	})
}

func requireOrganizationService(service OrganizationService) (OrganizationService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.Internal, "组织服务未配置", "", nil)
	}
	return service, nil
}

func positionInputFromBody(actorUserID string, body positionBody) organization.PositionInput {
	tags := make([]organization.ImplicitTagInput, 0, len(body.ImplicitTags))
	for _, tag := range body.ImplicitTags {
		tags = append(tags, organization.ImplicitTagInput{Name: tag.Name, Weight: tag.Weight})
	}
	return organization.PositionInput{
		ActorUserID:  actorUserID,
		Name:         body.Name,
		DepartmentID: body.DepartmentID,
		Chan:         body.Chan,
		Level:        body.Level,
		Status:       body.Status,
		Duties:       body.Duties,
		Must:         body.Must,
		Keywords:     body.Keywords,
		ImplicitTags: tags,
	}
}

func mapOrganizationError(err error) error {
	switch {
	case errors.Is(err, organization.ErrDepartmentNotFound):
		return apperror.NewProblem(apperror.DepartmentNotFound, "", "", nil)
	case errors.Is(err, organization.ErrDepartmentNameRequired):
		return apperror.NewProblem(apperror.DepartmentNameRequired, "", "", nil)
	case errors.Is(err, organization.ErrDepartmentNameDuplicate):
		return apperror.NewProblem(apperror.DepartmentNameDuplicate, "", "", nil)
	case errors.Is(err, organization.ErrDepartmentDeleteHasRelations):
		return apperror.NewProblem(apperror.DepartmentDeleteHasRelations, "", "", nil)
	case errors.Is(err, organization.ErrDepartmentSystemProtected):
		return apperror.NewProblem(apperror.DepartmentSystemProtected, "", "", nil)
	case errors.Is(err, organization.ErrPositionNotFound):
		return apperror.NewProblem(apperror.PositionNotFound, "", "", nil)
	case errors.Is(err, organization.ErrPositionNameRequired):
		return apperror.NewProblem(apperror.PositionNameRequired, "", "", nil)
	case errors.Is(err, organization.ErrPositionDepartmentRequired):
		return apperror.NewProblem(apperror.PositionDepartmentRequired, "", "", nil)
	case errors.Is(err, organization.ErrPositionDepartmentInvalid):
		return apperror.NewProblem(apperror.PositionDepartmentInvalid, "", "", nil)
	case errors.Is(err, organization.ErrPositionInvalidChannel):
		return apperror.NewProblem(apperror.PositionInvalidChannel, "", "", nil)
	case errors.Is(err, organization.ErrPositionInvalidStatus):
		return apperror.NewProblem(apperror.PositionInvalidStatus, "", "", nil)
	case errors.Is(err, organization.ErrPositionDuplicateKeyword):
		return apperror.NewProblem(apperror.PositionDuplicateKeyword, "", "", nil)
	case errors.Is(err, organization.ErrPositionDuplicateImplicitTag):
		return apperror.NewProblem(apperror.PositionDuplicateImplicitTag, "", "", nil)
	case errors.Is(err, organization.ErrPositionInvalidImplicitWeight):
		return apperror.NewProblem(apperror.PositionInvalidImplicitWeight, "", "", nil)
	case errors.Is(err, organization.ErrPositionDeleteHasHistory):
		return apperror.NewProblem(apperror.PositionDeleteHasHistory, "", "", nil)
	default:
		return apperror.NewProblem(apperror.Internal, "", "", nil)
	}
}
