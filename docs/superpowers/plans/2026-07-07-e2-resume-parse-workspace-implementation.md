# E2 Resume Parse Workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the full E2 resume parsing workspace: resume/JD selection, backend-owned matching, parsed relation persistence, explainable results, and three-round interview questions.

**Architecture:** Add a focused `apps/api/internal/matching` package that loads scoped resume and position data, calculates deterministic scores, upserts `position_resumes(kind='parsed')`, and generates deterministic interview questions behind an adapter interface. Expose Huma routes under `/matching`, regenerate OpenAPI/client artifacts, then render a `ResumeParsePage` that reuses existing resume import/list and position list APIs.

**Tech Stack:** Go, Echo, Huma, GORM, goose schema, React, TypeScript, Vite, Testing Library, shadcn/project UI wrappers, generated OpenAPI client.

---

## File Structure

- Create: `apps/api/internal/matching/types.go` for request/response/domain DTOs and errors.
- Create: `apps/api/internal/matching/scoring.go` for pure matching calculation.
- Create: `apps/api/internal/matching/service.go` for parse and interview orchestration.
- Create: `apps/api/internal/matching/sql_store.go` for scoped SQL loading and `position_resumes` upsert.
- Create: `apps/api/internal/matching/scoring_test.go`, `service_test.go`, and `sql_store_test.go`.
- Create: `apps/api/internal/app/matching_routes.go` and `matching_routes_test.go`.
- Modify: `apps/api/internal/app/server.go` to register the matching service.
- Modify: `apps/api/internal/http/apperror/error.go` for matching errors.
- Modify: `apps/api/internal/audit/audit.go` for parse audit event type.
- Modify generated artifacts after backend routes: `packages/api-contract/openapi.json`, `packages/api-client/src/schema.d.ts`, and `packages/api-client/src/index.ts`.
- Modify: `apps/web/src/api/client.ts` and `apps/web/src/api/client.test.ts` for matching wrappers.
- Create: `apps/web/src/resume-parse/types.ts`, `ResumeParsePage.tsx`, and `ResumeParsePage.test.tsx`.
- Modify: `apps/web/src/app/App.tsx` and `apps/web/src/app/App.test.tsx` to render `resume-parse`.
- Modify: `apps/web/src/i18n/zh-CN.ts` and `apps/web/src/i18n/en-US.ts` for parse messages.
- Modify: `docs/project-status.md` and `AGENTS.md` after verification.

## Task 1: Matching Score Calculator

**Files:**
- Create: `apps/api/internal/matching/types.go`
- Create: `apps/api/internal/matching/scoring.go`
- Test: `apps/api/internal/matching/scoring_test.go`

- [ ] **Step 1: Write failing score tests**

Add tests covering weighted scoring, empty JD keywords, empty implicit tags, case-insensitive matches, rounding, clamping, and judgement labels.

```go
func TestCalculateMatchScoreUsesPRDWeightsAndEvidence(t *testing.T) {
	result := matching.Calculate(matching.MatchInput{
		ResumeKeywords: []string{"go", "调度"},
		ResumeTraits:   []string{"稳定"},
		ExperienceBase: 82,
		PositionKeywords: []string{"Go", "Kubernetes"},
		PositionImplicitTags: []matching.ImplicitTag{{Name: "稳定", Weight: 40}},
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
```

- [ ] **Step 2: Run score tests and verify RED**

Run: `cd apps/api && go test ./internal/matching -run TestCalculateMatchScoreUsesPRDWeightsAndEvidence -count=1`

Expected: FAIL because package `matching` does not exist.

- [ ] **Step 3: Implement minimal calculator**

Create domain types for `ImplicitTag`, `MatchInput`, `Score`, `Evidence`, `EvidenceItem`, and `CalculationResult`. Implement `Calculate(input MatchInput) CalculationResult` with PRD weights.

- [ ] **Step 4: Run matching calculator tests**

Run: `cd apps/api && go test ./internal/matching -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/matching/types.go apps/api/internal/matching/scoring.go apps/api/internal/matching/scoring_test.go
git commit -m "feat(api): calculate resume match scores"
```

## Task 2: Matching SQL Store and Parse Persistence

**Files:**
- Create: `apps/api/internal/matching/sql_store.go`
- Test: `apps/api/internal/matching/sql_store_test.go`

