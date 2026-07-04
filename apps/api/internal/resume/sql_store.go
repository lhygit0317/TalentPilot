package resume

import (
	"context"
	"encoding/json"
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

func (s *SQLStore) List(ctx context.Context, query ListQuery) (ListResult, error) {
	counts, err := s.channelCounts(ctx, query.Scope)
	if err != nil {
		return ListResult{}, err
	}
	selectedChannel := query.Channel
	if selectedChannel == "" {
		selectedChannel = firstAvailableChannel(counts)
	}

	rowsQuery := s.scopedRows(ctx, query.Scope)
	if selectedChannel != "" {
		rowsQuery = rowsQuery.Where("resumes.chan = ?", selectedChannel)
	}
	if query.Search != "" {
		like := "%" + escapeLike(strings.ToLower(query.Search)) + "%"
		rowsQuery = rowsQuery.Where(
			`(lower(resumes.name) LIKE ? ESCAPE '\' OR lower(resumes.pos) LIKE ? ESCAPE '\' OR lower(resumes.keywords) LIKE ? ESCAPE '\')`,
			like,
			like,
			like,
		)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var rows []resumeRow
	if err := rowsQuery.Order("resumes.created_at DESC, resumes.id ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return ListResult{}, err
	}

	items := make([]ListItem, 0, len(rows))
	for _, row := range rows {
		item, err := row.listItem()
		if err != nil {
			return ListResult{}, err
		}
		item.CanGet = matchesScope(row, query.GetScope)
		item.CanDelete = matchesScope(row, query.DeleteScope)
		items = append(items, item)
	}

	return ListResult{
		Items:             items,
		ChannelCounts:     counts,
		AvailableChannels: availableChannels(counts),
		DataScopeSummary:  dataScopeSummary(query.Scope),
	}, nil
}

func (s *SQLStore) Get(ctx context.Context, id string, scope iam.ScopePredicate) (Detail, error) {
	var row resumeRow
	if err := s.scopedRows(ctx, scope).Where("resumes.id = ?", id).Limit(1).Scan(&row).Error; err != nil {
		return Detail{}, err
	}
	if row.ID == "" {
		return Detail{}, ErrNotFound
	}
	item, err := row.listItem()
	if err != nil {
		return Detail{}, err
	}
	profile, err := decodeProfile(row.Profile)
	if err != nil {
		return Detail{}, err
	}
	return Detail{ListItem: item, CreatedAt: row.CreatedAt, Expired: row.Expired, Profile: profile}, nil
}

func (s *SQLStore) Delete(ctx context.Context, id string, scope iam.ScopePredicate) error {
	if _, err := s.Get(ctx, id, scope); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Exec("DELETE FROM resumes WHERE id = ?", id).Error
}

func (s *SQLStore) channelCounts(ctx context.Context, scope iam.ScopePredicate) (map[Channel]int, error) {
	counts := map[Channel]int{ChannelSocial: 0, ChannelCampus: 0}
	var rows []struct {
		Channel string `gorm:"column:channel"`
		Count   int    `gorm:"column:count"`
	}
	if err := s.scopedRows(ctx, scope).
		Select("resumes.chan AS channel, COUNT(*) AS count").
		Group("resumes.chan").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[Channel(row.Channel)] = row.Count
	}
	return counts, nil
}

func (s *SQLStore) scopedRows(ctx context.Context, scope iam.ScopePredicate) *gorm.DB {
	query := s.resumeRows(ctx)
	where, args := scopeWhere(scope)
	return query.Where(where, args...)
}

func (s *SQLStore) resumeRows(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).
		Table("resumes").
		Select(`
			resumes.id,
			resumes.name,
			resumes.age,
			resumes.school,
			resumes.years_exp,
			resumes.pos,
			resumes.source,
			resumes.source_by,
			resumes.chan,
			resumes.expired,
			resumes.keywords,
			resumes.profile,
			resumes.created_at,
			department_resumes.department_id,
			departments.name AS department_name
		`).
		Joins("JOIN department_resumes ON department_resumes.resume_id = resumes.id").
		Joins("JOIN departments ON departments.id = department_resumes.department_id")
}

