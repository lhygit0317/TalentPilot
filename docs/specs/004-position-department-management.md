# 004 Department and Position Management SPEC

## 1. Scope and Goals

This SPEC defines E5: department and position management. It covers permission-filtered department listing/detail, department create/update/delete, position listing/detail, position create/update/delete, position on/off status changes, department-position ownership maintenance, delete protection, audit boundaries, frontend workflows, and tests.

E5 supplies the department and job data that E2 resume parsing and E3 recommendation consume. Backend authorization remains authoritative. Frontend visibility only improves user experience.

## 2. Dependencies and Readiness

Required before E5 implementation:

- E1 auth/session behavior from [001 Auth Session W3 SPEC](001-auth-session-w3.md).
- IAM runtime behavior from [002 IAM Permission Model SPEC](002-iam-permission-model.md).
- E4 resume library behavior from [003 Resume Library and Import SPEC](003-resume-library-import.md), especially `DepartmentResume` counts and import target behavior.
- Existing foundation tables: `departments`, `positions`, `department_positions`, `department_resumes`, `position_resumes`, `user_department_roles`, and `audit_logs`.

E5 extends the IAM whitelist and preset-role seeds to include `Department.Create`, `Department.Update`, and `Department.Delete` for super admins. Goose migrations remain the schema and seed source of truth.

## 3. Non-Goals

- User-role assignment, HRBP/manager/trainee membership display, or UserDepartmentRole management. Those belong to E6.
- Resume parsing, recommendation, matching calculation, and notifications. Those belong to E2/E3/E7.
- Custom role management. That belongs to E8.
- Bulk import/export for departments or positions.
- Full text search infrastructure. E5 uses database-backed filters first.

## 4. Design Approach

Three implementation scopes were considered:

1. Read-only department and position pages.
2. CRUD-only backend APIs without frontend workflows.
3. Full E5 workflow with permission-filtered read APIs, super-admin writes, delete protection, generated client wrappers, and a frontend management page.

This SPEC chooses option 3. It matches PRD Story 5.1-5.6 and creates the minimum reliable foundation for E2/E3.

## 5. Domain Model

E5 uses existing goose tables:

- `departments` stores department master data.
- `positions` stores JD fields:
  - `name`
  - `chan`: `social` or `campus`
  - `level`
  - `status`: `on` or `off`
  - `duties`: JSON string array
  - `must`: JSON string array
  - `keywords`: JSON string array
  - `implicit_tags`: JSON array of objects such as `{ "name": "系统设计", "w": 40 }`
- `department_positions` stores the current owning department for a position.
- `department_resumes`, `position_resumes`, and `user_department_roles` are used for counts and delete protection.

One position belongs to one department for E5 workflows. If existing data ever has multiple department-position rows for one position, APIs must return a deterministic primary department and keep update/delete behavior conservative.

## 6. Permissions and Data Scope

Routes must use session authentication first, then IAM guards:

| Capability | Required permissions |
| --- | --- |
| List departments | `Department.List` |
| Get department detail | `Department.Get` |
| Create department | `Department.Create` |
| Update department | `Department.Update` |
| Delete department | `Department.Delete` |
| List positions | `Position.List` + `DepartmentPosition.List` |
| Get position detail | `Position.Get` + `DepartmentPosition.List` |
| Create position | `Position.Create` + `DepartmentPosition.Create` |
| Update position | `Position.Update`; department changes also require `DepartmentPosition.Create` and `DepartmentPosition.Delete` |
| Change position status | `Position.Update` |
| Delete position | `Position.Delete` + `DepartmentPosition.Delete` |

Department and position list/detail queries must consume IAM scope predicates and push department-scope filtering into SQL. Do not fetch all departments or positions and filter in the frontend.

Data-scope rules:

- A concrete department binding sees that department.
- A system-scope binding sees all non-system departments.
- Department/position writes are limited to super admins by preset permissions.
- Frontend row capabilities such as `canUpdate` and `canDelete` are UX only; backend route checks remain mandatory.

## 7. API Contract

Backend route definitions remain the OpenAPI source of truth. DTOs must not expose GORM models directly.

### `GET /departments`

Query:

- `search`: optional plain-text department name search.
- `limit`: default 50, maximum 100.

Response:

```json
{
  "items": [
    {
      "id": "dept_1",
      "name": "算力训练平台部",
      "positionCount": 3,
      "resumeCount": 12,
      "updatedAt": "2026-07-04T08:00:00Z",
      "canGet": true,
      "canUpdate": false,
      "canDelete": false
    }
  ]
}
```

The system department `__system__` is not shown in business department management lists.

### `GET /departments/{departmentId}`

Returns department summary, position summaries, resume count, and row capabilities.

### `POST /departments`

Body:

```json
{ "name": "智算调度部" }
```

Creates a department. Name is required and unique after trimming.

### `PATCH /departments/{departmentId}`

Body:

```json
{ "name": "智算平台部" }
```

Updates a department name. The system department cannot be updated.

### `DELETE /departments/{departmentId}`

Deletes a department only when it has no `department_positions`, `department_resumes`, or `user_department_roles`. The system department cannot be deleted.

### `GET /positions`

Query:

- `departmentId`: optional department filter.
- `chan`: optional `social` or `campus`.
- `status`: optional `on` or `off`.
- `search`: optional plain-text name/keyword search.
- `limit`: default 50, maximum 100.

Response:

```json
{
  "items": [
    {
      "id": "pos_1",
      "name": "平台工程师",
      "department": { "id": "dept_1", "name": "算力训练平台部" },
      "chan": "social",
      "level": "P6",
      "status": "on",
      "keywordCount": 5,
      "implicitTagCount": 2,
      "canGet": true,
      "canUpdate": false,
      "canDelete": false
    }
  ]
}
```

Positions without a `DepartmentPosition` relation are excluded from list results.

### `GET /positions/{positionId}`

Returns JD detail:

```json
{
  "id": "pos_1",
  "name": "平台工程师",
  "department": { "id": "dept_1", "name": "算力训练平台部" },
  "chan": "social",
  "level": "P6",
  "status": "on",
  "duties": ["负责训练平台服务端研发"],
  "must": ["熟悉 Go"],
  "keywords": ["Go", "调度"],
  "implicitTags": [{ "name": "系统设计", "w": 40 }],
  "updatedAt": "2026-07-04T08:00:00Z",
  "canUpdate": false,
  "canDelete": false
}
```

### `POST /positions`

Body:

```json
{
  "name": "平台工程师",
  "departmentId": "dept_1",
  "chan": "social",
  "level": "P6",
  "status": "on",
  "duties": ["负责训练平台服务端研发"],
  "must": ["熟悉 Go"],
  "keywords": ["Go", "调度"],
  "implicitTags": [{ "name": "系统设计", "w": 40 }]
}
```

Creates a position and its `DepartmentPosition` relation in one transaction.

### `PATCH /positions/{positionId}`

Updates mutable JD fields. If `departmentId` changes, the service replaces the old `DepartmentPosition` relation with the new one in the same transaction.

### `DELETE /positions/{positionId}`

Deletes a position and its `DepartmentPosition` relation only when no `position_resumes` history exists. If history exists, return a stable error and keep the position; users should set `status=off`.

## 8. Frontend Behavior

The page key is `departments-positions`. It appears only when `/me.pageAccess` includes `departments-positions`.

UI requirements:

- Page title is `部门与岗位`.
- Top-level tabs: `部门管理` and `岗位管理`.
- Department table columns: department name, related position count, related resume count, updated time, operations.
- Department detail shows department name, position summary list, and resume count. It does not show HRBP/manager/trainee staff lists.
- Position list supports department, channel, status, and search filters. It shows name, department, channel, level, status, keyword count, implicit tag count, and operations.
- Position detail shows duties, must-have requirements, keyword chips, implicit tags with weights, status, and department.
- Non-super-admin users see read-only actions only.
- Super admins see create/edit/delete for departments and create/edit/on-off/delete for positions.
- Empty states:
  - no departments: `暂无部门`
  - no positions: `暂无岗位`
  - search no results: `暂无匹配结果`
- Success toasts:
  - department created: `已新增部门`
  - department updated: `已更新部门`
  - department deleted: `已删除部门`
  - position created: `已新增岗位`
  - position updated: `已更新岗位`
  - position on/off: `岗位已上架` / `岗位已下架`
  - position deleted: `已删除岗位`

