# Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first runnable, testable TalentPilot foundation: monorepo tooling, Go API, React web app, database migrations, generated API contract/client, CI, containers, and Agent documentation updates.

**Architecture:** Implement a modular monolith Go API and separate React frontend inside a monorepo. The backend owns security and API contracts; generated OpenAPI feeds a generated frontend client. The plan implements foundation only and intentionally avoids PRD business Story behavior.

**Tech Stack:** Node 24 LTS, pnpm, React, Vite, Tailwind CSS, shadcn-style project UI components, Vitest, React Testing Library, Go 1.26, Echo, Huma Echo adapter, GORM, goose, SQLite for tests, PostgreSQL for production/CI migration checks.

---

## File Structure Map

Create this structure:

```text
TalentPilot/
  .dockerignore
  .editorconfig
  .env.example
  .github/workflows/ci.yml
  .gitignore
  Makefile
  README.md
  package.json
  pnpm-workspace.yaml
  docker-compose.yml
  apps/
    api/
      Dockerfile
      go.mod
      cmd/api/main.go
      cmd/openapi/main.go
      internal/app/server.go
      internal/app/server_test.go
      internal/app/openapi_test.go
      internal/config/config.go
      internal/http/apperror/error.go
      internal/http/apperror/error_test.go
      internal/platform/db/db.go
      migrations/000001_create_foundation_tables.sql
      test/integration/migrations_test.go
    web/
      Dockerfile
      index.html
      package.json
      postcss.config.js
      tailwind.config.ts
      tsconfig.json
      tsconfig.node.json
      vite.config.ts
      vitest.config.ts
      src/main.tsx
      src/app/App.tsx
      src/app/App.test.tsx
      src/components/ui/button.tsx
      src/i18n/zh-CN.ts
      src/i18n/en-US.ts
      src/styles/globals.css
      src/test/setup.ts
  packages/
    api-contract/package.json
    api-contract/openapi.json
    api-client/package.json
    api-client/src/index.ts
    api-client/src/schema.d.ts
```

Do not implement PRD business modules in this plan. Only implement health, foundation wiring, migrations, contracts, and tooling.

## Task 1: Root Workspace and Command Surface

**Files:**
- Create: `.gitignore`
- Create: `.editorconfig`
- Create: `.env.example`
- Create: `package.json`
- Create: `pnpm-workspace.yaml`
- Create: `Makefile`
- Modify: `README.md`
- Modify: `docs/project-status.md`

- [ ] **Step 1: Declare TDD exception for scaffolding-only files**

No production behavior is introduced in this task. Verification is by command execution and file existence. Keep this exception in the task notes when implementing.

- [ ] **Step 2: Create root ignore and editor files**

Create `.gitignore`:

```gitignore
.DS_Store
.env
.env.local
node_modules/
dist/
coverage/
*.log
.turbo/
.vite/
apps/api/bin/
apps/api/tmp/
apps/api/*.db
apps/api/*.db-*
packages/api-client/src/schema.d.ts
```

Create `.dockerignore`:

```dockerignore
.git
node_modules
dist
coverage
.env
*.log
apps/api/tmp
apps/api/*.db
```

Create `.editorconfig`:

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
indent_style = space
indent_size = 2
trim_trailing_whitespace = true

[*.go]
indent_style = tab

[Makefile]
indent_style = tab
```

- [ ] **Step 3: Create root package and workspace files**

Create `package.json`:

```json
{
  "name": "talentpilot",
  "private": true,
  "packageManager": "pnpm@10.13.1",
  "scripts": {
    "dev": "pnpm -r --parallel dev",
    "test": "pnpm -r test",
    "test:web": "pnpm --filter @talentpilot/web test",
    "typecheck": "pnpm -r typecheck",
    "lint": "pnpm -r lint",
    "build": "pnpm -r build",
    "client:generate": "pnpm --filter @talentpilot/api-client generate",
    "openapi:check": "pnpm --filter @talentpilot/api-contract check"
  },
  "devDependencies": {
    "typescript": "latest"
  }
}
```

Create `pnpm-workspace.yaml`:

```yaml
packages:
  - "apps/*"
  - "packages/*"
