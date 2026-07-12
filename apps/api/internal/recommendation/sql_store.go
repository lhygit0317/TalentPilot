package recommendation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
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
	var result SendResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		source, err := getSourceResumeForSend(ctx, tx, command.ResumeID, command.ResumeGetScope)
		if err != nil {
			return err
		}
		target, err := getTargetPositionForSend(ctx, tx, command.PositionID)
		if err != nil {
			return err
		}
		if target.Status != "on" {
			return ErrTargetPositionOffline
		}
		if target.DepartmentID != command.DepartmentID {
			return ErrTargetPositionMismatch
		}
		if target.Channel != source.Channel {
			return ErrChannelMismatch
		}
		if !scopeAllowsResumeTarget(command.ResumeCreateScope, command.DepartmentID, source.Channel, source.Expired) ||
			!scopeAllowsDepartment(command.DepartmentResumeCreateScope, command.DepartmentID) ||
			!scopeAllowsDepartment(command.PositionResumeCreateScope, command.DepartmentID) {
			return ErrSendFailed
		}

		finalResumeID, reused, err := upsertRecommendedResumeCopy(ctx, tx, command, source)
		if err != nil {
			return err
		}
		score := matching.Calculate(matching.MatchInput{
			ResumeKeywords:       source.Keywords,
			ResumeTraits:         source.Traits,
			ExperienceBase:       source.ExpBase,
			PositionKeywords:     target.Keywords,
			PositionImplicitTags: target.ImplicitTags,
		}).Score.Total
		if err := upsertRecommendedRelation(ctx, tx, finalResumeID, target.ID, command.ActorUserID, score); err != nil {
			return err
		}

		result = SendResult{
			ResumeID:           finalResumeID,
			SourceResumeID:     source.ID,
			Department:         DepartmentSummary{ID: target.DepartmentID, Name: target.DepartmentName},
			Position:           PositionSummary{ID: target.ID, Name: target.Name, Channel: target.Channel, Level: target.Level},
			CandidateName:      source.Name,
			ReusedExistingCopy: reused,
		}
		return nil
	})
	if err != nil {
		return SendResult{}, err
	}

	notified, selfRead, err := s.createRecommendationNotifications(ctx, command, result)
	result.NotifiedCount = notified
	result.SelfNotificationRead = selfRead
	if err != nil {
		result.NotificationFailed = true
		result.NotificationErrorCode = "RECOMMENDATION_NOTIFICATION_FAILED"
	}
	result.Message = fmt.Sprintf("已推荐到「%s」· 已通知 %d 位相关人员", result.Department.Name, result.NotifiedCount)
	return result, nil
}

func getSourceResumeForSend(ctx context.Context, tx *gorm.DB, resumeID string, scope iam.ScopePredicate) (sourceResumeRow, error) {
	where, args := resumeScopeWhere(scope)
	var row sourceResumeRow
	if err := tx.WithContext(ctx).
		Table("resumes").
		Select(`
			resumes.id,
			resumes.normalized_name,
			resumes.name,
			resumes.age,
			resumes.school,
			resumes.years_exp,
			resumes.pos,
			resumes.chan,
			resumes.expired,
			resumes.keywords,
			resumes.traits,
			resumes.exp_base,
			resumes.profile
		`).
		Joins("JOIN department_resumes ON department_resumes.resume_id = resumes.id").
		Where(where, args...).
		Where("resumes.id = ?", resumeID).
		Limit(1).
		Scan(&row).Error; err != nil {
		return sourceResumeRow{}, err
	}
	if row.ID == "" {
		return sourceResumeRow{}, ErrResumeNotFound
	}
	if err := row.decode(); err != nil {
		return sourceResumeRow{}, err
	}
	return row, nil
}

func getTargetPositionForSend(ctx context.Context, tx *gorm.DB, positionID string) (targetPositionRow, error) {
	var row targetPositionRow
	if err := tx.WithContext(ctx).
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
		Where("positions.id = ?", positionID).
		Limit(1).
		Scan(&row).Error; err != nil {
		return targetPositionRow{}, err
	}
	if row.ID == "" {
		return targetPositionRow{}, ErrTargetPositionMismatch
	}
	if err := row.decode(); err != nil {
		return targetPositionRow{}, err
	}
	return row, nil
}

