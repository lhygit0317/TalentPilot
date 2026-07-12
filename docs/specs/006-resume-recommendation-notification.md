# 006 Resume Recommendation and Notification SPEC

## 1. Scope and Goals

This SPEC defines E3: resume recommendation and intelligent routing. It covers the "简历推荐" page, resume source selection, channel switching, backend-owned routing calculation, department-level recommendation results, recommendation deduplication, `PositionResume(kind=recommended)` persistence, and creation of notification records for users who can see the target department resume.

E3 consumes E4 resume library data, E5 department and position data, IAM data-scope predicates, and the shared E2 matching algorithm. Backend authorization remains authoritative. Frontend state is only a workflow aid and must not become a security boundary.

## 2. Dependencies and Readiness

Required before E3 implementation:

- E1 auth/session behavior from [001 Auth Session W3 SPEC](001-auth-session-w3.md).
- IAM runtime behavior from [002 IAM Permission Model SPEC](002-iam-permission-model.md), especially `Resume.Get`, `Resume.Create`, `Resume.Update`, `Position.List`, `DepartmentPosition.List`, `DepartmentResume.Create`, `PositionResume.Create`, and `Notification.Create`.
- E4 resume library behavior from [003 Resume Library and Import SPEC](003-resume-library-import.md), especially `GET /resumes`, `GET /resumes/{resumeId}`, and single PDF import for the recommendation page.
- E5 department and position behavior from [004 Department and Position Management SPEC](004-position-department-management.md), especially permission-filtered `GET /positions` and department-position ownership.
- E2 shared matching behavior from [005 Resume Parse Workspace SPEC](005-resume-parse-workspace.md), especially the deterministic scoring algorithm and match thresholds. If E2 implementation is not complete when E3 starts, the implementation plan must first land the reusable matching scoring boundary before E3 routing code.
- Existing `notifications` table and `position_resumes` unique `(resume_id, position_id, kind)` constraint.

## 3. Non-Goals

- Notification center UI: bell unread badge, notification dropdown, mark-all-read, and click-to-library navigation belong to E7.
- Full notification read APIs. E3 only creates recommendation notification records.
- Real model-provider integration. Routing uses the deterministic backend matching algorithm from E2.
- Asynchronous routing jobs. E3 uses synchronous routing calculation with a visible frontend `Thinking...` state.
- Custom role management or user-role administration.
- Cross-channel routing. Social resumes route only to social positions; campus resumes route only to campus positions.

## 4. Design Approach

Three implementation scopes were considered:

1. Reuse the E2 matching package and add a focused recommendation service.
2. Implement a separate recommendation scoring service inside E3.
3. Implement backend recommendation APIs only and defer the frontend page.

This SPEC chooses option 1. It keeps scoring rules centralized, gives E3 its own write boundary for recommendation copies and notifications, and delivers PRD Story 3.1-3.3 as one verifiable workflow. E7 can later consume the notification records created here without changing the recommendation transaction.

## 5. Domain Model

E3 introduces a focused backend package, `apps/api/internal/recommendation`, responsible for:

- loading an authorized source resume;
- loading scoped, on-shelf, same-channel positions with department ownership;
- calling the shared matching calculator for each candidate position;
- grouping route results by department and selecting each department's highest-scoring position;
- loading HRBP, manager, and trainee display names from current `UserDepartmentRole` bindings;
- creating or reusing a target-department resume copy;
- upserting `position_resumes(kind='recommended')`;
- creating notification rows for authorized target-department recipients.

No new database table is required for E3.

Recommended relation behavior:

- `position_resumes.kind` is `recommended`.
- `match_score` stores the routed score for the target position when available.
- `by_user_id` stores the recommender.
- repeated recommendation for the same `(resumeId, positionId, recommended)` updates `match_score`, `created_at`, and `by_user_id`.

Resume copy behavior:

