package matching

import (
	"errors"
	"time"
)

var (
	ErrResumeNotFound   = errors.New("matching resume not found")
	ErrPositionNotFound = errors.New("matching position not found")
)

type DepartmentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ImplicitTag struct {
	Name   string `json:"name"`
	Weight int    `json:"w"`
}

type ResumeContext struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Channel           string            `json:"chan"`
	Pos               string            `json:"pos"`
	Source            string            `json:"source"`
	SourceBy          string            `json:"sourceBy"`
	CurrentDepartment DepartmentSummary `json:"currentDepartment"`
	Keywords          []string          `json:"keywords" nullable:"false"`
	Traits            []string          `json:"traits" nullable:"false"`
	ExpBase           int               `json:"expBase"`
}

type PositionContext struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Department   DepartmentSummary `json:"department"`
	Channel      string            `json:"chan"`
	Level        string            `json:"level"`
	Status       string            `json:"status"`
	Keywords     []string          `json:"keywords" nullable:"false"`
	ImplicitTags []ImplicitTag     `json:"implicitTags" nullable:"false"`
}

type ParsedRelationInput struct {
	ResumeID    string
	PositionID  string
	MatchScore  int
	ActorUserID string
}

type ParsedRelation struct {
	ID         string    `json:"id"`
	ResumeID   string    `json:"resumeId"`
	PositionID string    `json:"positionId"`
	MatchScore int       `json:"matchScore"`
	CreatedAt  time.Time `json:"createdAt"`
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
