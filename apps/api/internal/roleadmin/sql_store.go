package roleadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"gorm.io/gorm"
)

const (
	defaultRoleListLimit = 100
	maxRoleListLimit     = 200
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

func (s *SQLStore) ListRoles(ctx context.Context, query RoleListQuery) (RoleListResult, error) {
	limit := normalizeRoleListLimit(query.Limit)
	rows := []roleListRow{}
	db := s.db.WithContext(ctx).Table("roles").
		Select(`
			roles.id,
			roles.label,
			roles.description,
			roles.is_system,
			roles.enabled,
			COALESCE(permission_counts.permission_count, 0) AS permission_count,
			COALESCE(child_counts.child_role_count, 0) AS child_role_count,
			COALESCE(reference_counts.reference_count, 0) AS reference_count
		`).
		Joins("LEFT JOIN (SELECT role_id, COUNT(*) AS permission_count FROM permissions GROUP BY role_id) AS permission_counts ON permission_counts.role_id = roles.id").
		Joins("LEFT JOIN (SELECT parent_role_id, COUNT(*) AS child_role_count FROM role_relations GROUP BY parent_role_id) AS child_counts ON child_counts.parent_role_id = roles.id").
		Joins("LEFT JOIN (SELECT role_id, COUNT(*) AS reference_count FROM user_department_roles GROUP BY role_id) AS reference_counts ON reference_counts.role_id = roles.id")
	if query.Search != "" {
		pattern := "%" + escapeLike(strings.ToLower(strings.TrimSpace(query.Search))) + "%"
		db = db.Where("lower(roles.label) LIKE ? ESCAPE '\\' OR lower(roles.description) LIKE ? ESCAPE '\\'", pattern, pattern)
	}
	if query.System != nil {
		db = db.Where("roles.is_system = ?", *query.System)
	}
	if query.Enabled != nil {
		db = db.Where("roles.enabled = ?", *query.Enabled)
	}
	if err := db.Order("roles.is_system DESC, roles.label ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return RoleListResult{}, err
	}

	conditions, err := s.roleConditionSummaries(ctx)
	if err != nil {
		return RoleListResult{}, err
	}

	items := make([]RoleListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, RoleListItem{
			ID:               row.ID,
			Label:            row.Label,
			Description:      row.Description,
			IsSystem:         row.IsSystem,
			Enabled:          row.Enabled,
			PermissionCount:  row.PermissionCount,
			ChildRoleCount:   row.ChildRoleCount,
			ReferenceCount:   row.ReferenceCount,
			ConditionSummary: conditionSummary(conditions[row.ID]),
			CanEdit:          query.ActorCanEdit,
			CanDelete:        query.ActorCanDelete && !row.IsSystem && row.ReferenceCount == 0,
			CanToggleEnabled: query.ActorCanToggle,
		})
	}
	return RoleListResult{
		Items:     items,
		Total:     len(items),
		CanCreate: query.ActorCanCreate,
	}, nil
}

func (s *SQLStore) GetRole(ctx context.Context, roleID string, query RoleCapabilityQuery) (RoleDetail, error) {
	var row roleListRow
	if err := s.db.WithContext(ctx).Table("roles").
		Select(`
			roles.id,
			roles.label,
			roles.description,
			roles.is_system,
			roles.enabled,
			0 AS permission_count,
			0 AS child_role_count,
			COALESCE(reference_counts.reference_count, 0) AS reference_count
		`).
		Joins("LEFT JOIN (SELECT role_id, COUNT(*) AS reference_count FROM user_department_roles GROUP BY role_id) AS reference_counts ON reference_counts.role_id = roles.id").
		Where("roles.id = ?", roleID).
		Scan(&row).Error; err != nil {
		return RoleDetail{}, err
	}
	if row.ID == "" {
		return RoleDetail{}, ErrRoleNotFound
	}

	permissions, err := s.rolePermissions(ctx, roleID)
	if err != nil {
		return RoleDetail{}, err
	}
	childRoleIDs, err := s.childRoleIDs(ctx, roleID)
	if err != nil {
		return RoleDetail{}, err
	}

	return RoleDetail{
		ID:               row.ID,
		Label:            row.Label,
		Description:      row.Description,
		IsSystem:         row.IsSystem,
		Enabled:          row.Enabled,
		ReferenceCount:   row.ReferenceCount,
		Permissions:      permissions,
		ChildRoleIDs:     childRoleIDs,
		CanEdit:          query.ActorCanEdit,
		CanDelete:        query.ActorCanDelete && !row.IsSystem && row.ReferenceCount == 0,
		CanToggleEnabled: query.ActorCanToggle,
	}, nil
}

