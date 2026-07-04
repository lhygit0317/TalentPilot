# E4 Resume Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the E4 resume library from `docs/specs/003-resume-library-import.md`: permission-filtered listing, detail, delete, single/batch PDF import jobs, generated API contract/client updates, and frontend resume library workflows.

**Architecture:** Add a focused backend `internal/resume` package that owns resume DTOs, parser boundary, SQL repository, import job service, and business validation. App route handlers remain thin: session/CSRF/IAM guard first, then delegate to the resume service and translate stable errors. Frontend extends the existing shell to render a `resume-library` page from generated client calls; backend IAM predicates remain the security boundary.

**Tech Stack:** Go 1.26, Echo, Huma, GORM, goose, SQLite/PostgreSQL-compatible SQL, React, TypeScript, Vite, Vitest, Testing Library, generated OpenAPI and `@talentpilot/api-client`.

---

## Preconditions

Do not execute E4 production-code tasks until the IAM implementation plan has completed and committed:

- `docs/superpowers/plans/2026-07-03-iam-permission-model-implementation.md`
- `apps/api/internal/iam` exists with `ResourceResume`, `ActionList`, `ActionGet`, `ActionCreate`, `ActionDelete`, `ResourceDepartmentResume`, `ResourceJob`, `ScopePredicate`, `DataScope`, `RoleSummary`, `Principal`, `Decision`
- `apps/api/internal/app` exposes route guard and scope helpers equivalent to `RequirePermission`, `RequireAuthenticated`, `AuthenticatedUserFromContext`, and `ScopePredicateFromContext`
- `/me` response includes `permissions`, `dataScope`, and `pageAccess`

If these names differ after IAM implementation, first update this plan to the actual names in the same small documentation commit. Do not bypass the IAM guard or implement role-label checks in E4.

## Planned File Structure

Backend:

- Create `apps/api/internal/resume/types.go`: public DTOs, request structs, error sentinels, parser/service interfaces.
- Create `apps/api/internal/resume/parser.go`: PDF parser interface and deterministic test parser helpers.
- Create `apps/api/internal/resume/sql_store.go`: GORM-backed resume and import-job persistence; translates IAM scope predicates into GORM clauses.
- Create `apps/api/internal/resume/service.go`: list/detail/delete/import orchestration, target department inference, audit hooks, parser calls.
- Create `apps/api/internal/audit/sql_recorder.go`: persist non-sensitive resume audit events into `audit_logs` if IAM has not already added a SQL recorder.
- Create `apps/api/internal/app/resume_routes.go`: Huma route registration for `/resumes`, `/resumes/{resumeId}`, import endpoints, and `/jobs/{jobId}`.
- Modify `apps/api/internal/app/server.go`: wire `ResumeService` and register resume routes.
- Modify `apps/api/cmd/api/main.go`: construct resume SQL store/service/parser and pass it into app options.
- Modify `apps/api/internal/http/apperror/error.go`: add stable E4 error codes/messages.
- Create migration `apps/api/migrations/000004_add_resume_import_job_metadata.sql`: job ownership and JSON result metadata.
- Add tests in `apps/api/internal/resume/*_test.go`, `apps/api/internal/app/resume_routes_test.go`, and `apps/api/internal/app/openapi_test.go`.

Frontend:

- Create `apps/web/src/resume-library/types.ts`: UI-facing resume library types and helpers.
- Create `apps/web/src/resume-library/highlight.tsx`: escaped literal highlighting helper.
- Create `apps/web/src/resume-library/ResumeLibraryPage.tsx`: page composition, table, detail drawer, import controls, toasts.
- Create `apps/web/src/resume-library/ResumeLibraryPage.test.tsx`: page behavior tests.
- Modify `apps/web/src/api/client.ts`: add typed wrappers around generated resume/job endpoints.
- Modify `apps/web/src/app/App.tsx`: route `resume-library` to the page and pass session summary.
- Modify `apps/web/src/app/App.test.tsx`: navigation/page access tests.
- Modify `apps/web/src/i18n/zh-CN.ts` and `apps/web/src/i18n/en-US.ts`: labels/errors/toasts.
- Regenerate `packages/api-contract/openapi.json` and `packages/api-client/src/schema.d.ts`.

Docs:

- Modify `docs/project-status.md`: move E4 implementation state and evidence.
- Modify `AGENTS.md`: keep E4 implementation plan in the documentation index.

