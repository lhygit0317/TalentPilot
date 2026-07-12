package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
)

type matchingParseBody struct {
	ResumeID   string `json:"resumeId" required:"true"`
	PositionID string `json:"positionId" required:"true"`
}

type matchingParseInput struct {
	Body matchingParseBody `json:"body"`
}

type matchingInterviewBody struct {
	ResumeID   string `json:"resumeId" required:"true"`
	PositionID string `json:"positionId" required:"true"`
	MatchScore *int   `json:"matchScore,omitempty" minimum:"0" maximum:"100"`
}

type matchingInterviewInput struct {
	Body matchingInterviewBody `json:"body"`
}

type matchingParseOutput struct {
	Body matching.ParseResult `json:"body"`
}

type matchingInterviewOutput struct {
	Body matching.InterviewQuestionResult `json:"body"`
}

func registerMatchingRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{
		OperationID: "post-matching-parse",
		Method:      http.MethodPost,
		Path:        "/matching/parse",
		Summary:     "Parse resume against a position",
		Tags:        []string{"matching"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *matchingParseInput) (*matchingParseOutput, error) {
		principal, resumeScope, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionList); err != nil {
			return nil, err
		}
		_, positionScope, err := authorizeRequest(ctx, options, iam.ResourcePosition, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		_, positionResumeCreateScope, err := authorizeRequest(ctx, options, iam.ResourcePositionResume, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		service, err := requireMatchingService(options.MatchingService)
		if err != nil {
			return nil, err
		}
		result, err := service.Parse(ctx, matching.ParseInput{
			ActorUserID:               principal.User.ID,
			ResumeID:                  input.Body.ResumeID,
			PositionID:                input.Body.PositionID,
			ResumeScope:               resumeScope,
			PositionScope:             positionScope,
			PositionResumeCreateScope: positionResumeCreateScope,
		})
		if err != nil {
			return nil, mapMatchingError(err)
		}
		return &matchingParseOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-matching-interview-questions",
		Method:      http.MethodPost,
		Path:        "/matching/interview-questions",
		Summary:     "Generate interview questions",
		Tags:        []string{"matching"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *matchingInterviewInput) (*matchingInterviewOutput, error) {
		_, resumeScope, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentPosition, iam.ActionList); err != nil {
			return nil, err
		}
		_, positionScope, err := authorizeRequest(ctx, options, iam.ResourcePosition, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		service, err := requireMatchingService(options.MatchingService)
		if err != nil {
			return nil, err
		}
		result, err := service.GenerateInterviewQuestions(ctx, matching.InterviewQuestionInput{
			ResumeID:      input.Body.ResumeID,
			PositionID:    input.Body.PositionID,
			MatchScore:    input.Body.MatchScore,
			ResumeScope:   resumeScope,
			PositionScope: positionScope,
		})
		if err != nil {
			return nil, mapMatchingError(err)
		}
		return &matchingInterviewOutput{Body: result}, nil
	})
}

func requireMatchingService(service MatchingService) (MatchingService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.Internal, "匹配服务未配置", "", nil)
	}
	return service, nil
}

func mapMatchingError(err error) error {
	switch {
	case errors.Is(err, matching.ErrResumeNotFound):
		return apperror.NewProblem(apperror.ResumeNotFound, "", "", nil)
	case errors.Is(err, matching.ErrPositionNotFound):
		return apperror.NewProblem(apperror.PositionNotFound, "", "", nil)
	case errors.Is(err, matching.ErrPositionOffline):
		return apperror.NewProblem(apperror.MatchingPositionOffline, "", "", nil)
	case errors.Is(err, matching.ErrPositionResumeCreateDenied):
		return apperror.NewProblem(apperror.PermissionDenied, "", "", nil)
	case errors.Is(err, matching.ErrInterviewQuestionGenerateFail):
		return apperror.NewProblem(apperror.MatchingInterviewFailed, "", "", nil)
	default:
		return apperror.NewProblem(apperror.MatchingParseFailed, "", "", nil)
	}
}
