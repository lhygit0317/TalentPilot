# E3 Resume Recommendation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the full E3 resume recommendation workflow: intelligent routing, department grouping, recommendation deduplication, `PositionResume(kind=recommended)`, notification record creation, and the `简历推荐` page.

**Architecture:** Reuse the shared E2 `matching` calculator for all scores. Add a focused `apps/api/internal/recommendation` package for routing, recommendation copies, recipient calculation, and notification writes; expose Huma routes under `/recommendations`; regenerate OpenAPI/client artifacts; then render `ResumeRecommendPage` with generated-client wrappers only.

**Tech Stack:** Go, Echo, Huma, GORM, goose schema, React, TypeScript, Vite, Testing Library, shadcn/project UI wrappers, generated OpenAPI client.

---

## File Structure

- Create or reuse: `apps/api/internal/matching/types.go` for shared scoring DTOs.
- Create or reuse: `apps/api/internal/matching/scoring.go` for deterministic E2/E3 match scoring.
- Create or reuse tests: `apps/api/internal/matching/scoring_test.go`.
- Create: `apps/api/internal/recommendation/types.go` for route/send DTOs and domain errors.
- Create: `apps/api/internal/recommendation/service.go` for routing and send orchestration.
- Create: `apps/api/internal/recommendation/sql_store.go` for scoped SQL loading, copy dedupe, recommended relation upsert, contact loading, recipient calculation, and notification insertion.
- Create: `apps/api/internal/recommendation/service_test.go`.
- Create: `apps/api/internal/recommendation/sql_store_test.go`.
- Create: `apps/api/internal/app/recommendation_routes.go`.
- Create: `apps/api/internal/app/recommendation_routes_test.go`.
- Modify: `apps/api/internal/app/server.go` to register recommendation routes and service option.
- Modify: `apps/api/internal/http/apperror/error.go` for recommendation error codes.
- Modify: `apps/api/internal/audit/audit.go` for recommendation audit event types.
- Modify generated artifacts after backend routes: `packages/api-contract/openapi.json`, `packages/api-client/src/schema.d.ts`, and `packages/api-client/src/index.ts`.
- Modify: `apps/web/src/api/client.ts` and `apps/web/src/api/client.test.ts` for recommendation wrappers.
- Create: `apps/web/src/resume-recommend/types.ts`.
- Create: `apps/web/src/resume-recommend/ResumeRecommendPage.tsx`.
- Create: `apps/web/src/resume-recommend/ResumeRecommendPage.test.tsx`.
- Modify: `apps/web/src/app/App.tsx` and `apps/web/src/app/App.test.tsx` to render `resume-recommend`.
- Modify: `apps/web/src/i18n/zh-CN.ts` and `apps/web/src/i18n/en-US.ts` for recommendation messages.
- Modify: `docs/project-status.md` and `AGENTS.md` after verification.

## Task 1: Shared Matching Calculator Boundary

**Files:**
- Create or reuse: `apps/api/internal/matching/types.go`
- Create or reuse: `apps/api/internal/matching/scoring.go`
- Test: `apps/api/internal/matching/scoring_test.go`

- [ ] **Step 1: Write the failing score tests**

Add tests that lock the shared E2/E3 scoring contract. If E2 already added this package, keep the public API below and add any missing tests.

```go
func TestCalculateMatchScoreUsesPRDWeightsAndEvidence(t *testing.T) {
	result := matching.Calculate(matching.MatchInput{
		ResumeKeywords:   []string{"go", "调度"},
		ResumeTraits:     []string{"稳定"},
		ExperienceBase:   82,
		PositionKeywords: []string{"Go", "Kubernetes"},
		PositionImplicitTags: []matching.ImplicitTag{
			{Name: "稳定", Weight: 40},
		},
	})

	if result.Score.Skill != 50 || result.Score.Experience != 82 || result.Score.Implicit != 100 {
		t.Fatalf("unexpected component scores: %#v", result.Score)
	}
	if result.Score.Total != 79 || result.Score.Judgement != "建议进入面试" {
		t.Fatalf("unexpected total or judgement: %#v", result.Score)
	}
	if !result.Evidence.Keywords[0].Matched || result.Evidence.Keywords[1].Matched {
		t.Fatalf("unexpected keyword evidence: %#v", result.Evidence.Keywords)
	}
}

func TestCalculateMatchScoreHandlesEmptyInputsAndClamping(t *testing.T) {
	result := matching.Calculate(matching.MatchInput{
		ExperienceBase: 140,
	})

	if result.Score.Skill != 0 || result.Score.Implicit != 0 || result.Score.Experience != 100 {
		t.Fatalf("unexpected empty input scores: %#v", result.Score)
	}
	if result.Score.Total != 25 || result.Score.Judgement != "谨慎或暂不推荐" {
		t.Fatalf("unexpected empty input total: %#v", result.Score)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `cd apps/api && go test ./internal/matching -run TestCalculateMatchScore -count=1`

Expected: FAIL because the `matching` package or `Calculate` API is missing, or because the current scoring behavior does not satisfy the E2/E3 contract.

- [ ] **Step 3: Implement the minimal shared calculator**

Implement these exported types and function exactly so E2 and E3 can share them:

```go
type ImplicitTag struct {
	Name   string
	Weight int
}

