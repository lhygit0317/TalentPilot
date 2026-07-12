package useradmin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
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

func (s *SQLStore) ListUsers(ctx context.Context, query ListUsersQuery) (UserListResult, error) {
	rows, err := s.loadVisibleBindingRows(ctx, query.ListScope, query.Search, "")
	if err != nil {
		return UserListResult{}, err
	}
	users := map[string]*UserSummary{}
	for _, row := range rows {
		user := users[row.UserID]
		if user == nil {
			user = &UserSummary{ID: row.UserID, EmployeeID: row.EmployeeID, Name: row.UserName, CanAssign: query.CanAssign}
			users[row.UserID] = user
		}
		user.addBinding(row.toBinding(ScopeAllowsDepartment(query.DeleteScope, row.DepartmentID)))
	}

	items := make([]UserSummary, 0, len(users))
	for _, user := range users {
		finalizeUserSummary(user)
		items = append(items, *user)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].EmployeeID < items[j].EmployeeID
		}
		return items[i].Name < items[j].Name
	})
	limit := normalizedLimit(query.Limit)
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		nextCursor = items[len(items)-1].ID
	}

	summary, err := s.dataScopeSummary(ctx, query.ListScope)
	if err != nil {
		return UserListResult{}, err
	}
	return UserListResult{Items: items, NextCursor: nextCursor, DataScopeSummary: summary, CanAssignRoles: query.CanAssign}, nil
}

func (s *SQLStore) GetUser(ctx context.Context, userID string, scope iam.ScopePredicate) (UserDetail, error) {
	identity, err := s.GetUserIdentity(ctx, userID)
	if err != nil {
		return UserDetail{}, err
	}
	rows, err := s.loadVisibleBindingRows(ctx, scope, "", userID)
	if err != nil {
		return UserDetail{}, err
	}
	if len(rows) == 0 {
		return UserDetail{}, ErrPermissionDenied
	}
	user := &UserSummary{ID: identity.ID, EmployeeID: identity.EmployeeID, Name: identity.Name}
	for _, row := range rows {
		user.addBinding(row.toBinding(false))
	}
	finalizeUserSummary(user)
	return *user, nil
}

func (s *SQLStore) GetUserIdentity(ctx context.Context, userID string) (UserIdentity, error) {
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
		return UserIdentity{}, err
	}
	if row.ID == "" {
		return UserIdentity{}, ErrUserNotFound
	}
	return UserIdentity{ID: row.ID, EmployeeID: row.EmployeeID, Name: row.Name}, nil
}