func upsertRecommendedResumeCopy(ctx context.Context, tx *gorm.DB, command SendCommand, source sourceResumeRow) (string, bool, error) {
	var existing struct {
		ID string
	}
	if err := tx.WithContext(ctx).Raw(`
		SELECT resumes.id
		FROM resumes
		JOIN department_resumes ON department_resumes.resume_id = resumes.id
		WHERE resumes.normalized_name = ? AND department_resumes.department_id = ?
		ORDER BY resumes.created_at ASC, resumes.id ASC
		LIMIT 1
	`, source.NormalizedName, command.DepartmentID).Scan(&existing).Error; err != nil {
		return "", false, err
	}
	if existing.ID != "" {
		err := tx.WithContext(ctx).Exec(`
			UPDATE resumes
			SET source_by = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, command.ActorName, existing.ID).Error
		return existing.ID, true, err
	}

	resumeID := newID("resume")
	if err := tx.WithContext(ctx).Exec(`
		INSERT INTO resumes (id, normalized_name, name, age, school, years_exp, pos, source, source_by, chan, expired, keywords, traits, exp_base, profile, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, '推荐', ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, resumeID, source.NormalizedName, source.Name, source.Age, source.School, source.YearsExp, source.Pos, command.ActorName, source.Channel, source.Expired, source.RawKeywords, source.RawTraits, source.ExpBase, source.Profile).Error; err != nil {
		return "", false, err
	}
	if err := tx.WithContext(ctx).Exec(`
		INSERT INTO department_resumes (id, department_id, resume_id, assigned_at, by_user_id)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?)
	`, newID("department_resume"), command.DepartmentID, resumeID, command.ActorUserID).Error; err != nil {
		return "", false, err
	}
	return resumeID, false, nil
}

func upsertRecommendedRelation(ctx context.Context, tx *gorm.DB, resumeID string, positionID string, actorUserID string, score int) error {
	return tx.WithContext(ctx).Exec(`
		INSERT INTO position_resumes (id, position_id, resume_id, kind, match_score, created_at, by_user_id)
		VALUES (?, ?, ?, 'recommended', ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT (resume_id, position_id, kind) DO UPDATE SET
			match_score = excluded.match_score,
			created_at = CURRENT_TIMESTAMP,
			by_user_id = excluded.by_user_id
	`, newID("position_resume"), positionID, resumeID, score, actorUserID).Error
}

func (s *SQLStore) createRecommendationNotifications(ctx context.Context, command SendCommand, result SendResult) (int, bool, error) {
	if !scopeAllowsDepartment(command.NotificationCreateScope, command.DepartmentID) {
		return 0, false, ErrSendFailed
	}
	recipients, err := s.notificationRecipients(ctx, command.DepartmentID, result.Position.Channel, false)
	if err != nil {
		return 0, false, err
	}
	if len(recipients) == 0 {
		return 0, false, nil
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, userID := range recipients {
			read := userID == command.ActorUserID
			if err := tx.Exec(`
				INSERT INTO notifications (id, to_user_id, resume_id, department_id, position_id, name, by_user_id, chan, time, read)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
			`, newID("notification"), userID, result.ResumeID, command.DepartmentID, command.PositionID, result.CandidateName, command.ActorUserID, result.Position.Channel, read).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return len(recipients), containsString(recipients, command.ActorUserID), err
}

func (s *SQLStore) notificationRecipients(ctx context.Context, departmentID string, channel string, expired bool) ([]string, error) {
	candidateIDs, err := s.notificationCandidateUserIDs(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	recipients := make([]string, 0, len(candidateIDs))
	for _, userID := range candidateIDs {
		principal, err := s.loadPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		if principalAllowsResume(principal, iam.ActionList, departmentID, channel, expired) ||
			principalAllowsResume(principal, iam.ActionGet, departmentID, channel, expired) {
			recipients = append(recipients, userID)
		}
	}
	sort.Strings(recipients)
	return recipients, nil
}

func (s *SQLStore) notificationCandidateUserIDs(ctx context.Context, departmentID string) ([]string, error) {
	var rows []struct {
		UserID string `gorm:"column:user_id"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT user_id
		FROM user_department_roles
		WHERE department_id IN (?, ?)
		ORDER BY user_id ASC
	`, departmentID, iam.SystemDepartmentID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	return ids, nil
}

func (s *SQLStore) loadPrincipal(ctx context.Context, userID string) (iam.Principal, error) {
	var user iam.User
	if err := s.db.WithContext(ctx).Raw("SELECT id, employee_id, name FROM users WHERE id = ?", userID).Scan(&user).Error; err != nil {
		return iam.Principal{}, err
	}
	var departments []iam.Department
	if err := s.db.WithContext(ctx).Raw("SELECT id, name FROM departments").Scan(&departments).Error; err != nil {
		return iam.Principal{}, err
	}
	var roles []iam.Role
	if err := s.db.WithContext(ctx).Raw("SELECT id, label, description, is_system, enabled FROM roles").Scan(&roles).Error; err != nil {
		return iam.Principal{}, err
	}
	permissions, err := s.loadPermissions(ctx)
	if err != nil {
		return iam.Principal{}, err
	}
	var relations []iam.RoleRelation
	if err := s.db.WithContext(ctx).Raw("SELECT id, parent_role_id, child_role_id FROM role_relations").Scan(&relations).Error; err != nil {
		return iam.Principal{}, err
	}
	var bindings []iam.RoleBinding
	if err := s.db.WithContext(ctx).Raw("SELECT id, user_id, department_id, role_id FROM user_department_roles WHERE user_id = ?", userID).Scan(&bindings).Error; err != nil {
		return iam.Principal{}, err
	}
	return iam.ResolvePrincipalFromSnapshot(iam.Snapshot{
		User:          user,
		Departments:   departments,
		RoleBindings:  bindings,
		Roles:         roles,
		Permissions:   permissions,
		RoleRelations: relations,
	})
}

func (s *SQLStore) loadPermissions(ctx context.Context) ([]iam.PermissionGrant, error) {
	var rows []struct {
		RoleID              string `gorm:"column:role_id"`
		Resource            string
		Action              string
		AttributeConditions string `gorm:"column:attribute_conditions"`
	}
	if err := s.db.WithContext(ctx).Raw("SELECT role_id, resource, action, attribute_conditions FROM permissions").Scan(&rows).Error; err != nil {
		return nil, err
	}
	grants := make([]iam.PermissionGrant, 0, len(rows))
	for _, row := range rows {
		var conditions iam.AttributeConditions
		if row.AttributeConditions != "" {
			if err := json.Unmarshal([]byte(row.AttributeConditions), &conditions); err != nil {
				return nil, err
			}
		}
		grants = append(grants, iam.PermissionGrant{
			RoleID:              row.RoleID,
			Resource:            iam.Resource(row.Resource),
			Action:              iam.Action(row.Action),
			AttributeConditions: conditions,
		})
	}
	return grants, nil
}

func principalAllowsResume(principal iam.Principal, action iam.Action, departmentID string, channel string, expired bool) bool {
	scope, err := iam.Scope(principal, iam.ResourceResume, action)
	if err != nil {
		return false
	}
	return scopeAllowsResumeTarget(scope, departmentID, channel, expired)
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
	ExpBase        int `gorm:"column:exp_base"`
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

type sourceResumeRow struct {
	ID             string
	NormalizedName string `gorm:"column:normalized_name"`
	Name           string
	Age            *int
	School         string
	YearsExp       *float64 `gorm:"column:years_exp"`
	Pos            string
	Channel        string `gorm:"column:chan"`
	Expired        bool
	RawKeywords    string `gorm:"column:keywords"`
	RawTraits      string `gorm:"column:traits"`
	ExpBase        int    `gorm:"column:exp_base"`
	Profile        string
	Keywords       []string `gorm:"-"`
	Traits         []string `gorm:"-"`
}

func (r *sourceResumeRow) decode() error {
	keywords, err := decodeStringSlice(r.RawKeywords)
	if err != nil {
		return err
	}
	traits, err := decodeStringSlice(r.RawTraits)
	if err != nil {
		return err
	}
	r.Keywords = keywords
	r.Traits = traits
	return nil
}

type targetPositionRow struct {
	ID             string
	Name           string
	Channel        string `gorm:"column:chan"`
	Level          string
	Status         string
	RawKeywords    string                         `gorm:"column:keywords"`
	RawImplicit    string                         `gorm:"column:implicit_tags"`
	DepartmentID   string                         `gorm:"column:department_id"`
	DepartmentName string                         `gorm:"column:department_name"`
	Keywords       []string                       `gorm:"-"`
	ImplicitTags   []matching.MatchingImplicitTag `gorm:"-"`
}

func (r *targetPositionRow) decode() error {
	keywords, err := decodeStringSlice(r.RawKeywords)
	if err != nil {
		return err
	}
	implicitTags, err := decodeImplicitTags(r.RawImplicit)
	if err != nil {
		return err
	}
	r.Keywords = keywords
	r.ImplicitTags = implicitTags
	return nil
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

func scopeAllowsDepartment(scope iam.ScopePredicate, departmentID string) bool {
	if departmentID == "" {
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

func scopeAllowsResumeTarget(scope iam.ScopePredicate, departmentID string, channel string, expired bool) bool {
	for _, branch := range scope.Branches {
		if !branch.AllDepartments && !containsString(branch.DepartmentIDs, departmentID) {
			continue
		}
		if len(branch.Channels) > 0 && !containsString(branch.Channels, channel) {
			continue
		}
		if len(branch.Expired) > 0 && !containsBool(branch.Expired, expired) {
			continue
		}
		return true
	}
	return false
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

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
