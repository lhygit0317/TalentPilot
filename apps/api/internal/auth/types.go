package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("w3 invalid credentials")
	ErrW3Timeout          = errors.New("w3 timeout")
	ErrW3Unavailable      = errors.New("w3 unavailable")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrCSRFInvalid        = errors.New("csrf invalid")
)

type W3Credentials struct {
	Account  string
	Password string
}

type W3Identity struct {
	ID         string
	Name       string
	EmployeeID string
}

type W3Adapter interface {
	Authenticate(context.Context, W3Credentials) (W3Identity, error)
}

type Store interface {
	UpsertUserWithGuestBinding(context.Context, W3Identity) (UserSummary, []RoleBinding, error)
	CreateSession(context.Context, CreateSessionInput) (SessionSummary, error)
	RotateSession(context.Context, CreateSessionInput) (SessionSummary, error)
	FindSessionByTokenHash(context.Context, string, time.Time) (SessionSummary, error)
	RevokeSession(context.Context, string, time.Time) error
	RevokeOtherSessions(context.Context, string, string, time.Time) error
}

type UserSummary struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
}

type RoleBinding struct {
	RoleLabel      string `json:"roleLabel"`
	DepartmentID   string `json:"departmentId"`
	DepartmentName string `json:"departmentName"`
}

type LoginInput struct {
	Account  string
	Password string
}

type LoginResult struct {
	User         UserSummary   `json:"user"`
	RoleBindings []RoleBinding `json:"roleBindings"`
	RoleLabels   []string      `json:"roleLabels"`
	PageAccess   []string      `json:"pageAccess"`
	DefaultRoute string        `json:"defaultRoute"`
	AuthToken    string        `json:"-"`
	CSRFToken    string        `json:"-"`
}

type TokenSource func() (authToken string, csrfToken string, err error)

type CreateSessionInput struct {
	UserID        string
	TokenHash     string
	CSRFTokenHash string
	ExpiresAt     time.Time
	Now           time.Time
}

type SessionSummary struct {
	ID            string
	TokenHash     string
	CSRFTokenHash string
	User          UserSummary
	RoleBindings  []RoleBinding
	ExpiresAt     time.Time
}
