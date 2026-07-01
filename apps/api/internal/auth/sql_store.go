package auth

import (
	"context"
	"time"

	"gorm.io/gorm"
)

const systemDepartmentID = "__system__"
const guestRoleID = "__role_guest__"

type SQLStore struct {
	db              *gorm.DB
	sessionIDSource func() (string, error)
}

func NewSQLStore(db *gorm.DB) *SQLStore {
	return &SQLStore{db: db, sessionIDSource: newSessionID}
}

func (s *SQLStore) UpsertUserWithGuestBinding(ctx context.Context, identity W3Identity) (UserSummary, []RoleBinding, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO users (id, employee_id, name, created_at, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET employee_id = excluded.employee_id, name = excluded.name, updated_at = CURRENT_TIMESTAMP
		`, identity.ID, identity.EmployeeID, identity.Name).Error; err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO user_department_roles (id, user_id, department_id, role_id, created_at, created_by)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, 'system')
			ON CONFLICT(user_id, department_id, role_id) DO NOTHING
		`, "udr_"+identity.ID+"_guest", identity.ID, systemDepartmentID, guestRoleID).Error
	})
	if err != nil {
		return UserSummary{}, nil, err
	}

	bindings, err := s.roleBindings(ctx, identity.ID)
	if err != nil {
		return UserSummary{}, nil, err
	}
	return UserSummary{ID: identity.ID, EmployeeID: identity.EmployeeID, Name: identity.Name}, bindings, nil
}

func (s *SQLStore) CreateSession(ctx context.Context, input CreateSessionInput) (SessionSummary, error) {
	return s.createSession(ctx, input)
}

func (s *SQLStore) RotateSession(ctx context.Context, input CreateSessionInput) (SessionSummary, error) {
	var session SessionSummary
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE users SET updated_at = updated_at WHERE id = ?", input.UserID).Error; err != nil {
			return err
		}

		txStore := &SQLStore{db: tx, sessionIDSource: s.sessionIDSource}
		created, err := txStore.createSession(ctx, input)
		if err != nil {
			return err
		}
		if err := txStore.RevokeOtherSessions(ctx, input.UserID, created.ID, input.Now); err != nil {
			return err
		}

		session, err = txStore.FindSessionByTokenHash(ctx, input.TokenHash, input.Now)
		return err
	})
	return session, err
}

func (s *SQLStore) createSession(ctx context.Context, input CreateSessionInput) (SessionSummary, error) {
	id, err := s.sessionIDSource()
	if err != nil {
		return SessionSummary{}, err
	}
	if err := s.db.WithContext(ctx).Exec(`
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, input.UserID, input.TokenHash, input.CSRFTokenHash, input.ExpiresAt, input.Now, input.Now).Error; err != nil {
		return SessionSummary{}, err
	}
	return s.FindSessionByTokenHash(ctx, input.TokenHash, input.Now)
}

func (s *SQLStore) FindSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (SessionSummary, error) {
	var row struct {
		ID            string
		UserID        string
		CSRFTokenHash string
		ExpiresAt     time.Time
		EmployeeID    string
		Name          string
	}
	err := s.db.WithContext(ctx).Raw(`
		SELECT s.id, s.user_id, s.csrf_token_hash, s.expires_at, u.employee_id, u.name
		FROM auth_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ?
	`, tokenHash, now).Scan(&row).Error
	if err != nil {
		return SessionSummary{}, err
	}
	if row.ID == "" {
		return SessionSummary{}, ErrUnauthenticated
	}

	bindings, err := s.roleBindings(ctx, row.UserID)
	if err != nil {
		return SessionSummary{}, err
	}
	return SessionSummary{
		ID:            row.ID,
		TokenHash:     tokenHash,
		CSRFTokenHash: row.CSRFTokenHash,
		ExpiresAt:     row.ExpiresAt,
		User:          UserSummary{ID: row.UserID, EmployeeID: row.EmployeeID, Name: row.Name},
		RoleBindings:  bindings,
	}, nil
}

func (s *SQLStore) RevokeSession(ctx context.Context, sessionID string, now time.Time) error {
	return s.db.WithContext(ctx).Exec(
		"UPDATE auth_sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL",
		now,
		sessionID,
	).Error
}

func (s *SQLStore) RevokeOtherSessions(ctx context.Context, userID string, keepSessionID string, now time.Time) error {
	return s.db.WithContext(ctx).Exec(
		"UPDATE auth_sessions SET revoked_at = ? WHERE user_id = ? AND id <> ? AND revoked_at IS NULL",
		now,
		userID,
		keepSessionID,
	).Error
}

func (s *SQLStore) roleBindings(ctx context.Context, userID string) ([]RoleBinding, error) {
	var rows []struct {
		RoleLabel      string
		DepartmentID   string
		DepartmentName string
	}
	err := s.db.WithContext(ctx).Raw(`
		SELECT r.label AS role_label, d.id AS department_id, d.name AS department_name
		FROM user_department_roles udr
		JOIN roles r ON r.id = udr.role_id
		JOIN departments d ON d.id = udr.department_id
		WHERE udr.user_id = ?
		ORDER BY r.label, d.name
	`, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	bindings := make([]RoleBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, RoleBinding{
			RoleLabel:      row.RoleLabel,
			DepartmentID:   row.DepartmentID,
			DepartmentName: row.DepartmentName,
		})
	}
	return bindings, nil
}

func newSessionID() (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	return "session_" + token, nil
}
