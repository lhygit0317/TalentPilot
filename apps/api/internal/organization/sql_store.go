package organization

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
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

func (s *SQLStore) ListDepartments(ctx context.Context, query DepartmentListQuery) (DepartmentListResult, error) {
	rowsQuery := s.departmentRows(ctx).
		Where("departments.id <> ?", iam.SystemDepartmentID)
	where, args := departmentScopeWhere(query.Scope, "departments.id")
	rowsQuery = rowsQuery.Where(where, args...)
	if query.Search != "" {
		like := "%" + escapeLike(strings.ToLower(strings.TrimSpace(query.Search))) + "%"
		rowsQuery = rowsQuery.Where("lower(departments.name) LIKE ? ESCAPE '\\'", like)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var rows []departmentRow
	if err := rowsQuery.Order("departments.updated_at DESC, departments.id ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return DepartmentListResult{}, err
	}

	items := make([]DepartmentListItem, 0, len(rows))
	for _, row := range rows {
		item := row.listItem()
		item.CanGet = departmentMatchesScope(row.ID, query.GetScope)
		item.CanUpdate = departmentMatchesScope(row.ID, query.UpdateScope)
		item.CanDelete = departmentMatchesScope(row.ID, query.DeleteScope)
		items = append(items, item)
	}
	return DepartmentListResult{Items: items}, nil
}

func (s *SQLStore) GetDepartment(ctx context.Context, id string, scope iam.ScopePredicate) (DepartmentDetail, error) {
	row, err := s.getScopedDepartmentRow(ctx, id, scope)
	if err != nil {
		return DepartmentDetail{}, err
	}
	positions, err := s.departmentPositions(ctx, id)
	if err != nil {
		return DepartmentDetail{}, err
	}
	item := row.listItem()
	item.CanGet = true
	return DepartmentDetail{DepartmentListItem: item, Positions: positions}, nil
}

func (s *SQLStore) CreateDepartment(ctx context.Context, name string) (DepartmentDetail, error) {
	exists, err := s.departmentNameExists(ctx, name, "")
	if err != nil {
		return DepartmentDetail{}, err
	}
	if exists {
		return DepartmentDetail{}, ErrDepartmentNameDuplicate
	}
	id := newID("department")
	if err := s.db.WithContext(ctx).Exec(`
		INSERT INTO departments (id, name, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, name).Error; err != nil {
		return DepartmentDetail{}, err
	}
	return s.GetDepartment(ctx, id, allDepartmentScope(iam.ActionGet))
}

func (s *SQLStore) UpdateDepartment(ctx context.Context, id string, name string, scope iam.ScopePredicate) (DepartmentDetail, error) {
	if id == iam.SystemDepartmentID {
		return DepartmentDetail{}, ErrDepartmentSystemProtected
	}
	if _, err := s.getScopedDepartmentRow(ctx, id, scope); err != nil {
		return DepartmentDetail{}, err
	}
	exists, err := s.departmentNameExists(ctx, name, id)
	if err != nil {
		return DepartmentDetail{}, err
	}
	if exists {
		return DepartmentDetail{}, ErrDepartmentNameDuplicate
	}
	if err := s.db.WithContext(ctx).Exec(`
		UPDATE departments
		SET name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, id).Error; err != nil {
		return DepartmentDetail{}, err
	}
	return s.GetDepartment(ctx, id, allDepartmentScope(iam.ActionGet))
}

func (s *SQLStore) DeleteDepartment(ctx context.Context, id string, scope iam.ScopePredicate) (DepartmentDetail, error) {
	if id == iam.SystemDepartmentID {
		return DepartmentDetail{}, ErrDepartmentSystemProtected
	}
	department, err := s.GetDepartment(ctx, id, scope)
	if err != nil {
		return DepartmentDetail{}, err
	}
	hasRelations, err := s.departmentHasRelations(ctx, id)
	if err != nil {
		return DepartmentDetail{}, err
	}
	if hasRelations {
		return DepartmentDetail{}, ErrDepartmentDeleteHasRelations
	}
	if err := s.db.WithContext(ctx).Exec("DELETE FROM departments WHERE id = ?", id).Error; err != nil {
		return DepartmentDetail{}, err
	}
	return department, nil
}

func (s *SQLStore) getScopedDepartmentRow(ctx context.Context, id string, scope iam.ScopePredicate) (departmentRow, error) {
	where, args := departmentScopeWhere(scope, "departments.id")
	var row departmentRow
	if err := s.departmentRows(ctx).
		Where("departments.id <> ?", iam.SystemDepartmentID).
		Where(where, args...).
		Where("departments.id = ?", id).
		Limit(1).
		Scan(&row).Error; err != nil {
		return departmentRow{}, err
	}
	if row.ID == "" {
		return departmentRow{}, ErrDepartmentNotFound
	}
	return row, nil
}

func (s *SQLStore) departmentRows(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).
		Table("departments").
		Select(`
			departments.id,
			departments.name,
			departments.updated_at,
			(SELECT COUNT(*) FROM department_positions WHERE department_positions.department_id = departments.id) AS position_count,
			(SELECT COUNT(*) FROM department_resumes WHERE department_resumes.department_id = departments.id) AS resume_count
		`)
}