```

- [ ] **Step 4: Create environment example**

Create `.env.example`:

```dotenv
APP_ENV=development
API_ADDR=:8080
DATABASE_DRIVER=sqlite
DATABASE_DSN=file:talentpilot_dev.db?_foreign_keys=on
WEB_API_BASE_URL=http://localhost:8080
```

- [ ] **Step 5: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: help setup dev dev-web dev-api test test-api test-web test-e2e lint typecheck build migrate-up migrate-down openapi-generate openapi-check client-generate ci

help:
	@grep -E '^[a-zA-Z_-]+:' Makefile | cut -d: -f1 | sort

setup:
	corepack enable
	pnpm install
	cd apps/api && go mod download

dev:
	pnpm dev

dev-web:
	pnpm --filter @talentpilot/web dev

dev-api:
	cd apps/api && go run ./cmd/api

test:
	$(MAKE) test-api
	$(MAKE) test-web

test-api:
	cd apps/api && go test ./...

test-web:
	pnpm --filter @talentpilot/web test -- --run

test-e2e:
	pnpm --filter @talentpilot/web test:e2e

lint:
	pnpm lint
	cd apps/api && go vet ./...

typecheck:
	pnpm typecheck

build:
	pnpm build
	cd apps/api && go build ./cmd/api

migrate-up:
	cd apps/api && go run github.com/pressly/goose/v3/cmd/goose sqlite3 "file:talentpilot_dev.db?_foreign_keys=on" up

migrate-down:
	cd apps/api && go run github.com/pressly/goose/v3/cmd/goose sqlite3 "file:talentpilot_dev.db?_foreign_keys=on" down

openapi-generate:
	cd apps/api && go run ./cmd/openapi > ../../packages/api-contract/openapi.json

openapi-check:
	$(MAKE) openapi-generate
	git diff --exit-code packages/api-contract/openapi.json

client-generate:
	pnpm --filter @talentpilot/api-client generate

ci:
	$(MAKE) lint
	$(MAKE) typecheck
	$(MAKE) test
	$(MAKE) openapi-check
	$(MAKE) client-generate
	$(MAKE) build
```

- [ ] **Step 6: Update README**

Create or replace `README.md`:

```markdown
# TalentPilot

TalentPilot is a frontend/backend separated recruiting intelligence assistant for the Computing Power Business Unit.

## Current Phase

Foundation implementation. See:

- `PRD.md`
- `AGENTS.md`
- `docs/specs/000-foundation-architecture.md`
- `docs/project-status.md`

## Planned Commands

```bash
make setup
make dev
make test
make ci
```

## Architecture

- Frontend: `apps/web`
- Backend: `apps/api`
- OpenAPI contract: `packages/api-contract`
- Generated frontend client: `packages/api-client`
```

- [ ] **Step 7: Verify root command surface**

Run:

```bash
make help
```

Expected: command names are printed, including `setup`, `test`, `ci`, and `openapi-generate`.

- [ ] **Step 8: Commit**

```bash
git add .gitignore .dockerignore .editorconfig .env.example package.json pnpm-workspace.yaml Makefile README.md
git commit -m "chore: add monorepo command surface"
```

## Task 2: Backend Health API with Echo and Huma

**Files:**
- Create: `apps/api/go.mod`
- Create: `apps/api/cmd/api/main.go`
- Create: `apps/api/internal/config/config.go`
- Create: `apps/api/internal/app/server_test.go`
- Create: `apps/api/internal/app/server.go`

- [ ] **Step 1: Initialize Go module and dependencies**

Run:

```bash
mkdir -p apps/api/cmd/api apps/api/internal/app apps/api/internal/config
cd apps/api
go mod init github.com/talentpilot/talentpilot/apps/api
go get github.com/labstack/echo/v4@latest
go get github.com/danielgtaylor/huma/v2@latest
go get github.com/danielgtaylor/huma/v2/adapters/humaecho@latest
```

Expected: `apps/api/go.mod` and `apps/api/go.sum` are created.

- [ ] **Step 2: Write the failing health test**

Create `apps/api/internal/app/server_test.go`:

```go
package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpointReturnsOK(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	expected := `{"status":"ok"}`
	if rec.Body.String() != expected+"\n" && rec.Body.String() != expected {
		t.Fatalf("expected body %s, got %s", expected, rec.Body.String())
	}
}
```

- [ ] **Step 3: Run test to verify RED**

Run:

```bash
cd apps/api && go test ./internal/app -run TestHealthEndpointReturnsOK -count=1
```

Expected: FAIL because `NewServer` is undefined.

- [ ] **Step 4: Implement config**

Create `apps/api/internal/config/config.go`:

```go
package config

import "os"

type Config struct {
	Env     string
	APIAddr string
}

func Load() Config {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		Env:     env,
		APIAddr: addr,
	}
}
```

- [ ] **Step 5: Implement server and health operation**

Create `apps/api/internal/app/server.go`:

