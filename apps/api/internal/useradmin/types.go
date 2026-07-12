package useradmin

import (
	"context"
	"errors"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrBindingNotFound       = errors.New("user role binding not found")
	ErrDuplicateBinding      = errors.New("user role binding duplicate")
	ErrBatchEmpty            = errors.New("user role binding batch empty")
	ErrBatchTooLarge         = errors.New("user role binding batch too large")
	ErrGuestBindingProtected = errors.New("guest binding protected")
	ErrSelfLockout           = errors.New("user role binding self lockout")
	ErrRoleDisabled          = errors.New("user role binding role disabled")
	ErrRoleNotFound          = errors.New("role not found")
	ErrDepartmentNotFound    = errors.New("department not found")
	ErrPermissionDenied      = errors.New("user role binding permission denied")
)

type Store interface {
	ListUsers(context.Context, ListUsersQuery) (UserListResult, error)
	GetUser(context.Context, string, iam.ScopePredicate) (UserDetail, error)
	GetUserIdentity(context.Context, string) (UserIdentity, error)
	ListAssignableRoles(context.Context) ([]AssignableRole, error)
	CreateRoleBindings(context.Context, CreateRoleBindingsCommand) ([]RoleBindingDetail, error)
	GetRoleBinding(context.Context, string) (RoleBindingDetail, error)
	DeleteRoleBinding(context.Context, string) (RoleBindingDetail, error)
	CountNonGuestBindings(context.Context, string) (int, error)
	EnsureGuestBinding(context.Context, string, string) (RoleBindingDetail, bool, error)
	WithTransaction(context.Context, func(Store) error) error
}

type IAMCache interface {
	InvalidateUser(string)
}

type UserAdminDepartmentSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	System bool   `json:"system"`
}

type DepartmentSummary = UserAdminDepartmentSummary

type UserAdminRoleSummary struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	IsSystem bool   `json:"isSystem"`
	Enabled  bool   `json:"enabled"`
}

type RoleSummary = UserAdminRoleSummary

type UserAdminRoleBindingDetail struct {
	ID         string                     `json:"id"`
	UserID     string                     `json:"-"`
	Role       UserAdminRoleSummary       `json:"role"`
	Department UserAdminDepartmentSummary `json:"department"`
	Guest      bool                       `json:"guest"`
	CreatedAt  time.Time                  `json:"createdAt"`
	CreatedBy  string                     `json:"createdBy"`
	CanDelete  bool                       `json:"canDelete"`
}

type RoleBindingDetail = UserAdminRoleBindingDetail

type UserAdminIdentity struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
}

type UserIdentity = UserAdminIdentity

type UserAdminUserSummary struct {
	ID           string                       `json:"id"`
	EmployeeID   string                       `json:"employeeId"`
	Name         string                       `json:"name"`
	Departments  []UserAdminDepartmentSummary `json:"departments" nullable:"false"`
	RoleBindings []UserAdminRoleBindingDetail `json:"roleBindings" nullable:"false"`
	RoleSummary  string                       `json:"roleSummary"`
	GuestOnly    bool                         `json:"guestOnly"`
	CanAssign    bool                         `json:"canAssign"`
}

type UserSummary = UserAdminUserSummary
type UserDetail = UserAdminUserSummary

type UserListResult struct {
	Items            []UserAdminUserSummary `json:"items" nullable:"false"`
	NextCursor       string                 `json:"nextCursor"`
	DataScopeSummary string                 `json:"dataScopeSummary"`
	CanAssignRoles   bool                   `json:"canAssignRoles"`
}

type AssignableRole struct {
	ID                        string `json:"id"`
	Label                     string `json:"label"`
	Description               string `json:"description"`
	IsSystem                  bool   `json:"isSystem"`
	SupportsSystemDepartment  bool   `json:"supportsSystemDepartment"`
	AttributeConditionSummary string `json:"attributeConditionSummary"`
}

type AssignableRoleListResult struct {
	Items []AssignableRole `json:"items" nullable:"false"`
}

type ListUsersQuery struct {
	ActorUserID string
	Search      string
	Limit       int
	Cursor      string
	ListScope   iam.ScopePredicate
	DeleteScope iam.ScopePredicate
	CanAssign   bool
}

type RoleBindingRequest struct {
	DepartmentID string `json:"departmentId" required:"true"`
	RoleID       string `json:"roleId" required:"true"`
}

type CreateRoleBindingsInput struct {
	ActorUserID string
	UserID      string
	CreateScope iam.ScopePredicate
	Bindings    []RoleBindingRequest
}

type CreateRoleBindingsCommand struct {
	ActorUserID string
	UserID      string
	CreateScope iam.ScopePredicate
	Bindings    []RoleBindingRequest
}

type CreateRoleBindingsResult struct {
	User    UserIdentity        `json:"user"`
	Created []RoleBindingDetail `json:"created" nullable:"false"`
	Message string              `json:"message"`
}

type DeleteRoleBindingInput struct {
	ActorUserID string
	UserID      string
	BindingID   string
	DeleteScope iam.ScopePredicate
}

type DeleteRoleBindingResult struct {
	DeletedBindingID string `json:"deletedBindingId"`
	UserID           string `json:"userId"`
	Message          string `json:"message"`
}
