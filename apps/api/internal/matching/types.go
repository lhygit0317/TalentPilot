package matching

type ImplicitTag struct {
	Name   string `json:"name"`
	Weight int    `json:"w"`
}

type MatchInput struct {
	ResumeKeywords       []string
	ResumeTraits         []string
	ExperienceBase       int
	PositionKeywords     []string
	PositionImplicitTags []ImplicitTag
}

type CalculationResult struct {
	Score    Score    `json:"score"`
	Evidence Evidence `json:"evidence"`
}

type Score struct {
	Total      int    `json:"total"`
	Skill      int    `json:"skill"`
	Experience int    `json:"experience"`
	Implicit   int    `json:"implicit"`
	Judgement  string `json:"judgement"`
}

type Evidence struct {
	Keywords     []EvidenceItem         `json:"keywords" nullable:"false"`
	ImplicitTags []WeightedEvidenceItem `json:"implicitTags" nullable:"false"`
	Analysis     string                 `json:"analysis"`
}

type EvidenceItem struct {
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
}

type WeightedEvidenceItem struct {
	Name    string `json:"name"`
	Weight  int    `json:"w"`
	Matched bool   `json:"matched"`
}