type MatchInput struct {
	ResumeKeywords       []string
	ResumeTraits         []string
	ExperienceBase       int
	PositionKeywords     []string
	PositionImplicitTags []ImplicitTag
}

type Score struct {
	Total      int    `json:"total"`
	Skill      int    `json:"skill"`
	Experience int    `json:"experience"`
	Implicit   int    `json:"implicit"`
	Judgement  string `json:"judgement"`
}

type EvidenceItem struct {
	Name    string `json:"name"`
	Weight  int    `json:"w,omitempty"`
	Matched bool   `json:"matched"`
}

type Evidence struct {
	Keywords     []EvidenceItem `json:"keywords" nullable:"false"`
	ImplicitTags []EvidenceItem `json:"implicitTags" nullable:"false"`
	Analysis     string         `json:"analysis"`
}

type CalculationResult struct {
	Score    Score
	Evidence Evidence
}

func Calculate(input MatchInput) CalculationResult
```

Use the exact PRD weights: skill `0.4`, experience `0.25`, implicit `0.35`. Trim and case-fold keyword/trait comparisons. Clamp experience and total to `0..100`. Use judgement labels `强烈推荐`, `建议进入面试`, and `谨慎或暂不推荐`.

- [ ] **Step 4: Run matching tests**

Run: `cd apps/api && go test ./internal/matching -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/matching
git commit -m "feat(api): share resume matching scores"
```

## Task 2: Recommendation Routing Service

**Files:**
- Create: `apps/api/internal/recommendation/types.go`
- Create: `apps/api/internal/recommendation/service.go`
- Test: `apps/api/internal/recommendation/service_test.go`

- [ ] **Step 1: Write the failing routing service tests**

Add fake-store tests that prove service-level grouping and ordering before SQL exists.

```go
func TestServiceRouteGroupsByDepartmentAndMarksBest(t *testing.T) {
	store := &fakeRecommendationStore{
		resume: recommendation.ResumeContext{
			ID: "resume_1", Name: "张三", Channel: "social", Keywords: []string{"Go"}, ExpBase: 88,
			Department: recommendation.DepartmentSummary{ID: "dept_source", Name: "来源部门"},
		},
		positions: []recommendation.PositionContext{
			{ID: "position_low", Name: "低分岗位", Channel: "social", Status: "on", Department: recommendation.DepartmentSummary{ID: "dept_a", Name: "A 部门"}, Keywords: []string{"Rust"}},
			{ID: "position_high", Name: "高分岗位", Channel: "social", Status: "on", Department: recommendation.DepartmentSummary{ID: "dept_a", Name: "A 部门"}, Keywords: []string{"Go"}},
			{ID: "position_b", Name: "B 岗位", Channel: "social", Status: "on", Department: recommendation.DepartmentSummary{ID: "dept_b", Name: "B 部门"}, Keywords: []string{"Go"}},
		},
		contacts: map[string]recommendation.DepartmentContacts{
			"dept_a": {HRBPs: []string{"李四"}, Managers: []string{"王五"}, Trainees: []string{"赵六"}},
			"dept_b": {HRBPs: []string{"孙七"}},
		},
	}
	service := recommendation.NewService(store, audit.NopRecorder{})

	result, err := service.Route(context.Background(), recommendation.RouteInput{ActorUserID: "user_1", ResumeID: "resume_1"})
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if len(result.Routes) != 2 {
		t.Fatalf("expected 2 department routes, got %#v", result.Routes)
	}
	if result.Routes[0].Position.ID != "position_high" || !result.Routes[0].Best {
		t.Fatalf("expected highest A position as best, got %#v", result.Routes[0])
	}
	if result.Routes[1].Best {
		t.Fatalf("only first route should be best: %#v", result.Routes)
	}
}
```

- [ ] **Step 2: Run service tests and verify RED**

Run: `cd apps/api && go test ./internal/recommendation -run TestServiceRouteGroupsByDepartmentAndMarksBest -count=1`

Expected: FAIL because the `recommendation` package and `Service.Route` do not exist.

- [ ] **Step 3: Define recommendation types and service interfaces**

Create the domain API with these exported names:

```go
var (
	ErrRouteFailed            = errors.New("recommendation route failed")
	ErrTargetPositionOffline  = errors.New("recommendation target position offline")
	ErrTargetPositionMismatch = errors.New("recommendation target position mismatch")
	ErrChannelMismatch        = errors.New("recommendation channel mismatch")
	ErrSendFailed             = errors.New("recommendation send failed")
)

