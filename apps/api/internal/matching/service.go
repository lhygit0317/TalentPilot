package matching

import (
	"context"
	"strings"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

type Store interface {
	GetResume(context.Context, string, iam.ScopePredicate) (ResumeContext, error)
	GetPosition(context.Context, string, iam.ScopePredicate) (PositionContext, error)
	UpsertParsedRelation(context.Context, ParsedRelationInput) (ParsedRelation, error)
}

type InterviewQuestionGenerator interface {
	Generate(InterviewQuestionContext) (InterviewQuestionResult, error)
}

type InterviewQuestionContext struct {
	Resume      ResumeContext
	Position    PositionContext
	Calculation CalculationResult
}

type Service struct {
	store     Store
	audit     audit.Recorder
	generator InterviewQuestionGenerator
}

func NewService(store Store, recorder audit.Recorder, generator InterviewQuestionGenerator) *Service {
	if recorder == nil {
		recorder = audit.NopRecorder{}
	}
	if generator == nil {
		generator = NewRuleQuestionGenerator()
	}
	return &Service{store: store, audit: recorder, generator: generator}
}

func (s *Service) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	resume, err := s.store.GetResume(ctx, input.ResumeID, input.ResumeScope)
	if err != nil {
		return ParseResult{}, err
	}
	position, err := s.store.GetPosition(ctx, input.PositionID, input.PositionScope)
	if err != nil {
		return ParseResult{}, err
	}
	if position.Status != "on" {
		return ParseResult{}, ErrPositionOffline
	}
	if !scopeAllowsDepartment(input.PositionResumeCreateScope, position.Department.ID) {
		return ParseResult{}, ErrPositionResumeCreateDenied
	}

	calculation := Calculate(matchInputFromContext(resume, position))
	relation, err := s.store.UpsertParsedRelation(ctx, ParsedRelationInput{
		ResumeID:    resume.ID,
		PositionID:  position.ID,
		MatchScore:  calculation.Score.Total,
		ActorUserID: input.ActorUserID,
	})
	if err != nil {
		return ParseResult{}, err
	}
	s.recordParsed(ctx, input.ActorUserID, relation.ID, resume, position, calculation.Score.Total)

	return ParseResult{
		ID:        relation.ID,
		Resume:    resume,
		Position:  position,
		Score:     calculation.Score,
		Evidence:  calculation.Evidence,
		CreatedAt: relation.CreatedAt,
	}, nil
}

func (s *Service) GenerateInterviewQuestions(ctx context.Context, input InterviewQuestionInput) (InterviewQuestionResult, error) {
	resume, err := s.store.GetResume(ctx, input.ResumeID, input.ResumeScope)
	if err != nil {
		return InterviewQuestionResult{}, err
	}
	position, err := s.store.GetPosition(ctx, input.PositionID, input.PositionScope)
	if err != nil {
		return InterviewQuestionResult{}, err
	}
	calculation := Calculate(matchInputFromContext(resume, position))
	if input.MatchScore != nil {
		calculation.Score.Total = *input.MatchScore
		calculation.Score.Judgement = judgementForScore(*input.MatchScore)
	}
	result, err := s.generator.Generate(InterviewQuestionContext{Resume: resume, Position: position, Calculation: calculation})
	if err != nil {
		return InterviewQuestionResult{}, ErrInterviewQuestionGenerateFail
	}
	return result, nil
}

func matchInputFromContext(resume ResumeContext, position PositionContext) MatchInput {
	return MatchInput{
		ResumeKeywords:       resume.Keywords,
		ResumeTraits:         resume.Traits,
		ExperienceBase:       resume.ExpBase,
		PositionKeywords:     position.Keywords,
		PositionImplicitTags: position.ImplicitTags,
	}
}

func (s *Service) recordParsed(ctx context.Context, actorUserID string, targetID string, resume ResumeContext, position PositionContext, score int) {
	_ = s.audit.Record(ctx, audit.Event{
		Type:        audit.EventResumeParsed,
		UserID:      actorUserID,
		ActorUserID: actorUserID,
		Resource:    string(iam.ResourcePositionResume),
		Action:      string(iam.ActionCreate),
		TargetID:    targetID,
		Result:      "succeeded",
		After: map[string]any{
			"resumeId":     resume.ID,
			"positionId":   position.ID,
			"matchScore":   score,
			"departmentId": position.Department.ID,
			"chan":         resume.Channel,
		},
	})
}

