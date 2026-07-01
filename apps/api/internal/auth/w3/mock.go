package w3

import (
	"context"
	"strings"

	"github.com/talentpilot/talentpilot/apps/api/internal/auth"
)

type MockAdapter struct{}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{}
}

func (a *MockAdapter) Authenticate(ctx context.Context, creds auth.W3Credentials) (auth.W3Identity, error) {
	account := strings.TrimSpace(creds.Account)
	if account == "" || creds.Password == "" {
		return auth.W3Identity{}, auth.ErrInvalidCredentials
	}
	return auth.W3Identity{
		ID:         "w3_" + account,
		Name:       account,
		EmployeeID: strings.ToUpper(account),
	}, nil
}
