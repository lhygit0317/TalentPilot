# 003 Resume Library and Import SPEC

## 1. Scope and Goals

This SPEC defines E4: the resume library. It covers permission-filtered resume listing, channel counts, keyword search, structured resume detail, single and batch PDF import, import target department selection, resume deletion, source labels, audit boundaries, and the tests required before implementation.

E4 is the first business EPIC that consumes IAM data-scope predicates in domain queries. Backend authorization remains authoritative; frontend page visibility and disabled controls only improve user experience.

## 2. Dependencies and Readiness

Required before E4 implementation:

- E1 auth/session behavior from [001 Auth Session W3 SPEC](001-auth-session-w3.md).
- IAM runtime behavior from [002 IAM Permission Model SPEC](002-iam-permission-model.md), especially `Resume.List`, `Resume.Get`, `Resume.Create`, `Resume.Delete`, `DepartmentResume.Create`, `DepartmentResume.Delete`, `Job.Get`, and scope predicates.
- Existing foundation tables: `resumes`, `department_resumes`, `position_resumes`, `notifications`, `jobs`, `departments`, `audit_logs`.
- A PDF parsing adapter boundary. Development and tests may use a mock parser; production must use production-safe storage and parsing configuration.

E5 department management is not implemented by this SPEC. E4 may expose a minimal read-only import-target selector backed by existing `departments` data, but it must not implement department create/update/delete or position management.

## 3. Non-Goals

- Full department and position administration. That belongs to E5.
- Resume-to-position matching, interview question generation, and parsing workspace behavior. Those belong to E2.
- Recommendation, resume copy deduplication, and notifications. Those belong to E3/E7.
- Custom role management. That belongs to E8.
- Search-index infrastructure. E4 uses database-backed filtering first; future search indexes must preserve backend permission filtering.
- Production sample resumes, users, or positions.

## 4. Design Approach

Three implementation scopes were considered:

1. List-only library: implement `GET /resumes` and details, defer import and deletion.
2. Synchronous import library: list, detail, delete, and parse uploads in the request path.
3. Permission-filtered library with asynchronous import jobs: list/detail/delete are synchronous; PDF extraction and batch import run through jobs.

This SPEC chooses option 3. It matches the PRD requirement for batch import and avoids request timeouts for PDF parsing. The implementation plan may still split delivery into verifiable batches: list/detail/delete first, then import jobs, then frontend workflows.

## 5. Domain Model

E4 uses the existing schema source of truth in goose migrations:

- `resumes` stores structured candidate fields and JSON payloads.
- `department_resumes` stores the current department owner for each resume. One active resume belongs to one department.
- `position_resumes` stores parse/recommend/manual relationships and is cascaded when a resume is deleted.
- `notifications` are not cascaded by resume deletion and preserve historical notification records.
- `jobs` stores asynchronous import status.
- `audit_logs` records sensitive writes and denied sensitive writes.

`Resume.profile` should store structured sections:

```json
{
  "basic": {},
  "education": [],
  "workExperience": [],
  "projects": [],
  "skills": [],
  "certificates": [],
  "rawTextRef": ""
}
```

Query-critical fields remain columns: `normalized_name`, `name`, `age`, `school`, `years_exp`, `pos`, `source`, `source_by`, `chan`, `expired`, and timestamps.

## 6. Permissions and Data Scope

Routes must use session authentication first, then IAM guards:

| Capability | Required permissions |
| --- | --- |
| List resumes | `Resume.List` |
| Get resume detail | `Resume.Get` |
| Single import | `Resume.Create` + `DepartmentResume.Create` |
| Batch import | `Resume.Create` + `DepartmentResume.Create` + `Job.Get` |
| Delete resume | `Resume.Delete` + `DepartmentResume.Delete` when deleting ownership relation |
| Import target selector | `Department.List` or IAM data-scope departments from `/me` |
| Import job status | `Job.Get` scoped to jobs created by the current user |

List/Get/Delete queries must consume IAM `ScopePredicate` objects and push predicates into database queries. Handlers must not concatenate raw SQL from IAM output, fetch all resumes, or filter permissions in the frontend.

Effective resume access is the OR union of all allowed IAM predicate branches. A branch may combine department scope and attribute conditions such as `chan` and `expired`. Examples:

- HRBP bound to department A sees resumes owned by department A.
- HRD bound to `__system__` sees all departments unless narrowed by another condition.
- 社招负责人 with system scope sees all social resumes and no campus resumes.
- Multiple bindings combine as a union.

