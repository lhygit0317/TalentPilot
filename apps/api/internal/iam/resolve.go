package iam

import (
	"sort"
	"strconv"
	"strings"
)

const maxRoleRelationDepth = 16

func ResolvePrincipalFromSnapshot(snapshot Snapshot) (Principal, error) {
	roleByID := map[string]Role{}
	for _, role := range snapshot.Roles {
		roleByID[role.ID] = role
	}
	departmentByID := map[string]Department{}
	for _, department := range snapshot.Departments {
		departmentByID[department.ID] = department
	}
	childrenByParent := map[string][]string{}
	for _, relation := range snapshot.RoleRelations {
		if relation.ParentRoleID == relation.ChildRoleID {
			return Principal{}, ErrRoleRelationCycle
		}
		childrenByParent[relation.ParentRoleID] = append(childrenByParent[relation.ParentRoleID], relation.ChildRoleID)
	}
	grantsByRole := map[string][]PermissionGrant{}
	for _, grant := range snapshot.Permissions {
		grantsByRole[grant.RoleID] = append(grantsByRole[grant.RoleID], grant)
	}

	principal := Principal{User: snapshot.User, Bindings: snapshot.RoleBindings}
	expandedSet := map[string]bool{}
	permissionSet := map[string]bool{}
	departmentSet := map[string]DepartmentScope{}
	channelSet := map[string]bool{}
	hasUnrestrictedResumeChannel := false
	hasGuestBinding := false

	for _, binding := range snapshot.RoleBindings {
		if binding.RoleID == RoleGuest {
			hasGuestBinding = true
		}
		if binding.DepartmentID == SystemDepartmentID && binding.RoleID != RoleGuest && !RoleSupportsGlobalScope(binding.RoleID) {
			return Principal{}, ErrScopeUnsupported
		}

		roleIDs, err := expandRoleIDs(binding.RoleID, roleByID, childrenByParent)
		if err != nil {
			return Principal{}, err
		}
		allDepartments := binding.DepartmentID == SystemDepartmentID && binding.RoleID != RoleGuest && RoleSupportsGlobalScope(binding.RoleID)
		department := departmentByID[binding.DepartmentID]
		if binding.DepartmentID != "" && binding.DepartmentID != SystemDepartmentID {
			departmentSet[binding.DepartmentID] = DepartmentScope{ID: binding.DepartmentID, Name: department.Name}
		}
		if allDepartments {
			principal.DataScope.AllDepartments = true
		}

		for _, roleID := range roleIDs {
			expandedSet[roleID] = true
			for _, grant := range grantsByRole[roleID] {
				if err := ValidatePermissionGrant(grant); err != nil {
					return Principal{}, err
				}
				permissionKey := grantIdentityKey(grant)
				if !permissionSet[permissionKey] {
					principal.Permissions = append(principal.Permissions, grant)
					permissionSet[permissionKey] = true
				}
				scoped := ScopedPermission{
					PermissionGrant: grant,
					BindingID:       binding.ID,
					DepartmentID:    binding.DepartmentID,
					DepartmentName:  department.Name,
					AllDepartments:  allDepartments,
				}
				if grant.AttributeConditions.Self {
					scoped.SelfUserID = snapshot.User.ID
				}
				principal.ScopedPermissions = append(principal.ScopedPermissions, scoped)

				if grant.Resource == ResourceResume {
					if len(grant.AttributeConditions.Channels) == 0 {
						hasUnrestrictedResumeChannel = true
					}
					for _, channel := range grant.AttributeConditions.Channels {
						channelSet[channel] = true
					}
				}
			}
		}
	}

	for roleID := range expandedSet {
		principal.ExpandedRoleIDs = append(principal.ExpandedRoleIDs, roleID)
	}
	sort.Strings(principal.ExpandedRoleIDs)
	sort.Slice(principal.Permissions, func(i, j int) bool {
		return grantIdentityKey(principal.Permissions[i]) < grantIdentityKey(principal.Permissions[j])
	})
	for _, departmentID := range sortedKeys(departmentSet) {
		principal.DataScope.Departments = append(principal.DataScope.Departments, departmentSet[departmentID])
	}
	if hasUnrestrictedResumeChannel {
		principal.DataScope.Channels = []string{"social", "campus"}
	} else {
		principal.DataScope.Channels = sortedStringSet(channelSet)
	}
	principal.PageAccess = pageAccessFromPermissions(principal.Permissions, hasGuestBinding)
	principal.DefaultRoute = defaultRoute(principal.PageAccess)
	return principal, nil
}

func Can(principal Principal, resource Resource, action Action, target Target) Decision {
	scope, err := Scope(principal, resource, action)
	if err != nil || len(scope.Branches) == 0 {
		return Decision{Allowed: false, Code: "IAM_PERMISSION_DENIED"}
	}
	matched := make([]string, 0, len(scope.Branches))
	for _, branch := range scope.Branches {
		matched = append(matched, branch.BindingID)
	}
	return Decision{Allowed: true, MatchedBindingIDs: uniqueStrings(matched)}
}

