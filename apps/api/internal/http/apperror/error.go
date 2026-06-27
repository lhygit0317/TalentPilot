package apperror

type Code string

const (
	Unauthenticated      Code = "AUTH_UNAUTHENTICATED"
	PermissionDenied     Code = "IAM_PERMISSION_DENIED"
	IAMRoleRelationCycle Code = "IAM_ROLE_RELATION_CYCLE"
	ValidationFailed     Code = "VALIDATION_FAILED"
	Internal             Code = "INTERNAL_ERROR"
)

type Problem struct {
	Code      Code           `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details,omitempty"`
}

func NewProblem(code Code, message string, requestID string, details map[string]any) Problem {
	return Problem{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Details:   details,
	}
}