Frontend controls may include row capabilities such as `canGet` and `canDelete`, but backend route checks remain mandatory.

## 7. API Contract

Backend route definitions remain the OpenAPI source of truth. DTOs must not expose GORM models directly.

### `GET /resumes`

Query:

- `chan`: `social` or `campus`; default is the first available channel, preferring `social`.
- `search`: optional plain-text keyword. It is case-insensitive and treated as literal text, not a regular expression.
- `limit`: default 50, maximum 100.
- `cursor`: optional opaque pagination cursor.

Response:

```json
{
  "items": [
    {
      "id": "resume_1",
      "name": "张三",
      "age": 29,
      "school": "浙江大学",
      "yearsExp": 6,
      "currentDepartment": {"id": "dept_1", "name": "算力训练平台部"},
      "pos": "算法工程师",
      "source": "导入",
      "sourceBy": "李四",
      "chan": "social",
      "keywords": ["Go", "调度"],
      "canGet": true,
      "canDelete": false
    }
  ],
  "channelCounts": {"social": 12, "campus": 3},
  "availableChannels": ["social", "campus"],
  "dataScopeSummary": "负责部门:算力训练平台部",
  "nextCursor": ""
}
```

`channelCounts` are computed after IAM permission filtering and before the selected-channel filter. Counts must not leak unauthorized channel data.

### `GET /resumes/{resumeId}`

Returns the structured detail header and `profile` sections for one authorized resume. If the resume is outside the caller's `Resume.Get` scope, return `IAM_PERMISSION_DENIED` or a not-found-shaped denial according to the central error policy, without leaking sensitive details.

### `POST /resumes/imports`

Multipart fields:

- `file`: required PDF, <= 10 MB.
- `chan`: required `social` or `campus`.
- `targetDepartmentId`: optional only when the backend can infer exactly one non-system target department.

Behavior:

1. Validate auth, CSRF, IAM permissions, file extension, MIME, and size.
2. Resolve target department:
   - one allowed non-system department: use it when omitted;
   - multiple allowed departments or only system scope: require `targetDepartmentId`;
   - target outside allowed create scope: return permission denial.
3. Create an import job and parse the file asynchronously.
4. On parse success, create `Resume` and `DepartmentResume` in one transaction.
5. On parse failure, do not create resume ownership rows; mark the job failed with a safe error code.

Response:

```json
{
  "jobId": "job_1",
  "status": "pending"
}
```

### `POST /resumes/batch-imports`

Multipart fields:

- `files`: one or more PDFs, each <= 10 MB.
- `chan`: required `social` or `campus`.
- `targetDepartmentId`: same resolution rules as single import.

Creates one batch job with per-file results. A single file failure must not block other files.

### `GET /jobs/{jobId}`

Returns job status only for jobs the current user is allowed to see. E4 only requires import job status:

```json
{
  "id": "job_1",
  "type": "resume_import",
  "status": "succeeded",
  "summary": {
    "total": 2,
    "succeeded": 1,
    "failed": 1
  },
  "results": [
    {"fileName": "a.pdf", "status": "succeeded", "resumeId": "resume_1", "name": "张三"},
    {"fileName": "b.pdf", "status": "failed", "errorCode": "RESUME_IMPORT_PARSE_FAILED"}
  ]
}
```

### `DELETE /resumes/{resumeId}`

Requires the target to be inside the caller's `Resume.Delete` scope. Deleting a resume cascades `DepartmentResume` and `PositionResume` through database constraints. Notifications remain.

## 8. Frontend Behavior

The resume library page key is `resume-library`. It appears only when `/me.pageAccess` includes `resume-library`.

UI requirements:

- Page title is `简历库`.
- Top area includes channel tabs with authorized counts, a keyword search input, a data-scope banner, and a batch import action when create permissions are available.
- Table columns: candidate name, age, school, years of experience, current department, intended position, source, keywords, operations.
- No avatar or initial-letter avatar is shown.
- Row operations include `查看详情`; `删除` appears only when the row is deletable by the current user's effective scope.
- Detail opens in a drawer or detail route and displays all structured sections. Empty structured fields show `未解析到`.
- Source chips:
  - `导入`: grey chip `{sourceBy}导入`.
  - `推荐`: cyan chip `{sourceBy}推荐`.
  - empty `sourceBy`: `未知来源`.
- Search highlights matches in candidate names and keyword chips with `<mark>` after HTML escaping.
- Search must preserve input focus and cursor position while results refresh.
- Empty states use PRD copy:
  - no channel resumes: `该渠道下暂无简历`;
  - search no results: `该渠道下暂无简历(无匹配关键词)`.

