package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/talentpilot/talentpilot/apps/api/internal/auth"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

type authResultContextKey struct{}
type iamServiceContextKey struct{}
type scopePredicateContextKey struct{}

func RequirePermission(resource iam.Resource, action iam.Action) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			authResult, ok := authResultFromContext(ctx)
			if !ok {
				return writeProblem(c, apperror.NewProblem(apperror.Unauthenticated, "", "", nil))
			}
			service, ok := iamServiceFromContext(ctx)
			if !ok || service == nil {
				return writeProblem(c, apperror.NewProblem(apperror.Internal, "权限服务未配置", "", nil))
			}
			principal, err := service.ResolvePrincipal(ctx, authResult.User.ID)
			if err != nil {
				return writeProblem(c, mapIAMError(err))
			}
			decision := service.Can(ctx, principal, resource, action, iam.Target{})
			if !decision.Allowed {
				return writeProblem(c, apperror.NewProblem(apperror.PermissionDenied, "", "", map[string]any{"resource": resource, "action": action}))
			}
			if action == iam.ActionList {
				scope, err := service.Scope(ctx, principal, resource, action)
				if err != nil {
					return writeProblem(c, mapIAMError(err))
				}
				ctx = context.WithValue(ctx, scopePredicateContextKey{}, scope)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}
}

func ScopePredicateFromContext(ctx context.Context) (iam.ScopePredicate, bool) {
	scope, ok := ctx.Value(scopePredicateContextKey{}).(iam.ScopePredicate)
	return scope, ok
}

func AuthenticatedUserFromContext(ctx context.Context) (auth.UserSummary, bool) {
	result, ok := authResultFromContext(ctx)
	return result.User, ok
}

func authResultFromContext(ctx context.Context) (auth.LoginResult, bool) {
	result, ok := ctx.Value(authResultContextKey{}).(auth.LoginResult)
	return result, ok
}

func iamServiceFromContext(ctx context.Context) (IAMService, bool) {
	service, ok := ctx.Value(iamServiceContextKey{}).(IAMService)
	return service, ok
}

func mapIAMError(err error) error {
	switch {
	case errors.Is(err, iam.ErrInvalidResource):
		return apperror.NewProblem(apperror.IAMInvalidResource, "", "", nil)
	case errors.Is(err, iam.ErrInvalidAction):
		return apperror.NewProblem(apperror.IAMInvalidAction, "", "", nil)
	case errors.Is(err, iam.ErrInvalidAttributeCondition):
		return apperror.NewProblem(apperror.IAMInvalidAttributeCondition, "", "", nil)
	case errors.Is(err, iam.ErrPermissionNotFound):
		return apperror.NewProblem(apperror.IAMPermissionNotFound, "", "", nil)
	case errors.Is(err, iam.ErrRoleRelationCycle):
		return apperror.NewProblem(apperror.IAMRoleRelationCycle, "", "", nil)
	case errors.Is(err, iam.ErrRoleRelationDepthExceeded):
		return apperror.NewProblem(apperror.IAMRoleRelationDepthExceeded, "", "", nil)
	case errors.Is(err, iam.ErrPrincipalNotFound):
		return apperror.NewProblem(apperror.IAMPrincipalNotFound, "", "", nil)
	case errors.Is(err, iam.ErrScopeUnsupported):
		return apperror.NewProblem(apperror.IAMScopeUnsupported, "", "", nil)
	default:
		return apperror.NewStatusProblem(http.StatusForbidden, apperror.PermissionDenied, "", "", nil)
	}
}
