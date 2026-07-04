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
	ErrImportScopeDenied    = errors.New("resume import scope denied")
	ErrParseFailed          = errors.New("resume import parse failed")
	ErrDeleteDenied         = errors.New("resume delete denied")
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
	Items             []ListItem      `json:"items" nullable:"false"`
	ChannelCounts     map[Channel]int `json:"channelCounts" nullable:"false"`
	AvailableChannels []Channel       `json:"availableChannels" nullable:"false"`
	DataScopeSummary  string          `json:"dataScopeSummary"`
	NextCursor        string          `json:"nextCursor"`
}

type ListItem struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Age               *int              `json:"age,omitempty"`
	School            string            `json:"school"`
	YearsExp          *float64          `json:"yearsExp,omitempty"`
	CurrentDepartment DepartmentSummary `json:"currentDepartment"`
	Pos               string            `json:"pos"`
	Source            Source            `json:"source"`
	SourceBy          string            `json:"sourceBy"`
	Channel           Channel           `json:"chan"`
	Keywords          []string          `json:"keywords" nullable:"false"`
	CanGet            bool              `json:"canGet"`
	CanDelete         bool              `json:"canDelete"`
}

type Detail struct {
	ListItem
	CreatedAt time.Time `json:"createdAt"`
	Expired   bool      `json:"expired"`
	Profile   Profile   `json:"profile"`
}

type Profile struct {
	Basic          map[string]any   `json:"basic" nullable:"false"`
	Education      []map[string]any `json:"education" nullable:"false"`
	WorkExperience []map[string]any `json:"workExperience" nullable:"false"`
	Projects       []map[string]any `json:"projects" nullable:"false"`
	Skills         []string         `json:"skills" nullable:"false"`
	Certificates   []string         `json:"certificates" nullable:"false"`
	RawTextRef     string           `json:"rawTextRef"`
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
	UserID                      string
	UserName                    string
	Channel                     Channel
	TargetDepartmentID          string
	DataScope                   iam.DataScope
	ResumeCreateScope           iam.ScopePredicate
	DepartmentResumeCreateScope iam.ScopePredicate
	File                        ImportFile
}

type BatchImportInput struct {
	UserID                      string
	UserName                    string
	Channel                     Channel
	TargetDepartmentID          string
	DataScope                   iam.DataScope
	ResumeCreateScope           iam.ScopePredicate
	DepartmentResumeCreateScope iam.ScopePredicate
	Files                       []ImportFile
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
