package apperror

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type Code string

const (
	Unauthenticated      Code = "AUTH_UNAUTHENTICATED"
	AuthCSRFInvalid      Code = "AUTH_CSRF_INVALID"
	AuthW3Invalid        Code = "AUTH_W3_INVALID_CREDENTIALS"
	AuthW3Unavailable    Code = "AUTH_W3_UNAVAILABLE"
	AuthW3Timeout        Code = "AUTH_W3_TIMEOUT"
	AuthSessionExpired   Code = "AUTH_SESSION_EXPIRED"
	AuthSessionRevoked   Code = "AUTH_SESSION_REVOKED"
	AuthLoginFailed      Code = "AUTH_LOGIN_FAILED"
	AuthHTTPSRequired    Code = "AUTH_HTTPS_REQUIRED"
	PermissionDenied     Code = "IAM_PERMISSION_DENIED"
	IAMRoleRelationCycle Code = "IAM_ROLE_RELATION_CYCLE"
	ValidationFailed     Code = "VALIDATION_FAILED"
	Internal             Code = "INTERNAL_ERROR"
)

type Problem struct {
	status    int
	Code      Code           `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
}

func NewProblem(code Code, message string, requestID string, details map[string]any) Problem {
	if message == "" {
		message = defaultMessage(code)
	}
	return NewStatusProblem(statusForCode(code), code, message, requestID, details)
}

func NewStatusProblem(status int, code Code, message string, requestID string, details map[string]any) Problem {
	if details == nil {
		details = map[string]any{}
	}
	return Problem{
		status:    status,
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Details:   details,
	}
}

func (p Problem) Error() string {
	return p.Message
}

func (p Problem) GetStatus() int {
	if p.status == 0 {
		return http.StatusInternalServerError
	}
	return p.status
}

func InstallHumaErrorFactory() {
	huma.NewError = func(status int, message string, errs ...error) huma.StatusError {
		return newHumaProblem(status, message, "", errs...)
	}
	huma.NewErrorWithContext = func(ctx huma.Context, status int, message string, errs ...error) huma.StatusError {
		return newHumaProblem(status, message, requestIDFromContext(ctx), errs...)
	}
}

func RequestIDTransformer(ctx huma.Context, status string, value any) (any, error) {
	switch problem := value.(type) {
	case Problem:
		if problem.RequestID == "" {
			problem.RequestID = requestIDFromContext(ctx)
		}
		return problem, nil
	case *Problem:
		if problem != nil && problem.RequestID == "" {
			problem.RequestID = requestIDFromContext(ctx)
		}
		return problem, nil
	default:
		return value, nil
	}
}

func newHumaProblem(status int, message string, requestID string, errs ...error) Problem {
	code := codeForStatus(status)
	if message == "" {
		message = defaultMessage(code)
	}
	return NewStatusProblem(status, code, message, requestID, detailsFromErrors(errs))
}

func codeForStatus(status int) Code {
	switch status {
	case http.StatusUnauthorized:
		return Unauthenticated
	case http.StatusForbidden:
		return PermissionDenied
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ValidationFailed
	default:
		return Internal
	}
}

func statusForCode(code Code) int {
	switch code {
	case Unauthenticated, AuthCSRFInvalid, AuthW3Invalid, AuthSessionExpired, AuthSessionRevoked, AuthHTTPSRequired:
		return http.StatusUnauthorized
	case AuthW3Unavailable:
		return http.StatusServiceUnavailable
	case AuthW3Timeout:
		return http.StatusGatewayTimeout
	case PermissionDenied:
		return http.StatusForbidden
	case ValidationFailed, IAMRoleRelationCycle:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func defaultMessage(code Code) string {
	switch code {
	case Unauthenticated:
		return "请先登录"
	case AuthCSRFInvalid:
		return "登录校验已失效，请刷新后重试"
	case AuthW3Invalid:
		return "W3 账号或密码不正确"
	case AuthW3Unavailable:
		return "W3 服务暂不可用"
	case AuthW3Timeout:
		return "W3 认证超时"
	case AuthSessionExpired:
		return "登录已过期"
	case AuthSessionRevoked:
		return "登录已在其他设备失效"
	case AuthLoginFailed:
		return "登录失败，请稍后重试"
	case AuthHTTPSRequired:
		return "生产环境必须使用 HTTPS 登录"
	case PermissionDenied:
		return "没有权限"
	case ValidationFailed, IAMRoleRelationCycle:
		return "请求参数不合法"
	default:
		return "服务器内部错误"
	}
}

func requestIDFromContext(ctx huma.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID := ctx.Header("X-Request-ID"); requestID != "" {
		return requestID
	}
	return ctx.Header("X-Request-Id")
}

func detailsFromErrors(errs []error) map[string]any {
	if len(errs) == 0 {
		return map[string]any{}
	}

	items := make([]map[string]any, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		item := map[string]any{"message": err.Error()}
		if detailer, ok := err.(huma.ErrorDetailer); ok {
			detail := detailer.ErrorDetail()
			item["message"] = detail.Message
			if detail.Location != "" {
				item["location"] = detail.Location
			}
			if detail.Value != nil {
				item["valuePresent"] = true
			}
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return map[string]any{}
	}
	return map[string]any{"errors": items}
}