```go
package app

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humaecho"
	"github.com/labstack/echo/v4"
)

type Server struct {
	Echo *echo.Echo
	API  huma.API
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func NewServer() *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	api := humaecho.New(e, huma.DefaultConfig("TalentPilot API", "0.1.0"))

	huma.Register(api, huma.Operation{
		OperationID: "get-healthz",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Health check",
		Tags:        []string{"system"},
	}, func(ctx context.Context, input *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})

	return &Server{Echo: e, API: api}
}
```

Create `apps/api/cmd/api/main.go`:

```go
package main

import (
	"log"

	"github.com/talentpilot/talentpilot/apps/api/internal/app"
	"github.com/talentpilot/talentpilot/apps/api/internal/config"
)

func main() {
	cfg := config.Load()
	server := app.NewServer()

	if err := server.Echo.Start(cfg.APIAddr); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 6: Run test to verify GREEN**

Run:

```bash
cd apps/api && go test ./internal/app -run TestHealthEndpointReturnsOK -count=1
```

Expected: PASS.

- [ ] **Step 7: Run backend package tests**

Run:

```bash
cd apps/api && go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/go.mod apps/api/go.sum apps/api/cmd/api/main.go apps/api/internal/config/config.go apps/api/internal/app/server.go apps/api/internal/app/server_test.go
git commit -m "feat(api): add health endpoint"
```

## Task 3: Backend Error Code Envelope

**Files:**
- Create: `apps/api/internal/http/apperror/error_test.go`
- Create: `apps/api/internal/http/apperror/error.go`

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/http/apperror/error_test.go`:

```go
package apperror

import "testing"

func TestNewProblemBuildsStableErrorEnvelope(t *testing.T) {
	problem := NewProblem(IAMRoleRelationCycle, "角色包含关系不能形成循环", "req_123", map[string]any{
		"roleLabel": "高级评审者",
	})

	if problem.Code != IAMRoleRelationCycle {
		t.Fatalf("expected code %s, got %s", IAMRoleRelationCycle, problem.Code)
	}
	if problem.Message != "角色包含关系不能形成循环" {
		t.Fatalf("unexpected message %q", problem.Message)
	}
	if problem.RequestID != "req_123" {
		t.Fatalf("unexpected request id %q", problem.RequestID)
	}
	if problem.Details["roleLabel"] != "高级评审者" {
		t.Fatalf("unexpected details %#v", problem.Details)
	}
}
```

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd apps/api && go test ./internal/http/apperror -run TestNewProblemBuildsStableErrorEnvelope -count=1
```

Expected: FAIL because package symbols are undefined.

- [ ] **Step 3: Implement the error envelope**

Create `apps/api/internal/http/apperror/error.go`:

```go
package apperror

type Code string

const (
	Unauthenticated       Code = "AUTH_UNAUTHENTICATED"
	PermissionDenied     Code = "IAM_PERMISSION_DENIED"
	IAMRoleRelationCycle Code = "IAM_ROLE_RELATION_CYCLE"
	ValidationFailed     Code = "VALIDATION_FAILED"
	Internal             Code = "INTERNAL_ERROR"
)

type Problem struct {
	Code      Code           `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details,omitempty"`
}