## Task 0: Verify IAM Prerequisite and Baseline

**Files:**
- Read: `docs/specs/002-iam-permission-model.md`
- Read: `docs/specs/003-resume-library-import.md`
- Read: `docs/superpowers/plans/2026-07-03-iam-permission-model-implementation.md`
- Read: `apps/api/internal/app/server.go`
- Read: `apps/api/internal/iam`

- [ ] **Step 1: Confirm the branch and prerequisite files**

Run:

```bash
git branch --show-current
test -d apps/api/internal/iam
test -f docs/specs/003-resume-library-import.md
```

Expected: branch is not `main`; both `test` commands exit 0.

- [ ] **Step 2: Confirm IAM APIs required by E4**

Run:

```bash
rg -n "type ScopePredicate|ResourceResume|ActionList|RequirePermission|ScopePredicateFromContext|AuthenticatedUserFromContext" apps/api/internal/iam apps/api/internal/app
```

Expected: output shows the IAM package and app guard helpers from the IAM implementation. If any required API is absent, stop and finish IAM first.

- [ ] **Step 3: Run backend and frontend baseline**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make test-api
pnpm --filter @talentpilot/web test -- --run
```

Expected: both commands PASS before E4 changes.

## Task 1: Resume Import Job Migration and Error Codes

**Files:**
- Create: `apps/api/migrations/000004_add_resume_import_job_metadata.sql`
- Modify: `apps/api/test/integration/migrations_test.go`
- Modify: `apps/api/internal/http/apperror/error.go`
- Test: `apps/api/internal/http/apperror/error_test.go`

- [ ] **Step 1: Write failing migration and error tests**

Add integration assertions:

```go
func TestResumeImportJobMigrationAddsOwnershipAndResultMetadata(t *testing.T) {
	database := newMigratedSQLiteDB(t)
	assertColumnExists(t, database, "jobs", "created_by_user_id")
	assertColumnExists(t, database, "jobs", "result_json")
}
```

Add error-code assertions:

```go
func TestResumeLibraryErrorCodesUseStableMessages(t *testing.T) {
	cases := []struct {
		code   Code
		status int
	}{
		{ResumeNotFound, http.StatusNotFound},
		{ResumeImportFileTooLarge, http.StatusUnprocessableEntity},
		{ResumeImportUnsupportedType, http.StatusUnprocessableEntity},
		{ResumeImportTargetDepartmentRequired, http.StatusUnprocessableEntity},
		{ResumeImportTargetDepartmentInvalid, http.StatusUnprocessableEntity},
		{ResumeImportParseFailed, http.StatusUnprocessableEntity},
		{ResumeImportEmptyFile, http.StatusUnprocessableEntity},
		{ResumeDeleteDenied, http.StatusForbidden},
		{JobNotFound, http.StatusNotFound},
		{JobAccessDenied, http.StatusForbidden},
	}
	for _, tc := range cases {
		problem := NewProblem(tc.code, "", "req_1", nil)
		if problem.GetStatus() != tc.status {
			t.Fatalf("%s status=%d want %d", tc.code, problem.GetStatus(), tc.status)
		}
		if problem.Message == "" || problem.RequestID != "req_1" {
			t.Fatalf("expected message and request id for %s: %#v", tc.code, problem)
		}
	}
}
```

- [ ] **Step 2: Run red tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./test/integration ./internal/http/apperror -run 'TestResumeImportJobMigrationAddsOwnershipAndResultMetadata|TestResumeLibraryErrorCodesUseStableMessages' -count=1
```

Expected: FAIL because migration columns and constants are missing.

- [ ] **Step 3: Add migration**

Create `apps/api/migrations/000004_add_resume_import_job_metadata.sql`:

```sql
-- +goose Up
ALTER TABLE jobs ADD COLUMN created_by_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN result_json TEXT NOT NULL DEFAULT '{}';
CREATE INDEX idx_jobs_created_by_user ON jobs(created_by_user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_created_by_user;
ALTER TABLE jobs DROP COLUMN result_json;
ALTER TABLE jobs DROP COLUMN created_by_user_id;
```

- [ ] **Step 4: Add error codes**

Add constants:

