package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/auth"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
)

const authCookieName = "tp_auth"
const csrfCookieName = "tp_csrf"

type csrfInput struct{}

type loginInput struct {
	Body struct {
		Account  string `json:"account" minLength:"1"`
		Password string `json:"password" minLength:"1"`
	}
	CSRFHeader string `header:"X-CSRF-Token"`
	CSRFCookie string `cookie:"tp_csrf"`
	Origin     string `header:"Origin"`
	Referer    string `header:"Referer"`
}

type meInput struct {
	AuthCookie string `cookie:"tp_auth"`
}

type logoutInput struct {
	AuthCookie string `cookie:"tp_auth"`
	CSRFHeader string `header:"X-CSRF-Token"`
	CSRFCookie string `cookie:"tp_csrf"`
	Origin     string `header:"Origin"`
	Referer    string `header:"Referer"`
}

type csrfOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type authOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      authResponse  `json:"body"`
}

type noContentOutput struct {
	Status    int           `json:"-"`
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type authResponse struct {
	User         auth.UserSummary   `json:"user"`
	RoleBindings []auth.RoleBinding `json:"roleBindings" nullable:"false"`
	RoleLabels   []string           `json:"roleLabels" nullable:"false"`
	PageAccess   []string           `json:"pageAccess" nullable:"false"`
	DefaultRoute string             `json:"defaultRoute"`
}

func registerAuthRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{
		OperationID: "get-auth-csrf",
		Method:      http.MethodGet,
		Path:        "/auth/csrf",
		Summary:     "Issue CSRF cookie",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, input *csrfInput) (*csrfOutput, error) {
		service, err := requireAuthService(options.AuthService)
		if err != nil {
			return nil, err
		}
		token, err := service.IssueCSRF(ctx)
		if err != nil {
			return nil, mapAuthError(err)
		}
		return &csrfOutput{SetCookie: []http.Cookie{csrfCookie(token, options.SecureCookies)}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-auth-w3-login",
		Method:      http.MethodPost,
		Path:        "/auth/w3/login",
		Summary:     "Login with W3 credentials",
		Tags:        []string{"auth"},
		Errors:      []int{http.StatusUnauthorized, http.StatusServiceUnavailable, http.StatusGatewayTimeout},
	}, func(ctx context.Context, input *loginInput) (*authOutput, error) {
		service, err := requireAuthService(options.AuthService)
		if err != nil {
			return nil, err
		}
		if err := validateCSRF(input.CSRFHeader, input.CSRFCookie, input.Origin, input.Referer, options.FrontendOrigin); err != nil {
			return nil, mapAuthError(err)
		}
		result, err := service.Login(ctx, auth.LoginInput{Account: input.Body.Account, Password: input.Body.Password})
		if err != nil {
			return nil, mapAuthError(err)
		}
		return &authOutput{
			SetCookie: []http.Cookie{
				authCookie(result.AuthToken, options.SecureCookies),
				csrfCookie(result.CSRFToken, options.SecureCookies),
			},
			Body: authResponseFromResult(result),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/me",
		Summary:     "Get current user",
		Tags:        []string{"auth"},
		Errors:      []int{http.StatusUnauthorized},
	}, func(ctx context.Context, input *meInput) (*authOutput, error) {
		service, err := requireAuthService(options.AuthService)
		if err != nil {
			return nil, err
		}
		result, err := service.CurrentUser(ctx, input.AuthCookie)
		if err != nil {
			return nil, mapAuthError(err)
		}
		return &authOutput{Body: authResponseFromResult(result)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-auth-logout",
		Method:      http.MethodPost,
		Path:        "/auth/logout",
		Summary:     "Logout current session",
		Tags:        []string{"auth"},
		Errors:      []int{http.StatusUnauthorized},
	}, func(ctx context.Context, input *logoutInput) (*noContentOutput, error) {
		service, err := requireAuthService(options.AuthService)
		if err != nil {
			return nil, err
		}
		if err := validateCSRF(input.CSRFHeader, input.CSRFCookie, input.Origin, input.Referer, options.FrontendOrigin); err != nil {
			return nil, mapAuthError(err)
		}
		if err := service.Logout(ctx, input.AuthCookie, input.CSRFCookie); err != nil {
			return nil, mapAuthError(err)
		}
		return &noContentOutput{
			Status: http.StatusNoContent,
			SetCookie: []http.Cookie{
				expiredCookie(authCookieName, true, options.SecureCookies),
				expiredCookie(csrfCookieName, false, options.SecureCookies),
			},
		}, nil
	})
}

func requireAuthService(service AuthService) (AuthService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.AuthLoginFailed, "认证服务未配置", "", nil)
	}
	return service, nil
}

func validateCSRF(header string, cookie string, origin string, referer string, frontendOrigin string) error {
	if header == "" || cookie == "" || header != cookie {
		return auth.ErrCSRFInvalid
	}
	if frontendOrigin == "" {
		return nil
	}
	if origin != "" {
		if origin == frontendOrigin {
			return nil
		}
		return auth.ErrCSRFInvalid
	}
	if referer == "" || !refererMatchesOrigin(referer, frontendOrigin) {
		return auth.ErrCSRFInvalid
	}
	return nil
}

func refererMatchesOrigin(rawReferer string, rawOrigin string) bool {
	refererURL, err := url.Parse(rawReferer)
	if err != nil {
		return false
	}
	originURL, err := url.Parse(rawOrigin)
	if err != nil {
		return false
	}
	return strings.EqualFold(refererURL.Scheme, originURL.Scheme) &&
		strings.EqualFold(refererURL.Host, originURL.Host)
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, auth.ErrCSRFInvalid):
		return apperror.NewProblem(apperror.AuthCSRFInvalid, "", "", nil)
	case errors.Is(err, auth.ErrInvalidCredentials):
		return apperror.NewProblem(apperror.AuthW3Invalid, "", "", nil)
	case errors.Is(err, auth.ErrW3Unavailable):
		return apperror.NewProblem(apperror.AuthW3Unavailable, "", "", nil)
	case errors.Is(err, auth.ErrW3Timeout):
		return apperror.NewProblem(apperror.AuthW3Timeout, "", "", nil)
	case errors.Is(err, auth.ErrUnauthenticated):
		return apperror.NewProblem(apperror.Unauthenticated, "", "", nil)
	default:
		return apperror.NewProblem(apperror.AuthLoginFailed, "", "", nil)
	}
}

func authResponseFromResult(result auth.LoginResult) authResponse {
	return authResponse{
		User:         result.User,
		RoleBindings: result.RoleBindings,
		RoleLabels:   result.RoleLabels,
		PageAccess:   result.PageAccess,
		DefaultRoute: result.DefaultRoute,
	}
}

func authCookie(value string, secure bool) http.Cookie {
	return baseCookie(authCookieName, value, true, secure)
}

func csrfCookie(value string, secure bool) http.Cookie {
	return baseCookie(csrfCookieName, value, false, secure)
}

func baseCookie(name string, value string, httpOnly bool, secure bool) http.Cookie {
	return http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func expiredCookie(name string, httpOnly bool, secure bool) http.Cookie {
	cookie := baseCookie(name, "", httpOnly, secure)
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(0, 0).UTC()
	return cookie
}
