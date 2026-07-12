package matching_test

import (
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
)

func TestCalculateMatchScoreUsesPRDWeightsAndEvidence(t *testing.T) {
	result := matching.Calculate(matching.MatchInput{
		ResumeKeywords: []string{"go", "调度"},
		ResumeTraits:   []string{"稳定"},
		ExperienceBase: 82,
		PositionKeywords: []string{
			"Go",
			"Kubernetes",
		},
		PositionImplicitTags: []matching.MatchingImplicitTag{{Name: "稳定", Weight: 40}},
	})

	if result.Score.Skill != 50 || result.Score.Experience != 82 || result.Score.Implicit != 100 {
		t.Fatalf("unexpected component scores: %#v", result.Score)
	}
	if result.Score.Total != 76 || result.Score.Judgement != "建议进入面试" {
		t.Fatalf("unexpected total or judgement: %#v", result.Score)
	}
	if !result.Evidence.Keywords[0].Matched || result.Evidence.Keywords[1].Matched {
		t.Fatalf("unexpected keyword evidence: %#v", result.Evidence.Keywords)
	}
	if !result.Evidence.ImplicitTags[0].Matched {
		t.Fatalf("expected implicit tag match: %#v", result.Evidence.ImplicitTags)
	}
	if result.Evidence.Analysis != "技能命中 1/2；隐性要求命中 1/1；建议进入面试。" {
		t.Fatalf("unexpected analysis: %q", result.Evidence.Analysis)
	}
}

func TestCalculateMatchScoreHandlesEmptyKeywordsAndImplicitTags(t *testing.T) {
	result := matching.Calculate(matching.MatchInput{
		ResumeKeywords:       []string{},
		ResumeTraits:         []string{"稳定"},
		ExperienceBase:       70,
		PositionKeywords:     []string{},
		PositionImplicitTags: []matching.MatchingImplicitTag{},
	})

	if result.Score.Skill != 0 || result.Score.Implicit != 0 {
		t.Fatalf("expected empty keyword and implicit scores to be zero, got %#v", result.Score)
	}
	if result.Score.Total != 18 || result.Score.Judgement != "谨慎或暂不推荐" {
		t.Fatalf("unexpected total or judgement: %#v", result.Score)
	}
	if len(result.Evidence.Keywords) != 0 || len(result.Evidence.ImplicitTags) != 0 {
		t.Fatalf("expected empty evidence slices, got %#v", result.Evidence)
	}
}

func TestCalculateMatchScoreIsCaseInsensitiveAndTrimsWhitespace(t *testing.T) {
	result := matching.Calculate(matching.MatchInput{
		ResumeKeywords: []string{" Go ", "SRE"},
		ResumeTraits:   []string{" 抗压稳定 "},
		ExperienceBase: 90,
		PositionKeywords: []string{
			"go",
			"sre",
		},
		PositionImplicitTags: []matching.MatchingImplicitTag{{Name: "抗压稳定", Weight: 60}},
	})

	if result.Score.Skill != 100 || result.Score.Implicit != 100 {
		t.Fatalf("expected normalized matches, got %#v", result.Score)
	}
	if result.Score.Total != 98 || result.Score.Judgement != "强烈推荐" {
		t.Fatalf("unexpected total or judgement: %#v", result.Score)
	}
}

func TestCalculateMatchScoreRoundsAndClampsExperience(t *testing.T) {
	low := matching.Calculate(matching.MatchInput{
		ResumeKeywords:       []string{"Go"},
		ResumeTraits:         []string{"稳定"},
		ExperienceBase:       -20,
		PositionKeywords:     []string{"Go", "Rust", "Java"},
		PositionImplicitTags: []matching.MatchingImplicitTag{{Name: "稳定", Weight: 1}},
	})
	if low.Score.Experience != 0 || low.Score.Total != 48 {
		t.Fatalf("expected low experience clamp and rounded total, got %#v", low.Score)
	}

	high := matching.Calculate(matching.MatchInput{
		ResumeKeywords:       []string{"Go"},
		ResumeTraits:         []string{"稳定"},
		ExperienceBase:       140,
		PositionKeywords:     []string{"Go"},
		PositionImplicitTags: []matching.MatchingImplicitTag{{Name: "稳定", Weight: 1}},
	})
	if high.Score.Experience != 100 || high.Score.Total != 100 {
		t.Fatalf("expected high experience clamp, got %#v", high.Score)
	}
}
