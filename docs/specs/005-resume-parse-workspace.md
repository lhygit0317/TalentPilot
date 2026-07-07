# 005 Resume Parse Workspace SPEC

## 1. Scope and Goals

This SPEC defines E2: the resume parsing workspace. It covers the "简历解析" page, resume source selection, channel switching, target JD selection, parse/matching execution, `PositionResume(kind=parsed)` persistence, explainable match details, and three-round interview question generation.

E2 consumes the E4 resume library and E5 position data. Backend authorization remains authoritative. Frontend state is only a workflow aid and must not become a security boundary.

## 2. Dependencies and Readiness

Required before E2 implementation:

- E1 auth/session behavior from [001 Auth Session W3 SPEC](001-auth-session-w3.md).
- IAM runtime behavior from [002 IAM Permission Model SPEC](002-iam-permission-model.md), especially `Resume.List`, `Resume.Get`, `Position.List`, `Position.Get`, `DepartmentPosition.List`, and `PositionResume.Create`.
- E4 resume library behavior from [003 Resume Library and Import SPEC](003-resume-library-import.md), especially `GET /resumes`, `GET /resumes/{resumeId}`, and single PDF import.
- E5 department and position behavior from [004 Department and Position Management SPEC](004-position-department-management.md), especially permission-filtered `GET /positions` and `GET /positions/{positionId}`.
- Existing `position_resumes` table with unique `(resume_id, position_id, kind)`.

## 3. Non-Goals

- Resume recommendation and routing. Those belong to E3.
- Notification creation. That belongs to E3/E7.
- Full asynchronous AI infrastructure for matching and interview questions. E2 uses a synchronous backend-owned matching service with a deterministic AI adapter boundary.
- Saving complete parse snapshots. PRD section 6.13 remains authoritative: `PositionResume` stores only relation metadata and `matchScore`.
- Real model-provider integration. Production model selection and provider credentials require a separate future AI-service decision.
- Department, position, user, role, or custom permission administration.

## 4. Design Approach

Three implementation scopes were considered:

1. Synchronous matching plus deterministic interview-question generation.
2. Fully asynchronous jobs for parse and interview-question generation.
3. Frontend-only matching with minimal backend persistence.

This SPEC chooses option 1. It completes PRD Story 2.1-2.5 in one delivery, keeps scoring and persistence in the backend, avoids introducing a broad job state machine in E2, and leaves a stable adapter seam for future real AI integration.

## 5. Domain Model

E2 introduces a focused backend package, `apps/api/internal/matching`, responsible for:

- loading authorized resume and position details;
- validating parse eligibility;
- calculating scores and explainable evidence;
- upserting `position_resumes(kind='parsed')`;
- generating deterministic interview questions through an `InterviewQuestionGenerator` interface.

No new database table is required for E2.

`position_resumes` behavior:

- `kind` is `parsed`;
- `match_score` stores the final rounded score;
- `by_user_id` stores the actor;
- repeated parse for the same `(resumeId, positionId, parsed)` updates `match_score`, `created_at`, and `by_user_id` so the visible result represents the latest run.

The full result detail is returned to the current request only. It is recomputed from current `Resume` and `Position` data on each parse.

## 6. Permissions and Data Scope

Routes must use session authentication first, then IAM guards:

| Capability | Required permissions |
| --- | --- |
| Use parse workspace | `/me.pageAccess` includes `resume-parse` |
| List selectable resumes | `Resume.List` |
| Get selected resume | `Resume.Get` |
| List selectable positions | `Position.List` + `DepartmentPosition.List` |
| Get selected position | `Position.Get` + `DepartmentPosition.List` |
| Create parsed relation | `PositionResume.Create` |
| Generate interview questions | `Resume.Get` + `Position.Get` |

Parse execution must verify both target records are inside the caller's effective scopes. It must not trust resume or position IDs selected in the frontend.

Data-scope rules:

- Resume scope uses `department_resumes.department_id` plus `Resume` attribute conditions such as `chan`.
- Position scope uses `department_positions.department_id`.
- The selected position must have `status='on'`.
- The selected resume and selected position may have different channels only if both are independently authorized, but the UI defaults to positions in the current channel. This preserves PRD visibility while avoiding hidden backend coupling.
- `PositionResume.Create` scope must allow the selected position department.

