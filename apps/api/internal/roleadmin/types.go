package roleadmin

import (
	"context"
	"errors"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

var (
	ErrRoleNotFound        = errors.New("role not found")
	ErrLabelInvalid        = errors.New("role label invalid")
	ErrLabelDuplicate      = errors.New("role label duplicate")
	ErrSystemRoleProtected = errors.New("system role protected")
	ErrRoleInUse           = errors.New("role in use")
	ErrPermissionInvalid   = errors.New("role permission invalid")
	ErrPermissionDuplicate = errors.New("role permission duplicate")
	ErrRelationInvalid     = errors.New("role relation invalid")
)

type Store interface {
	ListRoles(context.Context, RoleListQuery) (RoleListResult, error)
	GetRole(context.Context, string, RoleCapabilityQuery) (RoleDetail, error)
	PermissionOptions(context.Context) (PermissionOptionsResult, error)
	GetRoleRecord(context.Context, string) (RoleRecord, error)
	RoleLabelExists(context.Context, string, string) (bool, error)
	ChildRolesExist(context.Context, []string) (bool, error)
	LoadRoleRelations(context.Context) ([]iam.RoleRelation, error)
	CreateRole(context.Context, RoleDefinitionRecord) (string, error)
	UpdateRole(context.Context, string, RoleDefinitionRecord) error
	ReplaceRolePermissions(context.Context, string, []PermissionInput) error
	ReplaceRoleChildren(context.Context, string, []string) error
	ToggleRoleEnabled(context.Context, string, bool) error
	DeleteRole(context.Context, string) error
	WithTransaction(context.Context, func(Store) error) error
}

type IAMInvalidator interface {
	InvalidateRoleClosure(context.Context, []string) error
}

type RoleListQuery struct {
	ActorCanCreate bool
	ActorCanEdit   bool
	ActorCanDelete bool
	ActorCanToggle bool
	Search         string
	System         *bool
	Enabled        *bool
	Limit          int
}

type RoleCapabilityQuery struct {
	ActorCanEdit   bool
	ActorCanDelete bool
	ActorCanToggle bool
}

type RoleListItem struct {
	ID               string `json:"id"`
	Label            string `json:"label"`
	Description      string `json:"description"`
	IsSystem         bool   `json:"isSystem"`
	Enabled          bool   `json:"enabled"`
	PermissionCount  int    `json:"permissionCount"`
	ChildRoleCount   int    `json:"childRoleCount"`
	ReferenceCount   int    `json:"referenceCount"`
	ConditionSummary string `json:"conditionSummary"`
	CanEdit          bool   `json:"canEdit"`
	CanDelete        bool   `json:"canDelete"`
	CanToggleEnabled bool   `json:"canToggleEnabled"`
}

type RoleListResult struct {
	Items     []RoleListItem `json:"items" nullable:"false"`
	Total     int            `json:"total"`
	CanCreate bool           `json:"canCreate"`
}

type PermissionInput struct {
	Resource            iam.Resource            `json:"resource"`
	Action              iam.Action              `json:"action"`
	AttributeConditions iam.AttributeConditions `json:"attributeConditions,omitempty"`
}

type RoleDefinitionInput struct {
	ActorUserID  string
	Label        string            `json:"label"`
	Description  string            `json:"description"`
	Enabled      bool              `json:"enabled"`
	Permissions  []PermissionInput `json:"permissions" nullable:"false"`
	ChildRoleIDs []string          `json:"childRoleIds" nullable:"false"`
}

type ToggleEnabledInput struct {
	ActorUserID string
	Enabled     bool `json:"enabled"`
}

type RoleRecord struct {
	ID             string
	Label          string
	Description    string
	IsSystem       bool
	Enabled        bool
	ReferenceCount int
}

type RoleDefinitionRecord struct {
	Label       string
	Description string
	Enabled     bool
	ActorUserID string
}

type RoleDetail struct {
	ID               string            `json:"id"`
	Label            string            `json:"label"`
	Description      string            `json:"description"`
	IsSystem         bool              `json:"isSystem"`
	Enabled          bool              `json:"enabled"`
	ReferenceCount   int               `json:"referenceCount"`
	Permissions      []PermissionInput `json:"permissions" nullable:"false"`
	ChildRoleIDs     []string          `json:"childRoleIds" nullable:"false"`
	CanEdit          bool              `json:"canEdit"`
	CanDelete        bool              `json:"canDelete"`
	CanToggleEnabled bool              `json:"canToggleEnabled"`
}

type ConditionSupport struct {
	Channels bool `json:"chan"`
	Expired  bool `json:"expired"`
	Self     bool `json:"self"`
}

type PermissionActionOption struct {
	Action             iam.Action       `json:"action"`
	SupportsConditions ConditionSupport `json:"supportsConditions"`
}

type PermissionResourceOption struct {
	Resource iam.Resource             `json:"resource"`
	Actions  []PermissionActionOption `json:"actions" nullable:"false"`
}

type ConditionOptions struct {
	Channels []string `json:"chan" nullable:"false"`
	Expired  []bool   `json:"expired" nullable:"false"`
}

type PermissionOptionsResult struct {
	Resources        []PermissionResourceOption `json:"resources" nullable:"false"`
	ConditionOptions ConditionOptions           `json:"conditionOptions"`
}
