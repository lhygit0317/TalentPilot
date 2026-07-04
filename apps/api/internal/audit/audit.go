package audit

import (
	"context"
	"time"
)

type EventType string

const (
	EventLoginSucceeded            EventType = "auth.login_succeeded"
	EventLoginFailed               EventType = "auth.login_failed"
	EventLogoutSucceeded           EventType = "auth.logout_succeeded"
	EventUserDepartmentRoleCreated EventType = "iam.user_department_role_created"
	EventUserDepartmentRoleDeleted EventType = "iam.user_department_role_deleted"
	EventPermissionsReplaced       EventType = "iam.permissions_replaced"
	EventRoleRelationCreated       EventType = "iam.role_relation_created"
	EventRoleRelationDeleted       EventType = "iam.role_relation_deleted"
)

type Event struct {
	Type     EventType
	UserID   string
	Account  string
	Code     string
	Resource string
	Action   string
	TargetID string
	Result   string
	Details  map[string]any
	At       time.Time
}

type Recorder interface {
	Record(context.Context, Event) error
}

type NopRecorder struct{}

func (NopRecorder) Record(context.Context, Event) error {
	return nil
}