func scopeWhere(scope iam.ScopePredicate) (string, []any) {
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

func matchesScope(row resumeRow, scope iam.ScopePredicate) bool {
	for _, branch := range scope.Branches {
		if !branch.AllDepartments && !containsString(branch.DepartmentIDs, row.DepartmentID) {
			continue
		}
		if len(branch.Channels) > 0 && !containsString(branch.Channels, row.Channel) {
			continue
		}
		if len(branch.Expired) > 0 && !containsBool(branch.Expired, row.Expired) {
			continue
		}
		return true
	}
	return false
}

func firstAvailableChannel(counts map[Channel]int) Channel {
	for _, channel := range []Channel{ChannelSocial, ChannelCampus} {
		if counts[channel] > 0 {
			return channel
		}
	}
	return ""
}

func availableChannels(counts map[Channel]int) []Channel {
	var channels []Channel
	for _, channel := range []Channel{ChannelSocial, ChannelCampus} {
		if counts[channel] > 0 {
			channels = append(channels, channel)
		}
	}
	return channels
}

func dataScopeSummary(scope iam.ScopePredicate) string {
	if len(scope.Branches) == 0 {
		return ""
	}
	for _, branch := range scope.Branches {
		if branch.AllDepartments {
			return "全部部门"
		}
	}
	var departmentIDs []string
	seen := map[string]bool{}
	for _, branch := range scope.Branches {
		for _, departmentID := range branch.DepartmentIDs {
			if !seen[departmentID] {
				departmentIDs = append(departmentIDs, departmentID)
				seen[departmentID] = true
			}
		}
	}
	return strings.Join(departmentIDs, "、")
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

func containsBool(values []bool, expected bool) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

type resumeRow struct {
	ID             string
	Name           string
	Age            *int
	School         string
	YearsExp       *float64 `gorm:"column:years_exp"`
	Pos            string
	Source         string
	SourceBy       string `gorm:"column:source_by"`
	Channel        string `gorm:"column:chan"`
	Expired        bool
	Keywords       string
	Profile        string
	CreatedAt      time.Time `gorm:"column:created_at"`
	DepartmentID   string    `gorm:"column:department_id"`
	DepartmentName string    `gorm:"column:department_name"`
}

func (r resumeRow) listItem() (ListItem, error) {
	keywords, err := decodeStringSlice(r.Keywords)
	if err != nil {
		return ListItem{}, err
	}
	return ListItem{
		ID:                r.ID,
		Name:              r.Name,
		Age:               r.Age,
		School:            r.School,
		YearsExp:          r.YearsExp,
		CurrentDepartment: DepartmentSummary{ID: r.DepartmentID, Name: r.DepartmentName},
		Pos:               r.Pos,
		Source:            Source(r.Source),
		SourceBy:          r.SourceBy,
		Channel:           Channel(r.Channel),
		Keywords:          keywords,
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

func decodeProfile(raw string) (Profile, error) {
	profile := Profile{
		Basic:          map[string]any{},
		Education:      []map[string]any{},
		WorkExperience: []map[string]any{},
		Projects:       []map[string]any{},
		Skills:         []string{},
		Certificates:   []string{},
	}
	if raw == "" {
		return profile, nil
	}
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return Profile{}, err
	}
	if profile.Basic == nil {
		profile.Basic = map[string]any{}
	}
	if profile.Education == nil {
		profile.Education = []map[string]any{}
	}
	if profile.WorkExperience == nil {
		profile.WorkExperience = []map[string]any{}
	}
	if profile.Projects == nil {
		profile.Projects = []map[string]any{}
	}
	if profile.Skills == nil {
		profile.Skills = []string{}
	}
	if profile.Certificates == nil {
		profile.Certificates = []string{}
	}
	return profile, nil
}