func Scope(principal Principal, resource Resource, action Action) (ScopePredicate, error) {
	predicate := ScopePredicate{Resource: resource, Action: action}
	branchSet := map[string]bool{}
	departmentSet := map[string]bool{}
	channelSet := map[string]bool{}
	expiredSet := map[bool]bool{}

	for _, scoped := range principal.ScopedPermissions {
		if scoped.Resource != resource || scoped.Action != action {
			continue
		}
		branch := ScopeBranch{
			BindingID:      scoped.BindingID,
			AllDepartments: scoped.AllDepartments,
			Channels:       append([]string(nil), scoped.AttributeConditions.Channels...),
			Expired:        append([]bool(nil), scoped.AttributeConditions.Expired...),
			SelfUserID:     scoped.SelfUserID,
		}
		if scoped.DepartmentID != "" && scoped.DepartmentID != SystemDepartmentID && !scoped.AllDepartments {
			branch.DepartmentIDs = []string{scoped.DepartmentID}
		}
		key := scopeBranchKey(branch)
		if branchSet[key] {
			continue
		}
		branchSet[key] = true
		predicate.Branches = append(predicate.Branches, branch)

		if branch.AllDepartments {
			predicate.AllDepartments = true
		}
		for _, departmentID := range branch.DepartmentIDs {
			departmentSet[departmentID] = true
		}
		for _, channel := range branch.Channels {
			channelSet[channel] = true
		}
		for _, expired := range branch.Expired {
			expiredSet[expired] = true
		}
		if branch.SelfUserID != "" {
			predicate.SelfUserID = branch.SelfUserID
		}
	}
	if len(predicate.Branches) == 0 {
		return ScopePredicate{}, ErrPermissionNotFound
	}
	sort.Slice(predicate.Branches, func(i, j int) bool {
		return scopeBranchKey(predicate.Branches[i]) < scopeBranchKey(predicate.Branches[j])
	})
	predicate.DepartmentIDs = sortedBoolKeys(departmentSet)
	predicate.Channels = sortedBoolKeys(channelSet)
	predicate.Expired = sortedBoolValues(expiredSet)
	return predicate, nil
}

func expandRoleIDs(rootRoleID string, roleByID map[string]Role, childrenByParent map[string][]string) ([]string, error) {
	expanded := map[string]bool{}
	var walk func(roleID string, depth int, direct bool, stack map[string]bool) error
	walk = func(roleID string, depth int, direct bool, stack map[string]bool) error {
		if depth > maxRoleRelationDepth {
			return ErrRoleRelationDepthExceeded
		}
		if stack[roleID] {
			return ErrRoleRelationCycle
		}
		role, ok := roleByID[roleID]
		if !ok {
			return nil
		}
		if !direct && !role.Enabled {
			return nil
		}
		expanded[roleID] = true
		nextStack := cloneStack(stack)
		nextStack[roleID] = true
		for _, childRoleID := range childrenByParent[roleID] {
			if err := walk(childRoleID, depth+1, false, nextStack); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(rootRoleID, 0, true, map[string]bool{}); err != nil {
		return nil, err
	}
	return sortedBoolKeys(expanded), nil
}

func pageAccessFromPermissions(grants []PermissionGrant, hasGuestBinding bool) []string {
	if hasGuestBinding {
		return []string{"resume-parse", "resume-recommend"}
	}
	permissionSet := map[string]bool{}
	for _, grant := range grants {
		permissionSet[PermissionKey(grant.Resource, grant.Action)] = true
	}
	pages := []struct {
		key      string
		required []string
	}{
		{"resume-parse", []string{"Resume.List", "Resume.Get", "Position.List", "PositionResume.Create"}},
		{"resume-recommend", []string{"Resume.List", "Resume.Get", "Resume.Create", "Notification.Create", "DepartmentResume.Create", "PositionResume.Create"}},
		{"resume-library", []string{"Resume.List"}},
		{"departments-positions", []string{"Department.List", "Position.List"}},
		{"users", []string{"User.List", "UserDepartmentRole.List"}},
		{"roles", []string{"Role.List", "Permission.List", "RoleRelation.List"}},
		{"notifications", []string{"Notification.List"}},
		{"audit-logs", []string{"AuditLog.List"}},
	}
	var access []string
	for _, page := range pages {
		if hasAllPermissions(permissionSet, page.required) {
			access = append(access, page.key)
		}
	}
	return access
}

func hasAllPermissions(permissionSet map[string]bool, required []string) bool {
	for _, permission := range required {
		if !permissionSet[permission] {
			return false
		}
	}
	return true
}

func defaultRoute(pageAccess []string) string {
	if len(pageAccess) == 0 {
		return "/resume-parse"
	}
	for _, page := range pageAccess {
		if page == "resume-parse" {
			return "/resume-parse"
		}
	}
	return "/" + pageAccess[0]
}

func cloneStack(stack map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(stack)+1)
	for key, value := range stack {
		cloned[key] = value
	}
	return cloned
}

func grantIdentityKey(grant PermissionGrant) string {
	return PermissionKey(grant.Resource, grant.Action) + "|" + conditionKey(grant.AttributeConditions)
}

func conditionKey(conditions AttributeConditions) string {
	parts := []string{
		"chan=" + strings.Join(conditions.Channels, ","),
		"expired=" + boolSliceKey(conditions.Expired),
		"self=" + strconv.FormatBool(conditions.Self),
	}
	return strings.Join(parts, ";")
}

func scopeBranchKey(branch ScopeBranch) string {
	return strings.Join([]string{
		branch.BindingID,
		strconv.FormatBool(branch.AllDepartments),
		strings.Join(branch.DepartmentIDs, ","),
		strings.Join(branch.Channels, ","),
		boolSliceKey(branch.Expired),
		branch.SelfUserID,
	}, "|")
}

func boolSliceKey(values []bool) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatBool(value))
	}
	return strings.Join(parts, ",")
}

func sortedKeys(values map[string]DepartmentScope) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringSet(values map[string]bool) []string {
	return sortedBoolKeys(values)
}

func sortedBoolKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedBoolValues(values map[bool]bool) []bool {
	result := make([]bool, 0, len(values))
	if values[false] {
		result = append(result, false)
	}
	if values[true] {
		result = append(result, true)
	}
	return result
}

func uniqueStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	return sortedBoolKeys(set)
}