- [ ] **Step 1: Write failing SQL store tests**

Cover scoped resume loading, scoped position loading, excluding unauthorized targets, service-level off-shelf rejection in Task 3, and upserting parsed relations.

```go
func TestSQLStoreUpsertsParsedPositionResume(t *testing.T) {
	db := newMatchingMigratedSQLiteGormDB(t)
	seedMatchingFixture(t, db)
	store := matching.NewSQLStore(db)

	first, err := store.UpsertParsedRelation(context.Background(), matching.ParsedRelationInput{
		ResumeID: "resume_social_a", PositionID: "position_a", MatchScore: 79, ActorUserID: "user_owner",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	second, err := store.UpsertParsedRelation(context.Background(), matching.ParsedRelationInput{
		ResumeID: "resume_social_a", PositionID: "position_a", MatchScore: 88, ActorUserID: "user_owner",
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same parsed relation id, got %q and %q", first.ID, second.ID)
	}
	assertMatchingCount(t, db, "position_resumes", "resume_id = 'resume_social_a' AND position_id = 'position_a' AND kind = 'parsed' AND match_score = 88", 1)
}
```

- [ ] **Step 2: Run SQL store tests and verify RED**

Run: `cd apps/api && go test ./internal/matching -run TestSQLStoreUpsertsParsedPositionResume -count=1`

Expected: FAIL because `NewSQLStore` and persistence methods do not exist.

- [ ] **Step 3: Implement SQL store**

Implement store methods:

- `GetResume(ctx, resumeID, scope) (ResumeContext, error)`
- `GetPosition(ctx, positionID, scope) (PositionContext, error)`
- `UpsertParsedRelation(ctx, input) (ParsedRelation, error)`

Use existing tables: `resumes`, `department_resumes`, `departments`, `positions`, and `department_positions`. Build scope predicates with SQL conditions equivalent to E4/E5 and do not fetch all rows for frontend filtering.

- [ ] **Step 4: Run store tests**

Run: `cd apps/api && go test ./internal/matching -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/matching/sql_store.go apps/api/internal/matching/sql_store_test.go
git commit -m "feat(api): persist parsed resume matches"
```

## Task 3: Matching Service and Interview Questions

**Files:**
- Modify: `apps/api/internal/matching/service.go`
- Test: `apps/api/internal/matching/service_test.go`
- Modify: `apps/api/internal/audit/audit.go`

- [ ] **Step 1: Write failing service tests**

Cover off-shelf rejection, scope denial before persistence, successful parse audit, deterministic question groups, department name interpolation, and high-potential advanced question.

```go
func TestServiceParseRejectsOffShelfPositionBeforeWrite(t *testing.T) {
	store := &fakeMatchingStore{
		resume: matching.ResumeContext{ID: "resume_1", Keywords: []string{"Go"}, ExpBase: 80},
		position: matching.PositionContext{ID: "position_1", Status: "off", Keywords: []string{"Go"}},
	}
	service := matching.NewService(store, audit.NopRecorder{}, matching.NewRuleQuestionGenerator())

	_, err := service.Parse(context.Background(), matching.ParseInput{
		ActorUserID: "user_1", ResumeID: "resume_1", PositionID: "position_1",
	})
	if !errors.Is(err, matching.ErrPositionOffline) {
		t.Fatalf("expected offline error, got %v", err)
	}
	if store.upsertCalls != 0 {
		t.Fatalf("expected no parsed relation write, got %d", store.upsertCalls)
	}
}
```

- [ ] **Step 2: Run service tests and verify RED**

Run: `cd apps/api && go test ./internal/matching -run TestServiceParseRejectsOffShelfPositionBeforeWrite -count=1`

Expected: FAIL because `Service.Parse` does not exist.

- [ ] **Step 3: Implement service and generator**

Implement:

- `Service.Parse(ctx, ParseInput) (ParseResult, error)`
- `Service.GenerateInterviewQuestions(ctx, InterviewQuestionInput) (InterviewQuestionResult, error)`
- `RuleQuestionGenerator` with professional, manager, and qualification groups.
- `audit.EventResumeParsed` in `apps/api/internal/audit/audit.go`.

Keep audit payloads limited to IDs, score, department ID, and channel.

- [ ] **Step 4: Run matching package tests**

