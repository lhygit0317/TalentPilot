package organization

import (
	"context"
	"strings"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

type Store interface {
	ListDepartments(context.Context, DepartmentListQuery) (DepartmentListResult, error)
	GetDepartment(context.Context, string, iam.ScopePredicate) (DepartmentDetail, error)
	CreateDepartment(context.Context, string) (DepartmentDetail, error)
	UpdateDepartment(context.Context, string, string, iam.ScopePredicate) (DepartmentDetail, error)
	DeleteDepartment(context.Context, string, iam.ScopePredicate) (DepartmentDetail, error)
	ListPositions(context.Context, PositionListQuery) (PositionListResult, error)
	GetPosition(context.Context, string, iam.ScopePredicate) (PositionDetail, error)
	CreatePosition(context.Context, normalizedPositionInput) (PositionDetail, error)
	UpdatePosition(context.Context, string, normalizedPositionInput, iam.ScopePredicate) (PositionDetail, error)
	DeletePosition(context.Context, string, iam.ScopePredicate) (PositionDetail, error)
}

type Service struct {
	store Store
	audit audit.Recorder
}

func NewService(store Store, recorder audit.Recorder) *Service {
	if recorder == nil {
		recorder = audit.NopRecorder{}
	}
	return &Service{store: store, audit: recorder}
}

func (s *Service) ListDepartments(ctx context.Context, query DepartmentListQuery) (DepartmentListResult, error) {
	return s.store.ListDepartments(ctx, query)
}

func (s *Service) GetDepartment(ctx context.Context, id string, scope iam.ScopePredicate) (DepartmentDetail, error) {
	return s.store.GetDepartment(ctx, id, scope)
}

func (s *Service) CreateDepartment(ctx context.Context, input DepartmentInput) (DepartmentDetail, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return DepartmentDetail{}, ErrDepartmentNameRequired
	}
	department, err := s.store.CreateDepartment(ctx, name)
	if err != nil {
		return DepartmentDetail{}, err
	}
	s.recordAudit(ctx, audit.Event{
		Type:        audit.EventDepartmentCreated,
		UserID:      input.ActorUserID,
		ActorUserID: input.ActorUserID,
		Resource:    string(iam.ResourceDepartment),
		Action:      string(iam.ActionCreate),
		TargetID:    department.ID,
		Result:      "succeeded",
		After:       map[string]any{"departmentId": department.ID, "name": department.Name},
	})
	return department, nil
}

func (s *Service) UpdateDepartment(ctx context.Context, id string, input DepartmentInput, scope iam.ScopePredicate) (DepartmentDetail, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return DepartmentDetail{}, ErrDepartmentNameRequired
	}
	department, err := s.store.UpdateDepartment(ctx, id, name, scope)
	if err != nil {
		return DepartmentDetail{}, err
	}
	s.recordAudit(ctx, audit.Event{
		Type:        audit.EventDepartmentUpdated,
		UserID:      input.ActorUserID,
		ActorUserID: input.ActorUserID,
		Resource:    string(iam.ResourceDepartment),
		Action:      string(iam.ActionUpdate),
		TargetID:    department.ID,
		Result:      "succeeded",
		After:       map[string]any{"departmentId": department.ID, "name": department.Name},
	})
	return department, nil
}

func (s *Service) DeleteDepartment(ctx context.Context, id string, scope iam.ScopePredicate, actorUserID string) error {
	department, err := s.store.DeleteDepartment(ctx, id, scope)
	if err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{
		Type:        audit.EventDepartmentDeleted,
		UserID:      actorUserID,
		ActorUserID: actorUserID,
		Resource:    string(iam.ResourceDepartment),
		Action:      string(iam.ActionDelete),
		TargetID:    department.ID,
		Result:      "succeeded",
		After:       map[string]any{"departmentId": department.ID, "name": department.Name},
	})
	return nil
}

func (s *Service) ListPositions(ctx context.Context, query PositionListQuery) (PositionListResult, error) {
	return s.store.ListPositions(ctx, query)
}

func (s *Service) GetPosition(ctx context.Context, id string, scope iam.ScopePredicate) (PositionDetail, error) {
	return s.store.GetPosition(ctx, id, scope)
}

func (s *Service) CreatePosition(ctx context.Context, input PositionInput) (PositionDetail, error) {
	normalized, err := normalizePositionInput(input)
	if err != nil {
		return PositionDetail{}, err
	}
	position, err := s.store.CreatePosition(ctx, normalized)
	if err != nil {
		return PositionDetail{}, err
	}
	s.recordAudit(ctx, audit.Event{
		Type:        audit.EventPositionCreated,
		UserID:      input.ActorUserID,
		ActorUserID: input.ActorUserID,
		Resource:    string(iam.ResourcePosition),
		Action:      string(iam.ActionCreate),
		TargetID:    position.ID,
		Result:      "succeeded",
		After:       positionAuditPayload(position),
	})
	return position, nil
}

