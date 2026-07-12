# E7 Notification Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build E7 notification consumption: unread count, top bell dropdown, mark-all-read, single notification click-through, and resume-library recommendation banner.

**Architecture:** Add a focused `apps/api/internal/notification` package that only reads and updates the current user's notification rows. Expose Huma routes under `/notifications`, regenerate OpenAPI/client artifacts, add generated-client wrappers, and render a shell-level notification bell that passes a jump context into the existing resume library page.

**Tech Stack:** Go, Echo, Huma, GORM, goose migrations, React, TypeScript, Vite, Testing Library, lucide-react, shadcn/project UI wrappers, generated OpenAPI client.

---

## File Structure

- Create: `apps/api/migrations/000007_grant_notification_read_permissions.sql` for manager and trainee notification read/update permissions.
- Modify: `apps/api/test/integration/migrations_test.go` to assert E7 permission grants.
- Create: `apps/api/internal/notification/types.go` for DTOs, service inputs, and domain errors.
- Create: `apps/api/internal/notification/service.go` for limit normalization and store orchestration.
- Create: `apps/api/internal/notification/sql_store.go` for current-user unread count/list/read updates.
- Create: `apps/api/internal/notification/service_test.go`.
- Create: `apps/api/internal/notification/sql_store_test.go`.
- Create: `apps/api/internal/app/notification_routes.go`.
- Create: `apps/api/internal/app/notification_routes_test.go`.
- Modify: `apps/api/internal/app/server.go` to add `NotificationService` and register routes.
- Modify: `apps/api/cmd/api/main.go` to wire the notification SQL store and service.
- Modify: `apps/api/internal/http/apperror/error.go` for E7 stable error codes.
- Modify after generation: `packages/api-contract/openapi.json`.
- Modify after generation: `packages/api-client/src/schema.d.ts`.
- Modify: `apps/web/src/api/client.ts` and `apps/web/src/api/client.test.ts` for notification wrappers.
- Create: `apps/web/src/notifications/types.ts`.
- Create: `apps/web/src/notifications/NotificationBell.tsx`.
- Create: `apps/web/src/notifications/NotificationBell.test.tsx`.
- Modify: `apps/web/src/app/App.tsx` and `apps/web/src/app/App.test.tsx` for hash-route state, unread badge state, and resume-library jump context.
- Modify: `apps/web/src/resume-library/types.ts`, `apps/web/src/resume-library/ResumeLibraryPage.tsx`, and `apps/web/src/resume-library/ResumeLibraryPage.test.tsx` for notification jump handling.
- Modify: `apps/web/src/i18n/zh-CN.ts` and `apps/web/src/i18n/en-US.ts` for notification copy.
- Modify: `AGENTS.md` and `docs/project-status.md` after verification.

Generated OpenAPI and TypeScript schema changes are generated-artifact steps. They use generation and drift checks instead of behavior-first tests.

## Task 1: Permission Seed Migration

**Files:**
- Create: `apps/api/migrations/000007_grant_notification_read_permissions.sql`
- Modify: `apps/api/test/integration/migrations_test.go`

- [ ] **Step 1: Write the failing migration test**

In `assertPresetIAMSeeded`, add assertions:

```go
assertCount(t, database, "permissions", "role_id = '__role_manager__' AND resource = 'Notification' AND action = 'List'", 1)
assertCount(t, database, "permissions", "role_id = '__role_manager__' AND resource = 'Notification' AND action = 'Get'", 1)
assertCount(t, database, "permissions", "role_id = '__role_manager__' AND resource = 'Notification' AND action = 'Update'", 1)
assertCount(t, database, "permissions", "role_id = '__role_trainee__' AND resource = 'Notification' AND action = 'List'", 1)
assertCount(t, database, "permissions", "role_id = '__role_trainee__' AND resource = 'Notification' AND action = 'Get'", 1)
assertCount(t, database, "permissions", "role_id = '__role_trainee__' AND resource = 'Notification' AND action = 'Update'", 1)
```

Add a focused rollback test:

```go
func TestE7NotificationPermissionMigrationDownRemovesOnlyE7Grants(t *testing.T) {
	ctx := context.Background()
	database := openSQLite(t)
	provider := newMigrationProvider(t, database)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	if _, err := provider.DownTo(ctx, 6); err != nil {
		t.Fatalf("goose down to before E7 grants: %v", err)
	}

	assertCount(t, database, "permissions", "role_id = '__role_manager__' AND resource = 'Notification' AND action IN ('List','Get','Update')", 0)
	assertCount(t, database, "permissions", "role_id = '__role_manager__' AND resource = 'Notification' AND action = 'Create'", 1)
	assertCount(t, database, "permissions", "role_id = '__role_trainee__' AND resource = 'Notification' AND action IN ('List','Get','Update')", 0)
}
```

- [ ] **Step 2: Run migration test and verify RED**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./test/integration -run 'TestFoundationMigrationsCreateExpectedSchema|TestE7NotificationPermissionMigrationDownRemovesOnlyE7Grants' -count=1
```

Expected: FAIL because the E7 manager/trainee grants do not exist.

- [ ] **Step 3: Add the E7 migration**

Create `000007_grant_notification_read_permissions.sql`:

```sql
-- +goose Up
INSERT INTO permissions (id, role_id, resource, action, attribute_conditions, created_at)
VALUES
  ('__permission_manager_notification_list__', '__role_manager__', 'Notification', 'List', '{}', CURRENT_TIMESTAMP),
  ('__permission_manager_notification_get__', '__role_manager__', 'Notification', 'Get', '{}', CURRENT_TIMESTAMP),
  ('__permission_manager_notification_update__', '__role_manager__', 'Notification', 'Update', '{}', CURRENT_TIMESTAMP),
  ('__permission_trainee_notification_list__', '__role_trainee__', 'Notification', 'List', '{}', CURRENT_TIMESTAMP),
  ('__permission_trainee_notification_get__', '__role_trainee__', 'Notification', 'Get', '{}', CURRENT_TIMESTAMP),
  ('__permission_trainee_notification_update__', '__role_trainee__', 'Notification', 'Update', '{}', CURRENT_TIMESTAMP)
ON CONFLICT(role_id, resource, action, attribute_conditions) DO NOTHING;

-- +goose Down
DELETE FROM permissions
WHERE id IN (
  '__permission_manager_notification_list__',
  '__permission_manager_notification_get__',
  '__permission_manager_notification_update__',
  '__permission_trainee_notification_list__',
  '__permission_trainee_notification_get__',
  '__permission_trainee_notification_update__'
);
```

- [ ] **Step 4: Run migration test and verify GREEN**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./test/integration -run 'TestFoundationMigrationsCreateExpectedSchema|TestE7NotificationPermissionMigrationDownRemovesOnlyE7Grants' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/migrations/000007_grant_notification_read_permissions.sql apps/api/test/integration/migrations_test.go
git commit -m "feat(api): grant notification read permissions"
```

## Task 2: Notification Service and SQL Store

**Files:**
- Create: `apps/api/internal/notification/types.go`
- Create: `apps/api/internal/notification/service.go`
- Create: `apps/api/internal/notification/sql_store.go`
- Create: `apps/api/internal/notification/service_test.go`
- Create: `apps/api/internal/notification/sql_store_test.go`

- [ ] **Step 1: Write failing service tests**

Create tests covering:

- `Summary` forwards the current user ID and returns unread count.
- `ListUnread` defaults limit to 20 and caps limit at 50.
- `MarkRead` returns `ErrNotFound` when the store cannot find an owned row.

Use test names:

```go
func TestServiceSummaryReturnsUnreadCount(t *testing.T)
func TestServiceListUnreadNormalizesLimit(t *testing.T)
func TestServiceMarkReadMapsMissingOwnedNotification(t *testing.T)
```

- [ ] **Step 2: Run service tests and verify RED**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/notification -run TestService -count=1
```

Expected: FAIL because `internal/notification` does not exist.

- [ ] **Step 3: Implement minimal service contract**

Define these exported types in `types.go`:

```go
var ErrNotFound = errors.New("notification not found")