```go
ResumeNotFound                         Code = "RESUME_NOT_FOUND"
ResumeImportFileTooLarge               Code = "RESUME_IMPORT_FILE_TOO_LARGE"
ResumeImportUnsupportedType            Code = "RESUME_IMPORT_UNSUPPORTED_TYPE"
ResumeImportTargetDepartmentRequired   Code = "RESUME_IMPORT_TARGET_DEPARTMENT_REQUIRED"
ResumeImportTargetDepartmentInvalid    Code = "RESUME_IMPORT_TARGET_DEPARTMENT_INVALID"
ResumeImportParseFailed                Code = "RESUME_IMPORT_PARSE_FAILED"
ResumeImportEmptyFile                  Code = "RESUME_IMPORT_EMPTY_FILE"
ResumeDeleteDenied                     Code = "RESUME_DELETE_DENIED"
JobNotFound                            Code = "JOB_NOT_FOUND"
JobAccessDenied                        Code = "JOB_ACCESS_DENIED"
```

Map `ResumeNotFound` and `JobNotFound` to 404, `ResumeDeleteDenied` and `JobAccessDenied` to 403, and import validation failures to 422 with Chinese fallback messages.

- [ ] **Step 5: Run green tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./test/integration ./internal/http/apperror -run 'TestResumeImportJobMigrationAddsOwnershipAndResultMetadata|TestResumeLibraryErrorCodesUseStableMessages' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/migrations/000004_add_resume_import_job_metadata.sql apps/api/test/integration/migrations_test.go apps/api/internal/http/apperror/error.go apps/api/internal/http/apperror/error_test.go
git commit -m "feat(api): prepare resume import job metadata"
```

## Task 2: Resume SQL Store and Scope-Pushed Queries

**Files:**
- Create: `apps/api/internal/resume/types.go`
- Create: `apps/api/internal/resume/sql_store.go`
- Test: `apps/api/internal/resume/sql_store_test.go`

- [ ] **Step 1: Write failing SQL store tests**

Create tests:

```go
func TestSQLStoreListAppliesDepartmentScopeAndChannelCounts(t *testing.T)
func TestSQLStoreListAppliesChannelAttributeScope(t *testing.T)
func TestSQLStoreSearchIsLiteralCaseInsensitiveAndScoped(t *testing.T)
func TestSQLStoreGetRejectsOutOfScopeResume(t *testing.T)
func TestSQLStoreDeleteCascadesResumeRelationsButPreservesNotifications(t *testing.T)
```

Use migrated SQLite. Seed two departments, three resumes, three `department_resumes`, one `position_resumes`, and one `notifications` row. Use IAM scope predicates equivalent to:

```go
iam.ScopePredicate{
	Resource: iam.ResourceResume,
	Action:   iam.ActionList,
	Branches: []iam.ScopeBranch{{
		DepartmentIDs: []string{"dept_a"},
		Channels:      []string{"social"},
	}},
}
```

Assert the list query returns only scoped rows, counts only authorized rows, search treats `C++` as literal text, get returns `resume.ErrNotFound` for out-of-scope rows, and delete preserves notifications.

- [ ] **Step 2: Run red tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/resume -run 'TestSQLStore' -count=1
```

Expected: FAIL because `internal/resume` does not exist.

- [ ] **Step 3: Add resume types**

Create DTOs and store contracts:

```go
package resume

import (
	"context"
	"errors"
	"time"

	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

var (
	ErrNotFound                 = errors.New("resume not found")
	ErrImportTargetRequired     = errors.New("resume import target department required")
	ErrImportTargetInvalid      = errors.New("resume import target department invalid")
	ErrUnsupportedFileType      = errors.New("resume import unsupported file type")
	ErrFileTooLarge             = errors.New("resume import file too large")
	ErrEmptyFile                = errors.New("resume import empty file")
	ErrParseFailed              = errors.New("resume import parse failed")
	ErrJobNotFound              = errors.New("job not found")
	ErrJobAccessDenied          = errors.New("job access denied")
)

type Channel string
const (
	ChannelSocial Channel = "social"
	ChannelCampus Channel = "campus"
)

type Source string
const (
	SourceImported    Source = "导入"
	SourceRecommended Source = "推荐"
)

type DepartmentSummary struct { ID, Name string }
type ListQuery struct { Channel Channel; Search string; Limit int; Cursor string; Scope iam.ScopePredicate; GetScope iam.ScopePredicate; DeleteScope iam.ScopePredicate }
type ListResult struct { Items []ListItem; ChannelCounts map[Channel]int; AvailableChannels []Channel; DataScopeSummary string; NextCursor string }
type ListItem struct { ID, Name, School, Pos, SourceBy string; Age *int; YearsExp *float64; CurrentDepartment DepartmentSummary; Source Source; Channel Channel; Keywords []string; CanGet bool; CanDelete bool }
type Detail struct { ListItem; CreatedAt time.Time; Expired bool; Profile Profile }
type Profile struct { Basic map[string]any; Education []map[string]any; WorkExperience []map[string]any; Projects []map[string]any; Skills []string; Certificates []string; RawTextRef string }
```

