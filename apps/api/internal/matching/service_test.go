package matching_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
)

func TestServiceParseRejectsOffShelfPositionBeforeWrite(t *testing.T) {
	store := &fakeMatchingStore{
		resume:   matching.ResumeContext{ID: "resume_1", Keywords: []string{"Go"}, ExpBase: 80},
		position: matching.PositionContext{ID: "position_1", Status: "off", Keywords: []string{"Go"}},
	}
	service := matching.NewService(store, audit.NopRecorder{}, matching.NewRuleQuestionGenerator())

	_, err := service.Parse(context.Background(), matching.ParseInput{
		ActorUserID: "user_1",
		ResumeID:    "resume_1",
		PositionID:  "position_1",
	})
	if !errors.Is(err, matching.ErrPositionOffline) {
		t.Fatalf("expected offline error, got %v", err)
	}
	if store.upsertCalls != 0 {
		t.Fatalf("expected no parsed relation write, got %d", store.upsertCalls)
	}
}

func TestServiceParseRequiresPositionResumeCreateScope(t *testing.T) {
	store := &fakeMatchingStore{
		resume: matching.ResumeContext{ID: "resume_1", Keywords: []string{"Go"}, ExpBase: 80},
		position: matching.PositionContext{
			ID:         "position_1",
			Status:     "on",
			Department: matching.MatchingDepartmentSummary{ID: "dept_b", Name: "智算调度部"},
			Keywords:   []string{"Go"},
		},
	}
	service := matching.NewService(store, audit.NopRecorder{}, matching.NewRuleQuestionGenerator())

	_, err := service.Parse(context.Background(), matching.ParseInput{
		ActorUserID: "user_1",
		ResumeID:    "resume_1",
		PositionID:  "position_1",
		PositionResumeCreateScope: iam.ScopePredicate{
			Resource: iam.ResourcePositionResume,
			Action:   iam.ActionCreate,
			Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}},
		},
	})
	if !errors.Is(err, matching.ErrPositionResumeCreateDenied) {
		t.Fatalf("expected create scope denial, got %v", err)
	}
	if store.upsertCalls != 0 {
		t.Fatalf("expected no parsed relation write, got %d", store.upsertCalls)
	}
}

func TestServiceParseUpsertsRelationAndRecordsAudit(t *testing.T) {
	recorder := &recordingMatchingAudit{}
	store := &fakeMatchingStore{
		resume: matching.ResumeContext{
			ID:                "resume_1",
			Name:              "张三",
			Channel:           "social",
			Pos:               "平台工程师",
			Source:            "导入",
			SourceBy:          "李四",
			CurrentDepartment: matching.MatchingDepartmentSummary{ID: "dept_a", Name: "算力训练平台部"},
			Keywords:          []string{"Go", "调度"},
			Traits:            []string{"稳定"},
			ExpBase:           82,
		},
		position: matching.PositionContext{
			ID:           "position_1",
			Name:         "平台工程师",
			Department:   matching.MatchingDepartmentSummary{ID: "dept_a", Name: "算力训练平台部"},
			Channel:      "social",
			Level:        "P6",
			Status:       "on",
			Keywords:     []string{"Go", "Kubernetes"},
			ImplicitTags: []matching.MatchingImplicitTag{{Name: "稳定", Weight: 40}},
		},
		relation: matching.ParsedRelation{
			ID:         "position_resume_1",
			ResumeID:   "resume_1",
			PositionID: "position_1",
			MatchScore: 76,
			CreatedAt:  time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC),
		},
	}
	service := matching.NewService(store, recorder, matching.NewRuleQuestionGenerator())

	result, err := service.Parse(context.Background(), matching.ParseInput{
		ActorUserID: "user_1",
		ResumeID:    "resume_1",
		PositionID:  "position_1",
		PositionResumeCreateScope: iam.ScopePredicate{
			Resource: iam.ResourcePositionResume,
			Action:   iam.ActionCreate,
			Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}},
		},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if result.ID != "position_resume_1" || result.Score.Total != 76 || result.Evidence.Analysis == "" {
		t.Fatalf("unexpected parse result: %#v", result)
	}
	if store.upsertCalls != 1 || store.upsertInput.MatchScore != 76 {
		t.Fatalf("expected upsert with score 76, got calls=%d input=%#v", store.upsertCalls, store.upsertInput)
	}
	if len(recorder.events) != 1 || recorder.events[0].Type != audit.EventResumeParsed {
		t.Fatalf("expected resume parsed audit event, got %#v", recorder.events)
	}
	if strings.Contains(strings.ToLower(toAuditString(recorder.events[0])), "profile") {
		t.Fatalf("audit event leaked profile data: %#v", recorder.events[0])
	}
}

