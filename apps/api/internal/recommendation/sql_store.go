package recommendation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
	"gorm.io/gorm"
)

type SQLStore struct {
	db *gorm.DB
}

func NewSQLStore(db *gorm.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) GetResume(ctx context.Context, resumeID string, scope iam.ScopePredicate) (ResumeContext, error) {
	where, args := resumeScopeWhere(scope)
	var row resumeRow
	if err := s.db.WithContext(ctx).
		Table("resumes").
		Select(`
			resumes.id,
			resumes.normalized_name,
			resumes.name,
			resumes.chan,
			resumes.pos,
			resumes.source,
			resumes.source_by,
			resumes.keywords,
			resumes.traits,
			resumes.exp_base,
			resumes.expired,
			department_resumes.department_id,
			departments.name AS department_name
		`).
		Joins("JOIN department_resumes ON department_resumes.resume_id = resumes.id").
		Joins("JOIN departments ON departments.id = department_resumes.department_id").
		Where(where, args...).
		Where("resumes.id = ?", resumeID).
		Limit(1).
		Scan(&row).Error; err != nil {
		return ResumeContext{}, err
	}
	if row.ID == "" {
		return ResumeContext{}, ErrResumeNotFound
	}
	return row.context()
}

func (s *SQLStore) ListRoutePositions(ctx context.Context, query RoutePositionQuery) ([]PositionContext, error) {
	where, args := departmentScopeWhere(query.Scope, "department_positions.department_id")
	var rows []positionRow
	if err := s.db.WithContext(ctx).
		Table("positions").
		Select(`
			positions.id,
			positions.name,
			positions.chan,
			positions.level,
			positions.status,
			positions.keywords,
			positions.implicit_tags,
			department_positions.department_id,
			departments.name AS department_name
		`).
		Joins("JOIN department_positions ON department_positions.position_id = positions.id").
		Joins("JOIN departments ON departments.id = department_positions.department_id").
		Where(where, args...).
		Where("positions.chan = ? AND positions.status = 'on'", query.Channel).
		Order("departments.name ASC, positions.name ASC, positions.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	positions := make([]PositionContext, 0, len(rows))
	for _, row := range rows {
		position, err := row.context()
		if err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	return positions, nil
}

func (s *SQLStore) ListDepartmentContacts(ctx context.Context, departmentIDs []string) (map[string]DepartmentContacts, error) {
	contacts := map[string]DepartmentContacts{}
	if len(departmentIDs) == 0 {
		return contacts, nil
	}
	var rows []struct {
		DepartmentID string `gorm:"column:department_id"`
		RoleID       string `gorm:"column:role_id"`
		UserName     string `gorm:"column:name"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT user_department_roles.department_id, user_department_roles.role_id, users.name
		FROM user_department_roles
		JOIN users ON users.id = user_department_roles.user_id
		WHERE user_department_roles.department_id IN ?
			AND user_department_roles.role_id IN (?, ?, ?)
		ORDER BY users.name ASC, users.id ASC
	`, departmentIDs, iam.RoleHRBP, iam.RoleManager, iam.RoleTrainee).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		contact := contacts[row.DepartmentID]
		switch row.RoleID {
		case iam.RoleHRBP:
			contact.HRBPs = append(contact.HRBPs, row.UserName)
		case iam.RoleManager:
			contact.Managers = append(contact.Managers, row.UserName)
		case iam.RoleTrainee:
			contact.Trainees = append(contact.Trainees, row.UserName)
		}
		contacts[row.DepartmentID] = contact
	}
	for _, departmentID := range departmentIDs {
		contacts[departmentID] = normalizeContacts(contacts[departmentID])
	}
	return contacts, nil
}

func (s *SQLStore) SendRecommendation(ctx context.Context, command SendCommand) (SendResult, error) {
	return SendResult{}, ErrSendFailed
}

type resumeRow struct {
	ID             string
	NormalizedName string `gorm:"column:normalized_name"`
	Name           string
	Channel        string `gorm:"column:chan"`
	Pos            string
	Source         string
	SourceBy       string `gorm:"column:source_by"`
	Keywords       string
	Traits         string
	ExpBase        int    `gorm:"column:exp_base"`
	Expired        bool
	DepartmentID   string `gorm:"column:department_id"`
	DepartmentName string `gorm:"column:department_name"`
}

func (r resumeRow) context() (ResumeContext, error) {
	keywords, err := decodeStringSlice(r.Keywords)
	if err != nil {
		return ResumeContext{}, err
	}
	traits, err := decodeStringSlice(r.Traits)
	if err != nil {
		return ResumeContext{}, err
	}
	return ResumeContext{
		ID:             r.ID,
		NormalizedName: r.NormalizedName,
		Name:           r.Name,
		Channel:        r.Channel,
		Pos:            r.Pos,
		Source:         r.Source,
		SourceBy:       r.SourceBy,
		Department:     DepartmentSummary{ID: r.DepartmentID, Name: r.DepartmentName},
		Keywords:       keywords,
		Traits:         traits,
		ExpBase:        r.ExpBase,
		Expired:        r.Expired,
	}, nil
}

type positionRow struct {
	ID             string
	Name           string
	Channel        string `gorm:"column:chan"`
	Level          string
	Status         string
	Keywords       string
	ImplicitTags   string `gorm:"column:implicit_tags"`
	DepartmentID   string `gorm:"column:department_id"`
	DepartmentName string `gorm:"column:department_name"`
}

func (r positionRow) context() (PositionContext, error) {
	keywords, err := decodeStringSlice(r.Keywords)
	if err != nil {
		return PositionContext{}, err
	}
	implicitTags, err := decodeImplicitTags(r.ImplicitTags)
	if err != nil {
		return PositionContext{}, err
	}
	return PositionContext{
		ID:           r.ID,
		Name:         r.Name,
		Department:   DepartmentSummary{ID: r.DepartmentID, Name: r.DepartmentName},
		Channel:      r.Channel,
		Level:        r.Level,
		Status:       r.Status,
		Keywords:     keywords,
		ImplicitTags: implicitTags,
	}, nil
}

func resumeScopeWhere(scope iam.ScopePredicate) (string, []any) {
	var clauses []string
	var args []any
	for _, branch := range scope.Branches {
		var parts []string
		if !branch.AllDepartments {
			if len(branch.DepartmentIDs) == 0 {
				continue
			}
			parts = append(parts, "department_resumes.department_id IN ?")
			args = append(args, branch.DepartmentIDs)
		}
		if len(branch.Channels) > 0 {
			parts = append(parts, "resumes.chan IN ?")
			args = append(args, branch.Channels)
		}
		if len(branch.Expired) > 0 {
			parts = append(parts, "resumes.expired IN ?")
			args = append(args, branch.Expired)
		}
		if len(parts) == 0 {
			parts = append(parts, "1 = 1")
		}
		clauses = append(clauses, "("+strings.Join(parts, " AND ")+")")
	}
	if len(clauses) == 0 {
		return "1 = 0", nil
	}
	return strings.Join(clauses, " OR "), args
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

func decodeStringSlice(raw string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}

func decodeImplicitTags(raw string) ([]matching.MatchingImplicitTag, error) {
	if raw == "" {
		return []matching.MatchingImplicitTag{}, nil
	}
	var values []matching.MatchingImplicitTag
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []matching.MatchingImplicitTag{}, nil
	}
	return values, nil
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
