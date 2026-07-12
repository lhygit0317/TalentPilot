package recommendation

import (
	"context"
	"sort"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
)

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

func (s *Service) Route(ctx context.Context, input RouteInput) (RouteResult, error) {
	resume, err := s.store.GetResume(ctx, input.ResumeID, input.ResumeScope)
	if err != nil {
		return RouteResult{}, err
	}
	positions, err := s.store.ListRoutePositions(ctx, RoutePositionQuery{Channel: resume.Channel, Scope: input.PositionScope})
	if err != nil {
		return RouteResult{}, err
	}

	bestByDepartment := map[string]RouteRow{}
	for _, position := range positions {
		calculation := matching.Calculate(matching.MatchInput{
			ResumeKeywords:       resume.Keywords,
			ResumeTraits:         resume.Traits,
			ExperienceBase:       resume.ExpBase,
			PositionKeywords:     position.Keywords,
			PositionImplicitTags: position.ImplicitTags,
		})
		row := RouteRow{
			Department: position.Department,
			Position: PositionSummary{
				ID:      position.ID,
				Name:    position.Name,
				Channel: position.Channel,
				Level:   position.Level,
			},
			Score: calculation.Score,
		}
		current, ok := bestByDepartment[position.Department.ID]
		if !ok || routeLess(row, current) {
			bestByDepartment[position.Department.ID] = row
		}
	}

	routes := make([]RouteRow, 0, len(bestByDepartment))
	departmentIDs := make([]string, 0, len(bestByDepartment))
	for departmentID, row := range bestByDepartment {
		departmentIDs = append(departmentIDs, departmentID)
		routes = append(routes, row)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routeLess(routes[i], routes[j])
	})

	contacts, err := s.store.ListDepartmentContacts(ctx, departmentIDs)
	if err != nil {
		return RouteResult{}, err
	}
	for i := range routes {
		routes[i].Contacts = normalizeContacts(contacts[routes[i].Department.ID])
		routes[i].Best = i == 0
	}

	return RouteResult{
		Resume: ResumeSummary{
			ID:                resume.ID,
			Name:              resume.Name,
			Channel:           resume.Channel,
			Pos:               resume.Pos,
			CurrentDepartment: resume.Department,
			Keywords:          append([]string(nil), resume.Keywords...),
		},
		Routes:    routes,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (s *Service) Send(ctx context.Context, input SendInput) (SendResult, error) {
	return s.store.SendRecommendation(ctx, SendCommand(input))
}

func routeLess(left RouteRow, right RouteRow) bool {
	if left.Score.Total != right.Score.Total {
		return left.Score.Total > right.Score.Total
	}
	if left.Department.Name != right.Department.Name {
		return left.Department.Name < right.Department.Name
	}
	return left.Position.Name < right.Position.Name
}

func normalizeContacts(contacts DepartmentContacts) DepartmentContacts {
	if contacts.HRBPs == nil {
		contacts.HRBPs = []string{}
	}
	if contacts.Managers == nil {
		contacts.Managers = []string{}
	}
	if contacts.Trainees == nil {
		contacts.Trainees = []string{}
	}
	return contacts
}
