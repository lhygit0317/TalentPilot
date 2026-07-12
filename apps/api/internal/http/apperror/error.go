package apperror

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type Code string

const (
	Unauthenticated                      Code = "AUTH_UNAUTHENTICATED"
	AuthCSRFInvalid                      Code = "AUTH_CSRF_INVALID"
	AuthW3Invalid                        Code = "AUTH_W3_INVALID_CREDENTIALS"
	AuthW3Unavailable                    Code = "AUTH_W3_UNAVAILABLE"
	AuthW3Timeout                        Code = "AUTH_W3_TIMEOUT"
	AuthSessionExpired                   Code = "AUTH_SESSION_EXPIRED"
	AuthSessionRevoked                   Code = "AUTH_SESSION_REVOKED"
	AuthLoginFailed                      Code = "AUTH_LOGIN_FAILED"
	AuthHTTPSRequired                    Code = "AUTH_HTTPS_REQUIRED"
	PermissionDenied                     Code = "IAM_PERMISSION_DENIED"
	IAMPermissionNotFound                Code = "IAM_PERMISSION_NOT_FOUND"
	IAMInvalidResource                   Code = "IAM_INVALID_RESOURCE"
	IAMInvalidAction                     Code = "IAM_INVALID_ACTION"
	IAMInvalidAttributeCondition         Code = "IAM_INVALID_ATTRIBUTE_CONDITION"
	IAMRoleRelationCycle                 Code = "IAM_ROLE_RELATION_CYCLE"
	IAMRoleRelationDepthExceeded         Code = "IAM_ROLE_RELATION_DEPTH_EXCEEDED"
	IAMPrincipalNotFound                 Code = "IAM_PRINCIPAL_NOT_FOUND"
	IAMScopeUnsupported                  Code = "IAM_SCOPE_UNSUPPORTED"
	ResumeNotFound                       Code = "RESUME_NOT_FOUND"
	ResumeImportFileTooLarge             Code = "RESUME_IMPORT_FILE_TOO_LARGE"
	ResumeImportUnsupportedType          Code = "RESUME_IMPORT_UNSUPPORTED_TYPE"
	ResumeImportTargetDepartmentRequired Code = "RESUME_IMPORT_TARGET_DEPARTMENT_REQUIRED"
	ResumeImportTargetDepartmentInvalid  Code = "RESUME_IMPORT_TARGET_DEPARTMENT_INVALID"
	ResumeImportParseFailed              Code = "RESUME_IMPORT_PARSE_FAILED"
	ResumeImportEmptyFile                Code = "RESUME_IMPORT_EMPTY_FILE"
	ResumeDeleteDenied                   Code = "RESUME_DELETE_DENIED"
	JobNotFound                          Code = "JOB_NOT_FOUND"
	JobAccessDenied                      Code = "JOB_ACCESS_DENIED"
	DepartmentNotFound                   Code = "DEPARTMENT_NOT_FOUND"
	DepartmentNameRequired               Code = "DEPARTMENT_NAME_REQUIRED"
	DepartmentNameDuplicate              Code = "DEPARTMENT_NAME_DUPLICATE"
	DepartmentDeleteHasRelations         Code = "DEPARTMENT_DELETE_HAS_RELATIONS"
	DepartmentSystemProtected            Code = "DEPARTMENT_SYSTEM_PROTECTED"
	PositionNotFound                     Code = "POSITION_NOT_FOUND"
	PositionNameRequired                 Code = "POSITION_NAME_REQUIRED"
	PositionDepartmentRequired           Code = "POSITION_DEPARTMENT_REQUIRED"
	PositionDepartmentInvalid            Code = "POSITION_DEPARTMENT_INVALID"
	PositionInvalidChannel               Code = "POSITION_INVALID_CHANNEL"
	PositionInvalidStatus                Code = "POSITION_INVALID_STATUS"
	PositionDuplicateKeyword             Code = "POSITION_DUPLICATE_KEYWORD"
	PositionDuplicateImplicitTag         Code = "POSITION_DUPLICATE_IMPLICIT_TAG"
	PositionInvalidImplicitWeight        Code = "POSITION_INVALID_IMPLICIT_WEIGHT"
	PositionDeleteHasHistory             Code = "POSITION_DELETE_HAS_HISTORY"
	UserNotFound                         Code = "USER_NOT_FOUND"
	UserRoleBindingNotFound              Code = "USER_ROLE_BINDING_NOT_FOUND"
	UserRoleBindingDuplicate             Code = "USER_ROLE_BINDING_DUPLICATE"
	UserRoleBindingBatchEmpty            Code = "USER_ROLE_BINDING_BATCH_EMPTY"
	UserRoleBindingBatchTooLarge         Code = "USER_ROLE_BINDING_BATCH_TOO_LARGE"
	UserRoleBindingGuestProtected        Code = "USER_ROLE_BINDING_GUEST_PROTECTED"
	UserRoleBindingSelfLockout           Code = "USER_ROLE_BINDING_SELF_LOCKOUT"
	UserRoleBindingRoleDisabled          Code = "USER_ROLE_BINDING_ROLE_DISABLED"
	RecommendationRouteFailed            Code = "RECOMMENDATION_ROUTE_FAILED"
	RecommendationTargetPositionOffline  Code = "RECOMMENDATION_TARGET_POSITION_OFFLINE"
	RecommendationTargetPositionMismatch Code = "RECOMMENDATION_TARGET_POSITION_MISMATCH"
	RecommendationChannelMismatch        Code = "RECOMMENDATION_CHANNEL_MISMATCH"
	RecommendationSendFailed             Code = "RECOMMENDATION_SEND_FAILED"
	MatchingPositionOffline              Code = "MATCHING_POSITION_OFFLINE"
	MatchingParseFailed                  Code = "MATCHING_PARSE_FAILED"
	MatchingInterviewFailed              Code = "MATCHING_INTERVIEW_FAILED"
	ValidationFailed                     Code = "VALIDATION_FAILED"
	Internal                             Code = "INTERNAL_ERROR"
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
	case PermissionDenied, ResumeDeleteDenied, JobAccessDenied:
		return http.StatusForbidden
	case IAMPermissionNotFound, IAMPrincipalNotFound, ResumeNotFound, JobNotFound, DepartmentNotFound, PositionNotFound, UserNotFound, UserRoleBindingNotFound:
		return http.StatusNotFound
	case ValidationFailed, IAMInvalidResource, IAMInvalidAction, IAMInvalidAttributeCondition, IAMRoleRelationCycle, IAMRoleRelationDepthExceeded, IAMScopeUnsupported, ResumeImportFileTooLarge, ResumeImportUnsupportedType, ResumeImportTargetDepartmentRequired, ResumeImportTargetDepartmentInvalid, ResumeImportParseFailed, ResumeImportEmptyFile, DepartmentNameRequired, DepartmentNameDuplicate, DepartmentDeleteHasRelations, DepartmentSystemProtected, PositionNameRequired, PositionDepartmentRequired, PositionDepartmentInvalid, PositionInvalidChannel, PositionInvalidStatus, PositionDuplicateKeyword, PositionDuplicateImplicitTag, PositionInvalidImplicitWeight, PositionDeleteHasHistory, UserRoleBindingDuplicate, UserRoleBindingBatchEmpty, UserRoleBindingBatchTooLarge, UserRoleBindingGuestProtected, UserRoleBindingSelfLockout, UserRoleBindingRoleDisabled, RecommendationTargetPositionOffline, RecommendationTargetPositionMismatch, RecommendationChannelMismatch, MatchingPositionOffline, MatchingParseFailed, MatchingInterviewFailed:
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
	case IAMPermissionNotFound:
		return "权限不存在"
	case IAMInvalidResource:
		return "资源类型不合法"
	case IAMInvalidAction:
		return "操作类型不合法"
	case IAMInvalidAttributeCondition:
		return "权限属性条件不合法"
	case IAMRoleRelationCycle:
		return "角色包含关系不能形成循环"
	case IAMRoleRelationDepthExceeded:
		return "角色包含层级过深"
	case IAMPrincipalNotFound:
		return "权限主体不存在"
	case IAMScopeUnsupported:
		return "该角色不能绑定到系统部门"
	case ResumeNotFound:
		return "简历不存在"
	case ResumeImportFileTooLarge:
		return "简历文件不能超过 10MB"
	case ResumeImportUnsupportedType:
		return "仅支持 PDF 简历文件"
	case ResumeImportTargetDepartmentRequired:
		return "请选择导入目标部门"
	case ResumeImportTargetDepartmentInvalid:
		return "无权导入到该部门"
	case ResumeImportParseFailed:
		return "简历解析失败"
	case ResumeImportEmptyFile:
		return "简历文件为空"
	case ResumeDeleteDenied:
		return "无权删除该简历"
	case JobNotFound:
		return "任务不存在"
	case JobAccessDenied:
		return "无权查看该任务"
	case DepartmentNotFound:
		return "部门不存在"
	case DepartmentNameRequired:
		return "部门名称不能为空"
	case DepartmentNameDuplicate:
		return "部门名称已存在"
	case DepartmentDeleteHasRelations:
		return "部门仍有关联数据，不能删除"
	case DepartmentSystemProtected:
		return "系统部门不能修改或删除"
	case PositionNotFound:
		return "岗位不存在"
	case PositionNameRequired:
		return "岗位名称不能为空"
	case PositionDepartmentRequired:
		return "请选择岗位所属部门"
	case PositionDepartmentInvalid:
		return "岗位所属部门不存在或无权访问"
	case PositionInvalidChannel:
		return "岗位渠道不合法"
	case PositionInvalidStatus:
		return "岗位状态不合法"
	case PositionDuplicateKeyword:
		return "岗位关键词不能重复"
	case PositionDuplicateImplicitTag:
		return "隐性标签不能重复"
	case PositionInvalidImplicitWeight:
		return "隐性标签权重不合法"
	case PositionDeleteHasHistory:
		return "岗位已有解析或推荐历史，请使用下架"
	case UserNotFound:
		return "用户不存在"
	case UserRoleBindingNotFound:
		return "角色绑定不存在"
	case UserRoleBindingDuplicate:
		return "该用户已存在相同角色绑定"
	case UserRoleBindingBatchEmpty:
		return "请至少添加一条角色绑定"
	case UserRoleBindingBatchTooLarge:
		return "一次最多添加 20 条角色绑定"
	case UserRoleBindingGuestProtected:
		return "游客身份不可解除"
	case UserRoleBindingSelfLockout:
		return "不能解除自己的最后一个业务角色"
	case UserRoleBindingRoleDisabled:
		return "该角色已禁用，不能分配"
	case RecommendationRouteFailed:
		return "智能分流失败，请稍后重试"
	case RecommendationTargetPositionOffline:
		return "目标岗位已下架，不能推荐"
	case RecommendationTargetPositionMismatch:
		return "目标岗位与部门不匹配"
	case RecommendationChannelMismatch:
		return "目标岗位与简历渠道不一致"
	case RecommendationSendFailed:
		return "推荐失败，请稍后重试"
	case MatchingPositionOffline:
		return "岗位已下架，不能参与解析"
	case MatchingParseFailed:
		return "简历解析失败，请稍后重试"
	case MatchingInterviewFailed:
		return "面试题生成失败，请稍后重试"
	case ValidationFailed:
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
