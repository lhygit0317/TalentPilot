package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/recommendation"
)

type recommendationRouteBody struct {
	ResumeID string `json:"resumeId" required:"true"`
}

type recommendationRouteInput struct {
	Body recommendationRouteBody `json:"body"`
}

type recommendationSendBody struct {
	ResumeID     string `json:"resumeId" required:"true"`
	DepartmentID string `json:"departmentId" required:"true"`
	PositionID   string `json:"positionId" required:"true"`
}

type recommendationSendInput struct {
	Body recommendationSendBody `json:"body"`
}

type recommendationRouteOutput struct {
	Body recommendation.RouteResult `json:"body"`
}

type recommendationSendOutput struct {
	Body recommendation.SendResult `json:"body"`
}

func registerRecommendationRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{
		OperationID: "post-recommendation-route",
		Method:      http.MethodPost,
		Path:        "/recommendations/route",
		Summary:     "Route a resume to matching departments",
		Tags:        []string{"recommendations"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *recommendationRouteInput) (*recommendationRouteOutput, error) {
		principal, resumeScope, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		_, positionScope, err := authorizeRequest(ctx, options, iam.ResourcePosition, iam.ActionList)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionList); err != nil {
			return nil, err
		}
		service, err := requireRecommendationService(options.RecommendationService)
		if err != nil {
			return nil, err
		}
		result, err := service.Route(ctx, recommendation.RouteInput{
			ActorUserID:   principal.User.ID,
			ResumeID:      input.Body.ResumeID,
			ResumeScope:   resumeScope,
			PositionScope: positionScope,
		})
		if err != nil {
			return nil, mapRecommendationError(err, true)
		}
		return &recommendationRouteOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-recommendation-send",
		Method:      http.MethodPost,
		Path:        "/recommendations/send",
		Summary:     "Send a resume recommendation",
		Tags:        []string{"recommendations"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *recommendationSendInput) (*recommendationSendOutput, error) {
		principal, resumeGetScope, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		_, resumeCreateScope, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		_, departmentResumeCreateScope, err := authorizeRequest(ctx, options, iam.ResourceDepartmentResume, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		_, positionResumeCreateScope, err := authorizeRequest(ctx, options, iam.ResourcePositionResume, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		_, notificationCreateScope, err := authorizeRequest(ctx, options, iam.ResourceNotification, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		service, err := requireRecommendationService(options.RecommendationService)
		if err != nil {
			return nil, err
		}
		result, err := service.Send(ctx, recommendation.SendInput{
			ActorUserID:                 principal.User.ID,
			ActorName:                   principal.User.Name,
			ResumeID:                    input.Body.ResumeID,
			DepartmentID:                input.Body.DepartmentID,
			PositionID:                  input.Body.PositionID,
			ResumeGetScope:              resumeGetScope,
			ResumeCreateScope:           resumeCreateScope,
			DepartmentResumeCreateScope: departmentResumeCreateScope,
			PositionResumeCreateScope:   positionResumeCreateScope,
			NotificationCreateScope:     notificationCreateScope,
		})
		if err != nil {
			return nil, mapRecommendationError(err, false)
		}
		return &recommendationSendOutput{Body: result}, nil
	})
}

func requireRecommendationService(service RecommendationService) (RecommendationService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.Internal, "推荐服务未配置", "", nil)
	}
	return service, nil
}

func mapRecommendationError(err error, route bool) error {
	switch {
	case errors.Is(err, recommendation.ErrResumeNotFound):
		return apperror.NewProblem(apperror.ResumeNotFound, "", "", nil)
	case errors.Is(err, recommendation.ErrTargetPositionOffline):
		return apperror.NewProblem(apperror.RecommendationTargetPositionOffline, "", "", nil)
	case errors.Is(err, recommendation.ErrTargetPositionMismatch):
		return apperror.NewProblem(apperror.RecommendationTargetPositionMismatch, "", "", nil)
	case errors.Is(err, recommendation.ErrChannelMismatch):
		return apperror.NewProblem(apperror.RecommendationChannelMismatch, "", "", nil)
	default:
		if route {
			return apperror.NewProblem(apperror.RecommendationRouteFailed, "", "", nil)
		}
		return apperror.NewProblem(apperror.RecommendationSendFailed, "", "", nil)
	}
}