type Store interface {
	GetResume(context.Context, string, iam.ScopePredicate) (ResumeContext, error)
	ListRoutePositions(context.Context, RoutePositionQuery) ([]PositionContext, error)
	ListDepartmentContacts(context.Context, []string) (map[string]DepartmentContacts, error)
	SendRecommendation(context.Context, SendCommand) (SendResult, error)
}

type Service struct {
	store Store
	audit audit.Recorder
}

func NewService(store Store, recorder audit.Recorder) *Service
func (s *Service) Route(ctx context.Context, input RouteInput) (RouteResult, error)
func (s *Service) Send(ctx context.Context, input SendInput) (SendResult, error)
```

Include JSON DTOs for `RouteResult`, `RouteRow`, `SendResult`, `DepartmentSummary`, `PositionSummary`, and `DepartmentContacts` matching `docs/specs/006-resume-recommendation-notification.md`.

- [ ] **Step 4: Implement minimal routing orchestration**

`Service.Route` must:

- load the source resume through `store.GetResume`;
- call `store.ListRoutePositions` with source channel and position scope;
- score every returned position using `matching.Calculate`;
- retain only the top-scoring row per department;
- sort by `score.total DESC`, then department name ASC;
- set `Best=true` only on index `0`;
- load contacts for the final department IDs and attach them.

- [ ] **Step 5: Run recommendation service tests**

Run: `cd apps/api && go test ./internal/recommendation -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/recommendation/types.go apps/api/internal/recommendation/service.go apps/api/internal/recommendation/service_test.go
git commit -m "feat(api): calculate recommendation routes"
```

## Task 3: Recommendation SQL Store for Routing

**Files:**
- Create: `apps/api/internal/recommendation/sql_store.go`
- Test: `apps/api/internal/recommendation/sql_store_test.go`

- [ ] **Step 1: Write failing SQL route-loading tests**

Add migrated SQLite tests with departments, positions, a source resume, and role contacts.

```go
func TestSQLStoreListsScopedSameChannelOnShelfPositions(t *testing.T) {
	db := newRecommendationMigratedSQLiteGormDB(t)
	seedRecommendationRoutingFixture(t, db)
	store := recommendation.NewSQLStore(db)

	positions, err := store.ListRoutePositions(context.Background(), recommendation.RoutePositionQuery{
		Channel: "social",
		Scope: iam.ScopePredicate{Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_a"}}}},
	})
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}
	if len(positions) != 1 || positions[0].ID != "position_social_on" {
		t.Fatalf("expected only scoped social on-shelf position, got %#v", positions)
	}
}