Run: `cd apps/api && go test ./internal/matching -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/matching apps/api/internal/audit/audit.go
git commit -m "feat(api): orchestrate resume parsing"
```

## Task 4: Matching HTTP Routes and OpenAPI

**Files:**
- Create: `apps/api/internal/app/matching_routes.go`
- Test: `apps/api/internal/app/matching_routes_test.go`
- Modify: `apps/api/internal/app/server.go`
- Modify: `apps/api/internal/http/apperror/error.go`
- Modify generated: `packages/api-contract/openapi.json`, `packages/api-client/src/schema.d.ts`, `packages/api-client/src/index.ts`

- [ ] **Step 1: Write failing route tests**

Test `POST /matching/parse` permission composition: `Resume.Get`, `Position.Get`, `DepartmentPosition.List`, and `PositionResume.Create`. Test stable error mapping for off-shelf positions.

```go
func TestMatchingParseRouteRequiresPositionResumeCreate(t *testing.T) {
	service := &fakeMatchingService{}
	iamSvc := newFakeIAMServiceWithout(iam.ResourcePositionResume, iam.ActionCreate)
	server := NewServerWithOptions(Options{IAMService: iamSvc, MatchingService: service, AuthService: authenticatedFakeAuth()})

	req := authenticatedJSONRequest(http.MethodPost, "/matching/parse", `{"resumeId":"resume_1","positionId":"position_1"}`)
	rec := httptest.NewRecorder()
	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d with %s", rec.Code, rec.Body.String())
	}
	if service.parseCalls != 0 {
		t.Fatalf("service must not be called when permission is missing")
	}
}
```

- [ ] **Step 2: Run route tests and verify RED**

Run: `cd apps/api && go test ./internal/app -run TestMatchingParseRouteRequiresPositionResumeCreate -count=1`

Expected: FAIL because matching routes and `MatchingService` option do not exist.

- [ ] **Step 3: Implement routes and errors**

Add `MatchingService` to `Options`, register `registerMatchingRoutes`, map:

- `matching.ErrPositionOffline` to `MATCHING_POSITION_OFFLINE`
- unexpected parse failure to `MATCHING_PARSE_FAILED`
- question generation failure to `MATCHING_INTERVIEW_FAILED`

Route handlers must pass IAM scopes into service input, not into frontend state.

- [ ] **Step 4: Generate OpenAPI and client**

Run:

```bash
make openapi-generate
make client-generate
```

Expected: generated OpenAPI includes `/matching/parse` and `/matching/interview-questions`; generated client types compile.

- [ ] **Step 5: Run backend route and drift checks**

Run:

```bash
cd apps/api && go test ./internal/app -count=1
make openapi-check
make client-check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/app/matching_routes.go apps/api/internal/app/matching_routes_test.go apps/api/internal/app/server.go apps/api/internal/http/apperror/error.go packages/api-contract/openapi.json packages/api-client/src/schema.d.ts packages/api-client/src/index.ts
git commit -m "feat(api): expose resume matching endpoints"
```

## Task 5: Frontend API Wrappers

**Files:**
- Modify: `apps/web/src/api/client.ts`
- Modify: `apps/web/src/api/client.test.ts`
- Create: `apps/web/src/resume-parse/types.ts`

- [ ] **Step 1: Write failing client wrapper tests**

```ts
it("parses resumes and generates interview questions", async () => {
  const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
  vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ POST: post })) }));

  const { generateInterviewQuestions, parseResumeMatch } = await import("./client");
  await parseResumeMatch({ resumeId: "resume_1", positionId: "position_1" });
  await generateInterviewQuestions({ resumeId: "resume_1", positionId: "position_1", matchScore: 79 });

  expect(post).toHaveBeenCalledWith("/matching/parse", { body: { resumeId: "resume_1", positionId: "position_1" } });
  expect(post).toHaveBeenCalledWith("/matching/interview-questions", {
    body: { resumeId: "resume_1", positionId: "position_1", matchScore: 79 },
  });
});
```

- [ ] **Step 2: Run client wrapper test and verify RED**

Run: `CI=true pnpm --filter @talentpilot/web test -- src/api/client.test.ts --run`

Expected: FAIL because wrapper functions do not exist.

- [ ] **Step 3: Implement wrappers and frontend types**

