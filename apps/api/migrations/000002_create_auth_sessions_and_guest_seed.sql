-- +goose Up
CREATE TABLE auth_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  csrf_token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMP NOT NULL,
  revoked_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  last_seen_at TIMESTAMP NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CHECK (token_hash <> ''),
  CHECK (csrf_token_hash <> '')
);

CREATE INDEX idx_auth_sessions_user_active ON auth_sessions(user_id, revoked_at, expires_at);

INSERT INTO roles (id, label, description, is_system, enabled, created_at, created_by, updated_at)
VALUES ('__role_guest__', '游客', 'W3 首次登录默认角色', TRUE, TRUE, CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP);

INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
VALUES ('__permission_guest_department_list__', '__role_guest__', 'Department', 'List', '{}', CURRENT_TIMESTAMP);

INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
VALUES ('__permission_guest_user_get__', '__role_guest__', 'User', 'Get', '{}', CURRENT_TIMESTAMP);

-- +goose Down
DROP TABLE auth_sessions;

DELETE FROM user_department_roles
WHERE role_id = '__role_guest__';

DELETE FROM permissions
WHERE id IN ('__permission_guest_department_list__', '__permission_guest_user_get__')
  AND role_id = '__role_guest__';

DELETE FROM roles
WHERE id = '__role_guest__';
