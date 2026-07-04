package iam

type ConditionAllowance struct {
	Channels bool
	Expired  bool
	Self     bool
}

func PermissionWhitelist() map[Resource]map[Action]ConditionAllowance {
	return map[Resource]map[Action]ConditionAllowance{
		ResourceUser: {
			ActionList: {},
			ActionGet:  {Self: true},
		},
		ResourceDepartment: {
			ActionList:   {},
			ActionGet:    {},
			ActionCreate: {},
			ActionUpdate: {},
			ActionDelete: {},
		},
		ResourcePosition: {
			ActionList:   {},
			ActionGet:    {},
			ActionCreate: {},
			ActionUpdate: {},
			ActionDelete: {},
		},
		ResourceResume: {
			ActionList:   {Channels: true, Expired: true},
			ActionGet:    {Channels: true, Expired: true},
			ActionCreate: {Channels: true},
			ActionUpdate: {Channels: true, Expired: true},
			ActionDelete: {Channels: true, Expired: true},
		},
		ResourceDepartmentPosition: {
			ActionList:   {},
			ActionCreate: {},
			ActionDelete: {},
		},
		ResourceDepartmentResume: {
			ActionList:   {},
			ActionCreate: {},
			ActionDelete: {},
		},
		ResourcePositionResume: {
			ActionList:   {},
			ActionGet:    {},
			ActionCreate: {},
			ActionUpdate: {},
		},
		ResourceNotification: {
			ActionList:   {},
			ActionGet:    {},
			ActionCreate: {},
			ActionUpdate: {},
		},
		ResourceRole: {
			ActionList:   {},
			ActionGet:    {},
			ActionCreate: {},
			ActionUpdate: {},
			ActionDelete: {},
		},
		ResourcePermission: {
			ActionList:   {},
			ActionCreate: {},
			ActionDelete: {},
		},
		ResourceRoleRelation: {
			ActionList:   {},
			ActionCreate: {},
			ActionDelete: {},
		},
		ResourceUserDepartmentRole: {
			ActionList:   {},
			ActionCreate: {},
			ActionDelete: {},
		},
		ResourceAuditLog: {
			ActionList: {},
		},
		ResourceJob: {
			ActionList: {},
			ActionGet:  {},
		},
	}
}

func ValidatePermissionGrant(grant PermissionGrant) error {
	actions, ok := PermissionWhitelist()[grant.Resource]
	if !ok {
		return ErrInvalidResource
	}
	allowance, ok := actions[grant.Action]
	if !ok {
		return ErrInvalidAction
	}
	if err := validateAttributeConditions(grant.AttributeConditions, allowance); err != nil {
		return err
	}
	return nil
}

func validateAttributeConditions(conditions AttributeConditions, allowance ConditionAllowance) error {
	if len(conditions.Channels) > 0 {
		if !allowance.Channels {
			return ErrInvalidAttributeCondition
		}
		for _, channel := range conditions.Channels {
			if channel != "social" && channel != "campus" {
				return ErrInvalidAttributeCondition
			}
		}
	}
	if len(conditions.Expired) > 0 && !allowance.Expired {
		return ErrInvalidAttributeCondition
	}
	if conditions.Self && !allowance.Self {
		return ErrInvalidAttributeCondition
	}
	return nil
}