func TestSQLStoreListsDepartmentContactsByPresetRole(t *testing.T) {
	db := newRecommendationMigratedSQLiteGormDB(t)
	seedRecommendationRoutingFixture(t, db)
	store := recommendation.NewSQLStore(db)

	contacts, err := store.ListDepartmentContacts(context.Background(), []string{"dept_a"})
	if err != nil {
		t.Fatalf("contacts: %v", err)
	}
	if got := strings.Join(contacts["dept_a"].HRBPs, "、"); got != "李四" {
		t.Fatalf("unexpected HRBP contacts: %q", got)
	}
}
```

- [ ] **Step 2: Run SQL routing tests and verify RED**

Run: `cd apps/api && go test ./internal/recommendation -run TestSQLStoreLists -count=1`

Expected: FAIL because `NewSQLStore`, route position loading, and contact loading do not exist.

- [ ] **Step 3: Implement route SQL loading**

Implement:

```go
func NewSQLStore(db *gorm.DB) *SQLStore
func (s *SQLStore) GetResume(ctx context.Context, resumeID string, scope iam.ScopePredicate) (ResumeContext, error)
func (s *SQLStore) ListRoutePositions(ctx context.Context, query RoutePositionQuery) ([]PositionContext, error)
func (s *SQLStore) ListDepartmentContacts(ctx context.Context, departmentIDs []string) (map[string]DepartmentContacts, error)
```

Use SQL joins against `resumes`, `department_resumes`, `departments`, `positions`, `department_positions`, `user_department_roles`, `users`, and `roles`. Push IAM department/channel predicates into SQL. Exclude positions with no `department_positions` row and exclude `positions.status != 'on'`.

- [ ] **Step 4: Run SQL routing tests**

Run: `cd apps/api && go test ./internal/recommendation -run 'TestSQLStoreLists|TestSQLStoreGetResume' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/recommendation/sql_store.go apps/api/internal/recommendation/sql_store_test.go
git commit -m "feat(api): load recommendation routing data"
```

## Task 4: Recommendation Send Persistence and Notifications

**Files:**
- Modify: `apps/api/internal/recommendation/service.go`
- Modify: `apps/api/internal/recommendation/sql_store.go`
- Test: `apps/api/internal/recommendation/service_test.go`
- Test: `apps/api/internal/recommendation/sql_store_test.go`
- Modify: `apps/api/internal/audit/audit.go`

- [ ] **Step 1: Write failing send service tests**

Cover target validation before writes and notification failure not rolling back the main recommendation.

```go
func TestServiceSendRejectsOfflineTargetBeforeWrite(t *testing.T) {
	store := &fakeRecommendationStore{
		resume: recommendation.ResumeContext{ID: "resume_1", Channel: "social", NormalizedName: "zhangsan"},
		sendErr: recommendation.ErrTargetPositionOffline,
	}
	service := recommendation.NewService(store, audit.NopRecorder{})

	_, err := service.Send(context.Background(), recommendation.SendInput{
		ActorUserID: "user_1", ResumeID: "resume_1", DepartmentID: "dept_a", PositionID: "position_off",
	})
	if !errors.Is(err, recommendation.ErrTargetPositionOffline) {
		t.Fatalf("expected offline error, got %v", err)
	}
}
```

- [ ] **Step 2: Write failing SQL send tests**

Cover copy creation, dedupe reuse, `PositionResume` upsert, notification creation, and self-read behavior.

```go
func TestSQLStoreSendRecommendationCreatesCopyAndNotifications(t *testing.T) {
	db := newRecommendationMigratedSQLiteGormDB(t)
	seedRecommendationSendFixture(t, db)
	store := recommendation.NewSQLStore(db)

	result, err := store.SendRecommendation(context.Background(), recommendation.SendCommand{
		ActorUserID: "user_recommender",
		ActorName: "推荐人",
		SourceResumeID: "resume_source",
		DepartmentID: "dept_target",
		PositionID: "position_target",
		MatchScore: 86,
		ResumeCreateScope: iam.ScopePredicate{Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}}}},
		DepartmentResumeCreateScope: iam.ScopePredicate{Branches: []iam.ScopeBranch{{DepartmentIDs: []string{"dept_target"}}}},
		NotificationCreateScope: iam.ScopePredicate{Branches: []iam.ScopeBranch{{AllDepartments: true}}},
	})
	if err != nil {
		t.Fatalf("send recommendation: %v", err)
	}
	if result.ResumeID == "resume_source" || result.NotifiedCount != 2 {
		t.Fatalf("unexpected send result: %#v", result)
	}
	assertRecommendationRow(t, db, result.ResumeID, "position_target", "recommended", 86)
	assertNotificationReadState(t, db, "user_recommender", result.ResumeID, true)
}
```

- [ ] **Step 3: Run send tests and verify RED**

Run: `cd apps/api && go test ./internal/recommendation -run 'TestServiceSend|TestSQLStoreSendRecommendation' -count=1`

Expected: FAIL because send persistence and notification writes do not exist.

- [ ] **Step 4: Implement send orchestration and audit constants**

Add audit event constants:

```go
const (
	EventRecommendationSent               EventType = "recommendation.sent"
	EventRecommendationNotificationFailed EventType = "recommendation.notification_failed"
)
```

`Service.Send` must pass actor/user/scope fields into `store.SendRecommendation`, record `EventRecommendationSent` on success, and return domain errors unchanged for route mapping.

- [ ] **Step 5: Implement send SQL transaction**

`SendRecommendation` must:

- re-load source resume and target position inside the transaction;
- reject off-shelf target position;
- reject target position whose department does not equal `DepartmentID`;
- reject channel mismatch;
- find existing target copy by `resumes.normalized_name` plus `department_resumes.department_id`;
- create `Resume(source='推荐')` and `DepartmentResume` when absent;
- update `source_by` and `updated_at` when present;
- upsert `position_resumes(kind='recommended')`;
- commit the main transaction;
- compute recipients with effective `Resume.List` or `Resume.Get` permissions for target department and channel;
- insert notifications after the main transaction, with self-notification `read=true`;
- return success even if notification insert fails, with `NotificationFailed=true` and safe failure details for audit/logging.

- [ ] **Step 6: Run recommendation package tests**

Run: `cd apps/api && go test ./internal/recommendation -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/recommendation apps/api/internal/audit/audit.go
git commit -m "feat(api): send resume recommendations"
```

## Task 5: Recommendation HTTP Routes and Generated Contracts

**Files:**
- Create: `apps/api/internal/app/recommendation_routes.go`
- Test: `apps/api/internal/app/recommendation_routes_test.go`
- Modify: `apps/api/internal/app/server.go`
- Modify: `apps/api/internal/http/apperror/error.go`
- Modify generated: `packages/api-contract/openapi.json`
- Modify generated: `packages/api-client/src/schema.d.ts`
- Modify generated: `packages/api-client/src/index.ts`

- [ ] **Step 1: Write failing route tests**

Test permission composition before service calls and stable target error mapping.

```go
func TestRecommendationSendRouteRequiresNotificationCreate(t *testing.T) {
	service := &fakeRecommendationService{}
	iamSvc := newFakeIAMServiceWithout(iam.ResourceNotification, iam.ActionCreate)
	server := NewServerWithOptions(Options{
		AuthService: authenticatedFakeAuth(),
		IAMService: iamSvc,
		RecommendationService: service,
	})

	req := authenticatedJSONRequest(http.MethodPost, "/recommendations/send", `{"resumeId":"resume_1","departmentId":"dept_a","positionId":"position_a"}`)
	rec := httptest.NewRecorder()
	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with %s", rec.Code, rec.Body.String())
	}
	if service.sendCalls != 0 {
		t.Fatalf("service must not be called when permission is missing")
	}
}
```

- [ ] **Step 2: Run route tests and verify RED**

Run: `cd apps/api && go test ./internal/app -run TestRecommendation -count=1`

Expected: FAIL because recommendation routes and `RecommendationService` option do not exist.

- [ ] **Step 3: Implement routes, server option, and error codes**

Add:

```go
type RecommendationService interface {
	Route(context.Context, recommendation.RouteInput) (recommendation.RouteResult, error)
	Send(context.Context, recommendation.SendInput) (recommendation.SendResult, error)
}
```

Register:

- `POST /recommendations/route`
- `POST /recommendations/send`

Map stable codes:

- `RECOMMENDATION_ROUTE_FAILED`
- `RECOMMENDATION_TARGET_POSITION_OFFLINE`
- `RECOMMENDATION_TARGET_POSITION_MISMATCH`
- `RECOMMENDATION_CHANNEL_MISMATCH`
- `RECOMMENDATION_SEND_FAILED`

Route handlers must compute and pass IAM scopes into service input. They must require exactly the permissions listed in `docs/specs/006-resume-recommendation-notification.md`.

- [ ] **Step 4: Run route tests**

Run: `cd apps/api && go test ./internal/app -run TestRecommendation -count=1`

Expected: PASS.

- [ ] **Step 5: Generate OpenAPI and client**

Run:

```bash
make openapi-generate
make client-generate
```

Expected: generated contract and client include `/recommendations/route` and `/recommendations/send`.

- [ ] **Step 6: Run API contract checks**

Run:

```bash
make openapi-check
make client-check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/app/recommendation_routes.go apps/api/internal/app/recommendation_routes_test.go apps/api/internal/app/server.go apps/api/internal/http/apperror/error.go packages/api-contract/openapi.json packages/api-client/src/schema.d.ts packages/api-client/src/index.ts
git commit -m "feat(api): expose recommendation endpoints"
```

## Task 6: Frontend API Wrappers and Types

**Files:**
- Modify: `apps/web/src/api/client.ts`
- Modify: `apps/web/src/api/client.test.ts`
- Create: `apps/web/src/resume-recommend/types.ts`

- [ ] **Step 1: Write failing API client tests**

```ts
it("routes and sends recommendations", async () => {
  const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
  vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ POST: post })) }));

  const { routeRecommendation, sendRecommendation } = await import("./client");
  await routeRecommendation({ resumeId: "resume_1" });
  await sendRecommendation({ resumeId: "resume_1", departmentId: "dept_a", positionId: "position_a" });

  expect(post).toHaveBeenCalledWith("/recommendations/route", { body: { resumeId: "resume_1" } });
  expect(post).toHaveBeenCalledWith("/recommendations/send", {
    body: { resumeId: "resume_1", departmentId: "dept_a", positionId: "position_a" },
  });
});
```

- [ ] **Step 2: Run frontend API test and verify RED**

Run: `pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts`

Expected: FAIL because wrappers do not exist.

- [ ] **Step 3: Implement wrappers and page-local types**

Add wrappers:

```ts
export function routeRecommendation(body: { resumeId: string }) {
  return apiClient.POST("/recommendations/route", { body });
}