func (s *SQLStore) ListAssignableRoles(ctx context.Context) ([]AssignableRole, error) {
	var rows []struct {
		ID          string
		Label       string
		Description string
		IsSystem    bool `gorm:"column:is_system"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, label, description, is_system
		FROM roles
		WHERE enabled = TRUE
		ORDER BY is_system DESC, label ASC, id ASC
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	summaries, err := s.roleAttributeSummaries(ctx)
	if err != nil {
		return nil, err
	}
	roles := make([]AssignableRole, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, AssignableRole{
			ID:                        row.ID,
			Label:                     row.Label,
			Description:               row.Description,
			IsSystem:                  row.IsSystem,
			SupportsSystemDepartment:  row.ID == iam.RoleGuest || iam.RoleSupportsGlobalScope(row.ID),
			AttributeConditionSummary: summaries[row.ID],
		})
	}
	return roles, nil
}

func (s *SQLStore) CreateRoleBindings(ctx context.Context, command CreateRoleBindingsCommand) ([]RoleBindingDetail, error) {
	if _, err := s.GetUserIdentity(ctx, command.UserID); err != nil {
		return nil, err
	}
	for _, binding := range command.Bindings {
		if !ScopeAllowsDepartment(command.CreateScope, binding.DepartmentID) {
			return nil, ErrPermissionDenied
		}
		if err := validateDepartmentRolePair(binding.DepartmentID, binding.RoleID); err != nil {
			return nil, err
		}
		if err := s.validateRoleAssignable(ctx, binding.RoleID); err != nil {
			return nil, err
		}
		if err := s.validateDepartmentExists(ctx, binding.DepartmentID); err != nil {
			return nil, err
		}
		duplicate, err := s.bindingExists(ctx, command.UserID, binding.DepartmentID, binding.RoleID)
		if err != nil {
			return nil, err
		}
		if duplicate {
			return nil, ErrDuplicateBinding
		}
	}

	created := make([]RoleBindingDetail, 0, len(command.Bindings))
	for _, binding := range command.Bindings {
		id := newUserAdminID("udr")
		if err := s.db.WithContext(ctx).Exec(`
			INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
		`, id, command.UserID, binding.DepartmentID, binding.RoleID, command.ActorUserID).Error; err != nil {
			if isUniqueConstraintError(err) {
				return nil, ErrDuplicateBinding
			}
			return nil, err
		}
		detail, err := s.GetRoleBinding(ctx, id)
		if err != nil {
			return nil, err
		}
		created = append(created, detail)
	}
	return created, nil
}

func (s *SQLStore) GetRoleBinding(ctx context.Context, id string) (RoleBindingDetail, error) {
	rows, err := s.loadBindingRows(ctx, "udr.id = ?", id)
	if err != nil {
		return RoleBindingDetail{}, err
	}
	if len(rows) == 0 {
		return RoleBindingDetail{}, ErrBindingNotFound
	}
	return rows[0].toBinding(false), nil
}

func (s *SQLStore) DeleteRoleBinding(ctx context.Context, id string) (RoleBindingDetail, error) {
	detail, err := s.GetRoleBinding(ctx, id)
	if err != nil {
		return RoleBindingDetail{}, err
	}
	if err := s.db.WithContext(ctx).Exec(`
		DELETE FROM user_department_roles
		WHERE id = ?
	`, id).Error; err != nil {
		return RoleBindingDetail{}, err
	}
	return detail, nil
}

func (s *SQLStore) CountNonGuestBindings(ctx context.Context, userID string) (int, error) {
	var count int64
	if err := s.db.WithContext(ctx).Table("user_department_roles").
		Where("user_id = ? AND role_id <> ?", userID, iam.RoleGuest).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *SQLStore) EnsureGuestBinding(ctx context.Context, userID string, actorUserID string) (RoleBindingDetail, bool, error) {
	if _, err := s.GetUserIdentity(ctx, userID); err != nil {
		return RoleBindingDetail{}, false, err
	}
	existing, err := s.findBinding(ctx, userID, iam.SystemDepartmentID, iam.RoleGuest)
	if err != nil {
		return RoleBindingDetail{}, false, err
	}
	if existing.ID != "" {
		return existing, false, nil
	}
	id := newUserAdminID("udr")
	if err := s.db.WithContext(ctx).Exec(`
		INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
	`, id, userID, iam.SystemDepartmentID, iam.RoleGuest, actorUserID).Error; err != nil {
		return RoleBindingDetail{}, false, err
	}
	detail, err := s.GetRoleBinding(ctx, id)
	if err != nil {
		return RoleBindingDetail{}, false, err
	}
	return detail, true, nil
}

type bindingRow struct {
	ID             string
	UserID         string `gorm:"column:user_id"`
	EmployeeID     string `gorm:"column:employee_id"`
	UserName       string `gorm:"column:user_name"`
	DepartmentID   string `gorm:"column:department_id"`
	DepartmentName string `gorm:"column:department_name"`
	RoleID         string `gorm:"column:role_id"`
	RoleLabel      string `gorm:"column:role_label"`
	RoleIsSystem   bool   `gorm:"column:role_is_system"`
	RoleEnabled    bool   `gorm:"column:role_enabled"`
	CreatedAt      time.Time
	CreatedBy      string `gorm:"column:created_by"`
}

func (row bindingRow) toBinding(canDelete bool) RoleBindingDetail {
	guest := row.RoleID == iam.RoleGuest
	return RoleBindingDetail{
		ID:     row.ID,
		UserID: row.UserID,
		Role: RoleSummary{
			ID:       row.RoleID,
			Label:    row.RoleLabel,
			IsSystem: row.RoleIsSystem,
			Enabled:  row.RoleEnabled,
		},
		Department: DepartmentSummary{
			ID:     row.DepartmentID,
			Name:   row.DepartmentName,
			System: row.DepartmentID == iam.SystemDepartmentID,
		},
		Guest:     guest,
		CreatedAt: row.CreatedAt,
		CreatedBy: row.CreatedBy,
		CanDelete: canDelete && !guest,
	}
}

func (s *SQLStore) loadVisibleBindingRows(ctx context.Context, scope iam.ScopePredicate, search string, userID string) ([]bindingRow, error) {
	where, args := userDepartmentRoleScopeWhere(scope)
	query := s.bindingRowsQuery(ctx).Where(where, args...)
	if userID != "" {
		query = query.Where("udr.user_id = ?", userID)
	}
	if strings.TrimSpace(search) != "" {
		like := "%" + escapeLike(strings.ToLower(strings.TrimSpace(search))) + "%"
		query = query.Where(`(
			lower(users.name) LIKE ? ESCAPE '\' OR
			lower(users.employee_id) LIKE ? ESCAPE '\' OR
			lower(COALESCE(departments.name, 'system')) LIKE ? ESCAPE '\' OR
			lower(roles.label) LIKE ? ESCAPE '\'
		)`, like, like, like, like)
	}
	var rows []bindingRow
	if err := query.Order("users.name ASC, users.employee_id ASC, udr.created_at ASC, udr.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLStore) loadBindingRows(ctx context.Context, where string, args ...any) ([]bindingRow, error) {
	var rows []bindingRow
	if err := s.bindingRowsQuery(ctx).Where(where, args...).Order("udr.created_at ASC, udr.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *SQLStore) bindingRowsQuery(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).Table("user_department_roles AS udr").
		Select(`
			udr.id,
			udr.user_id,
			users.employee_id,
			users.name AS user_name,
			udr.department_id,
			COALESCE(departments.name, 'system') AS department_name,
			udr.role_id,
			roles.label AS role_label,
			roles.is_system AS role_is_system,
			roles.enabled AS role_enabled,
			udr.created_at,
			udr.created_by
		`).
		Joins("JOIN users ON users.id = udr.user_id").
		Joins("JOIN roles ON roles.id = udr.role_id").
		Joins("LEFT JOIN departments ON departments.id = udr.department_id")
}

func (s *SQLStore) validateRoleAssignable(ctx context.Context, roleID string) error {
	var row struct {
		ID      string
		Enabled bool
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, enabled
		FROM roles
		WHERE id = ?
	`, roleID).Scan(&row).Error; err != nil {
		return err
	}
	if row.ID == "" {
		return ErrRoleNotFound
	}
	if !row.Enabled {
		return ErrRoleDisabled
	}
	return nil
}