func (s *SQLStore) PermissionOptions(context.Context) (PermissionOptionsResult, error) {
	whitelist := iam.PermissionWhitelist()
	resources := make([]PermissionResourceOption, 0, len(whitelist))
	for _, resource := range permissionResourceOrder() {
		actions, ok := whitelist[resource]
		if !ok {
			continue
		}
		group := PermissionResourceOption{Resource: resource}
		for _, action := range permissionActionOrder() {
			allowance, ok := actions[action]
			if !ok {
				continue
			}
			group.Actions = append(group.Actions, PermissionActionOption{
				Action: action,
				SupportsConditions: ConditionSupport{
					Channels: allowance.Channels,
					Expired:  allowance.Expired,
					Self:     allowance.Self,
				},
			})
		}
		resources = append(resources, group)
	}
	return PermissionOptionsResult{
		Resources: resources,
		ConditionOptions: ConditionOptions{
			Channels: []string{"social", "campus"},
			Expired:  []bool{false, true},
		},
	}, nil
}

func (s *SQLStore) GetRoleRecord(ctx context.Context, roleID string) (RoleRecord, error) {
	var row roleListRow
	if err := s.db.WithContext(ctx).Table("roles").
		Select(`
			roles.id,
			roles.label,
			roles.description,
			roles.is_system,
			roles.enabled,
			0 AS permission_count,
			0 AS child_role_count,
			COALESCE(reference_counts.reference_count, 0) AS reference_count
		`).
		Joins("LEFT JOIN (SELECT role_id, COUNT(*) AS reference_count FROM user_department_roles GROUP BY role_id) AS reference_counts ON reference_counts.role_id = roles.id").
		Where("roles.id = ?", roleID).
		Scan(&row).Error; err != nil {
		return RoleRecord{}, err
	}
	if row.ID == "" {
		return RoleRecord{}, ErrRoleNotFound
	}
	return RoleRecord{
		ID:             row.ID,
		Label:          row.Label,
		Description:    row.Description,
		IsSystem:       row.IsSystem,
		Enabled:        row.Enabled,
		ReferenceCount: row.ReferenceCount,
	}, nil
}