func TestServiceGenerateInterviewQuestionsBuildsThreeGroupsWithDepartmentAndHighPotentialQuestion(t *testing.T) {
	store := &fakeMatchingStore{
		resume: matching.ResumeContext{
			ID:                "resume_1",
			Name:              "张三",
			Keywords:          []string{"Go"},
			Traits:            []string{"稳定"},
			ExpBase:           82,
			Channel:           "social",
			CurrentDepartment: matching.MatchingDepartmentSummary{ID: "dept_a", Name: "算力训练平台部"},
		},
		position: matching.PositionContext{
			ID:           "position_1",
			Name:         "平台工程师",
			Department:   matching.MatchingDepartmentSummary{ID: "dept_a", Name: "算力训练平台部"},
			Status:       "on",
			Keywords:     []string{"Go"},
			ImplicitTags: []matching.MatchingImplicitTag{{Name: "稳定", Weight: 40}},
		},
	}
	service := matching.NewService(store, audit.NopRecorder{}, matching.NewRuleQuestionGenerator())

	result, err := service.GenerateInterviewQuestions(context.Background(), matching.InterviewQuestionInput{
		ResumeID:   "resume_1",
		PositionID: "position_1",
	})
	if err != nil {
		t.Fatalf("generate interview questions: %v", err)
	}

	if len(result.Groups) != 3 {
		t.Fatalf("expected three question groups, got %#v", result.Groups)
	}
	if len(result.Groups[0].Questions) != 4 {
		t.Fatalf("expected high-potential professional bonus question, got %#v", result.Groups[0].Questions)
	}
	if !hasDifficulty(result.Groups[0].Questions, "拔高") {
		t.Fatalf("expected advanced professional question, got %#v", result.Groups[0].Questions)
	}
	if !groupContainsQuestion(result.Groups[1], "为什么选择算力训练平台部") {
		t.Fatalf("expected manager question to include department name, got %#v", result.Groups[1])
	}
}

type fakeMatchingStore struct {
	resume      matching.ResumeContext
	position    matching.PositionContext
	relation    matching.ParsedRelation
	upsertInput matching.ParsedRelationInput
	upsertCalls int
}

func (f *fakeMatchingStore) GetResume(ctx context.Context, resumeID string, scope iam.ScopePredicate) (matching.ResumeContext, error) {
	return f.resume, nil
}

func (f *fakeMatchingStore) GetPosition(ctx context.Context, positionID string, scope iam.ScopePredicate) (matching.PositionContext, error) {
	return f.position, nil
}

func (f *fakeMatchingStore) UpsertParsedRelation(ctx context.Context, input matching.ParsedRelationInput) (matching.ParsedRelation, error) {
	f.upsertCalls++
	f.upsertInput = input
	if f.relation.ID != "" {
		return f.relation, nil
	}
	return matching.ParsedRelation{ID: "position_resume_generated", ResumeID: input.ResumeID, PositionID: input.PositionID, MatchScore: input.MatchScore}, nil
}

type recordingMatchingAudit struct {
	events []audit.Event
}

func (r *recordingMatchingAudit) Record(ctx context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func hasDifficulty(questions []matching.InterviewQuestion, difficulty string) bool {
	for _, question := range questions {
		if question.Difficulty == difficulty {
			return true
		}
	}
	return false
}

func groupContainsQuestion(group matching.InterviewQuestionGroup, fragment string) bool {
	for _, question := range group.Questions {
		if strings.Contains(question.Question, fragment) {
			return true
		}
	}
	return false
}

func toAuditString(event audit.Event) string {
	return fmt.Sprintf("%#v", event)
}