- [ ] **Step 4: Implement SQL store**

Create:

```go
type SQLStore struct { db *gorm.DB }
func NewSQLStore(db *gorm.DB) *SQLStore
func (s *SQLStore) List(ctx context.Context, query ListQuery) (ListResult, error)
func (s *SQLStore) Get(ctx context.Context, id string, scope iam.ScopePredicate) (Detail, error)
func (s *SQLStore) Delete(ctx context.Context, id string, scope iam.ScopePredicate) error
```

Translate each scope branch to grouped GORM clauses:

```go
branch := db.Where("department_resumes.department_id IN ?", branch.DepartmentIDs)
if len(branch.Channels) > 0 { branch = branch.Where("resumes.chan IN ?", branch.Channels) }
if len(branch.Expired) > 0 { branch = branch.Where("resumes.expired IN ?", branch.Expired) }
```

Combine branches with OR. Apply search after scope:

```go
like := "%" + strings.ToLower(search) + "%"
query = query.Where("(lower(resumes.name) LIKE ? OR lower(resumes.pos) LIKE ? OR lower(resumes.keywords) LIKE ?)", like, like, like)
```

- [ ] **Step 5: Run green tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/resume -run 'TestSQLStore' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/resume/types.go apps/api/internal/resume/sql_store.go apps/api/internal/resume/sql_store_test.go
git commit -m "feat(api): query resumes with IAM scope"
```

## Task 3: Resume Service, Parser Boundary, and Import Jobs

**Files:**
- Modify: `apps/api/internal/audit/audit.go`
- Create: `apps/api/internal/audit/sql_recorder.go`
- Test: `apps/api/internal/audit/sql_recorder_test.go`
- Create: `apps/api/internal/resume/parser.go`
- Create: `apps/api/internal/resume/service.go`
- Modify: `apps/api/internal/resume/sql_store.go`
- Test: `apps/api/internal/resume/service_test.go`

- [ ] **Step 1: Write failing service tests**

Create tests:

```go
func TestServiceSingleImportInfersOnlyConcreteDepartment(t *testing.T)
func TestServiceSingleImportRequiresTargetForMultipleDepartments(t *testing.T)
func TestServiceSingleImportRejectsInvalidFileBeforeParser(t *testing.T)
func TestServiceSingleImportCreatesResumeAndDepartmentResumeInTransaction(t *testing.T)
func TestServiceParseFailureCreatesNoResumeOwnershipRowsAndMarksJobFailed(t *testing.T)
func TestServiceBatchImportRecordsPerFileResults(t *testing.T)
func TestServiceGetJobRejectsDifferentOwner(t *testing.T)
func TestServiceWritesAuditForImportAndDeleteWithoutSensitivePayloads(t *testing.T)
func TestSQLRecorderWritesResumeAuditRows(t *testing.T)
```

Use a fake parser:

```go
type fakeParser struct { calls int; result resume.ParsedResume; err error }
func (f *fakeParser) Parse(ctx context.Context, input resume.ParseInput) (resume.ParsedResume, error) {
	f.calls++
	return f.result, f.err
}
```

- [ ] **Step 2: Run red tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/resume -run 'TestService' -count=1
```

Expected: FAIL because service/parser are missing.

- [ ] **Step 3: Add parser and service contracts**

Create:

```go
type ParseInput struct { FileName string; ContentType string; Bytes []byte }
type ParsedResume struct { NormalizedName, Name, School, Pos string; Age *int; YearsExp *float64; Keywords []string; Traits []string; ExpBase int; Profile Profile }
type Parser interface { Parse(context.Context, ParseInput) (ParsedResume, error) }
type ImportFile struct { FileName string; ContentType string; Bytes []byte }
type ImportInput struct { UserID string; UserName string; Channel Channel; TargetDepartmentID string; DataScope iam.DataScope; File ImportFile }
type BatchImportInput struct { UserID string; UserName string; Channel Channel; TargetDepartmentID string; DataScope iam.DataScope; Files []ImportFile }
type JobStatus struct { ID, Type, Status string; Summary JobSummary; Results []JobResult }
```