func NewProblem(code Code, message string, requestID string, details map[string]any) Problem {
	return Problem{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Details:   details,
	}
}
```

- [ ] **Step 4: Run test to verify GREEN**

Run:

```bash
cd apps/api && go test ./internal/http/apperror -run TestNewProblemBuildsStableErrorEnvelope -count=1
```

Expected: PASS.

- [ ] **Step 5: Run backend tests**

Run:

```bash
cd apps/api && go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/http/apperror/error.go apps/api/internal/http/apperror/error_test.go
git commit -m "feat(api): add stable error envelope"
```

## Task 4: Database Adapter and Foundation Migrations

**Files:**
- Create: `apps/api/internal/platform/db/db.go`
- Create: `apps/api/migrations/000001_create_foundation_tables.sql`
- Create: `apps/api/test/integration/migrations_test.go`

- [ ] **Step 1: Add database dependencies**

Run:

```bash
cd apps/api
go get gorm.io/gorm@latest
go get gorm.io/driver/sqlite@latest
go get gorm.io/driver/postgres@latest
go get github.com/pressly/goose/v3@latest
go get github.com/mattn/go-sqlite3@latest
```

Expected: dependencies are added to `go.mod`.

- [ ] **Step 2: Write the failing migration test**

Create `apps/api/test/integration/migrations_test.go`:

```go
package integration

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestFoundationMigrationsCreateCoreTables(t *testing.T) {
	database, err := sql.Open("sqlite3", "file::memory:?cache=shared&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	migrationDir := filepath.Join("..", "..", "migrations")
	if err := goose.Up(database, migrationDir); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	rows, err := database.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name IN ('users', 'roles', 'resumes', 'audit_logs') ORDER BY name`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		names = append(names, name)
	}

	expected := []string{"audit_logs", "resumes", "roles", "users"}
	if len(names) != len(expected) {
		t.Fatalf("expected tables %v, got %v", expected, names)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("expected tables %v, got %v", expected, names)
		}
	}

	if err := goose.Down(database, migrationDir); err != nil {
		t.Fatalf("goose down: %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify RED**

Run:

```bash
cd apps/api && go test ./test/integration -run TestFoundationMigrationsCreateCoreTables -count=1
```

Expected: FAIL because the migration directory or tables do not exist.

- [ ] **Step 4: Implement DB adapter**

Create `apps/api/internal/platform/db/db.go`:

```go
package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	Driver string
	DSN    string
}

func Open(cfg Config) (*gorm.DB, error) {
	switch cfg.Driver {
	case "sqlite":
		return gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{})
	case "postgres":
		return gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}
```

- [ ] **Step 5: Add foundation migration**

Create `apps/api/migrations/000001_create_foundation_tables.sql`:

```sql
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
```

- [ ] **Step 6: Run migration test to verify GREEN**

Run:

```bash
cd apps/api && go test ./test/integration -run TestFoundationMigrationsCreateCoreTables -count=1
```

Expected: PASS.

- [ ] **Step 7: Run backend tests**

Run:

```bash
cd apps/api && go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/go.mod apps/api/go.sum apps/api/internal/platform/db/db.go apps/api/migrations/000001_create_foundation_tables.sql apps/api/test/integration/migrations_test.go
git commit -m "feat(api): add foundation migrations"
```

## Task 5: OpenAPI Generation and Contract Package

**Files:**
- Create: `apps/api/internal/app/openapi_test.go`
- Create: `apps/api/cmd/openapi/main.go`
- Create: `packages/api-contract/package.json`
- Generate: `packages/api-contract/openapi.json`

- [ ] **Step 1: Write the failing OpenAPI test**

Create `apps/api/internal/app/openapi_test.go`:

```go
package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestOpenAPIDocumentIncludesHealthEndpoint(t *testing.T) {
	server := NewServer()

	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}

	if !bytes.Contains(raw, []byte(`"/healthz"`)) {
		t.Fatalf("expected OpenAPI document to contain /healthz, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte(`"get-healthz"`)) {
		t.Fatalf("expected OpenAPI document to contain get-healthz operation, got %s", string(raw))
	}
}
```

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd apps/api && go test ./internal/app -run TestOpenAPIDocumentIncludesHealthEndpoint -count=1
```

Expected: If Task 2 Huma wiring is present, this may already PASS. If it passes, record that OpenAPI behavior was introduced by the health endpoint test and proceed to command generation. If it fails, fix only Huma registration in `server.go` until it passes.

- [ ] **Step 3: Add OpenAPI generator command**

Create `apps/api/cmd/openapi/main.go`:

```go
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/talentpilot/talentpilot/apps/api/internal/app"
)

func main() {
	server := app.NewServer()

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(server.API.OpenAPI()); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 4: Add contract package metadata**

Create `packages/api-contract/package.json`:

```json
{
  "name": "@talentpilot/api-contract",
  "private": true,
  "type": "module",
  "scripts": {
    "generate": "cd ../../apps/api && go run ./cmd/openapi > ../../packages/api-contract/openapi.json",
    "check": "pnpm generate && git diff --exit-code openapi.json"
  }
}
```

- [ ] **Step 5: Generate OpenAPI artifact**

Run:

```bash
pnpm --filter @talentpilot/api-contract generate
```

Expected: `packages/api-contract/openapi.json` is created and contains `/healthz`.

- [ ] **Step 6: Verify OpenAPI check**

Run:

```bash
pnpm --filter @talentpilot/api-contract check
```

Expected: PASS with no git diff for `packages/api-contract/openapi.json`.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/app/openapi_test.go apps/api/cmd/openapi/main.go packages/api-contract/package.json packages/api-contract/openapi.json
git commit -m "feat(contract): generate openapi from backend"
```

## Task 6: Frontend App, UI Wrapper, and i18n Foundation

**Files:**
- Create: `apps/web/package.json`
- Create: `apps/web/index.html`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/tsconfig.node.json`
- Create: `apps/web/vite.config.ts`
- Create: `apps/web/vitest.config.ts`
- Create: `apps/web/postcss.config.js`
- Create: `apps/web/tailwind.config.ts`
- Create: `apps/web/src/test/setup.ts`
- Create: `apps/web/src/app/App.test.tsx`
- Create: `apps/web/src/app/App.tsx`
- Create: `apps/web/src/components/ui/button.tsx`
- Create: `apps/web/src/i18n/zh-CN.ts`
- Create: `apps/web/src/i18n/en-US.ts`
- Create: `apps/web/src/styles/globals.css`
- Create: `apps/web/src/main.tsx`

- [ ] **Step 1: Create frontend package metadata**

Create `apps/web/package.json`:

```json
{
  "name": "@talentpilot/web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite --host 0.0.0.0",
    "build": "tsc -b && vite build",
    "typecheck": "tsc -b",
    "lint": "eslint .",
    "test": "vitest",
    "test:e2e": "playwright test"
  },
  "dependencies": {
    "@vitejs/plugin-react": "latest",
    "class-variance-authority": "latest",
    "clsx": "latest",
    "lucide-react": "latest",
    "motion": "latest",
    "react": "latest",
    "react-dom": "latest",
    "tailwind-merge": "latest",
    "vite": "latest"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "latest",
    "@testing-library/react": "latest",
    "@testing-library/user-event": "latest",
    "@types/node": "latest",
    "@types/react": "latest",
    "@types/react-dom": "latest",
    "@vitest/coverage-v8": "latest",
    "autoprefixer": "latest",
    "eslint": "latest",
    "jsdom": "latest",
    "postcss": "latest",
    "tailwindcss": "latest",
    "typescript": "latest",
    "vitest": "latest"
  }
}
```

- [ ] **Step 2: Write the failing app behavior test**

Create `apps/web/src/app/App.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";

