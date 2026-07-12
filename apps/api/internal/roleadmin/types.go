package roleadmin

import (
	"errors"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

var ErrRoleNotFound = errors.New("role not found")

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
