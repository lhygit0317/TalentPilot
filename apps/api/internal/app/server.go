package app

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humaecho"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/talentpilot/talentpilot/apps/api/internal/auth"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
)

type Server struct {
	Echo *echo.Echo
	API  huma.API
}

type Options struct {
	AuthService         AuthService
	FrontendOrigin      string
	RequireHTTPS        bool
	SecureCookies       bool
	TrustForwardedProto bool
}

type AuthService interface {
	IssueCSRF(context.Context) (string, error)
	Login(context.Context, auth.LoginInput) (auth.LoginResult, error)
	CurrentUser(context.Context, string) (auth.LoginResult, error)
	Logout(context.Context, string, string) error
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func NewServer() *Server {
	return NewServerWithOptions(Options{})
}

func NewServerWithOptions(options Options) *Server {
	apperror.InstallHumaErrorFactory()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	registerMiddleware(e, options)

	cfg := huma.DefaultConfig("TalentPilot API", "0.1.0")
	cfg.CreateHooks = nil
	cfg.Transformers = append(cfg.Transformers, apperror.RequestIDTransformer)

	api := humaecho.New(e, cfg)

	registerHealth(api)
	registerAuthRoutes(api, options)

	return &Server{Echo: e, API: api}
}

func registerMiddleware(e *echo.Echo, options Options) {
	if options.FrontendOrigin != "" {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:     []string{options.FrontendOrigin},
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
			AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "X-CSRF-Token"},
			AllowCredentials: true,
		}))
	}
	if options.RequireHTTPS {
		e.Use(requireHTTPSForCredentialLogin(options.TrustForwardedProto))
	}
	e.Use(authenticatedRequestGuard(options))
}

func requireHTTPSForCredentialLogin(trustForwardedProto bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			if request.Method == http.MethodPost && request.URL.Path == "/auth/w3/login" && !isHTTPSRequest(request, trustForwardedProto) {
				return c.JSON(http.StatusUnauthorized, apperror.NewProblem(apperror.AuthHTTPSRequired, "", "", nil))
			}
			return next(c)
		}
	}
}

func isHTTPSRequest(request *http.Request, trustForwardedProto bool) bool {
	if request.TLS != nil {
		return true
	}
	if !trustForwardedProto {
		return false
	}
	forwardedProto := strings.ToLower(strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")))
	return forwardedProto == "https"
}

func authenticatedRequestGuard(options Options) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			if isAuthBootstrapPath(request.URL.Path) || request.Method == http.MethodOptions {
				return next(c)
			}

			authCookie, err := request.Cookie(authCookieName)
			if err == http.ErrNoCookie {
				return next(c)
			}
			if err != nil {
				return writeProblem(c, mapAuthError(auth.ErrUnauthenticated))
			}

			if isUnsafeMethod(request.Method) {
				csrfCookie, err := request.Cookie(csrfCookieName)
				csrfValue := ""
				if err == nil {
					csrfValue = csrfCookie.Value
				}
				if err := validateCSRF(request.Header.Get("X-CSRF-Token"), csrfValue, request.Header.Get("Origin"), request.Header.Get("Referer"), options.FrontendOrigin); err != nil {
					return writeProblem(c, mapAuthError(err))
				}
			}

			service, err := requireAuthService(options.AuthService)
			if err != nil {
				return writeProblem(c, err)
			}
			if _, err := service.CurrentUser(request.Context(), authCookie.Value); err != nil {
				return writeProblem(c, mapAuthError(err))
			}

			return next(c)
		}
	}
}

func isAuthBootstrapPath(path string) bool {
	return path == "/auth/csrf" || path == "/auth/w3/login"
}

func isUnsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func writeProblem(c echo.Context, err error) error {
	if problem, ok := err.(apperror.Problem); ok {
		return c.JSON(problem.GetStatus(), problem)
	}
	return c.JSON(http.StatusInternalServerError, apperror.NewProblem(apperror.Internal, "", "", nil))
}

func registerHealth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-healthz",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Health check",
		Tags:        []string{"system"},
	}, func(ctx context.Context, input *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}
