package organization

import (
	"errors"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

var (
	ErrDepartmentNotFound            = errors.New("department not found")
	ErrDepartmentNameRequired        = errors.New("department name required")
	ErrDepartmentNameDuplicate       = errors.New("department name duplicate")
	ErrDepartmentDeleteHasRelations  = errors.New("department delete has relations")
	ErrDepartmentSystemProtected     = errors.New("system department is protected")
	ErrPositionNotFound              = errors.New("position not found")
	ErrPositionNameRequired          = errors.New("position name required")
	ErrPositionDepartmentRequired    = errors.New("position department required")
	ErrPositionDepartmentInvalid     = errors.New("position department invalid")
	ErrPositionInvalidChannel        = errors.New("position invalid channel")
	ErrPositionInvalidStatus         = errors.New("position invalid status")
	ErrPositionDuplicateKeyword      = errors.New("position duplicate keyword")
	ErrPositionDuplicateImplicitTag  = errors.New("position duplicate implicit tag")
	ErrPositionInvalidImplicitWeight = errors.New("position invalid implicit weight")
	ErrPositionDeleteHasHistory      = errors.New("position delete has history")
)

type DepartmentListQuery struct {
	Search      string
	Limit       int
	Scope       iam.ScopePredicate
	GetScope    iam.ScopePredicate
	UpdateScope iam.ScopePredicate
	DeleteScope iam.ScopePredicate
}

type DepartmentListResult struct {
	Items []DepartmentListItem `json:"items" nullable:"false"`
}

type DepartmentListItem struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	PositionCount int       `json:"positionCount"`
	ResumeCount   int       `json:"resumeCount"`
	UpdatedAt     time.Time `json:"updatedAt"`
	CanGet        bool      `json:"canGet"`
	CanUpdate     bool      `json:"canUpdate"`
	CanDelete     bool      `json:"canDelete"`
}

type DepartmentDetail struct {
	DepartmentListItem
	Positions []PositionSummary `json:"positions" nullable:"false"`
}

type PositionSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Chan   string `json:"chan"`
	Level  string `json:"level"`
	Status string `json:"status"`
}

type DepartmentInput struct {
	ActorUserID string
	Name        string
}

type PositionDepartmentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PositionListQuery struct {
	DepartmentID string
	Chan         string
	Status       string
	Search       string
	Limit        int
	Scope        iam.ScopePredicate
	GetScope     iam.ScopePredicate
	UpdateScope  iam.ScopePredicate
	DeleteScope  iam.ScopePredicate
}

type PositionListResult struct {
	Items []PositionListItem `json:"items" nullable:"false"`
}

type PositionListItem struct {
	ID               string                    `json:"id"`
	Name             string                    `json:"name"`
	Department       PositionDepartmentSummary `json:"department"`
	Chan             string                    `json:"chan"`
	Level            string                    `json:"level"`
	Status           string                    `json:"status"`
	KeywordCount     int                       `json:"keywordCount"`
	ImplicitTagCount int                       `json:"implicitTagCount"`
	UpdatedAt        time.Time                 `json:"updatedAt"`
	CanGet           bool                      `json:"canGet"`
	CanUpdate        bool                      `json:"canUpdate"`
	CanDelete        bool                      `json:"canDelete"`
}

type PositionDetail struct {
	PositionListItem
	Duties       []string      `json:"duties" nullable:"false"`
	Must         []string      `json:"must" nullable:"false"`
	Keywords     []string      `json:"keywords" nullable:"false"`
	ImplicitTags []ImplicitTag `json:"implicitTags" nullable:"false"`
}

type ImplicitTagInput struct {
	Name   string `json:"name"`
	Weight *int   `json:"w,omitempty"`
}

type ImplicitTag struct {
	Name   string `json:"name"`
	Weight int    `json:"w"`
}

type PositionInput struct {
	ActorUserID  string
	Name         string
	DepartmentID string
	Chan         string
	Level        string
	Status       string
	Duties       []string
	Must         []string
	Keywords     []string
	ImplicitTags []ImplicitTagInput
}
