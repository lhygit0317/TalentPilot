package auth

import (
	"context"
	"errors"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
)

type Service struct {
	w3          W3Adapter
	store       Store
	tokenSource TokenSource
	audit       audit.Recorder
	now         func() time.Time
}

type ServiceConfig struct {
	W3          W3Adapter
	Store       Store
	TokenSource TokenSource
	Audit       audit.Recorder
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
	auditRecorder := cfg.Audit
	if auditRecorder == nil {
		auditRecorder = audit.NopRecorder{}
	}

	return &Service{
		w3:          cfg.W3,
		store:       cfg.Store,
		tokenSource: tokenSource,
		audit:       auditRecorder,
		now:         now,
	}
}

func (s *Service) IssueCSRF(ctx context.Context) (string, error) {
	_, csrfToken, err := s.tokenSource()
	if err != nil {
		return "", err
	}
	return csrfToken, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	identity, err := s.authenticateWithRetry(ctx, W3Credentials{Account: input.Account, Password: input.Password})
	if err != nil {
		s.recordAudit(ctx, audit.Event{Type: audit.EventLoginFailed, Account: input.Account, Code: auditCode(err), At: s.now()})
		return LoginResult{}, err
	}

	user, bindings, err := s.store.UpsertUserWithGuestBinding(ctx, identity)
	if err != nil {
		s.recordAudit(ctx, audit.Event{Type: audit.EventLoginFailed, Account: input.Account, UserID: identity.ID, Code: "STORE_ERROR", At: s.now()})
		return LoginResult{}, err
	}

	authToken, csrfToken, err := s.tokenSource()
	if err != nil {
		s.recordAudit(ctx, audit.Event{Type: audit.EventLoginFailed, Account: input.Account, UserID: identity.ID, Code: "TOKEN_ERROR", At: s.now()})
		return LoginResult{}, err
	}

	now := s.now()
	if _, err := s.store.RotateSession(ctx, CreateSessionInput{
		UserID:        user.ID,
		TokenHash:     HashToken(authToken),
		CSRFTokenHash: HashToken(csrfToken),
		ExpiresAt:     now.Add(12 * time.Hour),
		Now:           now,
	}); err != nil {
		s.recordAudit(ctx, audit.Event{Type: audit.EventLoginFailed, Account: input.Account, UserID: identity.ID, Code: "SESSION_ERROR", At: now})
		return LoginResult{}, err
	}
	s.recordAudit(ctx, audit.Event{Type: audit.EventLoginSucceeded, Account: input.Account, UserID: user.ID, At: now})

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

func (s *Service) CurrentUser(ctx context.Context, authToken string) (LoginResult, error) {
	if authToken == "" {
		return LoginResult{}, ErrUnauthenticated
	}
	session, err := s.store.FindSessionByTokenHash(ctx, HashToken(authToken), s.now())
	if err != nil {
		return LoginResult{}, err
	}
	return loginResultFromSession(session), nil
}

func (s *Service) Logout(ctx context.Context, authToken string, csrfToken string) error {
	if authToken == "" {
		return ErrUnauthenticated
	}
	session, err := s.store.FindSessionByTokenHash(ctx, HashToken(authToken), s.now())
	if err != nil {
		return err
	}
	if csrfToken == "" || session.CSRFTokenHash != HashToken(csrfToken) {
		return ErrCSRFInvalid
	}
	now := s.now()
	if err := s.store.RevokeSession(ctx, session.ID, now); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{Type: audit.EventLogoutSucceeded, UserID: session.User.ID, At: now})
	return nil
}

func (s *Service) authenticateWithRetry(ctx context.Context, creds W3Credentials) (W3Identity, error) {
	identity, err := s.w3.Authenticate(ctx, creds)
	if errors.Is(err, ErrW3Timeout) {
		return s.w3.Authenticate(ctx, creds)
	}
	return identity, err
}

func loginResultFromSession(session SessionSummary) LoginResult {
	return LoginResult{
		User:         session.User,
		RoleBindings: session.RoleBindings,
		RoleLabels:   roleLabels(session.RoleBindings),
		PageAccess:   []string{"resume-parse", "resume-recommend"},
		DefaultRoute: "/resume-parse",
	}
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

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	_ = s.audit.Record(ctx, event)
}

func auditCode(err error) string {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return "AUTH_W3_INVALID_CREDENTIALS"
	case errors.Is(err, ErrW3Timeout):
		return "AUTH_W3_TIMEOUT"
	case errors.Is(err, ErrW3Unavailable):
		return "AUTH_W3_UNAVAILABLE"
	default:
		return "AUTH_LOGIN_FAILED"
	}
}
