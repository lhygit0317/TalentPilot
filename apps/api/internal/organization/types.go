package organization

import (
	"errors"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

var (
	ErrDepartmentNotFound           = errors.New("department not found")
	ErrDepartmentNameRequired       = errors.New("department name required")
	ErrDepartmentNameDuplicate      = errors.New("department name duplicate")
	ErrDepartmentDeleteHasRelations = errors.New("department delete has relations")
	ErrDepartmentSystemProtected    = errors.New("system department is protected")
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
