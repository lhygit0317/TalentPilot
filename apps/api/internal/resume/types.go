package resume

import (
	"context"
	"errors"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

var (
	ErrNotFound             = errors.New("resume not found")
	ErrImportTargetRequired = errors.New("resume import target department required")
	ErrImportTargetInvalid  = errors.New("resume import target department invalid")
	ErrUnsupportedFileType  = errors.New("resume import unsupported file type")
	ErrFileTooLarge         = errors.New("resume import file too large")
	ErrEmptyFile            = errors.New("resume import empty file")
	ErrParseFailed          = errors.New("resume import parse failed")
	ErrJobNotFound          = errors.New("job not found")
	ErrJobAccessDenied      = errors.New("job access denied")
)

type Channel string

const (
	ChannelSocial Channel = "social"
	ChannelCampus Channel = "campus"
)

type Source string

const (
	SourceImported    Source = "导入"
	SourceRecommended Source = "推荐"
)

type DepartmentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ListQuery struct {
	Channel     Channel
	Search      string
	Limit       int
	Cursor      string
	Scope       iam.ScopePredicate
	GetScope    iam.ScopePredicate
	DeleteScope iam.ScopePredicate
}

type ListResult struct {
	Items             []ListItem
	ChannelCounts     map[Channel]int
	AvailableChannels []Channel
	DataScopeSummary  string
	NextCursor        string
}

type ListItem struct {
	ID                string
	Name              string
	Age               *int
	School            string
	YearsExp          *float64
	CurrentDepartment DepartmentSummary
	Pos               string
	Source            Source
	SourceBy          string
	Channel           Channel
	Keywords          []string
	CanGet            bool
	CanDelete         bool
}

type Detail struct {
	ListItem
	CreatedAt time.Time
	Expired   bool
	Profile   Profile
}

type Profile struct {
	Basic          map[string]any
	Education      []map[string]any
	WorkExperience []map[string]any
	Projects       []map[string]any
	Skills         []string
	Certificates   []string
	RawTextRef     string
}

type ParseInput struct {
	FileName    string
	ContentType string
	Bytes       []byte
}

type ParsedResume struct {
	NormalizedName string
	Name           string
	School         string
	Pos            string
	Age            *int
	YearsExp       *float64
	Keywords       []string
	Traits         []string
	ExpBase        int
	Profile        Profile
}

type Parser interface {
	Parse(context.Context, ParseInput) (ParsedResume, error)
}

type ImportFile struct {
	FileName    string
	ContentType string
	Bytes       []byte
}

type ImportInput struct {
	UserID             string
	UserName           string
	Channel            Channel
	TargetDepartmentID string
	DataScope          iam.DataScope
	File               ImportFile
}

type BatchImportInput struct {
	UserID             string
	UserName           string
	Channel            Channel
	TargetDepartmentID string
	DataScope          iam.DataScope
	Files              []ImportFile
}

type JobSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

type JobResult struct {
	FileName  string `json:"fileName"`
	Status    string `json:"status"`
	ResumeID  string `json:"resumeId,omitempty"`
	Name      string `json:"name,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
}

type JobStatus struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Status  string      `json:"status"`
	Summary JobSummary  `json:"summary"`
	Results []JobResult `json:"results"`
}

type ImportJobInput struct {
	Channel            Channel
	TargetDepartmentID string
	FileNames          []string
}

type CreateImportedResumeInput struct {
	UserID             string
	Channel            Channel
	TargetDepartmentID string
	SourceBy           string
	Parsed             ParsedResume
}
