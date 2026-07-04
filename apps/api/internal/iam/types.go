package iam

import "errors"

var (
	ErrInvalidResource           = errors.New("iam invalid resource")
	ErrInvalidAction             = errors.New("iam invalid action")
	ErrInvalidAttributeCondition = errors.New("iam invalid attribute condition")
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

type RoleRelation struct {
	ID           string
	ParentRoleID string
	ChildRoleID  string
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
