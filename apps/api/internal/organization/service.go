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

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	_ = s.audit.Record(ctx, event)
}