func (s *SQLStore) RoleLabelExists(ctx context.Context, label string, excludeRoleID string) (bool, error) {
	var count int64
	db := s.db.WithContext(ctx).Table("roles").Where("label = ?", label)
	if excludeRoleID != "" {
		db = db.Where("id <> ?", excludeRoleID)
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLStore) ChildRolesExist(ctx context.Context, roleIDs []string) (bool, error) {
	if len(roleIDs) == 0 {
		return true, nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Table("roles").Where("id IN ?", roleIDs).Count(&count).Error; err != nil {
		return false, err
	}
	return count == int64(len(roleIDs)), nil
}

func (s *SQLStore) LoadRoleRelations(ctx context.Context) ([]iam.RoleRelation, error) {
	var rows []struct {
		ID           string `gorm:"column:id"`
		ParentRoleID string `gorm:"column:parent_role_id"`
		ChildRoleID  string `gorm:"column:child_role_id"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, parent_role_id, child_role_id
		FROM role_relations
		ORDER BY parent_role_id ASC, child_role_id ASC
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	relations := make([]iam.RoleRelation, 0, len(rows))
	for _, row := range rows {
		relations = append(relations, iam.RoleRelation{ID: row.ID, ParentRoleID: row.ParentRoleID, ChildRoleID: row.ChildRoleID})
	}
	return relations, nil
}

func (s *SQLStore) CreateRole(ctx context.Context, input RoleDefinitionRecord) (string, error) {
	roleID := stableMutationID("role", input.Label)
	if err := s.db.WithContext(ctx).Exec(`
		INSERT INTO roles (id, label, description, is_system, enabled, created_at, created_by, updated_at)
		VALUES (?, ?, ?, FALSE, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP)
	`, roleID, input.Label, input.Description, input.Enabled, input.ActorUserID).Error; err != nil {
		return "", err
	}
	return roleID, nil
}

func (s *SQLStore) UpdateRole(ctx context.Context, roleID string, input RoleDefinitionRecord) error {
	return s.db.WithContext(ctx).Exec(`
		UPDATE roles
		SET label = ?, description = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.Label, input.Description, input.Enabled, roleID).Error
}

func (s *SQLStore) ReplaceRolePermissions(ctx context.Context, roleID string, permissions []PermissionInput) error {
	if err := s.db.WithContext(ctx).Exec(`
		DELETE FROM permissions
		WHERE role_id = ?
	`, roleID).Error; err != nil {
		return err
	}
	for _, permission := range permissions {
		rawConditions, err := encodeAttributeConditions(permission.AttributeConditions)
		if err != nil {
			return err
		}
		id := stableMutationID("permission", roleID, string(permission.Resource), string(permission.Action), rawConditions)
		if err := s.db.WithContext(ctx).Exec(`
			INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, id, roleID, permission.Resource, permission.Action, rawConditions).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) ReplaceRoleChildren(ctx context.Context, roleID string, childRoleIDs []string) error {
	if err := s.db.WithContext(ctx).Exec(`
		DELETE FROM role_relations
		WHERE parent_role_id = ?
	`, roleID).Error; err != nil {
		return err
	}
	for _, childRoleID := range childRoleIDs {
		id := stableMutationID("role_relation", roleID, childRoleID)
		if err := s.db.WithContext(ctx).Exec(`
			INSERT INTO role_relations (id, parent_role_id, child_role_id, created_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		`, id, roleID, childRoleID).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) ToggleRoleEnabled(ctx context.Context, roleID string, enabled bool) error {
	return s.db.WithContext(ctx).Exec(`
		UPDATE roles
		SET enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, enabled, roleID).Error
}

func (s *SQLStore) DeleteRole(ctx context.Context, roleID string) error {
	return s.db.WithContext(ctx).Exec(`
		DELETE FROM roles
		WHERE id = ?
	`, roleID).Error
}

type roleListRow struct {
	ID              string `gorm:"column:id"`
	Label           string `gorm:"column:label"`
	Description     string `gorm:"column:description"`
	IsSystem        bool   `gorm:"column:is_system"`
	Enabled         bool   `gorm:"column:enabled"`
	PermissionCount int    `gorm:"column:permission_count"`
	ChildRoleCount  int    `gorm:"column:child_role_count"`
	ReferenceCount  int    `gorm:"column:reference_count"`
}

func (s *SQLStore) rolePermissions(ctx context.Context, roleID string) ([]PermissionInput, error) {
	var rows []struct {
		Resource            iam.Resource `gorm:"column:resource"`
		Action              iam.Action   `gorm:"column:action"`
		AttributeConditions string       `gorm:"column:attribute_conditions"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT resource, action, attribute_conditions
		FROM permissions
		WHERE role_id = ?
		ORDER BY resource ASC, action ASC, attribute_conditions ASC
	`, roleID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	permissions := make([]PermissionInput, 0, len(rows))
	for _, row := range rows {
		conditions, err := decodeAttributeConditions(row.AttributeConditions)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, PermissionInput{
			Resource:            row.Resource,
			Action:              row.Action,
			AttributeConditions: conditions,
		})
	}
	return permissions, nil
}

func (s *SQLStore) childRoleIDs(ctx context.Context, roleID string) ([]string, error) {
	var rows []struct {
		ChildRoleID string `gorm:"column:child_role_id"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT child_role_id
		FROM role_relations
		WHERE parent_role_id = ?
		ORDER BY child_role_id ASC
	`, roleID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	childRoleIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		childRoleIDs = append(childRoleIDs, row.ChildRoleID)
	}
	return childRoleIDs, nil
}

func (s *SQLStore) roleConditionSummaries(ctx context.Context) (map[string]map[string]bool, error) {
	var rows []struct {
		RoleID              string `gorm:"column:role_id"`
		AttributeConditions string `gorm:"column:attribute_conditions"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT role_id, attribute_conditions
		FROM permissions
		WHERE resource = ? AND attribute_conditions <> '{}'
	`, iam.ResourceResume).Scan(&rows).Error; err != nil {
		return nil, err
	}
	summaries := map[string]map[string]bool{}
	for _, row := range rows {
		conditions, err := decodeAttributeConditions(row.AttributeConditions)
		if err != nil {
			return nil, err
		}
		if len(conditions.Channels) == 0 {
			continue
		}
		if summaries[row.RoleID] == nil {
			summaries[row.RoleID] = map[string]bool{}
		}
		for _, channel := range conditions.Channels {
			summaries[row.RoleID][channel] = true
		}
	}
	return summaries, nil
}

func decodeAttributeConditions(raw string) (iam.AttributeConditions, error) {
	if raw == "" || raw == "{}" {
		return iam.AttributeConditions{}, nil
	}
	var conditions iam.AttributeConditions
	if err := json.Unmarshal([]byte(raw), &conditions); err != nil {
		return iam.AttributeConditions{}, err
	}
	sort.Strings(conditions.Channels)
	return conditions, nil
}

func encodeAttributeConditions(conditions iam.AttributeConditions) (string, error) {
	conditions = normalizeAttributeConditions(conditions)
	raw, err := json.Marshal(conditions)
	if err != nil {
		return "", err
	}
	if string(raw) == "null" {
		return "{}", nil
	}
	return string(raw), nil
}

func conditionSummary(channels map[string]bool) string {
	if len(channels) == 0 {
		return "全部渠道"
	}
	parts := make([]string, 0, 2)
	if channels["social"] {
		parts = append(parts, "社招")
	}
	if channels["campus"] {
		parts = append(parts, "校招")
	}
	if len(parts) == 0 {
		return "全部渠道"
	}
	return strings.Join(parts, "、")
}

func normalizeRoleListLimit(limit int) int {
	if limit <= 0 {
		return defaultRoleListLimit
	}
	if limit > maxRoleListLimit {
		return maxRoleListLimit
	}
	return limit
}

func permissionResourceOrder() []iam.Resource {
	return []iam.Resource{
		iam.ResourceUser,
		iam.ResourceDepartment,
		iam.ResourcePosition,
		iam.ResourceResume,
		iam.ResourceRole,
		iam.ResourcePermission,
		iam.ResourceUserDepartmentRole,
		iam.ResourceDepartmentPosition,
		iam.ResourceDepartmentResume,
		iam.ResourcePositionResume,
		iam.ResourceRoleRelation,
		iam.ResourceNotification,
		iam.ResourceAuditLog,
		iam.ResourceJob,
	}
}

func permissionActionOrder() []iam.Action {
	return []iam.Action{iam.ActionList, iam.ActionGet, iam.ActionCreate, iam.ActionUpdate, iam.ActionDelete}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
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