type Channel string
const (
	ChannelSocial Channel = "social"
	ChannelCampus Channel = "campus"
)

type DepartmentSummary struct { ID, Name string }
type PositionSummary struct { ID, Name string }
type UserSummary struct { ID, Name string }

type Item struct {
	ID                   string            `json:"id"`
	ResumeID             string            `json:"resumeId"`
	CandidateName        string            `json:"candidateName"`
	Department           DepartmentSummary `json:"department"`
	Position             *PositionSummary  `json:"position,omitempty"`
	Recommender          UserSummary       `json:"recommender"`
	Channel              Channel           `json:"chan"`
	CreatedAt            string            `json:"createdAt"`
	Read                 bool              `json:"read"`
	CanOpenResumeLibrary bool              `json:"canOpenResumeLibrary"`
}

type SummaryResult struct { UnreadCount int `json:"unreadCount"` }
type ListResult struct {
	Items       []Item `json:"items" nullable:"false"`
	UnreadCount int    `json:"unreadCount"`
	NextCursor  string `json:"nextCursor"`
}
type ReadAllResult struct { UpdatedCount, UnreadCount int }
type MarkReadResult struct {
	Notification Item `json:"notification"`
	UnreadCount  int  `json:"unreadCount"`
}
```

Use `Service` methods:

```go
Summary(ctx context.Context, userID string) (SummaryResult, error)
ListUnread(ctx context.Context, query ListQuery) (ListResult, error)
MarkAllRead(ctx context.Context, userID string) (ReadAllResult, error)
MarkRead(ctx context.Context, input MarkReadInput) (MarkReadResult, error)
```

- [ ] **Step 4: Write failing SQL store tests**

Create tests with SQLite/GORM setup that insert users, departments, positions, resumes, and notifications. Cover:

```go
func TestSQLStoreListUnreadFiltersCurrentUserAndSortsNewestFirst(t *testing.T)
func TestSQLStoreListUnreadUsesSafeFallbacks(t *testing.T)
func TestSQLStoreMarkAllReadUpdatesOnlyCurrentUser(t *testing.T)
func TestSQLStoreMarkReadIsOwnedAndIdempotent(t *testing.T)
```

- [ ] **Step 5: Run SQL store tests and verify RED**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/notification -run TestSQLStore -count=1
```

Expected: FAIL because SQL store methods are not implemented.

- [ ] **Step 6: Implement SQL store**

Implement `NewSQLStore(db *gorm.DB) *SQLStore`. SQL requirements:

- summary: `COUNT(*) FROM notifications WHERE to_user_id = ? AND read = FALSE`;
- list: current user, `read = FALSE`, `ORDER BY time DESC, id DESC`, `LIMIT ?`;
- enrichment: `LEFT JOIN departments`, `LEFT JOIN positions`, `LEFT JOIN users recommender`;
- mark all: `UPDATE notifications SET read = TRUE WHERE to_user_id = ? AND read = FALSE`;
- mark one: transactionally load `WHERE id = ? AND to_user_id = ?`, update `read = TRUE`, then return fresh unread count.

- [ ] **Step 7: Run package tests and verify GREEN**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/notification -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/notification
git commit -m "feat(api): add notification service"
```

## Task 3: Backend Routes and Error Mapping

**Files:**
- Create: `apps/api/internal/app/notification_routes.go`
- Create: `apps/api/internal/app/notification_routes_test.go`
- Modify: `apps/api/internal/app/server.go`
- Modify: `apps/api/cmd/api/main.go`
- Modify: `apps/api/internal/http/apperror/error.go`

- [ ] **Step 1: Write failing route tests**

Cover:

```go
func TestNotificationRoutesSummaryRequiresListPermission(t *testing.T)
func TestNotificationRoutesListPassesCanOpenResumeLibrary(t *testing.T)
func TestNotificationRoutesMarkAllRequiresUpdatePermission(t *testing.T)
func TestNotificationRoutesMarkReadRequiresGetAndUpdatePermission(t *testing.T)
func TestNotificationRoutesMarkReadMapsNotFound(t *testing.T)
func TestOpenAPIDocumentIncludesNotificationEndpoints(t *testing.T)
```

The route tests should use a fake `NotificationService` and existing fake auth/IAM helpers in `apps/api/internal/app`.

- [ ] **Step 2: Run route tests and verify RED**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app -run TestNotification -count=1
```