- [ ] **Step 4: Extend audit event contract and SQL recorder**

Extend `audit.Event` with resource audit fields while preserving existing auth fields:

```go
const (
	EventResumeImportSucceeded EventType = "resume.import_succeeded"
	EventResumeImportFailed    EventType = "resume.import_failed"
	EventResumeDeleted         EventType = "resume.deleted"
)

type Event struct {
	Type             EventType
	UserID           string
	Account          string
	Code             string
	RequestID        string
	ActorUserID      string
	ActorEmployeeID  string
	ActorRoleSummary string
	Resource         string
	Action           string
	TargetID         string
	Result           string
	Before           map[string]any
	After            map[string]any
	At               time.Time
}
```

Create `SQLRecorder.Record`:

```go
type SQLRecorder struct { db *gorm.DB }
func NewSQLRecorder(db *gorm.DB) *SQLRecorder { return &SQLRecorder{db: db} }
func (r *SQLRecorder) Record(ctx context.Context, event Event) error {
	before, _ := json.Marshal(safeAuditMap(event.Before))
	after, _ := json.Marshal(safeAuditMap(event.After))
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO audit_logs (id, request_id, actor_user_id, actor_employee_id, actor_role_summary, resource, action, target_id, result, before_value, after_value, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newAuditID(), event.RequestID, event.ActorUserID, event.ActorEmployeeID, event.ActorRoleSummary, event.Resource, event.Action, event.TargetID, event.Result, string(before), string(after), event.At).Error
}
func newAuditID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil { return "audit_" + strconv.FormatInt(time.Now().UnixNano(), 10) }
	return "audit_" + hex.EncodeToString(b[:])
}
```

`safeAuditMap` must drop keys named `rawText`, `rawTextRef`, `pdf`, `profile`, `phone`, `email`, `idCard`, and `content`.

- [ ] **Step 5: Implement validation and target resolution**

Validation rules:

```go
const maxPDFBytes = 10 * 1024 * 1024
func validatePDF(file ImportFile) error {
	if len(file.Bytes) == 0 { return ErrEmptyFile }
	if len(file.Bytes) > maxPDFBytes { return ErrFileTooLarge }
	nameOK := strings.HasSuffix(strings.ToLower(file.FileName), ".pdf")
	typeOK := file.ContentType == "application/pdf" || file.ContentType == "application/octet-stream"
	if !nameOK || !typeOK { return ErrUnsupportedFileType }
	return nil
}
```

Target resolution:

```go
func resolveTargetDepartment(inputTarget string, scope iam.DataScope) (string, error) {
	if inputTarget != "" && scope.AllDepartments { return inputTarget, nil }
	allowed := nonSystemDepartmentIDs(scope.Departments)
	if inputTarget == "" {
		if len(allowed) == 1 { return allowed[0], nil }
		return "", ErrImportTargetRequired
	}
	if slices.Contains(allowed, inputTarget) { return inputTarget, nil }
	return "", ErrImportTargetInvalid
}
```

- [ ] **Step 6: Implement import job persistence**

Add store methods:

```go
func (s *SQLStore) CreateImportJob(ctx context.Context, ownerUserID string, batch bool, input ImportJobInput) (string, error)
func (s *SQLStore) MarkImportJobSucceeded(ctx context.Context, jobID string, result JobStatus) error
func (s *SQLStore) MarkImportJobFailed(ctx context.Context, jobID string, code string, result JobStatus) error
func (s *SQLStore) GetJob(ctx context.Context, jobID string, ownerUserID string) (JobStatus, error)
func (s *SQLStore) CreateImportedResume(ctx context.Context, input CreateImportedResumeInput) (string, error)
```

`CreateImportedResume` must wrap `INSERT INTO resumes` and `INSERT INTO department_resumes` in one transaction.

- [ ] **Step 7: Record audit events from service methods**

On successful import, record:

```go
audit.Event{
	Type: audit.EventResumeImportSucceeded, Resource: "Resume", Action: "Create",
	TargetID: resumeID, Result: "succeeded",
	After: map[string]any{"resumeId": resumeID, "chan": input.Channel, "targetDepartmentId": targetDepartmentID, "jobId": jobID},
}
```

On parse failure, record `audit.EventResumeImportFailed` with `Result: "failed"` and safe `errorCode`. On delete, record `audit.EventResumeDeleted` with `Resource: "Resume"`, `Action: "Delete"`, `TargetID: resumeID`, and no profile or raw-text data.

- [ ] **Step 8: Run green tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/resume ./internal/audit -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/resume apps/api/internal/audit
git commit -m "feat(api): import resumes through jobs"
```

## Task 4: Resume Routes and OpenAPI Contract

**Files:**
- Create: `apps/api/internal/app/resume_routes.go`
- Modify: `apps/api/internal/app/server.go`
- Modify: `apps/api/cmd/api/main.go`
- Modify: `apps/api/internal/app/openapi_test.go`
- Test: `apps/api/internal/app/resume_routes_test.go`

- [ ] **Step 1: Write failing route tests**

Create tests:

```go
func TestResumeListRequiresAuthAndPermission(t *testing.T)
func TestResumeListPassesScopePredicateToService(t *testing.T)
func TestResumeDetailMapsUnauthorizedToIAMDenied(t *testing.T)
func TestResumeImportRequiresCSRFAndCreatePermissions(t *testing.T)
func TestResumeDeleteMapsStableErrors(t *testing.T)
func TestJobStatusRequiresCurrentUserOwnership(t *testing.T)
func TestOpenAPIDocumentIncludesResumeAndJobEndpoints(t *testing.T)
```

Use fake auth, fake IAM, and fake resume service. Assert operation IDs:

```text
get-resumes
get-resume
post-resume-import
post-resume-batch-import
delete-resume
get-job
```

- [ ] **Step 2: Run red tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app -run 'TestResume|TestJob|TestOpenAPIDocumentIncludesResume' -count=1
```

Expected: FAIL because routes are not registered.

- [ ] **Step 3: Add app service interface**

Extend `Options`:

```go
ResumeService ResumeService
```

Add:

```go
type ResumeService interface {
	List(context.Context, resume.ListQuery) (resume.ListResult, error)
	Get(context.Context, string, iam.ScopePredicate) (resume.Detail, error)
	Delete(context.Context, string, iam.ScopePredicate) error
	ImportOne(context.Context, resume.ImportInput) (resume.JobStatus, error)
	ImportBatch(context.Context, resume.BatchImportInput) (resume.JobStatus, error)
	GetJob(context.Context, string, string) (resume.JobStatus, error)
}
```

- [ ] **Step 4: Register Huma routes**

Register:

```go
GET /resumes
GET /resumes/{resumeId}
POST /resumes/imports
POST /resumes/batch-imports
DELETE /resumes/{resumeId}
GET /jobs/{jobId}
```

Each route must call existing IAM helpers:

```go
RequirePermission(iam.ResourceResume, iam.ActionList)
RequirePermission(iam.ResourceResume, iam.ActionGet)
RequirePermission(iam.ResourceResume, iam.ActionCreate)
RequirePermission(iam.ResourceResume, iam.ActionDelete)
RequirePermission(iam.ResourceJob, iam.ActionGet)
```

State-changing routes must rely on the existing authenticated mutation middleware for CSRF.

- [ ] **Step 5: Wire main**

In `apps/api/cmd/api/main.go`:

```go
resumeStore := resume.NewSQLStore(database)
resumeParser := resume.NewPDFParser()
resumeService := resume.NewService(resumeStore, resumeParser, audit.NopRecorder{})
```

Pass `ResumeService: resumeService` to `app.NewServerWithOptions`.

- [ ] **Step 6: Run green route tests**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app ./internal/resume -count=1
```

Expected: PASS.

- [ ] **Step 7: Regenerate and check OpenAPI**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-generate
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-check
```

