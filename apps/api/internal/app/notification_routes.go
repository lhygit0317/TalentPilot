package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/notification"
)

type notificationListInput struct {
	Limit  int    `query:"limit" minimum:"1" maximum:"50"`
	Cursor string `query:"cursor"`
}

type notificationIDInput struct {
	NotificationID string `path:"notificationId"`
}

type notificationSummaryOutput struct {
	Body notification.SummaryResult `json:"body"`
}

type notificationListOutput struct {
	Body notification.ListResult `json:"body"`
}

type notificationReadAllOutput struct {
	Body notification.ReadAllResult `json:"body"`
}

type notificationMarkReadOutput struct {
	Body notification.MarkReadResult `json:"body"`
}

func registerNotificationRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{
		OperationID: "get-notifications-summary",
		Method:      http.MethodGet,
		Path:        "/notifications/summary",
		Summary:     "Get notification unread summary",
		Tags:        []string{"notifications"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *struct{}) (*notificationSummaryOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceNotification, iam.ActionList)
		if err != nil {
			return nil, err
		}
		service, err := requireNotificationService(options.NotificationService)
		if err != nil {
			return nil, err
		}
		result, err := service.Summary(ctx, principal.User.ID)
		if err != nil {
			return nil, mapNotificationError(err, apperror.NotificationListFailed)
		}
		return &notificationSummaryOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-notifications",
		Method:      http.MethodGet,
		Path:        "/notifications",
		Summary:     "List unread notifications",
		Tags:        []string{"notifications"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *notificationListInput) (*notificationListOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceNotification, iam.ActionList)
		if err != nil {
			return nil, err
		}
		service, err := requireNotificationService(options.NotificationService)
		if err != nil {
			return nil, err
		}
		result, err := service.ListUnread(ctx, notification.ListQuery{
			UserID:               principal.User.ID,
			Limit:                input.Limit,
			Cursor:               input.Cursor,
			CanOpenResumeLibrary: hasResumeLibraryAccess(ctx, options, principal),
		})
		if err != nil {
			return nil, mapNotificationError(err, apperror.NotificationListFailed)
		}
		return &notificationListOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-notifications-read-all",
		Method:      http.MethodPost,
		Path:        "/notifications/read-all",
		Summary:     "Mark all current-user notifications read",
		Tags:        []string{"notifications"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *struct{}) (*notificationReadAllOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceNotification, iam.ActionUpdate)
		if err != nil {
			return nil, err
		}
		service, err := requireNotificationService(options.NotificationService)
		if err != nil {
			return nil, err
		}
		result, err := service.MarkAllRead(ctx, principal.User.ID)
		if err != nil {
			return nil, mapNotificationError(err, apperror.NotificationUpdateFailed)
		}
		return &notificationReadAllOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-notification-read",
		Method:      http.MethodPost,
		Path:        "/notifications/{notificationId}/read",
		Summary:     "Mark a current-user notification read",
		Tags:        []string{"notifications"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
	}, func(ctx context.Context, input *notificationIDInput) (*notificationMarkReadOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceNotification, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceNotification, iam.ActionUpdate); err != nil {
			return nil, err
		}
		service, err := requireNotificationService(options.NotificationService)
		if err != nil {
			return nil, err
		}
		result, err := service.MarkRead(ctx, notification.MarkReadInput{
			UserID:               principal.User.ID,
			NotificationID:       input.NotificationID,
			CanOpenResumeLibrary: hasResumeLibraryAccess(ctx, options, principal),
		})
		if err != nil {
			return nil, mapNotificationError(err, apperror.NotificationUpdateFailed)
		}
		return &notificationMarkReadOutput{Body: result}, nil
	})
}

func requireNotificationService(service NotificationService) (NotificationService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.Internal, "通知服务未配置", "", nil)
	}
	return service, nil
}

func hasResumeLibraryAccess(ctx context.Context, options Options, principal iam.Principal) bool {
	if options.IAMService == nil {
		return false
	}
	return options.IAMService.Can(ctx, principal, iam.ResourceResume, iam.ActionList, iam.Target{}).Allowed
}

func mapNotificationError(err error, fallback apperror.Code) error {
	switch {
	case errors.Is(err, notification.ErrNotFound):
		return apperror.NewProblem(apperror.NotificationNotFound, "", "", nil)
	default:
		return apperror.NewProblem(fallback, "", "", nil)
	}
}