- recommendation to another department creates a resume copy instead of attaching one `resumeId` to multiple departments;
- deduplication uses `(normalized_name + target_department_id)`;
- if a target-department copy already exists, E3 reuses that resume and updates `source_by` and `updated_at`;
- if no target copy exists, E3 creates a new `Resume` with `source='推荐'`, copies safe structured fields from the source resume, sets `source_by` from the current recommender, and creates one `DepartmentResume` for the target department.

## 6. Permissions and Data Scope

Routes must use session authentication first, then IAM guards:

| Capability | Required permissions |
| --- | --- |
| Use recommendation page | `/me.pageAccess` includes `resume-recommend` |
| Select/list resumes | `Resume.List` |
| Get selected resume | `Resume.Get` |
| Calculate routing | `Resume.Get` + `Position.List` + `DepartmentPosition.List` |
| Send recommendation | `Resume.Get` + `Resume.Create` + `DepartmentResume.Create` + `PositionResume.Create` + `Notification.Create` |

Routing calculation must verify the selected resume is inside the caller's `Resume.Get` scope. Candidate positions must be inside the caller's `Position.List` scope and have a `DepartmentPosition` relation inside the same scope semantics used by E5.

Send recommendation must re-check all target state server-side:

- source resume is still inside `Resume.Get` scope;
- target position still exists;
- target position is still `status='on'`;
- target position belongs to the submitted target department;
- target position channel equals the source resume channel;
- target department is allowed by the caller's `DepartmentResume.Create` scope;
- created target resume satisfies the caller's `Resume.Create` scope.

Frontend-provided routing results are not trusted.

Refreshing `source_by` on an existing target-department copy is a narrow idempotency update owned by the recommendation service. It must not expose general resume editing and does not add a route-level `Resume.Update` requirement, matching the PRD Story 3.3 access condition and current preset-role page access.

## 7. Routing Algorithm

Routing uses the shared E2 matching calculator:

- skill score, experience score, implicit score, total score, evidence, and judgement labels must match E2 exactly;
- candidate positions are filtered to `status='on'`, same `chan` as the source resume, and scoped by `Position.List`;
- positions without `DepartmentPosition` are excluded;
- each matching position is scored independently;
- results are grouped by department;
- each department keeps only its highest-scoring position;
- department rows are sorted by total score descending, then department name ascending for deterministic ties;
- the first row is marked as the best route.

Threshold display:

- `>= 80`: strong, green, "最佳去向" when first row;
- `>= 65`: moderate, amber;
- `< 65`: cautious, red.

If no scoped on-shelf same-channel positions exist, return an empty routing result and the frontend shows `该渠道下暂无在架岗位 / 请在「部门与岗位管理」中上架岗位`.

## 8. Notification Recipient Calculation

E3 creates notification rows after the recommendation main transaction succeeds.

Recipients are current users who satisfy both conditions:

- they have a `UserDepartmentRole` binding whose effective scope can see `targetDepartmentId`, including valid system-scope bindings;
- their expanded permissions allow `Resume.List` or `Resume.Get` for the recommended resume's channel and expiry state.

The sender is included if they satisfy the same rule, but their own notification is created with `read=true`.

Notification row fields:

- `to_user_id`: recipient user ID;
- `resume_id`: final target-department resume ID, not necessarily the source resume ID;
- `department_id`: target department ID;
- `position_id`: target position ID;
- `name`: candidate display name snapshot;
- `by_user_id`: recommender user ID;
- `chan`: recommended resume channel snapshot;
- `time`: creation time;
- `read`: true only for self-notification, otherwise false.

Notification creation failure must not roll back the recommendation main flow. The backend must record the failure through audit or structured logs with request ID, recommender ID, target department ID, final resume ID, and safe error code.

Short-window notification deduplication is deferred to E7 unless product requires it earlier.

## 9. API Contract

Backend route definitions remain the OpenAPI source of truth. DTOs must not expose GORM models directly.

### `POST /recommendations/route`

Request:

```json
{
  "resumeId": "resume_1"
}
```

Response:

