package notification

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("notification not found")

type Channel string

const (
	ChannelSocial Channel = "social"
	ChannelCampus Channel = "campus"
)

type Store interface {
	CountUnread(context.Context, string) (int, error)
	ListUnread(context.Context, ListQuery) ([]Item, error)
	MarkAllRead(context.Context, string) (int, error)
	MarkRead(context.Context, MarkReadInput) (Item, error)
}

type NotificationDepartmentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DepartmentSummary = NotificationDepartmentSummary

type NotificationPositionSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PositionSummary = NotificationPositionSummary

type NotificationUserSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserSummary = NotificationUserSummary

type NotificationItem struct {
	ID                   string            `json:"id"`
	ResumeID             string            `json:"resumeId"`
	CandidateName        string            `json:"candidateName"`
	Department           DepartmentSummary `json:"department"`
	Position             *PositionSummary  `json:"position,omitempty"`
	Recommender          UserSummary       `json:"recommender"`
	Channel              Channel           `json:"chan"`
	CreatedAt            time.Time         `json:"createdAt"`
	Read                 bool              `json:"read"`
	CanOpenResumeLibrary bool              `json:"canOpenResumeLibrary"`
}

type Item = NotificationItem

type NotificationSummaryResult struct {
	UnreadCount int `json:"unreadCount"`
}

type SummaryResult = NotificationSummaryResult

type NotificationListResult struct {
	Items       []Item `json:"items" nullable:"false"`
	UnreadCount int    `json:"unreadCount"`
	NextCursor  string `json:"nextCursor"`
}

type ListResult = NotificationListResult

type NotificationReadAllResult struct {
	UpdatedCount int `json:"updatedCount"`
	UnreadCount  int `json:"unreadCount"`
}

type ReadAllResult = NotificationReadAllResult

type NotificationMarkReadResult struct {
	Notification Item `json:"notification"`
	UnreadCount  int  `json:"unreadCount"`
}

type MarkReadResult = NotificationMarkReadResult

type ListQuery struct {
	UserID               string
	Limit                int
	Cursor               string
	CanOpenResumeLibrary bool
}

type MarkReadInput struct {
	UserID               string
	NotificationID       string
	CanOpenResumeLibrary bool
}
