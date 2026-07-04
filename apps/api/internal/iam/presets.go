package iam

func PresetRoles() []PresetRole {
	return []PresetRole{
		{ID: RoleGuest, Label: "游客", IsSystem: true, Enabled: true},
		{ID: RoleHRBP, Label: "HRBP", IsSystem: true, Enabled: true},
		{ID: RoleHRD, Label: "HRD", IsSystem: true, Enabled: true},
		{ID: RoleManager, Label: "主管", IsSystem: true, Enabled: true},
		{ID: RoleTrainee, Label: "锻炼干部", IsSystem: true, Enabled: true},
		{ID: RoleSocialOwner, Label: "社招负责人", IsSystem: true, Enabled: true},
		{ID: RoleCampusOwner, Label: "校招负责人", IsSystem: true, Enabled: true},
		{ID: RoleSuperAdmin, Label: "超级管理员", IsSystem: true, Enabled: true},
	}
}

func PresetRolePermissions() map[string][]PermissionGrant {
	matrix := map[string][]PermissionGrant{
		RoleGuest: {
			grant(RoleGuest, ResourceDepartment, ActionList),
			grantWithConditions(RoleGuest, ResourceUser, ActionGet, AttributeConditions{Self: true}),
		},
		RoleHRBP:        businessOwnerGrants(RoleHRBP, AttributeConditions{}),
		RoleManager:     managerGrants(),
		RoleTrainee:     traineeGrants(),
		RoleHRD:         hrdGrants(),
		RoleSocialOwner: businessOwnerGrants(RoleSocialOwner, AttributeConditions{Channels: []string{"social"}}),
		RoleCampusOwner: businessOwnerGrants(RoleCampusOwner, AttributeConditions{Channels: []string{"campus"}}),
	}
	matrix[RoleSuperAdmin] = superAdminGrants()
	return matrix
}

func PresetRoleRelations() []RoleRelation {
	return []RoleRelation{
		{ID: "__role_relation_hrd_hrbp__", ParentRoleID: RoleHRD, ChildRoleID: RoleHRBP},
		{ID: "__role_relation_hrd_manager__", ParentRoleID: RoleHRD, ChildRoleID: RoleManager},
		{ID: "__role_relation_hrd_trainee__", ParentRoleID: RoleHRD, ChildRoleID: RoleTrainee},
		{ID: "__role_relation_super_admin_hrd__", ParentRoleID: RoleSuperAdmin, ChildRoleID: RoleHRD},
		{ID: "__role_relation_super_admin_social_owner__", ParentRoleID: RoleSuperAdmin, ChildRoleID: RoleSocialOwner},
		{ID: "__role_relation_super_admin_campus_owner__", ParentRoleID: RoleSuperAdmin, ChildRoleID: RoleCampusOwner},
	}
}

func RoleSupportsGlobalScope(roleID string) bool {
	switch roleID {
	case RoleHRD, RoleSocialOwner, RoleCampusOwner, RoleSuperAdmin:
		return true
	default:
		return false
	}
}

func businessOwnerGrants(roleID string, resumeConditions AttributeConditions) []PermissionGrant {
	return []PermissionGrant{
		grant(roleID, ResourceUser, ActionList),
		grant(roleID, ResourceUser, ActionGet),
		grant(roleID, ResourceDepartment, ActionList),
		grant(roleID, ResourceDepartment, ActionGet),
		grant(roleID, ResourceUserDepartmentRole, ActionList),
		grant(roleID, ResourceDepartmentPosition, ActionList),
		resumeGrant(roleID, ActionList, resumeConditions),
		resumeGrant(roleID, ActionGet, resumeConditions),
		resumeGrant(roleID, ActionCreate, resumeConditions),
		resumeGrant(roleID, ActionUpdate, resumeConditions),
		resumeGrant(roleID, ActionDelete, resumeConditions),
		grant(roleID, ResourcePosition, ActionList),
		grant(roleID, ResourcePosition, ActionGet),
		grant(roleID, ResourceDepartmentResume, ActionCreate),
		grant(roleID, ResourceDepartmentResume, ActionDelete),
		grant(roleID, ResourcePositionResume, ActionCreate),
		grant(roleID, ResourceNotification, ActionList),
		grant(roleID, ResourceNotification, ActionGet),
		grant(roleID, ResourceNotification, ActionCreate),
		grant(roleID, ResourceNotification, ActionUpdate),
	}
}

