package auth

import (
	"context"
	"errors"
	"time"
)

type Service struct {
	w3          W3Adapter
	store       Store
	tokenSource TokenSource
	now         func() time.Time
}

type ServiceConfig struct {
	W3          W3Adapter
	Store       Store
	TokenSource TokenSource
	Now         func() time.Time
}

func NewService(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	tokenSource := cfg.TokenSource
	if tokenSource == nil {
		tokenSource = NewRandomTokenSource()
	}

	return &Service{
		w3:          cfg.W3,
		store:       cfg.Store,
		tokenSource: tokenSource,
		now:         now,
	}
}

func (s *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	identity, err := s.authenticateWithRetry(ctx, W3Credentials{Account: input.Account, Password: input.Password})
	if err != nil {
		return LoginResult{}, err
	}

	user, bindings, err := s.store.UpsertUserWithGuestBinding(ctx, identity)
	if err != nil {
		return LoginResult{}, err
	}

	authToken, csrfToken, err := s.tokenSource()
	if err != nil {
		return LoginResult{}, err
	}

	if err := s.store.RevokeOtherSessions(ctx, user.ID, "", s.now()); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		User:         user,
		RoleBindings: bindings,
		RoleLabels:   roleLabels(bindings),
		PageAccess:   []string{"resume-parse", "resume-recommend"},
		DefaultRoute: "/resume-parse",
		AuthToken:    authToken,
		CSRFToken:    csrfToken,
	}, nil
}

func (s *Service) authenticateWithRetry(ctx context.Context, creds W3Credentials) (W3Identity, error) {
	identity, err := s.w3.Authenticate(ctx, creds)
	if errors.Is(err, ErrW3Timeout) {
		return s.w3.Authenticate(ctx, creds)
	}
	return identity, err
}

func roleLabels(bindings []RoleBinding) []string {
	seen := map[string]bool{}
	labels := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if !seen[binding.RoleLabel] {
			seen[binding.RoleLabel] = true
			labels = append(labels, binding.RoleLabel)
		}
	}
	return labels
}