## 7. Matching Algorithm

The acceptance algorithm is deterministic and centralized in the backend:

- Skill score = `len(JD keywords intersect candidate keywords) / len(JD keywords) * 100`.
- If JD keywords are empty, skill score is `0`.
- Implicit score = `sum(matched implicit tag weights) / sum(all implicit tag weights) * 100`.
- If JD implicit tags are empty or total weight is `0`, implicit score is `0`.
- Experience score = `Resume.exp_base`, clamped to `0..100`, defaulting to `60` when missing or zero.
- Total score = `round(skill * 0.4 + experience * 0.25 + implicit * 0.35)`, clamped to `0..100`.

Keyword and implicit matching is case-insensitive after trimming whitespace. Returned details include matched and missing entries in JD order.

Judgement labels:

- `>= 80`: `强烈推荐`
- `>= 65`: `建议进入面试`
- `< 65`: `谨慎或暂不推荐`

Result colors are frontend presentation only:

- green for `>= 80`;
- amber for `>= 65`;
- red for `< 65`.

## 8. API Contract

Backend route definitions remain the OpenAPI source of truth. DTOs must not expose GORM models directly.

### `POST /matching/parse`

Request:

```json
{
  "resumeId": "resume_1",
  "positionId": "position_1"
}
```

Response:

```json
{
  "id": "position_resume_1",
  "resume": {
    "id": "resume_1",
    "name": "张三",
    "chan": "social",
    "pos": "平台工程师",
    "source": "导入",
    "sourceBy": "李四",
    "currentDepartment": { "id": "dept_a", "name": "算力训练平台部" },
    "keywords": ["Go", "调度"],
    "traits": ["稳定"]
  },
  "position": {
    "id": "position_1",
    "name": "平台工程师",
    "department": { "id": "dept_a", "name": "算力训练平台部" },
    "chan": "social",
    "level": "P6",
    "keywords": ["Go", "Kubernetes"],
    "implicitTags": [{ "name": "稳定", "w": 40 }]
  },
  "score": {
    "total": 78,
    "skill": 50,
    "experience": 80,
    "implicit": 100,
    "judgement": "建议进入面试"
  },
  "evidence": {
    "keywords": [
      { "name": "Go", "matched": true },
      { "name": "Kubernetes", "matched": false }
    ],
    "implicitTags": [
      { "name": "稳定", "w": 40, "matched": true }
    ],
    "analysis": "技能命中 1/2；隐性要求命中 1/1；建议进入面试。"
  },
  "createdAt": "2026-07-07T08:00:00Z"
}
```

Behavior:

1. Validate auth and IAM permissions.
2. Load scoped resume and scoped position.
3. Reject an off-shelf position with `MATCHING_POSITION_OFFLINE`.
4. Calculate match details.
5. Upsert `position_resumes(kind='parsed')`.
6. Record an audit event without profile, raw text, or full resume content.
7. Return the current explainable result.

### `POST /matching/interview-questions`

Request:

```json
{
  "resumeId": "resume_1",
  "positionId": "position_1",
  "matchScore": 78
}
```

`matchScore` is optional. If omitted, the backend recalculates the current match score.

Response:

```json
{
  "groups": [
    {
      "type": "professional",
      "label": "专业面试",
      "questions": [
        {
          "order": 1,
          "question": "请结合 Go 项目说明你如何处理高并发调度场景。",
          "why": "验证候选人关键词与岗位核心技能的真实经验。",
          "difficulty": "核心"
        }
      ]
    }
  ]
}
```

Question rules:

- Professional interview has at least 3 questions. It uses the candidate's first keyword when available, otherwise the position's first keyword, otherwise the position name.
- If experience score is `>= 82`, professional interview adds one advanced question with difficulty `拔高`.
- Manager interview has 3 questions: cross-team collaboration, motivation/stability, and handling disagreement with a manager. The "为什么选择本部门" question includes the selected position's department name.
- Qualification interview has 3 questions: background-check confirmation, salary/onboarding confirmation, and travel/overtime acceptance.

