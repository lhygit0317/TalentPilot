-- +goose Up
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  employee_id TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE TABLE departments (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE TABLE roles (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  is_system BOOLEAN NOT NULL DEFAULT FALSE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL,
  created_by TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMP NOT NULL
);

CREATE TABLE permissions (
  id TEXT PRIMARY KEY,
  role_id TEXT NOT NULL,
  resource TEXT NOT NULL,
  action TEXT NOT NULL,
  attribute_conditions TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  UNIQUE (role_id, resource, action, attribute_conditions)
);

CREATE TABLE role_relations (
  id TEXT PRIMARY KEY,
  parent_role_id TEXT NOT NULL,
  child_role_id TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (parent_role_id) REFERENCES roles(id) ON DELETE CASCADE,
  FOREIGN KEY (child_role_id) REFERENCES roles(id) ON DELETE CASCADE,
  UNIQUE (parent_role_id, child_role_id),
  CHECK (parent_role_id <> child_role_id)
);

CREATE TABLE user_department_roles (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  department_id TEXT NOT NULL,
  role_id TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  created_by TEXT NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE RESTRICT,
  UNIQUE (user_id, department_id, role_id)
);

CREATE TABLE positions (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  chan TEXT NOT NULL,
  level TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  duties TEXT NOT NULL DEFAULT '[]',
  must TEXT NOT NULL DEFAULT '[]',
  keywords TEXT NOT NULL DEFAULT '[]',
  implicit_tags TEXT NOT NULL DEFAULT '[]',
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  CHECK (chan IN ('social', 'campus')),
  CHECK (status IN ('on', 'off'))
);

CREATE TABLE department_positions (
  id TEXT PRIMARY KEY,
  department_id TEXT NOT NULL,
  position_id TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  FOREIGN KEY (position_id) REFERENCES positions(id) ON DELETE CASCADE,
  UNIQUE (department_id, position_id)
);

CREATE TABLE resumes (
  id TEXT PRIMARY KEY,
  normalized_name TEXT NOT NULL,
  name TEXT NOT NULL,
  age INTEGER,
  school TEXT NOT NULL DEFAULT '',
  years_exp REAL,
  pos TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL,
  source_by TEXT NOT NULL DEFAULT '',
  chan TEXT NOT NULL,
  expired BOOLEAN NOT NULL DEFAULT FALSE,
  keywords TEXT NOT NULL DEFAULT '[]',
  traits TEXT NOT NULL DEFAULT '[]',
  exp_base INTEGER NOT NULL DEFAULT 60,
  profile TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  CHECK (source IN ('导入', '推荐')),
  CHECK (chan IN ('social', 'campus'))
);

CREATE TABLE department_resumes (
  id TEXT PRIMARY KEY,
  department_id TEXT NOT NULL,
  resume_id TEXT NOT NULL UNIQUE,
  assigned_at TIMESTAMP NOT NULL,
  by_user_id TEXT NOT NULL,
  FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  FOREIGN KEY (resume_id) REFERENCES resumes(id) ON DELETE CASCADE
);

CREATE TABLE position_resumes (
  id TEXT PRIMARY KEY,
  position_id TEXT NOT NULL,
  resume_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  match_score INTEGER,
  created_at TIMESTAMP NOT NULL,
  by_user_id TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (position_id) REFERENCES positions(id) ON DELETE CASCADE,
  FOREIGN KEY (resume_id) REFERENCES resumes(id) ON DELETE CASCADE,
  UNIQUE (resume_id, position_id, kind),
  CHECK (kind IN ('parsed', 'recommended', 'manual'))
);

CREATE TABLE notifications (
  id TEXT PRIMARY KEY,
  to_user_id TEXT NOT NULL,
  resume_id TEXT NOT NULL,
  department_id TEXT NOT NULL,
  position_id TEXT,
  name TEXT NOT NULL,
  by_user_id TEXT NOT NULL,
  chan TEXT NOT NULL,
  time TIMESTAMP NOT NULL,
  read BOOLEAN NOT NULL DEFAULT FALSE,
  FOREIGN KEY (to_user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  CHECK (chan IN ('social', 'campus'))
);

CREATE TABLE audit_logs (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL,
  actor_user_id TEXT NOT NULL,
  actor_employee_id TEXT NOT NULL,
  actor_role_summary TEXT NOT NULL,
  resource TEXT NOT NULL,
  action TEXT NOT NULL,
  target_id TEXT NOT NULL,
  result TEXT NOT NULL,
  before_value TEXT NOT NULL DEFAULT '{}',
  after_value TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMP NOT NULL
);

CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  input_summary TEXT NOT NULL DEFAULT '{}',
  result_ref TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  request_id TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled'))
);

CREATE INDEX idx_user_department_roles_user ON user_department_roles(user_id);
CREATE INDEX idx_permissions_role ON permissions(role_id);
CREATE INDEX idx_department_positions_department ON department_positions(department_id);
CREATE INDEX idx_department_resumes_department ON department_resumes(department_id);
CREATE INDEX idx_resumes_chan ON resumes(chan);
CREATE INDEX idx_notifications_to_read ON notifications(to_user_id, read);
CREATE INDEX idx_audit_logs_actor_created ON audit_logs(actor_user_id, created_at);

INSERT INTO departments (id, name, created_at, updated_at)
VALUES ('__system__', 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- +goose Down
DROP TABLE jobs;
DROP TABLE audit_logs;
DROP TABLE notifications;
DROP TABLE position_resumes;
DROP TABLE department_resumes;
DROP TABLE resumes;
DROP TABLE department_positions;
DROP TABLE positions;
DROP TABLE user_department_roles;
DROP TABLE role_relations;
DROP TABLE permissions;
DROP TABLE roles;
DROP TABLE departments;
DROP TABLE users;