func (s *SQLStore) validateDepartmentExists(ctx context.Context, departmentID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Table("departments").Where("id = ?", departmentID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrDepartmentNotFound
	}
	return nil
}

func validateDepartmentRolePair(departmentID string, roleID string) error {
	if departmentID == iam.SystemDepartmentID && roleID != iam.RoleGuest && !iam.RoleSupportsGlobalScope(roleID) {
		return iam.ErrScopeUnsupported
	}
	return nil
}

func (s *SQLStore) bindingExists(ctx context.Context, userID string, departmentID string, roleID string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Table("user_department_roles").
		Where("user_id = ? AND department_id = ? AND role_id = ?", userID, departmentID, roleID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLStore) findBinding(ctx context.Context, userID string, departmentID string, roleID string) (RoleBindingDetail, error) {
	var row struct {
		ID string
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id
		FROM user_department_roles
		WHERE user_id = ? AND department_id = ? AND role_id = ?
	`, userID, departmentID, roleID).Scan(&row).Error; err != nil {
		return RoleBindingDetail{}, err
	}
	if row.ID == "" {
		return RoleBindingDetail{}, nil
	}
	return s.GetRoleBinding(ctx, row.ID)
}

func (s *SQLStore) dataScopeSummary(ctx context.Context, scope iam.ScopePredicate) (string, error) {
	if scope.AllDepartments {
		return "全部部门", nil
	}
	departmentIDs := scopeDepartmentIDs(scope)
	if len(departmentIDs) == 0 {
		return "暂无部门范围", nil
	}
	var rows []struct {
		ID   string
		Name string
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, name
		FROM departments
		WHERE id IN ?
		ORDER BY name ASC, id ASC
	`, departmentIDs).Scan(&rows).Error; err != nil {
		return "", err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	if len(names) == 0 {
		return "暂无部门范围", nil
	}
	return "负责部门:" + strings.Join(names, "、"), nil
}

func (s *SQLStore) roleAttributeSummaries(ctx context.Context) (map[string]string, error) {
	var rows []struct {
		RoleID              string `gorm:"column:role_id"`
		AttributeConditions string `gorm:"column:attribute_conditions"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT role_id, attribute_conditions
		FROM permissions
		WHERE attribute_conditions <> '{}'
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	channelsByRole := map[string]map[string]bool{}
	for _, row := range rows {
		if channelsByRole[row.RoleID] == nil {
			channelsByRole[row.RoleID] = map[string]bool{}
		}
		if strings.Contains(row.AttributeConditions, `"social"`) {
			channelsByRole[row.RoleID]["社招"] = true
		}
		if strings.Contains(row.AttributeConditions, `"campus"`) {
			channelsByRole[row.RoleID]["校招"] = true
		}
	}
	summaries := map[string]string{}
	for roleID, channels := range channelsByRole {
		labels := make([]string, 0, len(channels))
		for channel := range channels {
			labels = append(labels, channel)
		}
		sort.Strings(labels)
		summaries[roleID] = strings.Join(labels, "、")
	}
	return summaries, nil
}

func userDepartmentRoleScopeWhere(scope iam.ScopePredicate) (string, []any) {
	if scope.AllDepartments {
		return "1 = 1", nil
	}
	for _, branch := range scope.Branches {
		if branch.AllDepartments {
			return "1 = 1", nil
		}
	}
	departmentIDs := scopeDepartmentIDs(scope)
	if len(departmentIDs) == 0 {
		return "1 = 0", nil
	}
	return "udr.department_id IN ?", []any{departmentIDs}
}

func scopeDepartmentIDs(scope iam.ScopePredicate) []string {
	seen := map[string]bool{}
	for _, departmentID := range scope.DepartmentIDs {
		if departmentID != "" {
			seen[departmentID] = true
		}
	}
	for _, branch := range scope.Branches {
		for _, departmentID := range branch.DepartmentIDs {
			if departmentID != "" {
				seen[departmentID] = true
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (user *UserSummary) addBinding(binding RoleBindingDetail) {
	user.RoleBindings = append(user.RoleBindings, binding)
	if !binding.Guest {
		user.Departments = appendUniqueDepartment(user.Departments, binding.Department)
	}
}

func finalizeUserSummary(user *UserSummary) {
	if user.Departments == nil {
		user.Departments = []DepartmentSummary{}
	}
	if user.RoleBindings == nil {
		user.RoleBindings = []RoleBindingDetail{}
	}
	user.GuestOnly = hasOnlyGuest(user.RoleBindings)
	user.RoleSummary = formatRoleSummary(user.RoleBindings)
}

func appendUniqueDepartment(departments []DepartmentSummary, department DepartmentSummary) []DepartmentSummary {
	for _, existing := range departments {
		if existing.ID == department.ID {
			return departments
		}
	}
	return append(departments, department)
}

func hasOnlyGuest(bindings []RoleBindingDetail) bool {
	return len(bindings) > 0 && func() bool {
		for _, binding := range bindings {
			if !binding.Guest {
				return false
			}
		}
		return true
	}()
}

func formatRoleSummary(bindings []RoleBindingDetail) string {
	if len(bindings) == 0 {
		return ""
	}
	byRole := map[string]map[string]bool{}
	for _, binding := range bindings {
		if byRole[binding.Role.Label] == nil {
			byRole[binding.Role.Label] = map[string]bool{}
		}
		byRole[binding.Role.Label][binding.Department.Name] = true
	}
	roles := make([]string, 0, len(byRole))
	for role := range byRole {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	parts := make([]string, 0, len(roles))
	for _, role := range roles {
		departments := make([]string, 0, len(byRole[role]))
		for department := range byRole[role] {
			departments = append(departments, department)
		}
		sort.Strings(departments)
		parts = append(parts, role+"(部门:"+strings.Join(departments, "、")+")")
	}
	return strings.Join(parts, "；")
}

func normalizedLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func newUserAdminID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return prefix + "_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	}
	return prefix + "_" + hex.EncodeToString(raw[:])
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key")
}
