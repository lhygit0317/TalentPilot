package iam

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type SQLStore struct {
	db *gorm.DB
}

func NewSQLStore(db *gorm.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) WithTransaction(ctx context.Context, fn func(Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&SQLStore{db: tx})
	})
}

func (s *SQLStore) LoadSnapshot(ctx context.Context, userID string) (Snapshot, error) {
	user, err := s.loadUser(ctx, userID)
	if err != nil {
		return Snapshot{}, err
	}
	bindings, err := s.loadRoleBindings(ctx, userID)
	if err != nil {
		return Snapshot{}, err
	}
	departments, err := s.loadDepartments(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	roles, err := s.loadRoles(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	permissions, err := s.loadPermissions(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	relations, err := s.LoadRoleRelations(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		User:          user,
		Departments:   departments,
		RoleBindings:  bindings,
		Roles:         roles,
		Permissions:   permissions,
		RoleRelations: relations,
	}, nil
}

func (s *SQLStore) UsersForRoleClosure(ctx context.Context, roleIDs []string) ([]string, error) {
	relations, err := s.LoadRoleRelations(ctx)
	if err != nil {
		return nil, err
	}
	parentsByChild := map[string][]string{}
	for _, relation := range relations {
		parentsByChild[relation.ChildRoleID] = append(parentsByChild[relation.ChildRoleID], relation.ParentRoleID)
	}
	closure := map[string]bool{}
	var walk func(roleID string, depth int, stack map[string]bool) error
	walk = func(roleID string, depth int, stack map[string]bool) error {
		if depth > maxRoleRelationDepth {
			return ErrRoleRelationDepthExceeded
		}
		if stack[roleID] {
			return ErrRoleRelationCycle
		}
		closure[roleID] = true
		nextStack := cloneStack(stack)
		nextStack[roleID] = true
		for _, parentRoleID := range parentsByChild[roleID] {
			if err := walk(parentRoleID, depth+1, nextStack); err != nil {
				return err
			}
		}
		return nil
	}
	for _, roleID := range roleIDs {
		if err := walk(roleID, 0, map[string]bool{}); err != nil {
			return nil, err
		}
	}

	var rows []struct {
		UserID string `gorm:"column:user_id"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT user_id
		FROM user_department_roles
		WHERE role_id IN ?
	`, sortedBoolKeys(closure)).Scan(&rows).Error; err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
	}
	sort.Strings(userIDs)
	return userIDs, nil
}

func (s *SQLStore) CreateUserDepartmentRole(ctx context.Context, input UserDepartmentRoleInput) error {
	id := input.ID
	if id == "" {
		id = stableMutationID("user_department_role", input.UserID, input.DepartmentID, input.RoleID)
	}
	return s.db.WithContext(ctx).Exec(`
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
	`, id, input.UserID, input.DepartmentID, input.RoleID, input.ActorUserID).Error
}

func (s *SQLStore) DeleteUserDepartmentRole(ctx context.Context, id string) (RoleBinding, error) {
	var binding RoleBinding
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, user_id, department_id, role_id
		FROM user_department_roles
		WHERE id = ?
	`, id).Scan(&binding).Error; err != nil {
		return RoleBinding{}, err
	}
	if binding.ID == "" {
		return RoleBinding{}, ErrPrincipalNotFound
	}
	if err := s.db.WithContext(ctx).Exec(`
		DELETE FROM user_department_roles
		WHERE id = ?
	`, id).Error; err != nil {
		return RoleBinding{}, err
	}
	return binding, nil
}

func (s *SQLStore) ReplaceRolePermissions(ctx context.Context, roleID string, grants []PermissionGrant) error {
	if err := s.db.WithContext(ctx).Exec(`
		DELETE FROM permissions
		WHERE role_id = ?
	`, roleID).Error; err != nil {
		return err
	}
	for _, grant := range grants {
		rawConditions, err := encodeAttributeConditions(grant.AttributeConditions)
		if err != nil {
			return err
		}
		id := stableMutationID("permission", roleID, string(grant.Resource), string(grant.Action), rawConditions)
		if err := s.db.WithContext(ctx).Exec(`
			INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, id, roleID, grant.Resource, grant.Action, rawConditions).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) CreateRoleRelation(ctx context.Context, relation RoleRelation) error {
	id := relation.ID
	if id == "" {
		id = stableMutationID("role_relation", relation.ParentRoleID, relation.ChildRoleID)
	}
	return s.db.WithContext(ctx).Exec(`
		INSERT INTO role_relations (id, parent_role_id, child_role_id, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, id, relation.ParentRoleID, relation.ChildRoleID).Error
}

func (s *SQLStore) DeleteRoleRelation(ctx context.Context, id string) (RoleRelation, error) {
	var relation RoleRelation
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, parent_role_id, child_role_id
		FROM role_relations
		WHERE id = ?
	`, id).Scan(&relation).Error; err != nil {
		return RoleRelation{}, err
	}
	if relation.ID == "" {
		return RoleRelation{}, ErrPermissionNotFound
	}
	if err := s.db.WithContext(ctx).Exec(`
		DELETE FROM role_relations
		WHERE id = ?
	`, id).Error; err != nil {
		return RoleRelation{}, err
	}
	return relation, nil
}

func (s *SQLStore) loadUser(ctx context.Context, userID string) (User, error) {
	var row struct {
		ID         string
		EmployeeID string `gorm:"column:employee_id"`
		Name       string
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, employee_id, name
		FROM users
		WHERE id = ?
	`, userID).Scan(&row).Error; err != nil {
		return User{}, err
	}
	if row.ID == "" {
		return User{}, ErrPrincipalNotFound
	}
	return User{ID: row.ID, EmployeeID: row.EmployeeID, Name: row.Name}, nil
}

func (s *SQLStore) loadRoleBindings(ctx context.Context, userID string) ([]RoleBinding, error) {
	var rows []struct {
		ID           string
		UserID       string `gorm:"column:user_id"`
		DepartmentID string `gorm:"column:department_id"`
		RoleID       string `gorm:"column:role_id"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, user_id, department_id, role_id
		FROM user_department_roles
		WHERE user_id = ?
		ORDER BY id
	`, userID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	bindings := make([]RoleBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, RoleBinding{ID: row.ID, UserID: row.UserID, DepartmentID: row.DepartmentID, RoleID: row.RoleID})
	}
	return bindings, nil
}

func (s *SQLStore) loadDepartments(ctx context.Context) ([]Department, error) {
	var rows []Department
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, name
		FROM departments
		ORDER BY id
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLStore) loadRoles(ctx context.Context) ([]Role, error) {
	var rows []struct {
		ID          string
		Label       string
		Description string
		IsSystem    bool `gorm:"column:is_system"`
		Enabled     bool
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, label, description, is_system, enabled
		FROM roles
		ORDER BY id
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	roles := make([]Role, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, Role{ID: row.ID, Label: row.Label, Description: row.Description, IsSystem: row.IsSystem, Enabled: row.Enabled})
	}
	return roles, nil
}

func (s *SQLStore) loadPermissions(ctx context.Context) ([]PermissionGrant, error) {
	var rows []struct {
		RoleID              string `gorm:"column:role_id"`
		Resource            string
		Action              string
		AttributeConditions string `gorm:"column:attribute_conditions"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT role_id, resource, action, attribute_conditions
		FROM permissions
		ORDER BY role_id, resource, action, attribute_conditions
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	grants := make([]PermissionGrant, 0, len(rows))
	for _, row := range rows {
		conditions, err := decodeAttributeConditions(row.AttributeConditions)
		if err != nil {
			return nil, err
		}
		grants = append(grants, PermissionGrant{
			RoleID:              row.RoleID,
			Resource:            Resource(row.Resource),
			Action:              Action(row.Action),
			AttributeConditions: conditions,
		})
	}
	return grants, nil
}

func (s *SQLStore) LoadRoleRelations(ctx context.Context) ([]RoleRelation, error) {
	var rows []struct {
		ID           string
		ParentRoleID string `gorm:"column:parent_role_id"`
		ChildRoleID  string `gorm:"column:child_role_id"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, parent_role_id, child_role_id
		FROM role_relations
		ORDER BY id
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	relations := make([]RoleRelation, 0, len(rows))
	for _, row := range rows {
		relations = append(relations, RoleRelation{ID: row.ID, ParentRoleID: row.ParentRoleID, ChildRoleID: row.ChildRoleID})
	}
	return relations, nil
}

func encodeAttributeConditions(conditions AttributeConditions) (string, error) {
	raw, err := json.Marshal(conditions)
	if err != nil {
		return "", err
	}
	if string(raw) == "null" {
		return "{}", nil
	}
	return string(raw), nil
}

func decodeAttributeConditions(raw string) (AttributeConditions, error) {
	if raw == "" {
		return AttributeConditions{}, nil
	}
	var conditions AttributeConditions
	if err := json.Unmarshal([]byte(raw), &conditions); err != nil {
		return AttributeConditions{}, err
	}
	return conditions, nil
}

func stableMutationID(prefix string, parts ...string) string {
	slugParts := make([]string, 0, len(parts)+1)
	slugParts = append(slugParts, prefix)
	for _, part := range parts {
		slugParts = append(slugParts, stableIDPart(part))
	}
	return fmt.Sprintf("__%s__", strings.Join(slugParts, "_"))
}

func stableIDPart(value string) string {
	replacer := strings.NewReplacer(
		"__role_", "",
		"__", "_",
		".", "_",
		"-", "_",
		" ", "_",
		"{", "_",
		"}", "_",
		"[", "_",
		"]", "_",
		"\"", "",
		":", "_",
		";", "_",
		"=", "_",
		",", "_",
	)
	value = replacer.Replace(strings.ToLower(value))
	value = strings.Trim(value, "_")
	if value == "" {
		return "empty"
	}
	return value
}
