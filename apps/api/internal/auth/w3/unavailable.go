package w3

import (
	"context"

	"github.com/talentpilot/talentpilot/apps/api/internal/auth"
)

type UnavailableAdapter struct{}

func NewUnavailableAdapter() *UnavailableAdapter {
	return &UnavailableAdapter{}
}

func (a *UnavailableAdapter) Authenticate(ctx context.Context, creds auth.W3Credentials) (auth.W3Identity, error) {
	return auth.W3Identity{}, auth.ErrW3Unavailable
}