describe("App", () => {
  it("renders the foundation shell using project UI copy", () => {
    render(<App />);

    expect(screen.getByRole("main", { name: "TalentPilot foundation" })).toBeInTheDocument();
    expect(screen.getByText("TalentPilot")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "进入工作台" })).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Add test setup**

Create `apps/web/src/test/setup.ts`:

```ts
import "@testing-library/jest-dom/vitest";
```

Create `apps/web/vitest.config.ts`:

```ts
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
  },
});
```

- [ ] **Step 4: Run test to verify RED**

Run:

```bash
pnpm install
pnpm --filter @talentpilot/web test -- --run src/app/App.test.tsx
```

Expected: FAIL because `App.tsx` does not exist.

- [ ] **Step 5: Add TypeScript, Vite, Tailwind, and HTML config**

Create `apps/web/index.html`:

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>TalentPilot</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

Create `apps/web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["DOM", "DOM.Iterable", "ES2022"],
    "allowJs": false,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "module": "ESNext",
    "moduleResolution": "Node",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "types": ["vitest/globals"]
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

Create `apps/web/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "composite": true,
    "module": "ESNext",
    "moduleResolution": "Node",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts", "vitest.config.ts", "tailwind.config.ts"]
}
```

Create `apps/web/vite.config.ts`:

```ts
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
  },
});
```

Create `apps/web/postcss.config.js`:

```js
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};
```

Create `apps/web/tailwind.config.ts`:

```ts
import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg: "oklch(10% 0.018 245)",
        shell: "oklch(13% 0.018 245)",
        fg: "oklch(93% 0.01 240)",
        muted: "oklch(66% 0.018 240)",
        accent: "oklch(73% 0.13 190)",
      },
      fontFamily: {
        body: [
          "-apple-system",
          "BlinkMacSystemFont",
          "SF Pro Text",
          "PingFang SC",
          "Microsoft YaHei",
          "sans-serif",
        ],
      },
    },
  },
  plugins: [],
} satisfies Config;
```

- [ ] **Step 6: Implement i18n and UI button wrapper**

Create `apps/web/src/i18n/zh-CN.ts`:

```ts
export const zhCN = {
  appName: "TalentPilot",
  foundation: {
    mainLabel: "TalentPilot foundation",
    eyebrow: "算力事业部",
    title: "TalentPilot",
    summary: "招聘智能助手工程底座已就绪。",
    primaryAction: "进入工作台",
  },
} as const;
```

Create `apps/web/src/i18n/en-US.ts`:

```ts
export const enUS = {
  appName: "TalentPilot",
  foundation: {
    mainLabel: "TalentPilot foundation",
    eyebrow: "Computing Power Business Unit",
    title: "TalentPilot",
    summary: "The recruiting assistant foundation is ready.",
    primaryAction: "Enter workspace",
  },
} as const;
```

Create `apps/web/src/components/ui/button.tsx`:

```tsx
import * as React from "react";
import { type VariantProps, cva } from "class-variance-authority";
import { cn } from "./cn";