## 9. Frontend Behavior

The page key is `resume-parse`. It appears only when `/me.pageAccess` includes `resume-parse`.

UI requirements:

- Page title is `简历解析`.
- Top channel tabs show authorized channels only: `社招 SOCIAL` and/or `校招 CAMPUS`; default is social when authorized, otherwise the first authorized channel.
- Left workflow panel includes a segmented source selector:
  - `从简历库选择`
  - `导入新简历`
- Library mode lists resumes from `GET /resumes` for the current channel and scope. Items show name, intended position, current department, keywords, and recommended source suffix when `source='推荐'`.
- Upload mode reuses the E4 single import flow and selects the imported resume after job success.
- Switching source mode, channel, selected resume, or selected position clears existing parse results and interview questions.
- Target JD selector lists `GET /positions?status=on&chan=currentChannel` within permission scope. Items display `{部门} · {岗位名}({渠道})`.
- If no scoped on-shelf position exists, show `请先在「部门与岗位管理」中维护岗位`.
- The parse action is enabled only when a resume and position are selected.
- During parse, show a scanner-style result panel with `Thinking...`. Respect `prefers-reduced-motion`.
- Result card contains all PRD Story 2.4 sections.
- `生成面试题` appears only after a parse result exists. During generation, show `Thinking...`, then render nested tabs: `专业`, `主管`, `资格`.

Business pages must use shadcn/ui or project-wrapped components for interactive controls.

## 10. Error Codes

Add stable matching errors:

- `MATCHING_POSITION_OFFLINE`: selected position is off-shelf and cannot participate in parsing.
- `MATCHING_PARSE_FAILED`: unexpected parse/matching failure.
- `MATCHING_INTERVIEW_FAILED`: interview-question generation failed.

Existing IAM, resume, and position errors continue to apply:

- `IAM_PERMISSION_DENIED`
- `RESUME_NOT_FOUND`
- `POSITION_NOT_FOUND`
- `VALIDATION_FAILED`

Frontend display text must be mapped through i18n.

## 11. Audit, Privacy, and Observability

Audit events are required for:

- successful parse relation upsert;
- failed parse caused by an off-shelf position or out-of-scope target when the backend can safely record it.

Audit payloads may include:

- `resumeId`
- `positionId`
- `matchScore`
- `departmentId`
- `chan`

Audit payloads must not include:

- raw PDF bytes;
- raw extracted resume text;
- profile JSON;
- phone numbers, emails, ID numbers, or other unnecessary personal data.

## 12. Testing Requirements

Backend tests:

- matching score calculation covers skill, experience, implicit tags, empty keywords, empty implicit tags, rounding, and clamping.
- parse service rejects off-shelf positions before writing `position_resumes`.
- parse service upserts the parsed relation and updates repeated parses.
- parse service enforces resume, position, and `PositionResume.Create` scopes.
- interview generator returns all three groups and includes department name in manager questions.
- high-potential candidates add the advanced professional question.
- route tests verify permission composition and stable error mapping.
- OpenAPI drift check covers new routes.

Frontend tests:

- parse page renders source selector, authorized channel tabs, library list, and scoped on-shelf JD selector.
- changing channel/source/resume/position clears results.
- upload mode reuses single import and selects the imported resume.
- clicking parse calls `POST /matching/parse` and renders score, judgement, chips, and analysis.
- clicking interview generation renders three tabs and question metadata.
- file validation, API errors, empty resume list, and empty position list show PRD/i18n messages.

Generated artifacts:

- Run OpenAPI generation after backend route changes.
- Run client generation after OpenAPI changes.
- Frontend must call generated client wrappers only.

## 13. Implementation Order

1. Backend matching domain tests and score calculator.
2. Backend SQL store and parse persistence tests.
3. Backend routes, errors, audit event, OpenAPI generation.
4. Frontend API client wrappers and tests.
5. Frontend `ResumeParsePage` with source selection, parse result, and interview questions.
6. Project status and agent guide updates after verification.