func (s *SQLStore) departmentPositions(ctx context.Context, departmentID string) ([]PositionSummary, error) {
	var positions []PositionSummary
	if err := s.db.WithContext(ctx).
		Table("positions").
		Select("positions.id, positions.name, positions.chan, positions.level, positions.status").
		Joins("JOIN department_positions ON department_positions.position_id = positions.id").
		Where("department_positions.department_id = ?", departmentID).
		Order("positions.updated_at DESC, positions.id ASC").
		Scan(&positions).Error; err != nil {
		return nil, err
	}
	if positions == nil {
		positions = []PositionSummary{}
	}
	return positions, nil
}

func (s *SQLStore) departmentNameExists(ctx context.Context, name string, excludeID string) (bool, error) {
	query := s.db.WithContext(ctx).Table("departments").Where("name = ?", name)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLStore) departmentHasRelations(ctx context.Context, departmentID string) (bool, error) {
	for _, table := range []string{"department_positions", "department_resumes", "user_department_roles"} {
		var count int64
		if err := s.db.WithContext(ctx).Table(table).Where("department_id = ?", departmentID).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

type departmentRow struct {
	ID            string
	Name          string
	UpdatedAt     time.Time `gorm:"column:updated_at"`
	PositionCount int       `gorm:"column:position_count"`
	ResumeCount   int       `gorm:"column:resume_count"`
}

func (r departmentRow) listItem() DepartmentListItem {
	return DepartmentListItem{
		ID:            r.ID,
		Name:          r.Name,
		PositionCount: r.PositionCount,
		ResumeCount:   r.ResumeCount,
		UpdatedAt:     r.UpdatedAt,
	}
}

func departmentScopeWhere(scope iam.ScopePredicate, departmentColumn string) (string, []any) {
	var clauses []string
	var args []any
	for _, branch := range scope.Branches {
		if branch.AllDepartments {
			clauses = append(clauses, "1 = 1")
			continue
		}
		if len(branch.DepartmentIDs) == 0 {
			continue
		}
		clauses = append(clauses, departmentColumn+" IN ?")
		args = append(args, branch.DepartmentIDs)
	}
	if len(clauses) == 0 {
		return "1 = 0", nil
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func departmentMatchesScope(departmentID string, scope iam.ScopePredicate) bool {
	if departmentID == "" || departmentID == iam.SystemDepartmentID {
		return false
	}
	for _, branch := range scope.Branches {
		if branch.AllDepartments {
			return true
		}
		if containsString(branch.DepartmentIDs, departmentID) {
			return true
		}
	}
	return false
}

func allDepartmentScope(action iam.Action) iam.ScopePredicate {
	return iam.ScopePredicate{
		Resource: iam.ResourceDepartment,
		Action:   action,
		Branches: []iam.ScopeBranch{{AllDepartments: true}},
	}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