export function sendRecommendation(body: { resumeId: string; departmentId: string; positionId: string }) {
  return apiClient.POST("/recommendations/send", { body });
}
```

Create `apps/web/src/resume-recommend/types.ts` with `ResumeRecommendSession`, `RouteResult`, `RouteRow`, `SendRecommendationResult`, and `ResumeChannel` matching generated API shapes used by the page.

- [ ] **Step 4: Run frontend API test**

Run: `pnpm --filter @talentpilot/web test -- --run src/api/client.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/client.ts apps/web/src/api/client.test.ts apps/web/src/resume-recommend/types.ts
git commit -m "feat(web): add recommendation client wrappers"
```

## Task 7: Resume Recommendation Page

**Files:**
- Create: `apps/web/src/resume-recommend/ResumeRecommendPage.tsx`
- Create: `apps/web/src/resume-recommend/ResumeRecommendPage.test.tsx`
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing page tests**

Cover initial empty state, routing result rendering, best marker, and send success.

```tsx
it("routes a selected resume and sends the best recommendation", async () => {
  vi.mock("../api/client", () => ({
    getJob: vi.fn(),
    importResume: vi.fn(),
    listResumes: vi.fn().mockResolvedValue({
      data: {
        availableChannels: ["social"],
        channelCounts: { social: 1, campus: 0 },
        dataScopeSummary: "全部部门",
        items: [{ id: "resume_1", name: "张三", chan: "social", pos: "平台工程师", currentDepartment: { id: "dept_source", name: "来源部门" }, keywords: ["Go"], source: "导入", sourceBy: "李四", school: "浙大", canGet: true, canDelete: false }],
        nextCursor: "",
      },
      error: undefined,
    }),
    routeRecommendation: vi.fn().mockResolvedValue({
      data: {
        resume: { id: "resume_1", name: "张三", chan: "social", pos: "平台工程师", currentDepartment: { id: "dept_source", name: "来源部门" }, keywords: ["Go"] },
        routes: [{ department: { id: "dept_a", name: "智算调度部" }, position: { id: "position_a", name: "调度工程师", chan: "social", level: "P6" }, score: { total: 86, skill: 100, experience: 82, implicit: 80, judgement: "强烈推荐" }, contacts: { hrbps: ["李四"], managers: ["王五"], trainees: ["赵六"] }, best: true }],
        createdAt: "2026-07-12T08:00:00Z",
      },
      error: undefined,
    }),
    sendRecommendation: vi.fn().mockResolvedValue({
      data: { resumeId: "resume_copy", sourceResumeId: "resume_1", department: { id: "dept_a", name: "智算调度部" }, position: { id: "position_a", name: "调度工程师" }, candidateName: "张三", reusedExistingCopy: false, notifiedCount: 3, selfNotificationRead: true, message: "已推荐到「智算调度部」· 已通知 3 位相关人员" },
      error: undefined,
    }),
  }));

  render(<ResumeRecommendPage session={recommendSession()} />);

  expect(await screen.findByText("张三")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "张三" }));
  await userEvent.click(screen.getByRole("button", { name: "智能分流" }));

  expect(await screen.findByText("最佳去向")).toBeInTheDocument();
  expect(screen.getByText("智算调度部")).toBeInTheDocument();

  await userEvent.click(screen.getByRole("button", { name: "推荐到" }));
  expect(await screen.findByText("已推荐到「智算调度部」· 已通知 3 位相关人员")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run page tests and verify RED**

Run: `pnpm --filter @talentpilot/web test -- --run src/resume-recommend/ResumeRecommendPage.test.tsx`

Expected: FAIL because the page does not exist.

- [ ] **Step 3: Implement recommendation i18n messages**

Add `resumeRecommend` to both locale files with:

- title `简历推荐`;
- source modes `从简历库选择`, `导入新简历`;
- action labels `智能分流`, `推荐到`;
- empty copy `分流结果将显示在这里 / 选择/导入简历后点击「智能分流」`;
- no route copy `该渠道下暂无在架岗位 / 请在「部门与岗位管理」中上架岗位`;
- loading copy `Thinking...`;
- error messages for route/send/import failures and mapped recommendation error codes.

- [ ] **Step 4: Implement `ResumeRecommendPage`**

The page must:

- derive default channel from `session.dataScope.channels`;
- call `listResumes({ chan: channel })`;
- support library selection and single import using the existing `importResume`/`getJob` pattern from `ResumeLibraryPage`;
- clear route results on source mode, channel, or selected resume change;
- call `routeRecommendation({ resumeId })` from `智能分流`;
- render one row per route with score, department, position, contacts, best marker, and `推荐到`;
- call `sendRecommendation({ resumeId, departmentId, positionId })`;
- show the returned success message.

Use existing project UI wrappers (`Button`, `Field`, `Input`, `Select`) for interactive controls.

- [ ] **Step 5: Wire the page into the app shell**

In `apps/web/src/app/App.tsx`, import `ResumeRecommendPage` and render it when `activePage === "resume-recommend"`.

Update `App.test.tsx` to assert that a session whose `defaultRoute` is `/resume-recommend` renders the recommendation page instead of the fallback title.

- [ ] **Step 6: Run frontend tests**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run src/resume-recommend/ResumeRecommendPage.test.tsx src/app/App.test.tsx
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/resume-recommend apps/web/src/app/App.tsx apps/web/src/app/App.test.tsx apps/web/src/i18n/zh-CN.ts apps/web/src/i18n/en-US.ts
git commit -m "feat(web): render resume recommendation workflow"
```

## Task 8: Final Verification and Status Updates

**Files:**
- Modify: `docs/project-status.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Run backend verification**

Run:

```bash
make test-api
make openapi-check
```

Expected: PASS.

- [ ] **Step 2: Run frontend verification**

Run:

```bash
make test-web
make typecheck
```

Expected: PASS.

- [ ] **Step 3: Run full quality checks**

Run:

```bash
make lint
make build
git diff --check
```

Expected: PASS.

- [ ] **Step 4: Update project status and agent guide**

Update `docs/project-status.md`:

- E3 status becomes `Done`;
- Evidence lists the SPEC, this implementation plan, and verification commands that passed;
- E7 remains `Not Started` and points to a future notification center SPEC.

Update `AGENTS.md`:

- current phase moves to the next approved phase;
- E3 SPEC and implementation plan remain in the Documentation Index.

- [ ] **Step 5: Commit final docs**

```bash
git add docs/project-status.md AGENTS.md
git commit -m "docs: mark E3 implementation complete"
```

- [ ] **Step 6: Final status**

Run: `git status --short`

Expected: only unrelated pre-existing files remain untracked or modified. Mention any such files in the final response.