Business pages must use shadcn/ui or project-wrapped components for interactive controls.

## 9. Import Workflows

### Single Import

Single import is available from the parse/recommend upload controls and may also be reused by the library. After job success, show `✓ 已导入「{姓名}」并加入简历库`. If parsing fails, keep the upload entry available and show a recoverable error.

### Batch Import

Batch import is available from the resume library. The frontend selects or confirms target department first, then uploads multiple PDFs for the current channel. The final toast is `已批量导入 {N} 份{渠道}简历`, where `N` counts successes. Per-file failures remain visible in the batch result.

### Parser Boundary

E4 introduces a parser interface that returns normalized resume fields and profile JSON. It must not log full resume text, full PDFs, phone numbers, emails, ID numbers, or other unnecessary sensitive data.

## 10. Error Codes

Add stable resume/job errors as needed:

- `RESUME_NOT_FOUND`
- `RESUME_IMPORT_FILE_TOO_LARGE`
- `RESUME_IMPORT_UNSUPPORTED_TYPE`
- `RESUME_IMPORT_TARGET_DEPARTMENT_REQUIRED`
- `RESUME_IMPORT_TARGET_DEPARTMENT_INVALID`
- `RESUME_IMPORT_PARSE_FAILED`
- `RESUME_IMPORT_EMPTY_FILE`
- `RESUME_DELETE_DENIED`
- `JOB_NOT_FOUND`
- `JOB_ACCESS_DENIED`

IAM denials should continue using IAM error codes. Messages are Chinese backend fallbacks; frontend display text maps through i18n.

## 11. Audit, Privacy, and Observability

Audit events are required for:

- successful resume import;
- failed import after file validation has accepted the request;
- resume deletion;
- sensitive write denials that reach IAM guard.

Audit details may include resume ID, normalized name, channel, target department ID, source, sourceBy, job ID, file count, and result counts. Audit and logs must not include full PDF content, full raw text, phone numbers, emails, ID numbers, or full profile payloads.

Structured logs for import jobs should include `requestId`, `jobId`, `userId`, `chan`, `targetDepartmentId`, file count, status, duration, and safe error code.

## 12. Testing Requirements

Implementation must follow red-green-refactor TDD.

Backend tests:

- `GET /resumes` applies IAM scope predicates in repository queries.
- multiple department bindings union their visible resumes.
- channel attribute conditions hide unauthorized channels and counts.
- search is literal, case-insensitive, and applied after IAM filtering.
- `GET /resumes/{id}` denies unauthorized detail access.
- single import rejects non-PDF files and files over 10 MB before parser execution.
- single import requires target department when it cannot be inferred.
- single import creates `Resume` and `DepartmentResume` in one transaction on parse success.
- parse failure creates no resume ownership rows and marks job failed.
- batch import records per-file success/failure and does not stop after one failure.
- delete cascades `DepartmentResume` and `PositionResume` while preserving `Notification`.
- audit rows are written for import and delete without sensitive payloads.
- OpenAPI and generated client stay in sync after route changes.

Frontend tests:

- navigation renders `简历库` when `pageAccess` includes `resume-library`.
- channel tabs show authorized counts and hide or disable unauthorized channels.
- data-scope banner is displayed.
- list table renders required columns and no avatar.
- search preserves focus and highlights escaped matches.
- detail drawer shows `未解析到` for empty structured sections.
- unauthorized row actions are hidden or disabled.
- single and batch import show validation errors and success toasts.

E2E coverage may be deferred until Playwright is installed, but the implementation plan must state that exception explicitly.

## 13. Acceptance Criteria

E4 is complete when:

- authorized users can list resumes within backend-enforced IAM data scope;
- channel counts and channel availability do not leak unauthorized data;
- keyword search filters and highlights safely;
- authorized users can view structured details;
- PDF upload validation is enforced on both frontend and backend;
- single import creates `Resume` and `DepartmentResume` transactionally after successful parsing;
- batch import reports per-file outcomes through jobs;
- authorized users can delete resumes, with expected cascade behavior and preserved notifications;
- source labels and empty states match the PRD;
- audit logs and structured logs avoid sensitive resume payloads;
- generated OpenAPI and frontend client are current;
- `docs/project-status.md` and `AGENTS.md` point to this SPEC as the active E4 source;
- all behavior is covered by failing-first tests before production implementation.