func managerGrants() []PermissionGrant {
	return []PermissionGrant{
		grant(RoleManager, ResourceUser, ActionList),
		grant(RoleManager, ResourceUser, ActionGet),
		grant(RoleManager, ResourceDepartment, ActionList),
		grant(RoleManager, ResourceDepartment, ActionGet),
		grant(RoleManager, ResourceUserDepartmentRole, ActionList),
		grant(RoleManager, ResourceDepartmentPosition, ActionList),
		grant(RoleManager, ResourceResume, ActionList),
		grant(RoleManager, ResourceResume, ActionGet),
		grant(RoleManager, ResourceResume, ActionCreate),
		grant(RoleManager, ResourcePosition, ActionList),
		grant(RoleManager, ResourcePosition, ActionGet),
		grant(RoleManager, ResourceDepartmentResume, ActionCreate),
		grant(RoleManager, ResourcePositionResume, ActionCreate),
		grant(RoleManager, ResourceNotification, ActionCreate),
	}
}

func traineeGrants() []PermissionGrant {
	return []PermissionGrant{
		grant(RoleTrainee, ResourceUser, ActionList),
		grant(RoleTrainee, ResourceUser, ActionGet),
		grant(RoleTrainee, ResourceDepartment, ActionList),
		grant(RoleTrainee, ResourceDepartment, ActionGet),
		grant(RoleTrainee, ResourceUserDepartmentRole, ActionList),
		grant(RoleTrainee, ResourceDepartmentPosition, ActionList),
		grant(RoleTrainee, ResourceResume, ActionList),
		grant(RoleTrainee, ResourceResume, ActionGet),
		grant(RoleTrainee, ResourcePosition, ActionList),
		grant(RoleTrainee, ResourcePosition, ActionGet),
		grant(RoleTrainee, ResourcePositionResume, ActionCreate),
	}
}

func hrdGrants() []PermissionGrant {
	return []PermissionGrant{
		grant(RoleHRD, ResourceUserDepartmentRole, ActionCreate),
		grant(RoleHRD, ResourceUserDepartmentRole, ActionDelete),
		grant(RoleHRD, ResourceDepartmentResume, ActionCreate),
	}
}

func superAdminGrants() []PermissionGrant {
	whitelist := PermissionWhitelist()
	grants := make([]PermissionGrant, 0, 64)
	resourceOrder := []Resource{
		ResourceUser,
		ResourceDepartment,
		ResourcePosition,
		ResourceResume,
		ResourceRole,
		ResourcePermission,
		ResourceUserDepartmentRole,
		ResourceDepartmentPosition,
		ResourceDepartmentResume,
		ResourcePositionResume,
		ResourceRoleRelation,
		ResourceNotification,
		ResourceAuditLog,
		ResourceJob,
	}
	actionOrder := []Action{ActionList, ActionGet, ActionCreate, ActionUpdate, ActionDelete}
	for _, resource := range resourceOrder {
		for _, action := range actionOrder {
			if _, ok := whitelist[resource][action]; ok {
				grants = append(grants, grant(RoleSuperAdmin, resource, action))
			}
		}
	}
	return grants
}

func resumeGrant(roleID string, action Action, conditions AttributeConditions) PermissionGrant {
	return grantWithConditions(roleID, ResourceResume, action, conditions)
}

func grant(roleID string, resource Resource, action Action) PermissionGrant {
	return grantWithConditions(roleID, resource, action, AttributeConditions{})
}

func grantWithConditions(roleID string, resource Resource, action Action, conditions AttributeConditions) PermissionGrant {
	return PermissionGrant{
		RoleID:              roleID,
		Resource:            resource,
		Action:              action,
		AttributeConditions: conditions,
	}
}