Add:

- `parseResumeMatch(body)`
- `generateInterviewQuestions(body)`
- frontend types for parse result and interview groups.

- [ ] **Step 4: Run client wrapper tests**

Run: `CI=true pnpm --filter @talentpilot/web test -- src/api/client.test.ts --run`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/client.ts apps/web/src/api/client.test.ts apps/web/src/resume-parse/types.ts
git commit -m "feat(web): add matching API wrappers"
```

## Task 6: Resume Parse Page UI

**Files:**
- Create: `apps/web/src/resume-parse/ResumeParsePage.tsx`
- Test: `apps/web/src/resume-parse/ResumeParsePage.test.tsx`
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/App.test.tsx`
- Modify: `apps/web/src/i18n/zh-CN.ts`
- Modify: `apps/web/src/i18n/en-US.ts`

- [ ] **Step 1: Write failing page tests**

Cover channel tabs, source selector, library list, upload mode, on-shelf JD selector, clearing results on selection changes, parse result rendering, and interview tabs.

```tsx
it("renders a parse result and generated interview questions", async () => {
  const user = userEvent.setup();
  render(<ResumeParsePage session={parseSession} />);

  await user.click(await screen.findByRole("button", { name: /张三/ }));
  await user.selectOptions(await screen.findByLabelText("目标岗位 JD"), "position_1");
  await user.click(screen.getByRole("button", { name: "开始解析" }));

  expect(await screen.findByText("建议进入面试")).toBeInTheDocument();
  expect(screen.getByText("技能匹配")).toBeInTheDocument();
  expect(screen.getByText("Go")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "生成面试题" }));
  expect(await screen.findByRole("tab", { name: "专业" })).toBeInTheDocument();
  expect(screen.getByText(/为什么选择算力训练平台部/)).toBeInTheDocument();
});
```

- [ ] **Step 2: Run page tests and verify RED**

Run: `CI=true pnpm --filter @talentpilot/web test -- src/resume-parse/ResumeParsePage.test.tsx --run`

Expected: FAIL because the page does not exist.

- [ ] **Step 3: Implement page and i18n**

Implement `ResumeParsePage` with project UI wrappers, no raw business-page interactive HTML elements. Reuse E4 upload validation behavior, call existing `listResumes`, `importResume`, `getJob`, and `listPositions`, then call matching wrappers. Clear parse and question state on channel, source, resume, or position changes.

- [ ] **Step 4: Wire App route**

Render `ResumeParsePage` when active page is `resume-parse`. Keep fallback heading for unimplemented future pages.

- [ ] **Step 5: Run frontend tests**

Run:

```bash
CI=true pnpm --filter @talentpilot/web test -- src/resume-parse/ResumeParsePage.test.tsx src/app/App.test.tsx --run
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/resume-parse apps/web/src/app/App.tsx apps/web/src/app/App.test.tsx apps/web/src/i18n/zh-CN.ts apps/web/src/i18n/en-US.ts
git commit -m "feat(web): render resume parse workspace"
```

## Task 7: Final Verification and Status

**Files:**
- Modify: `docs/project-status.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Run focused checks**

Run:

```bash
cd apps/api && go test ./internal/matching ./internal/app -count=1
CI=true pnpm --filter @talentpilot/web test -- src/api/client.test.ts src/resume-parse/ResumeParsePage.test.tsx src/app/App.test.tsx --run
make openapi-check
make client-check
make typecheck
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

Mark E2 Done in `docs/project-status.md` with verification evidence. Update `AGENTS.md` phase to indicate E2 implemented and E3 planning next.

- [ ] **Step 4: Commit**

```bash
git add docs/project-status.md AGENTS.md
git commit -m "docs: mark E2 implementation complete"
```

## Self-Review

- Spec coverage: Tasks cover Story 2.1 source/channel selection, Story 2.3 JD selection, Story 2.4 parse execution/results/persistence, and Story 2.5 interview questions. E4 single import is reused rather than reimplemented.
- Completion scan: No empty sections, no deferred behavior inside the E2 scope, and no undefined endpoints.
- Type consistency: Backend names use `matching.ParseInput`, `ParseResult`, `InterviewQuestionInput`, and `InterviewQuestionResult`; frontend wrappers use `parseResumeMatch` and `generateInterviewQuestions`.