Expected: FAIL because routes and service wiring do not exist.

- [ ] **Step 3: Implement routes and server wiring**

Add `NotificationService` to `Options` and register:

- `GET /notifications/summary` with `Notification.List`;
- `GET /notifications` with `Notification.List`;
- `POST /notifications/read-all` with `Notification.Update`;
- `POST /notifications/{notificationId}/read` with `Notification.Get` and `Notification.Update`.

Set `CanOpenResumeLibrary` by checking the current principal has `Resume.List` scope. Do not trust any client parameter for this value.

- [ ] **Step 4: Add E7 error codes**

Add codes in `apperror/error.go`:

```go
NotificationAccessDenied Code = "NOTIFICATION_ACCESS_DENIED"
NotificationNotFound     Code = "NOTIFICATION_NOT_FOUND"
NotificationListFailed   Code = "NOTIFICATION_LIST_FAILED"
NotificationUpdateFailed Code = "NOTIFICATION_UPDATE_FAILED"
```

Map `NotificationNotFound` to 404, access denial to 403, and list/update failures to 500 with Chinese default messages from the SPEC.

- [ ] **Step 5: Wire production main**

In `apps/api/cmd/api/main.go`, create `notification.NewSQLStore(db)` and `notification.NewService(store)`, then pass it into `app.Options`.

- [ ] **Step 6: Run route and app tests**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app -run 'TestNotification|TestOpenAPIDocumentIncludesNotificationEndpoints' -count=1
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/app/notification_routes.go apps/api/internal/app/notification_routes_test.go apps/api/internal/app/server.go apps/api/cmd/api/main.go apps/api/internal/http/apperror/error.go
git commit -m "feat(api): expose notification endpoints"
```

## Task 4: Contract Generation and Frontend Client Wrappers

**Files:**
- Modify: `packages/api-contract/openapi.json`
- Modify: `packages/api-client/src/schema.d.ts`
- Modify: `apps/web/src/api/client.ts`
- Modify: `apps/web/src/api/client.test.ts`

- [ ] **Step 1: Generate OpenAPI**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-generate
```

Expected: `packages/api-contract/openapi.json` contains `/notifications/summary`, `/notifications`, `/notifications/read-all`, and `/notifications/{notificationId}/read`.

- [ ] **Step 2: Generate TypeScript client schema**

Run:

```bash
make client-generate
```

Expected: `packages/api-client/src/schema.d.ts` contains the notification paths.

- [ ] **Step 3: Write failing wrapper test**

Add a `client.test.ts` case that expects:

```ts
await getNotificationSummary();
await listNotifications({ limit: 20 });
await markAllNotificationsRead();
await markNotificationRead("notification_1");

expect(get).toHaveBeenCalledWith("/notifications/summary");
expect(get).toHaveBeenCalledWith("/notifications", { params: { query: { limit: 20 } } });
expect(post).toHaveBeenCalledWith("/notifications/read-all");
expect(post).toHaveBeenCalledWith("/notifications/{notificationId}/read", {
  params: { path: { notificationId: "notification_1" } },
});
```

- [ ] **Step 4: Run wrapper test and verify RED**

Run:

```bash
CI=true pnpm --filter @talentpilot/web exec vitest run src/api/client.test.ts
```

Expected: FAIL because wrappers do not exist.

- [ ] **Step 5: Add wrappers**

Add to `apps/web/src/api/client.ts`:

```ts
export function getNotificationSummary() {
  return apiClient.GET("/notifications/summary");
}

export function listNotifications(query: { limit?: number; cursor?: string } = {}) {
  return apiClient.GET("/notifications", { params: { query } });
}

export function markAllNotificationsRead() {
  return apiClient.POST("/notifications/read-all");
}

export function markNotificationRead(notificationId: string) {
  return apiClient.POST("/notifications/{notificationId}/read", { params: { path: { notificationId } } });
}
```

- [ ] **Step 6: Run wrapper test and contract checks**

Run:

```bash
CI=true pnpm --filter @talentpilot/web exec vitest run src/api/client.test.ts
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make openapi-check
make client-check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add packages/api-contract/openapi.json packages/api-client/src/schema.d.ts apps/web/src/api/client.ts apps/web/src/api/client.test.ts
git commit -m "feat(web): add notification client wrappers"
```

## Task 5: Notification Bell Component

**Files:**
- Create: `apps/web/src/notifications/types.ts`
- Create: `apps/web/src/notifications/NotificationBell.tsx`
- Create: `apps/web/src/notifications/NotificationBell.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing component tests**

Cover:

```ts
it("hides the badge when unread count is zero")
it("shows unread count and loads unread notifications on open")
it("marks all notifications read")
it("clicks one notification and returns jump context")
it("hides read actions without Notification.Update")
```

Mock `getNotificationSummary`, `listNotifications`, `markAllNotificationsRead`, and `markNotificationRead` from `../api/client`.

- [ ] **Step 2: Run component tests and verify RED**

Run:

```bash
CI=true pnpm --filter @talentpilot/web exec vitest run src/notifications/NotificationBell.test.tsx
```

Expected: FAIL because the component does not exist.

- [ ] **Step 3: Implement types and component**

`NotificationBell` props:

```ts
type NotificationBellProps = {
  canUpdate: boolean;
  onUnreadCountChange: (count: number) => void;
  onOpenResume: (jump: NotificationJumpContext) => void;
  unreadCount: number;
};
```

Use `Bell` from `lucide-react`, project `Button`, no raw business-page interactive HTML except through project UI wrappers. Render:

- button label `推荐提醒`;
- badge only when `unreadCount > 0`;
- dropdown title `推荐提醒(${unreadCount} 条未读)`;
- row primary text `{candidateName} 被推荐到「{department.name}」`;
- row secondary text `{recommender.name} · {relative or formatted time}`;
- empty text `暂无新的推荐提醒`;
- mark-all text `全部已读`.

- [ ] **Step 4: Add i18n copy**

Add `notifications` copy under both locale files. Chinese must include the exact PRD strings. English copy can be direct translation.

- [ ] **Step 5: Run component tests and verify GREEN**

Run:

```bash
CI=true pnpm --filter @talentpilot/web exec vitest run src/notifications/NotificationBell.test.tsx
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/notifications apps/web/src/i18n/zh-CN.ts apps/web/src/i18n/en-US.ts
git commit -m "feat(web): add notification bell"
```

## Task 6: App Shell Integration

**Files:**
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`

- [ ] **Step 1: Write failing app tests**

Add tests:

```ts
it("shows notification bell and resume-library nav badge for notification users")
it("does not show notification bell without Notification.List")
it("navigates to resume library when a notification is clicked")
it("keeps hash navigation in sync with visible page")
```

The notification click test should mock `markNotificationRead` to return a social-channel notification for `张三` and assert the resume library is rendered.

- [ ] **Step 2: Run app tests and verify RED**

Run:

```bash
CI=true pnpm --filter @talentpilot/web exec vitest run src/app/App.test.tsx -t "notification|hash"
```

Expected: FAIL because shell integration does not exist.

- [ ] **Step 3: Implement shell state**

In `App.tsx`:

- keep `activePage` in React state initialized from `window.location.hash || session.defaultRoute`;
- update active page on `hashchange`;
- render `NotificationBell` when session permissions include `Notification.List`;
- pass `canUpdate = permissions.includes("Notification.Update")`;
- keep `unreadCount` in app state;
- render the same badge next to the `resume-library` nav link;
- on notification jump, set `resumeLibraryJump`, set hash to `#resume-library`, and set active page to `resume-library`.