func (s *Service) UpdatePosition(ctx context.Context, id string, input PositionInput, scope iam.ScopePredicate) (PositionDetail, error) {
	normalized, err := normalizePositionInput(input)
	if err != nil {
		return PositionDetail{}, err
	}
	position, err := s.store.UpdatePosition(ctx, id, normalized, scope)
	if err != nil {
		return PositionDetail{}, err
	}
	s.recordAudit(ctx, audit.Event{
		Type:        audit.EventPositionUpdated,
		UserID:      input.ActorUserID,
		ActorUserID: input.ActorUserID,
		Resource:    string(iam.ResourcePosition),
		Action:      string(iam.ActionUpdate),
		TargetID:    position.ID,
		Result:      "succeeded",
		After:       positionAuditPayload(position),
	})
	return position, nil
}

func (s *Service) DeletePosition(ctx context.Context, id string, scope iam.ScopePredicate, actorUserID string) error {
	position, err := s.store.DeletePosition(ctx, id, scope)
	if err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{
		Type:        audit.EventPositionDeleted,
		UserID:      actorUserID,
		ActorUserID: actorUserID,
		Resource:    string(iam.ResourcePosition),
		Action:      string(iam.ActionDelete),
		TargetID:    position.ID,
		Result:      "succeeded",
		After:       positionAuditPayload(position),
	})
	return nil
}

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	_ = s.audit.Record(ctx, event)
}

type normalizedPositionInput struct {
	Name         string
	DepartmentID string
	Chan         string
	Level        string
	Status       string
	Duties       []string
	Must         []string
	Keywords     []string
	ImplicitTags []ImplicitTag
}

func normalizePositionInput(input PositionInput) (normalizedPositionInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return normalizedPositionInput{}, ErrPositionNameRequired
	}
	departmentID := strings.TrimSpace(input.DepartmentID)
	if departmentID == "" {
		return normalizedPositionInput{}, ErrPositionDepartmentRequired
	}
	channel := strings.TrimSpace(input.Chan)
	if channel != "social" && channel != "campus" {
		return normalizedPositionInput{}, ErrPositionInvalidChannel
	}
	status := strings.TrimSpace(input.Status)
	if status != "on" && status != "off" {
		return normalizedPositionInput{}, ErrPositionInvalidStatus
	}
	keywords, err := normalizeKeywords(input.Keywords)
	if err != nil {
		return normalizedPositionInput{}, err
	}
	implicitTags, err := normalizeImplicitTags(input.ImplicitTags)
	if err != nil {
		return normalizedPositionInput{}, err
	}
	return normalizedPositionInput{
		Name:         name,
		DepartmentID: departmentID,
		Chan:         channel,
		Level:        strings.TrimSpace(input.Level),
		Status:       status,
		Duties:       normalizeStringSlice(input.Duties),
		Must:         normalizeStringSlice(input.Must),
		Keywords:     keywords,
		ImplicitTags: implicitTags,
	}, nil
}

func normalizeStringSlice(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}

func normalizeKeywords(values []string) ([]string, error) {
	keywords := normalizeStringSlice(values)
	seen := map[string]bool{}
	for _, keyword := range keywords {
		key := strings.ToLower(keyword)
		if seen[key] {
			return nil, ErrPositionDuplicateKeyword
		}
		seen[key] = true
	}
	return keywords, nil
}

func normalizeImplicitTags(values []ImplicitTagInput) ([]ImplicitTag, error) {
	tags := make([]ImplicitTag, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		name := strings.TrimSpace(value.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			return nil, ErrPositionDuplicateImplicitTag
		}
		seen[key] = true
		weight := 40
		if value.Weight != nil {
			weight = *value.Weight
		}
		if weight < 0 || weight > 100 {
			return nil, ErrPositionInvalidImplicitWeight
		}
		tags = append(tags, ImplicitTag{Name: name, Weight: weight})
	}
	return tags, nil
}

func positionAuditPayload(position PositionDetail) map[string]any {
	return map[string]any{
		"positionId":     position.ID,
		"name":           position.Name,
		"departmentId":   position.Department.ID,
		"departmentName": position.Department.Name,
		"chan":           position.Chan,
		"status":         position.Status,
	}
}
