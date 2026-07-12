package matching

import (
	"fmt"
	"math"
	"strings"
)

func Calculate(input MatchInput) CalculationResult {
	keywordEvidence, keywordMatches := keywordEvidence(input.PositionKeywords, input.ResumeKeywords)
	implicitEvidence, implicitMatches, implicitScore := implicitEvidence(input.PositionImplicitTags, input.ResumeTraits)
	skillScore := ratioScore(keywordMatches, len(input.PositionKeywords))
	experienceScore := normalizeExperience(input.ExperienceBase)
	total := clampInt(int(math.Round(float64(skillScore)*0.4 + float64(experienceScore)*0.25 + float64(implicitScore)*0.35)))
	judgement := judgementForScore(total)

	return CalculationResult{
		Score: Score{
			Total:      total,
			Skill:      skillScore,
			Experience: experienceScore,
			Implicit:   implicitScore,
			Judgement:  judgement,
		},
		Evidence: Evidence{
			Keywords:     keywordEvidence,
			ImplicitTags: implicitEvidence,
			Analysis: fmt.Sprintf(
				"技能命中 %d/%d；隐性要求命中 %d/%d；%s。",
				keywordMatches,
				len(input.PositionKeywords),
				implicitMatches,
				len(input.PositionImplicitTags),
				judgement,
			),
		},
	}
}

func keywordEvidence(positionKeywords []string, resumeKeywords []string) ([]EvidenceItem, int) {
	resumeSet := normalizedSet(resumeKeywords)
	evidence := make([]EvidenceItem, 0, len(positionKeywords))
	matches := 0
	for _, keyword := range positionKeywords {
		matched := resumeSet[normalizeText(keyword)]
		if matched {
			matches++
		}
		evidence = append(evidence, EvidenceItem{Name: keyword, Matched: matched})
	}
	return evidence, matches
}

func implicitEvidence(positionTags []MatchingImplicitTag, resumeTraits []string) ([]WeightedEvidenceItem, int, int) {
	traitSet := normalizedSet(resumeTraits)
	evidence := make([]WeightedEvidenceItem, 0, len(positionTags))
	matches := 0
	totalWeight := 0
	matchedWeight := 0
	for _, tag := range positionTags {
		weight := tag.Weight
		if weight < 0 {
			weight = 0
		}
		totalWeight += weight
		matched := traitSet[normalizeText(tag.Name)]
		if matched {
			matches++
			matchedWeight += weight
		}
		evidence = append(evidence, WeightedEvidenceItem{Name: tag.Name, Weight: tag.Weight, Matched: matched})
	}
	if totalWeight == 0 {
		return evidence, matches, 0
	}
	return evidence, matches, ratioScore(matchedWeight, totalWeight)
}

func normalizedSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		normalized := normalizeText(value)
		if normalized != "" {
			set[normalized] = true
		}
	}
	return set
}

func normalizeText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func ratioScore(numerator int, denominator int) int {
	if denominator <= 0 || numerator <= 0 {
		return 0
	}
	return clampInt(int(math.Round(float64(numerator) / float64(denominator) * 100)))
}

func normalizeExperience(value int) int {
	if value == 0 {
		return 60
	}
	return clampInt(value)
}

func clampInt(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func judgementForScore(total int) string {
	if total >= 80 {
		return "强烈推荐"
	}
	if total >= 65 {
		return "建议进入面试"
	}
	return "谨慎或暂不推荐"
}