- [ ] **Step 4: Run app tests and verify GREEN**

Run:

```bash
CI=true pnpm --filter @talentpilot/web exec vitest run src/app/App.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/app/App.tsx apps/web/src/app/App.test.tsx
git commit -m "feat(web): integrate notifications in app shell"
```

## Task 7: Resume Library Jump Banner

**Files:**
- Modify: `apps/web/src/resume-library/types.ts`
- Modify: `apps/web/src/resume-library/ResumeLibraryPage.tsx`
- Modify: `apps/web/src/resume-library/ResumeLibraryPage.test.tsx`

- [ ] **Step 1: Write failing resume-library tests**

Add tests:

```ts
it("switches channel and search from a notification jump")
it("shows recommendation banner with candidate department and recommender")
```

Render `ResumeLibraryPage` with:

```ts
notificationJump={{
  items: [{
    resumeId: "resume_1",
    candidateName: "张三",
    chan: "campus",
    department: { id: "dept_b", name: "智算调度部" },
    recommender: { id: "user_1", name: "李四" },
  }],
}}
```

Assert `listResumes` is called with `{ chan: "campus", search: "张三" }` and the banner text contains `有 1 份简历被推荐到你可查看的部门`.

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
CI=true pnpm --filter @talentpilot/web exec vitest run src/resume-library/ResumeLibraryPage.test.tsx -t "notification|recommendation banner"
```

Expected: FAIL because the prop and banner do not exist.

- [ ] **Step 3: Implement jump prop and banner**

Add `NotificationJumpContext` to `types.ts`, accept `notificationJump?: NotificationJumpContext`, and in `ResumeLibraryPage`:

- initialize/sync channel from first jump item `chan`;
- initialize/sync search from first jump item `candidateName`;
- show the green banner above filters;
- list candidate name, department name, and recommender name.

- [ ] **Step 4: Run resume-library tests and verify GREEN**

Run:

```bash
CI=true pnpm --filter @talentpilot/web exec vitest run src/resume-library/ResumeLibraryPage.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/resume-library/types.ts apps/web/src/resume-library/ResumeLibraryPage.tsx apps/web/src/resume-library/ResumeLibraryPage.test.tsx
git commit -m "feat(web): show recommendation jump banner"
```

## Task 8: Final Verification and Status

**Files:**
- Modify: `AGENTS.md`
- Modify: `docs/project-status.md`

- [ ] **Step 1: Run targeted E7 tests**

Run:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/notification -count=1
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app -run TestNotification -count=1
CI=true pnpm --filter @talentpilot/web exec vitest run src/notifications/NotificationBell.test.tsx src/app/App.test.tsx src/resume-library/ResumeLibraryPage.test.tsx src/api/client.test.ts
```

Expected: PASS.

- [ ] **Step 2: Run full CI gate**

Run:

```bash
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH CI=true make ci
git diff --check
```

Expected: PASS.

- [ ] **Step 3: Update status docs**

Update `docs/project-status.md` E7 row to Done with evidence:

```text
Verification: PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH CI=true make ci, git diff --check.
```

Update `AGENTS.md`:

- phase becomes E8 Custom role management planning;
- source-of-truth line includes this E7 implementation plan;
- documentation index includes this E7 implementation plan.

- [ ] **Step 4: Commit final status**

```bash
git add AGENTS.md docs/project-status.md
git commit -m "docs: mark E7 implementation complete"
```

## Final Acceptance Commands

Run before asking for acceptance:

```bash
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/notification -count=1
cd apps/api && PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH go test ./internal/app -run TestNotification -count=1
CI=true pnpm --filter @talentpilot/web exec vitest run src/notifications/NotificationBell.test.tsx src/app/App.test.tsx src/resume-library/ResumeLibraryPage.test.tsx src/api/client.test.ts
PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH CI=true make ci
git diff --check
```

Acceptance requires the commands above to pass and `git status --short` to show no uncommitted implementation files except the pre-existing untracked `.codex-work/`.
