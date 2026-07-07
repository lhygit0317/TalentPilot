package matching

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

func (s *SQLStore) GetResume(ctx context.Context, resumeID string, scope iam.ScopePredicate) (ResumeContext, error) {
	where, args := resumeScopeWhere(scope)
	var row resumeContextRow
	if err := s.db.WithContext(ctx).
		Table("resumes").
		Select(`
			resumes.id,
			resumes.name,
			resumes.chan,
			resumes.pos,
			resumes.source,
			resumes.source_by,
			resumes.keywords,
			resumes.traits,
			resumes.exp_base,
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

func (s *SQLStore) GetPosition(ctx context.Context, positionID string, scope iam.ScopePredicate) (PositionContext, error) {
	where, args := departmentScopeWhere(scope, "primary_department_positions.department_id")
	var row positionContextRow
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
			primary_department_positions.department_id,
			departments.name AS department_name
		`).
		Joins(`
			JOIN (
				SELECT position_id, MIN(department_id) AS department_id
				FROM department_positions
				GROUP BY position_id
			) primary_department_positions ON primary_department_positions.position_id = positions.id
		`).
		Joins("JOIN departments ON departments.id = primary_department_positions.department_id").
		Where(where, args...).
		Where("positions.id = ?", positionID).
		Limit(1).
		Scan(&row).Error; err != nil {
		return PositionContext{}, err
	}
	if row.ID == "" {
		return PositionContext{}, ErrPositionNotFound
	}
	return row.context()
}

func (s *SQLStore) UpsertParsedRelation(ctx context.Context, input ParsedRelationInput) (ParsedRelation, error) {
	relationID := newID("position_resume")
	if err := s.db.WithContext(ctx).Exec(`
		INSERT INTO position_resumes (id, position_id, resume_id, kind, match_score, created_at, by_user_id)
		VALUES (?, ?, ?, 'parsed', ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT (resume_id, position_id, kind) DO UPDATE SET
			match_score = excluded.match_score,
			created_at = CURRENT_TIMESTAMP,
			by_user_id = excluded.by_user_id
	`, relationID, input.PositionID, input.ResumeID, input.MatchScore, input.ActorUserID).Error; err != nil {
		return ParsedRelation{}, err
	}

	var relation ParsedRelation
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, resume_id, position_id, match_score, created_at
		FROM position_resumes
		WHERE resume_id = ? AND position_id = ? AND kind = 'parsed'
	`, input.ResumeID, input.PositionID).Scan(&relation).Error; err != nil {
		return ParsedRelation{}, err
	}
	return relation, nil
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

type resumeContextRow struct {
	ID             string
	Name           string
	Channel        string `gorm:"column:chan"`
	Pos            string
	Source         string
	SourceBy       string `gorm:"column:source_by"`
	Keywords       string
	Traits         string
	ExpBase        int    `gorm:"column:exp_base"`
	DepartmentID   string `gorm:"column:department_id"`
	DepartmentName string `gorm:"column:department_name"`
}

func (r resumeContextRow) context() (ResumeContext, error) {
	keywords, err := decodeStringSlice(r.Keywords)
	if err != nil {
		return ResumeContext{}, err
	}
	traits, err := decodeStringSlice(r.Traits)
	if err != nil {
		return ResumeContext{}, err
	}
	return ResumeContext{
		ID:                r.ID,
		Name:              r.Name,
		Channel:           r.Channel,
		Pos:               r.Pos,
		Source:            r.Source,
		SourceBy:          r.SourceBy,
		CurrentDepartment: MatchingDepartmentSummary{ID: r.DepartmentID, Name: r.DepartmentName},
		Keywords:          keywords,
		Traits:            traits,
		ExpBase:           r.ExpBase,
	}, nil
}

type positionContextRow struct {
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

func (r positionContextRow) context() (PositionContext, error) {
	keywords, err := decodeStringSlice(r.Keywords)
	if err != nil {
		return PositionContext{}, err
	}
	implicitTags, err := decodeMatchingImplicitTags(r.ImplicitTags)
	if err != nil {
		return PositionContext{}, err
	}
	return PositionContext{
		ID:           r.ID,
		Name:         r.Name,
		Department:   MatchingDepartmentSummary{ID: r.DepartmentID, Name: r.DepartmentName},
		Channel:      r.Channel,
		Level:        r.Level,
		Status:       r.Status,
		Keywords:     keywords,
		ImplicitTags: implicitTags,
	}, nil
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

func decodeMatchingImplicitTags(raw string) ([]MatchingImplicitTag, error) {
	if raw == "" {
		return []MatchingImplicitTag{}, nil
	}
	var values []MatchingImplicitTag
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []MatchingImplicitTag{}, nil
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