const buttonVariants = cva(
  "inline-flex min-h-11 items-center justify-center border px-4 py-2 text-sm font-medium transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-45 active:translate-y-px",
  {
    variants: {
      variant: {
        primary:
          "border-transparent bg-accent text-black hover:bg-[color-mix(in_oklch,oklch(73%_0.13_190),black_10%)] focus-visible:outline-accent",
        secondary:
          "border-white/15 bg-white/5 text-fg hover:border-white/25 hover:bg-white/10 focus-visible:outline-accent",
      },
    },
    defaultVariants: {
      variant: "secondary",
    },
  },
);

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants>;

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, ...props }, ref) => (
    <button ref={ref} className={cn(buttonVariants({ variant, className }))} {...props} />
  ),
);
Button.displayName = "Button";
```

Create `apps/web/src/components/ui/cn.ts`:

```ts
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

- [ ] **Step 7: Implement App and styles**

Create `apps/web/src/app/App.tsx`:

```tsx
import { Button } from "../components/ui/button";
import { zhCN } from "../i18n/zh-CN";
import "../styles/globals.css";

export function App() {
  const text = zhCN.foundation;

  return (
    <main aria-label={text.mainLabel} className="min-h-screen bg-bg px-8 py-10 text-fg">
      <section className="mx-auto grid max-w-5xl gap-6 border border-white/15 bg-white/[0.04] p-8">
        <p className="text-xs font-semibold uppercase tracking-[0.08em] text-muted">{text.eyebrow}</p>
        <div className="grid gap-3">
          <h1 className="text-3xl font-semibold tracking-normal">{text.title}</h1>
          <p className="max-w-2xl text-sm leading-7 text-muted">{text.summary}</p>
        </div>
        <div>
          <Button variant="primary">{text.primaryAction}</Button>
        </div>
      </section>
    </main>
  );
}
```

Create `apps/web/src/styles/globals.css`:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  color-scheme: dark;
  font-family:
    -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Microsoft YaHei", sans-serif;
  background: oklch(10% 0.018 245);
}

body {
  margin: 0;
  min-width: 320px;
  min-height: 100vh;
}

button {
  font: inherit;
}
```

Create `apps/web/src/main.tsx`:

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./app/App";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

- [ ] **Step 8: Run test to verify GREEN**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/app/App.test.tsx
```

Expected: PASS.

- [ ] **Step 9: Run frontend typecheck and build**

Run:

```bash
pnpm --filter @talentpilot/web typecheck
pnpm --filter @talentpilot/web build
```

Expected: both commands PASS.

- [ ] **Step 10: Commit**

```bash
git add apps/web package.json pnpm-lock.yaml
git commit -m "feat(web): add react foundation shell"
```

## Task 7: Generated Frontend API Client Package

**Files:**
- Create: `packages/api-client/package.json`
- Create: `packages/api-client/src/index.ts`
- Generate: `packages/api-client/src/schema.d.ts`

- [ ] **Step 1: Create API client package**

Create `packages/api-client/package.json`:

```json
{
  "name": "@talentpilot/api-client",
  "private": true,
  "type": "module",
  "main": "./src/index.ts",
  "types": "./src/index.ts",
  "scripts": {
    "generate": "openapi-typescript ../api-contract/openapi.json -o src/schema.d.ts",
    "typecheck": "tsc --noEmit",
    "test": "tsc --noEmit",
    "build": "tsc --noEmit",
    "lint": "tsc --noEmit"
  },
  "dependencies": {
    "openapi-fetch": "latest"
  },
  "devDependencies": {
    "openapi-typescript": "latest",
    "typescript": "latest"
  }
}
```

- [ ] **Step 2: Generate schema types**

Run:

```bash
pnpm install
pnpm --filter @talentpilot/api-client generate
```

Expected: `packages/api-client/src/schema.d.ts` is generated and includes `/healthz`.

- [ ] **Step 3: Implement typed client factory**

Create `packages/api-client/src/index.ts`:

```ts
import createClient from "openapi-fetch";
import type { paths } from "./schema";

export type TalentPilotClient = ReturnType<typeof createTalentPilotClient>;

export function createTalentPilotClient(baseUrl: string) {
  return createClient<paths>({
    baseUrl,
    credentials: "include",
  });
}
```