```json
{
  "resume": {
    "id": "resume_1",
    "name": "张三",
    "chan": "social",
    "pos": "平台工程师",
    "currentDepartment": { "id": "dept_a", "name": "算力训练平台部" },
    "keywords": ["Go", "调度"]
  },
  "routes": [
    {
      "department": { "id": "dept_b", "name": "智算调度部" },
      "position": { "id": "position_b", "name": "调度平台工程师", "chan": "social", "level": "P6" },
      "score": {
        "total": 86,
        "skill": 80,
        "experience": 82,
        "implicit": 95,
        "judgement": "强烈推荐"
      },
      "contacts": {
        "hrbps": ["李四"],
        "managers": ["王五"],
        "trainees": ["赵六"]
      },
      "best": true
    }
  ],
  "createdAt": "2026-07-12T08:00:00Z"
}
```

Behavior:

1. Validate auth and IAM permissions.
2. Load the scoped source resume.
3. Load scoped, same-channel, on-shelf positions with departments.
4. Score all positions with the shared matching calculator.
5. Group by department and keep each department's top position.
6. Load current HRBP, manager, and trainee names for result departments.
7. Return sorted route rows without writing recommendation data.

### `POST /recommendations/send`

Request:

```json
{
  "resumeId": "resume_1",
  "departmentId": "dept_b",
  "positionId": "position_b"
}
```

Response:

```json
{
  "resumeId": "resume_copy_1",
  "sourceResumeId": "resume_1",
  "department": { "id": "dept_b", "name": "智算调度部" },
  "position": { "id": "position_b", "name": "调度平台工程师" },
  "candidateName": "张三",
  "reusedExistingCopy": false,
  "notifiedCount": 3,
  "selfNotificationRead": true,
  "message": "已推荐到「智算调度部」· 已通知 3 位相关人员"
}
```

Behavior:

1. Validate auth and IAM permissions.
2. Re-load source resume and target position under current scopes.
3. Reject invalid target state before writing.
4. In one transaction, find an existing target-department resume by `(normalized_name + target_department_id)`.
5. If absent, create the recommendation copy and `DepartmentResume`.
6. If present, update `source_by` and `updated_at` on the existing target copy.
7. Upsert `position_resumes(kind='recommended')`.
8. Commit the recommendation transaction.
9. Compute notification recipients and create notification rows.
10. Record recommendation audit.
11. Return the final target resume and notification summary.

## 10. Frontend Behavior

The page key is `resume-recommend`. It appears only when `/me.pageAccess` includes `resume-recommend`.

UI requirements:

- Page title is `简历推荐`.
- Top channel tabs show authorized channels only: `社招 SOCIAL` and/or `校招 CAMPUS`; default is social when authorized, otherwise the first authorized channel.
- Left workflow panel reuses the E2/E4 source selector:
  - `从简历库选择`
  - `导入新简历`
- Library mode lists resumes from `GET /resumes` for the current channel and scope.
- Upload mode reuses the E4 single import flow and selects the imported resume after job success.
- Switching source mode, channel, or selected resume clears existing routing results.
- `智能分流` is enabled only when a resume is selected.
- During routing, show `Thinking...` with a light scanning or reasoning motion. Respect `prefers-reduced-motion`.
- Before routing, show `分流结果将显示在这里 / 选择/导入简历后点击「智能分流」`.
- Routing results render one row per department with:
  - score;
  - department name;
  - recommended position;
  - HRBP, manager, and trainee names, with multiple names joined by `、`;
  - best route marker on the first row;
  - `推荐到` action.
- After successful send, show `已推荐到「{部门名}」· 已通知 {N} 位相关人员` and keep the result list visible.
- If route calculation returns no rows, show `该渠道下暂无在架岗位 / 请在「部门与岗位管理」中上架岗位`.

Business pages must use shadcn/ui or project-wrapped components for interactive controls.

## 11. Error Codes

Add stable recommendation errors:

- `RECOMMENDATION_ROUTE_FAILED`: unexpected routing failure.
- `RECOMMENDATION_NO_ROUTE_TARGETS`: no scoped on-shelf same-channel positions; frontend usually handles empty result instead of showing an error.
- `RECOMMENDATION_TARGET_POSITION_OFFLINE`: target position is off-shelf and cannot receive a recommendation.
- `RECOMMENDATION_TARGET_POSITION_MISMATCH`: target position does not belong to the submitted department.
- `RECOMMENDATION_CHANNEL_MISMATCH`: target position channel differs from source resume channel.
- `RECOMMENDATION_SEND_FAILED`: unexpected recommendation write failure.
- `RECOMMENDATION_NOTIFICATION_FAILED`: notification creation failed after the main recommendation succeeded; this is logged/audited and not normally returned as a failing user response.

Existing IAM, resume, and position errors continue to apply:

- `IAM_PERMISSION_DENIED`
- `RESUME_NOT_FOUND`
- `POSITION_NOT_FOUND`
- `VALIDATION_FAILED`

Frontend display text must be mapped through i18n.

## 12. Audit, Privacy, and Observability

Audit events are required for:

- successful recommendation send;
- recommendation denied or failed after a safe target ID can be recorded;
- notification creation failure after successful recommendation.

Audit payloads may include:

- source resume ID;
- final target resume ID;
- target department ID;
- target position ID;
- match score;
- channel;
- notified count;
- whether an existing target copy was reused.

Audit payloads must not include:

- raw PDF bytes;
- raw extracted resume text;
- profile JSON;
- phone numbers, emails, ID numbers, or other unnecessary personal data.

Structured logs for notification failure must include `requestId`, recommender ID, final target resume ID, target department ID, and safe error code.

## 13. Testing Requirements

Backend tests:

- route calculation uses the shared matching calculator and preserves E2 scoring behavior.
- route calculation filters by source resume channel, position status, department-position relation, and `Position.List` scope.
- route calculation groups by department and returns only each department's top-scoring position.
- route calculation sorts by score descending and marks only the first row as best.
- contacts are derived from latest `UserDepartmentRole` bindings for HRBP, manager, and trainee roles.
- send recommendation rejects off-shelf target positions before writing.
- send recommendation rejects department-position mismatch and channel mismatch before writing.
- send recommendation creates a target-department resume copy and `DepartmentResume` in one transaction.
- repeated recommendation to the same department reuses the existing target copy and updates `source_by`.
- repeated recommendation to the same position upserts `PositionResume(kind=recommended)`.
- notification recipients include users with effective `Resume.List` or `Resume.Get` scope for the target department and channel.
- self-notification is written as read.
- notification creation failure does not roll back the recommendation copy or `PositionResume`.
- route tests verify permission composition and stable error mapping.
- OpenAPI drift check covers new routes.

Frontend tests:

- recommendation page renders source selector, authorized channel tabs, library list, and initial empty result copy.
- changing channel/source/resume clears routing results.
- upload mode reuses single import and selects the imported resume.
- clicking `智能分流` calls `POST /recommendations/route` and renders sorted department rows.
- best route uses the required marker and action styling.
- clicking `推荐到` calls `POST /recommendations/send` and renders the success summary.
- no-route empty state, API errors, and permission-denied errors show PRD/i18n messages.

Generated artifacts:

- Run OpenAPI generation after backend route changes.
- Run client generation after OpenAPI changes.
- Frontend must call generated client wrappers only.

## 14. Implementation Order

1. Complete or extract the shared E2 matching calculator needed by E3.
2. Backend recommendation route calculation tests and service.
3. Backend recommendation SQL store tests for scoped position loading, grouping inputs, recommendation copy deduplication, and `PositionResume` upsert.
4. Backend notification recipient calculation and notification insert tests.
5. Backend routes, errors, audit event, OpenAPI generation, and client generation.
6. Frontend API client wrappers and tests.
7. Frontend `ResumeRecommendPage` with source selection, routing result, and send action.
8. Project status and agent guide updates after verification.