Business pages must use shadcn/ui or project-wrapped components for interactive controls.

## 9. Validation Rules

Department validation:

- `name` is required after trimming.
- `name` must be unique.
- `__system__` cannot be updated or deleted through E5.

Position validation:

- `name` and `departmentId` are required.
- `chan` must be `social` or `campus`.
- `status` must be `on` or `off`.
- `duties`, `must`, `keywords`, and `implicitTags` are arrays.
- Empty strings are removed from string arrays.
- Duplicate keywords are rejected case-insensitively.
- Duplicate implicit tag names are rejected case-insensitively.
- Implicit tag weight defaults to `40` when omitted and must be between `0` and `100`.

## 10. Error Codes

Add these stable department/position errors:

- `DEPARTMENT_NOT_FOUND`
- `DEPARTMENT_NAME_REQUIRED`
- `DEPARTMENT_NAME_DUPLICATE`
- `DEPARTMENT_DELETE_HAS_RELATIONS`
- `DEPARTMENT_SYSTEM_PROTECTED`
- `POSITION_NOT_FOUND`
- `POSITION_NAME_REQUIRED`
- `POSITION_DEPARTMENT_REQUIRED`
- `POSITION_DEPARTMENT_INVALID`
- `POSITION_INVALID_CHANNEL`
- `POSITION_INVALID_STATUS`
- `POSITION_DUPLICATE_KEYWORD`
- `POSITION_DUPLICATE_IMPLICIT_TAG`
- `POSITION_INVALID_IMPLICIT_WEIGHT`
- `POSITION_DELETE_HAS_HISTORY`

IAM denials should continue using IAM error codes. Messages are Chinese backend fallbacks; frontend display text maps through i18n.

## 11. Audit, Privacy, and Observability

Audit events are required for:

- department create/update/delete;
- position create/update/status change/delete;
- sensitive write denials that reach IAM guard.

Audit details may include IDs, department name, position name, channel, status, changed field names, and relation IDs. Audit and logs must not include resumes, raw JD documents beyond stored structured fields, or unrelated user-role data.

## 12. Testing Requirements

Implementation must follow red-green-refactor TDD.

Backend tests:

- IAM whitelist and preset seeds include department write permissions only for super admins.
- `GET /departments` applies department data scope and excludes `__system__`.
- `GET /departments/{id}` denies out-of-scope access.
- department create rejects empty and duplicate names.
- department update rejects duplicate names and protected system department.
- department delete rejects departments with positions, resumes, or user-role bindings.
- `GET /positions` applies department data scope and excludes orphan positions.
- `GET /positions/{id}` denies out-of-scope access.
- position create validates required fields, channel/status, duplicate keywords, and implicit tags.
- position create creates `Position` and `DepartmentPosition` transactionally.
- position update can move department relation transactionally.
- position status update changes `on/off`.
- position delete rejects existing `PositionResume` history and deletes relation when safe.
- audit rows are written for department and position writes without unrelated payloads.
- OpenAPI and generated client stay in sync after route changes.

Frontend tests:

- navigation renders `部门与岗位` when page access includes `departments-positions`.
- department tab renders list columns and read-only actions for non-super-admin users.
- super admin can open create/edit department forms and sees validation errors.
- department delete protection errors show stable translated messages.
- position tab renders filters, list, and JD detail.
- position form rejects duplicate keywords and duplicate implicit tags before submission.
- super admin can create/edit/on-off/delete positions through generated client wrappers.
- unauthorized write actions are hidden or disabled.

E2E coverage may be deferred until Playwright is installed; `make test-e2e` remains outside the current passing gate.

## 13. Acceptance Criteria

E5 is complete when:

- authorized users can list and view departments and positions within backend-enforced IAM data scope;
- non-super-admin users cannot create/update/delete departments or positions from UI or API;
- super admins can create, update, and delete safe departments;
- super admins can create, update, on/off, and delete safe positions;
- delete protection prevents removing departments or positions with active relationships/history;
- position JD detail fields are available for E2/E3 consumption;
- generated OpenAPI and frontend client are current;
- `docs/project-status.md` and `AGENTS.md` point to this SPEC as the active E5 source;
- all behavior is covered by failing-first tests before production implementation.