- [ ] **Step 4: Typecheck API client**

Run:

```bash
pnpm --filter @talentpilot/api-client typecheck
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/api-client package.json pnpm-lock.yaml
git commit -m "feat(contract): add generated frontend api client"
```

## Task 8: CI Workflow

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  pull_request:
  push:
    branches:
      - main

jobs:
  verify:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_DB: talentpilot_ci
          POSTGRES_USER: talentpilot
          POSTGRES_PASSWORD: talentpilot
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: apps/api/go.mod
          cache-dependency-path: apps/api/go.sum

      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: pnpm

      - uses: pnpm/action-setup@v4
        with:
          version: 10.13.1

      - name: Install frontend dependencies
        run: pnpm install --frozen-lockfile

      - name: Download backend dependencies
        run: cd apps/api && go mod download

      - name: Lint
        run: make lint

      - name: Typecheck
        run: make typecheck

      - name: Test
        run: make test

      - name: OpenAPI drift check
        run: make openapi-check

      - name: Client generation
        run: make client-generate

      - name: Build
        run: make build
```

- [ ] **Step 2: Verify CI commands locally**

Run:

```bash
make ci
```

Expected: PASS. If the local environment lacks Docker or PostgreSQL, `make ci` still passes because the current migration tests use SQLite; PostgreSQL migration validation is added in a later backend hardening task.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add foundation verification workflow"
```

## Task 9: Container Baseline

**Files:**
- Create: `apps/api/Dockerfile`
- Create: `apps/web/Dockerfile`
- Create: `docker-compose.yml`

- [ ] **Step 1: Create API Dockerfile**

Create `apps/api/Dockerfile`:

```Dockerfile
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api ./
RUN go build -o /out/talentpilot-api ./cmd/api

FROM alpine:3.22
WORKDIR /app
COPY --from=build /out/talentpilot-api /app/talentpilot-api
EXPOSE 8080
CMD ["/app/talentpilot-api"]
```

- [ ] **Step 2: Create Web Dockerfile**

Create `apps/web/Dockerfile`:

```Dockerfile
FROM node:24-alpine AS build

WORKDIR /src
RUN corepack enable
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/package.json
COPY packages/api-client/package.json packages/api-client/package.json
COPY packages/api-contract/package.json packages/api-contract/package.json
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm --filter @talentpilot/web build

FROM nginx:1.27-alpine
COPY --from=build /src/apps/web/dist /usr/share/nginx/html
EXPOSE 80
```

- [ ] **Step 3: Create docker compose**

Create `docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: talentpilot
      POSTGRES_USER: talentpilot
      POSTGRES_PASSWORD: talentpilot
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U talentpilot -d talentpilot"]
      interval: 10s
      timeout: 5s
      retries: 5

  api:
    build:
      context: .
      dockerfile: apps/api/Dockerfile
    environment:
      APP_ENV: development
      API_ADDR: ":8080"
      DATABASE_DRIVER: postgres
      DATABASE_DSN: "host=postgres user=talentpilot password=talentpilot dbname=talentpilot port=5432 sslmode=disable"
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy

  web:
    build:
      context: .
      dockerfile: apps/web/Dockerfile
    ports:
      - "5173:80"
    depends_on:
      - api
```

- [ ] **Step 4: Verify compose syntax**

Run:

```bash
docker compose config
```

Expected: Compose renders a valid configuration.

- [ ] **Step 5: Verify Docker builds**

Run:

```bash
docker compose build
```

Expected: API and web images build successfully.

- [ ] **Step 6: Commit**

```bash
git add apps/api/Dockerfile apps/web/Dockerfile docker-compose.yml
git commit -m "chore: add container baseline"
```

## Task 10: Agent Memory, Status, and ADR Updates

**Files:**
- Modify: `AGENTS.md`
- Modify: `docs/project-status.md`
- Create: `docs/adr/0006-use-huma-for-openapi-generation.md`

- [ ] **Step 1: Update AGENTS with implementation decisions**

Modify `AGENTS.md` stable decisions to include:

```markdown
- JavaScript package manager: pnpm.
- Runtime baseline: Node 24 LTS for frontend tooling and Go 1.26 for backend tooling.
- OpenAPI generation library: Huma with the Echo adapter.
- Output-size rule: Agents must not output or patch more than about 1000 lines of code in one response, one patch, or one implementation step unless explicitly approved.
```

Modify command index so each listed command has confirmed behavior after implementation. Keep command names from the existing Makefile.

