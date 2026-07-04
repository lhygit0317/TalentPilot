package iam

import "errors"

var (
	ErrInvalidResource           = errors.New("iam invalid resource")
	ErrInvalidAction             = errors.New("iam invalid action")
	ErrInvalidAttributeCondition = errors.New("iam invalid attribute condition")
	ErrPermissionNotFound        = errors.New("iam permission not found")
	ErrRoleRelationCycle         = errors.New("iam role relation cycle")
	ErrRoleRelationDepthExceeded = errors.New("iam role relation depth exceeded")
	ErrScopeUnsupported          = errors.New("iam scope unsupported")
)

type Resource string
type Action string

type AttributeConditions struct {
	Channels []string
	Expired  []bool
	Self     bool
}

type PermissionGrant struct {
	RoleID              string
	Resource            Resource
	Action              Action
	AttributeConditions AttributeConditions
}

type PresetRole struct {
	ID          string
	Label       string
	Description string
	IsSystem    bool
	Enabled     bool
}

type Role = PresetRole

type RoleRelation struct {
	ID           string
	ParentRoleID string
	ChildRoleID  string
}

type User struct {
	ID         string
	EmployeeID string
	Name       string
}

type Department struct {
	ID   string
	Name string
}

type RoleBinding struct {
	ID           string
	UserID       string
	DepartmentID string
	RoleID       string
}

type Snapshot struct {
	User          User
	Departments   []Department
	RoleBindings  []RoleBinding
	Roles         []Role
	Permissions   []PermissionGrant
	RoleRelations []RoleRelation
}

type DepartmentScope struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DataScope struct {
	Departments    []DepartmentScope `json:"departments" nullable:"false"`
	AllDepartments bool              `json:"allDepartments"`
	Channels       []string          `json:"channels" nullable:"false"`
}

type ScopedPermission struct {
	PermissionGrant
	BindingID      string
	DepartmentID   string
	DepartmentName string
	AllDepartments bool
	SelfUserID     string
}

type Principal struct {
	User              User
	Bindings          []RoleBinding
	ExpandedRoleIDs   []string
	Permissions       []PermissionGrant
	ScopedPermissions []ScopedPermission
	DataScope         DataScope
	PageAccess        []string
	DefaultRoute      string
}

type ScopeBranch struct {
	BindingID      string
	DepartmentIDs  []string
	AllDepartments bool
	Channels       []string
	Expired        []bool
	SelfUserID     string
}

type ScopePredicate struct {
	Resource       Resource
	Action         Action
	DepartmentIDs  []string
	AllDepartments bool
	Channels       []string
	Expired        []bool
	SelfUserID     string
	Branches       []ScopeBranch
}

type Target struct {
	ID           string
	UserID       string
	DepartmentID string
	Channel      string
	Expired      *bool
}

type Decision struct {
	Allowed           bool
	Code              string
	MatchedBindingIDs []string
}

const (
	ResourceUser               Resource = "User"
	ResourceDepartment         Resource = "Department"
	ResourcePosition           Resource = "Position"
	ResourceResume             Resource = "Resume"
	ResourceRole               Resource = "Role"
	ResourcePermission         Resource = "Permission"
	ResourceUserDepartmentRole Resource = "UserDepartmentRole"
	ResourceDepartmentPosition Resource = "DepartmentPosition"
	ResourceDepartmentResume   Resource = "DepartmentResume"
	ResourcePositionResume     Resource = "PositionResume"
	ResourceRoleRelation       Resource = "RoleRelation"
	ResourceNotification       Resource = "Notification"
	ResourceAuditLog           Resource = "AuditLog"
	ResourceJob                Resource = "Job"

	ActionList   Action = "List"
	ActionGet    Action = "Get"
	ActionCreate Action = "Create"
	ActionUpdate Action = "Update"
	ActionDelete Action = "Delete"

	RoleGuest       = "__role_guest__"
	RoleHRBP        = "__role_hrbp__"
	RoleHRD         = "__role_hrd__"
	RoleManager     = "__role_manager__"
	RoleTrainee     = "__role_trainee__"
	RoleSocialOwner = "__role_social_owner__"
	RoleCampusOwner = "__role_campus_owner__"
	RoleSuperAdmin  = "__role_super_admin__"

	SystemDepartmentID = "__system__"
)

func PermissionKey(resource Resource, action Action) string {
	return string(resource) + "." + string(action)
}
