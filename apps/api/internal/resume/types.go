package resume

import (
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