- [ ] **Step 2: Add ADR for Huma**

Create `docs/adr/0006-use-huma-for-openapi-generation.md`:

```markdown
# ADR 0006: Use Huma Echo Adapter for OpenAPI Generation

## Status

Accepted

## Context

TalentPilot uses Echo for the backend and requires backend code to generate OpenAPI for frontend client generation. Plain Echo does not provide OpenAPI generation by itself.

## Decision

Use Huma with the Echo adapter for API operation registration and OpenAPI generation. Echo remains the HTTP server and middleware foundation. Huma provides typed operation registration and OpenAPI output.

## Consequences

- API routes can stay code-first while producing OpenAPI.
- Handlers must follow Huma input/output conventions.
- Business services remain independent from Huma and Echo.
- Future route additions must update generated OpenAPI and generated clients through CI drift checks.
```

- [ ] **Step 3: Update project status**

Modify `docs/project-status.md` foundation rows:

```markdown
| Repository | Monorepo skeleton | Done | Root workspace, Makefile, package workspace, app/package folders exist. | Keep command index current. |
| Frontend | React/Vite app | Done | `apps/web` builds and tests. | Start UI design-system SPEC. |
| Backend | Go/Echo API app | Done | `apps/api` health endpoint tests pass. | Start auth/session SPEC. |
| Backend | goose migrations | Done | SQLite migration integration test passes. | Add PostgreSQL migration CI hardening. |
| API Contract | Backend-generated OpenAPI | Done | `packages/api-contract/openapi.json` generated from backend. | Add PRD endpoint contracts by SPEC. |
| API Contract | Generated frontend client | Done | `packages/api-client` generated from OpenAPI. | Consume from web API wrapper. |
| Testing | Backend tests | Done | `go test ./...` passes in `apps/api`. | Add domain tests per business SPEC. |
| Testing | Frontend tests | Done | `pnpm --filter @talentpilot/web test -- --run` passes. | Add component tests per page. |
| CI | Quality gate workflow | Done | `.github/workflows/ci.yml` exists and `make ci` passes locally. | Harden PostgreSQL migration check. |
| Containers | Local compose stack | Done | `docker compose config` and build pass. | Add storage/cache when needed. |
```

Keep any row that was not actually completed as `Not Started` or `In Progress`. Do not mark rows Done unless the evidence exists.

- [ ] **Step 4: Run documentation consistency checks**

Run:

```bash
python3 - <<'PY'
from pathlib import Path
markers = ["TB" + "D", "TO" + "DO", "FIX" + "ME", "待" + "定"]
for path in [Path("AGENTS.md"), *Path("docs").rglob("*.md")]:
    text = path.read_text()
    for marker in markers:
        if marker in text:
            print(f"{path}: contains unresolved marker {marker}")
PY
git status --short
```

Expected: no incomplete-marker matches. `git status` shows only intentional changes.

- [ ] **Step 5: Run full verification**

Run:

```bash
make ci
docker compose config
```

Expected: both pass.

- [ ] **Step 6: Commit**

```bash
git add AGENTS.md docs/project-status.md docs/adr/0006-use-huma-for-openapi-generation.md
git commit -m "docs: update foundation implementation memory"
```

## Self-Review

Spec coverage:

- Repository structure is covered by Task 1.
- Agent memory is covered by Task 10.
- System architecture is represented by separate `apps/web`, `apps/api`, `packages/api-contract`, and `packages/api-client`.
- Backend architecture is covered by Tasks 2, 3, 4, and 5.
- Frontend architecture is covered by Task 6.
- API contract generation is covered by Tasks 5 and 7.
- Database and migrations are covered by Task 4.
- Error codes and i18n are covered by Tasks 3 and 6.
- TDD and CI are covered by all behavior tasks and Task 8.
- Container strategy is covered by Task 9.
- Project status checklist is covered by Task 10.

Known intentional gaps:

- No PRD business Story is implemented.
- No W3 mock/session flow is implemented.
- No IAM `user_can()` behavior is implemented.
- No PostgreSQL migration test is implemented beyond CI service availability; add this in backend hardening or IAM plan.
- No Playwright test is implemented because there is no business flow yet.

Incomplete-marker scan:

- The plan contains no unresolved marker words or open-ended sections.

Type consistency:

- Backend server constructor is consistently `NewServer()`.
- Backend server fields are consistently `Echo` and `API`.
- OpenAPI artifact path is consistently `packages/api-contract/openapi.json`.
- API client generated schema path is consistently `packages/api-client/src/schema.d.ts`.