func scopeAllowsDepartment(scope iam.ScopePredicate, departmentID string) bool {
	if departmentID == "" {
		return false
	}
	for _, branch := range scope.Branches {
		if branch.AllDepartments {
			return true
		}
		for _, scopedDepartmentID := range branch.DepartmentIDs {
			if scopedDepartmentID == departmentID {
				return true
			}
		}
	}
	return false
}

type RuleQuestionGenerator struct{}

func NewRuleQuestionGenerator() RuleQuestionGenerator {
	return RuleQuestionGenerator{}
}

func (RuleQuestionGenerator) Generate(ctx InterviewQuestionContext) (InterviewQuestionResult, error) {
	anchor := questionAnchor(ctx.Resume, ctx.Position)
	professional := []InterviewQuestion{
		{Order: 1, Question: "请结合 " + anchor + " 项目说明你如何解决岗位「" + ctx.Position.Name + "」中的核心问题。", Why: "验证候选人关键词与岗位核心技能的真实经验。", Difficulty: "核心"},
		{Order: 2, Question: "如果线上任务出现性能瓶颈，你会如何定位并拆解改进计划？", Why: "观察问题拆解、指标意识和工程落地能力。", Difficulty: "进阶"},
		{Order: 3, Question: "请介绍一次你在复杂约束下交付技术方案的经历。", Why: "判断候选人的系统思考和交付稳定性。", Difficulty: "核心"},
	}
	if ctx.Calculation.Score.Experience >= 82 {
		professional = append(professional, InterviewQuestion{Order: 4, Question: "如果让你重新设计现有方案，你会怎样提升可扩展性和可维护性？", Why: "高潜候选人加测架构抽象和长期演进判断。", Difficulty: "拔高"})
	}

	manager := []InterviewQuestion{
		{Order: 1, Question: "请说明你过去一次跨团队协作中的角色、冲突和结果。", Why: "评估协同方式和复杂组织沟通能力。", Difficulty: "行为"},
		{Order: 2, Question: "为什么选择" + ctx.Position.Department.Name + "，以及你期待如何与该部门协作？", Why: "确认动机、稳定性和部门匹配度。", Difficulty: "动机"},
		{Order: 3, Question: "当你与上级对技术方案判断不一致时，会如何推进？", Why: "观察向上沟通、事实依据和决策执行方式。", Difficulty: "行为"},
	}

	qualification := []InterviewQuestion{
		{Order: 1, Question: "是否可以配合入职前背景调查，并说明需要提前沟通的信息？", Why: "确认背调前置风险和候选人配合度。", Difficulty: "合规"},
		{Order: 2, Question: "请确认薪酬预期、可到岗时间和当前流程进展。", Why: "验证推进条件是否清晰可落地。", Difficulty: "流程"},
		{Order: 3, Question: "岗位可能涉及阶段性出差、加班或应急支持，你的接受边界是什么？", Why: "确认工作方式和业务节奏匹配度。", Difficulty: "流程"},
	}

	return InterviewQuestionResult{Groups: []InterviewQuestionGroup{
		{Type: "professional", Label: "专业面试", Questions: professional},
		{Type: "manager", Label: "主管面试", Questions: manager},
		{Type: "qualification", Label: "资格面试", Questions: qualification},
	}}, nil
}

func questionAnchor(resume ResumeContext, position PositionContext) string {
	for _, keyword := range resume.Keywords {
		if strings.TrimSpace(keyword) != "" {
			return strings.TrimSpace(keyword)
		}
	}
	for _, keyword := range position.Keywords {
		if strings.TrimSpace(keyword) != "" {
			return strings.TrimSpace(keyword)
		}
	}
	if strings.TrimSpace(position.Name) != "" {
		return strings.TrimSpace(position.Name)
	}
	return "候选人经历"
}
