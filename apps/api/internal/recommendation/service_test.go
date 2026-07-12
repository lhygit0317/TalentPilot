package recommendation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
	"github.com/talentpilot/talentpilot/apps/api/internal/recommendation"
)

func TestServiceRouteGroupsByDepartmentAndMarksBest(t *testing.T) {
	store := &fakeRecommendationStore{
		resume: recommendation.ResumeContext{
			ID:             "resume_1",
			Name:           "张三",
			Channel:        "social",
			Pos:            "平台工程师",
			Keywords:       []string{"Go"},
			Traits:         []string{"稳定"},
			ExpBase:        88,
			Department:     recommendation.RecommendationDepartmentSummary{ID: "dept_source", Name: "来源部门"},
			NormalizedName: "zhangsan",
		},
		positions: []recommendation.PositionContext{
			{ID: "position_low", Name: "低分岗位", Channel: "social", Status: "on", Department: recommendation.RecommendationDepartmentSummary{ID: "dept_a", Name: "A 部门"}, Keywords: []string{"Rust"}},
			{ID: "position_high", Name: "高分岗位", Channel: "social", Status: "on", Department: recommendation.RecommendationDepartmentSummary{ID: "dept_a", Name: "A 部门"}, Keywords: []string{"Go"}, ImplicitTags: []matching.MatchingImplicitTag{{Name: "稳定", Weight: 40}}},
			{ID: "position_b", Name: "B 岗位", Channel: "social", Status: "on", Department: recommendation.RecommendationDepartmentSummary{ID: "dept_b", Name: "B 部门"}, Keywords: []string{"Go"}},
		},
		contacts: map[string]recommendation.RecommendationDepartmentContacts{
			"dept_a": {HRBPs: []string{"李四"}, Managers: []string{"王五"}, Trainees: []string{"赵六"}},
			"dept_b": {HRBPs: []string{"孙七"}},
		},
	}
	service := recommendation.NewService(store, audit.NopRecorder{})

	result, err := service.Route(context.Background(), recommendation.RouteInput{
		ActorUserID:   "user_1",
		ResumeID:      "resume_1",
		ResumeScope:   iam.ScopePredicate{Branches: []iam.ScopeBranch{{AllDepartments: true}}},
		PositionScope: iam.ScopePredicate{Branches: []iam.ScopeBranch{{AllDepartments: true}}},
	})
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if len(result.Routes) != 2 {
		t.Fatalf("expected 2 department routes, got %#v", result.Routes)
	}
	if result.Routes[0].Position.ID != "position_high" || !result.Routes[0].Best {
		t.Fatalf("expected highest A position as best, got %#v", result.Routes[0])
	}
	if result.Routes[0].Contacts.HRBPs[0] != "李四" || result.Routes[0].Contacts.Managers[0] != "王五" {
		t.Fatalf("expected contacts on best route, got %#v", result.Routes[0].Contacts)
	}
	if result.Routes[1].Best {
		t.Fatalf("only first route should be best: %#v", result.Routes)
	}
	if store.routeQuery.Channel != "social" {
		t.Fatalf("expected source resume channel passed into route query, got %#v", store.routeQuery)
	}
}

func TestServiceSendReturnsTargetValidationErrors(t *testing.T) {
	store := &fakeRecommendationStore{sendErr: recommendation.ErrTargetPositionOffline}
	service := recommendation.NewService(store, audit.NopRecorder{})

	_, err := service.Send(context.Background(), recommendation.SendInput{
		ActorUserID:  "user_1",
		ResumeID:     "resume_1",
		DepartmentID: "dept_a",
		PositionID:   "position_off",
	})
	if !errors.Is(err, recommendation.ErrTargetPositionOffline) {
		t.Fatalf("expected offline error, got %v", err)
	}
	if store.sendCommand.PositionID != "position_off" {
		t.Fatalf("expected send command to reach store, got %#v", store.sendCommand)
	}
}

func TestServiceSendRecordsRecommendationAudit(t *testing.T) {
	recorder := &recordingRecommendationAudit{}
	store := &fakeRecommendationStore{sendResult: recommendation.SendResult{
		ResumeID:           "resume_copy",
		SourceResumeID:     "resume_source",
		Department:         recommendation.RecommendationDepartmentSummary{ID: "dept_a", Name: "智算调度部"},
		Position:           recommendation.RecommendationPositionSummary{ID: "position_a", Name: "调度工程师"},
		CandidateName:      "张三",
		NotifiedCount:      3,
		ReusedExistingCopy: true,
	}}
	service := recommendation.NewService(store, recorder)

	_, err := service.Send(context.Background(), recommendation.SendInput{
		ActorUserID:  "user_1",
		ResumeID:     "resume_source",
		DepartmentID: "dept_a",
		PositionID:   "position_a",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(recorder.events) != 1 || recorder.events[0].Type != audit.EventRecommendationSent {
		t.Fatalf("expected recommendation audit event, got %#v", recorder.events)
	}
	if recorder.events[0].After["finalResumeId"] != "resume_copy" || recorder.events[0].After["notifiedCount"] != 3 {
		t.Fatalf("unexpected audit payload: %#v", recorder.events[0])
	}
}

type fakeRecommendationStore struct {
	resume      recommendation.ResumeContext
	positions   []recommendation.PositionContext
	contacts    map[string]recommendation.RecommendationDepartmentContacts
	routeQuery  recommendation.RoutePositionQuery
	sendCommand recommendation.SendCommand
	sendResult  recommendation.SendResult
	sendErr     error
}

func (f *fakeRecommendationStore) GetResume(ctx context.Context, resumeID string, scope iam.ScopePredicate) (recommendation.ResumeContext, error) {
	return f.resume, nil
}

func (f *fakeRecommendationStore) ListRoutePositions(ctx context.Context, query recommendation.RoutePositionQuery) ([]recommendation.PositionContext, error) {
	f.routeQuery = query
	return f.positions, nil
}

func (f *fakeRecommendationStore) ListDepartmentContacts(ctx context.Context, departmentIDs []string) (map[string]recommendation.RecommendationDepartmentContacts, error) {
	return f.contacts, nil
}

func (f *fakeRecommendationStore) SendRecommendation(ctx context.Context, command recommendation.SendCommand) (recommendation.SendResult, error) {
	f.sendCommand = command
	if f.sendErr != nil {
		return recommendation.SendResult{}, f.sendErr
	}
	if f.sendResult.ResumeID != "" {
		return f.sendResult, nil
	}
	return recommendation.SendResult{ResumeID: "resume_copy"}, nil
}

type recordingRecommendationAudit struct {
	events []audit.Event
}

func (r *recordingRecommendationAudit) Record(ctx context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}