Expected: generated contract is current and `openapi-check` exits 0.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/app/resume_routes.go apps/api/internal/app/server.go apps/api/cmd/api/main.go apps/api/internal/app/*test.go packages/api-contract/openapi.json
git commit -m "feat(api): expose resume library endpoints"
```

## Task 5: Generated Client and Frontend API Wrappers

**Files:**
- Modify: `packages/api-client/src/schema.d.ts`
- Modify: `apps/web/src/api/client.ts`
- Test: `apps/web/src/api/client.test.ts`

- [ ] **Step 1: Write failing frontend client wrapper tests**

Add tests:

```ts
it("lists resumes with channel and search parameters", async () => {
  const get = vi.fn().mockResolvedValue({ data: { items: [] }, error: undefined });
  vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ GET: get })) }));
  const { listResumes } = await import("./client");
  await listResumes({ chan: "social", search: "Go" });
  expect(get).toHaveBeenCalledWith("/resumes", { params: { query: { chan: "social", search: "Go" } } });
});
```

Cover `getResume`, `deleteResume`, `importResume`, `importResumesBatch`, and `getJob`.

- [ ] **Step 2: Run red tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts
```

Expected: FAIL because wrappers do not exist.

- [ ] **Step 3: Generate client types**

Run:

```bash
make client-generate
```

Expected: `packages/api-client/src/schema.d.ts` includes `/resumes`, `/resumes/{resumeId}`, `/resumes/imports`, `/resumes/batch-imports`, and `/jobs/{jobId}`.

- [ ] **Step 4: Implement wrappers**

Add wrappers:

```ts
export function listResumes(query: { chan?: "social" | "campus"; search?: string; limit?: number; cursor?: string }) {
  return apiClient.GET("/resumes", { params: { query } });
}
export function getResume(resumeId: string) {
  return apiClient.GET("/resumes/{resumeId}", { params: { path: { resumeId } } });
}
export function deleteResume(resumeId: string) {
  return apiClient.DELETE("/resumes/{resumeId}", { params: { path: { resumeId } } });
}
export function importResume(body: FormData) {
  return apiClient.POST("/resumes/imports", { body: body as never });
}
export function importResumesBatch(body: FormData) {
  return apiClient.POST("/resumes/batch-imports", { body: body as never });
}
export function getJob(jobId: string) {
  return apiClient.GET("/jobs/{jobId}", { params: { path: { jobId } } });
}
```

- [ ] **Step 5: Run green tests and typecheck**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts
pnpm --filter @talentpilot/api-client typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/api-client/src/schema.d.ts apps/web/src/api/client.ts apps/web/src/api/client.test.ts
git commit -m "feat(web): add resume API client wrappers"
```

## Task 6: Resume Library Frontend List, Search, and Detail

**Files:**
- Create: `apps/web/src/resume-library/types.ts`
- Create: `apps/web/src/resume-library/highlight.tsx`
- Create: `apps/web/src/resume-library/ResumeLibraryPage.tsx`
- Create: `apps/web/src/resume-library/ResumeLibraryPage.test.tsx`
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing frontend tests**

Add tests:

```ts
it("renders resume library navigation when page access includes resume-library", async () => {});
it("shows channel counts and the data scope banner", async () => {});
it("renders table columns without an avatar", async () => {});
it("preserves search focus and highlights escaped literal matches", async () => {});
it("opens detail and shows 未解析到 for empty sections", async () => {});
it("hides delete action when canDelete is false", async () => {});
```

Mock `listResumes` and `getResume` from `../api/client`.

- [ ] **Step 2: Run red tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/app/App.test.tsx src/resume-library/ResumeLibraryPage.test.tsx
```

Expected: FAIL because the page and route do not exist.

- [ ] **Step 3: Add highlight helper**

Implement literal escaping:

```tsx
export function highlightLiteral(value: string, query: string) {
  if (!query) return value;
  const index = value.toLocaleLowerCase().indexOf(query.toLocaleLowerCase());
  if (index < 0) return value;
  return (
    <>
      {value.slice(0, index)}
      <mark>{value.slice(index, index + query.length)}</mark>
      {value.slice(index + query.length)}
    </>
  );
}
```

- [ ] **Step 4: Add ResumeLibraryPage**

Implement page state:

```tsx
const [channel, setChannel] = React.useState<"social" | "campus">("social");
const [search, setSearch] = React.useState("");
const [list, setList] = React.useState<ResumeListResponse | null>(null);
const [detail, setDetail] = React.useState<ResumeDetail | null>(null);
```

Render channel tabs, search input, data-scope banner, table, detail drawer, and row actions using existing `Button`, `Input`, `Field`, and non-interactive semantic table elements.

- [ ] **Step 5: Wire App route labels**

Add route label:

```ts
"resume-library": zhCN.session.nav.resumeLibrary,
```

Render:

```tsx
{activePage === "resume-library" ? (
  <ResumeLibraryPage session={session} />
) : (
  <h1 className="text-2xl font-semibold tracking-normal">{routeLabels[activePage] ?? text.nav.resumeParse}</h1>
)}
```

- [ ] **Step 6: Run green tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/app/App.test.tsx src/resume-library/ResumeLibraryPage.test.tsx
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/app apps/web/src/resume-library apps/web/src/i18n
git commit -m "feat(web): render resume library"
```

## Task 7: Frontend Import and Delete Workflows

**Files:**
- Modify: `apps/web/src/resume-library/ResumeLibraryPage.tsx`
- Modify: `apps/web/src/resume-library/ResumeLibraryPage.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing workflow tests**

Add tests:

```ts
it("rejects non-PDF and files over 10 MB before upload", async () => {});
it("submits single import form data and shows imported toast after job success", async () => {});
it("submits batch import form data and shows success count toast", async () => {});
it("deletes a deletable resume and refreshes the list", async () => {});
it("shows stable translated errors for import failures", async () => {});
```

- [ ] **Step 2: Run red tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/resume-library/ResumeLibraryPage.test.tsx
```

Expected: FAIL because import/delete workflows are not implemented.

- [ ] **Step 3: Implement client-side upload validation**

Use:

```ts
const maxBytes = 10 * 1024 * 1024;
function validatePDF(file: File) {
  if (file.size > maxBytes) return "文件不能超过 10MB";
  if (file.type !== "application/pdf" && !file.name.toLocaleLowerCase().endsWith(".pdf")) return "仅支持 PDF 文件";
  return "";
}
```

- [ ] **Step 4: Implement job polling**

Use a bounded poll:

```ts
async function waitForJob(jobId: string) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    const { data, error } = await getJob(jobId);
    if (error) throw new Error(error.code);
    if (data?.status === "succeeded" || data?.status === "failed") return data;
    await new Promise((resolve) => window.setTimeout(resolve, 500));
  }
  throw new Error("JOB_TIMEOUT");
}
```

- [ ] **Step 5: Implement import and delete handlers**

Create `FormData` with `file/files`, `chan`, and `targetDepartmentId`. On successful single import show `✓ 已导入「{姓名}」并加入简历库`; on batch import show `已批量导入 {N} 份{渠道}简历`; on delete show `已删除该简历` and refresh the list.

- [ ] **Step 6: Run green tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/resume-library/ResumeLibraryPage.test.tsx
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/resume-library apps/web/src/i18n
git commit -m "feat(web): support resume import and delete"
```

## Task 8: Final Verification and Documentation Status

**Files:**
- Modify: `docs/project-status.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Run full verification**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make test-api
pnpm --filter @talentpilot/web test -- --run
make openapi-check
make client-check
make typecheck
make lint
make build
```

Expected: all commands PASS. If `make lint` or `make build` exposes unrelated pre-existing failures, document the exact command, failing output, and affected files before deciding whether to fix them.

- [ ] **Step 2: Update project status**

Set E4 to `Done` only after all verification commands pass. Evidence must list the commands from Step 1 and mention generated OpenAPI/client checks.

If Playwright is still not installed, leave E2E as not part of the passing gate and mention that `make test-e2e` remains reserved.

- [ ] **Step 3: Update AGENTS documentation index**

Ensure `AGENTS.md` includes:

- `docs/specs/003-resume-library-import.md`
- `docs/superpowers/plans/2026-07-04-e4-resume-library-implementation.md`

Keep IAM as an implemented prerequisite once IAM completion is reflected in status.

- [ ] **Step 4: Commit docs**

```bash
git add docs/project-status.md AGENTS.md
git commit -m "docs: mark E4 implementation complete"
```

## Scope Guardrails

- Do not implement E2 parsing workspace matching or interview questions.
- Do not implement E3 recommendation copies or notifications.
- Do not implement E5 department/position management.
- Do not implement E8 custom role management.
- Do not filter permissions in frontend state.
- Do not expose full resume PDF/raw text in logs, audit details, or OpenAPI response examples.
- Do not add production sample resumes or positions.

## Completion Criteria

This plan is complete when all tasks are committed, full verification passes, generated OpenAPI/client files are current, and `docs/project-status.md` records E4 as Done with evidence.
