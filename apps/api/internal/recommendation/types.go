package recommendation

import (
	"context"
	"errors"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
)

var (
	ErrRouteFailed            = errors.New("recommendation route failed")
	ErrTargetPositionOffline  = errors.New("recommendation target position offline")
	ErrTargetPositionMismatch = errors.New("recommendation target position mismatch")
	ErrChannelMismatch        = errors.New("recommendation channel mismatch")
	ErrSendFailed             = errors.New("recommendation send failed")
)

type Store interface {
	GetResume(context.Context, string, iam.ScopePredicate) (ResumeContext, error)
	ListRoutePositions(context.Context, RoutePositionQuery) ([]PositionContext, error)
	ListDepartmentContacts(context.Context, []string) (map[string]DepartmentContacts, error)
	SendRecommendation(context.Context, SendCommand) (SendResult, error)
}

type DepartmentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ResumeContext struct {
	ID             string
	NormalizedName string
	Name           string
	Channel        string
	Pos            string
	Source         string
	SourceBy       string
	Department     DepartmentSummary
	Keywords       []string
	Traits         []string
	ExpBase        int
	Expired        bool
}

type PositionContext struct {
	ID           string
	Name         string
	Department   DepartmentSummary
	Channel      string
	Level        string
	Status       string
	Keywords     []string
	ImplicitTags []matching.MatchingImplicitTag
}

type RoutePositionQuery struct {
	Channel string
	Scope   iam.ScopePredicate
}

type RouteInput struct {
	ActorUserID   string
	ResumeID      string
	ResumeScope   iam.ScopePredicate
	PositionScope iam.ScopePredicate
}

type RouteResult struct {
	Resume    ResumeSummary `json:"resume"`
	Routes    []RouteRow    `json:"routes" nullable:"false"`
	CreatedAt time.Time     `json:"createdAt"`
}

type ResumeSummary struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Channel           string            `json:"chan"`
	Pos               string            `json:"pos"`
	CurrentDepartment DepartmentSummary `json:"currentDepartment"`
	Keywords          []string          `json:"keywords" nullable:"false"`
}

type RouteRow struct {
	Department DepartmentSummary  `json:"department"`
	Position   PositionSummary    `json:"position"`
	Score      matching.Score     `json:"score"`
	Contacts   DepartmentContacts `json:"contacts"`
	Best       bool               `json:"best"`
}

type PositionSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Channel string `json:"chan"`
	Level   string `json:"level"`
}

type DepartmentContacts struct {
	HRBPs    []string `json:"hrbps" nullable:"false"`
	Managers []string `json:"managers" nullable:"false"`
	Trainees []string `json:"trainees" nullable:"false"`
}

type SendInput struct {
	ActorUserID                 string
	ActorName                   string
	ResumeID                    string
	DepartmentID                string
	PositionID                  string
	ResumeGetScope              iam.ScopePredicate
	ResumeCreateScope           iam.ScopePredicate
	DepartmentResumeCreateScope iam.ScopePredicate
	PositionResumeCreateScope   iam.ScopePredicate
	NotificationCreateScope     iam.ScopePredicate
}

type SendCommand = SendInput

type SendResult struct {
	ResumeID              string            `json:"resumeId"`
	SourceResumeID        string            `json:"sourceResumeId"`
	Department            DepartmentSummary `json:"department"`
	Position              PositionSummary   `json:"position"`
	CandidateName         string            `json:"candidateName"`
	ReusedExistingCopy    bool              `json:"reusedExistingCopy"`
	NotifiedCount         int               `json:"notifiedCount"`
	SelfNotificationRead  bool              `json:"selfNotificationRead"`
	Message               string            `json:"message"`
	NotificationFailed    bool              `json:"-"`
	NotificationErrorCode string            `json:"-"`
}
