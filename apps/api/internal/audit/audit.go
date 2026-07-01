package audit

import (
	"context"
	"time"
)

type EventType string

const (
	EventLoginSucceeded  EventType = "auth.login_succeeded"
	EventLoginFailed     EventType = "auth.login_failed"
	EventLogoutSucceeded EventType = "auth.logout_succeeded"
)

type Event struct {
	Type    EventType
	UserID  string
	Account string
	Code    string
	At      time.Time
}

type Recorder interface {
	Record(context.Context, Event) error
}

type NopRecorder struct{}

func (NopRecorder) Record(context.Context, Event) error {
	return nil
}
