# 002 IAM Permission Model SPEC

## 1. Scope and Goals

This SPEC defines the IAM foundation required before the resume, position, recommendation, notification, user-role, and custom-role EPICs. It covers runtime authorization, permission expansion, data-scope predicates, preset role and permission seeds, RoleRelation inheritance, guest fallback, cache invalidation, audit boundaries, backend APIs needed by the frontend shell, and the tests required before implementation.

The backend remains the only trusted authorization boundary. Frontend permission state only improves navigation and button visibility.

## 2. Non-Goals

- This SPEC does not implement the E6 user-role management UI.
- This SPEC does not implement the E8 custom role management UI.
- This SPEC does not implement resume, position, recommendation, or notification business APIs.
- This SPEC does not add a production cache service. It defines cache keys and invalidation rules so an in-memory implementation can later be replaced.
- This SPEC does not change W3 authentication behavior from [001 Auth Session W3 SPEC](001-auth-session-w3.md).

## 3. Design Approach

Three implementation scopes were considered:

1. Minimal checker only: expose `Can(user, resource, action)`.
2. Runtime IAM core: resolve roles, inherited permissions, department scope, attribute conditions, and query predicates.
3. Full IAM administration: runtime IAM plus role/user management screens.

This SPEC chooses option 2. It is the smallest slice that unblocks E4, E5, E2, E3, E6, and E8 without overbuilding management workflows. Full administration remains in E6 and E8, but their future APIs must reuse this IAM core.

## 4. Resource and Action Model

Resources are stable backend enum values:

```text
User
Department
Position
Resume
Role
Permission
UserDepartmentRole
DepartmentPosition
DepartmentResume
PositionResume
RoleRelation
Notification
AuditLog
Job
```

Actions are stable backend enum values:

```text
List
Get
Create
Update
Delete
```

Permission names may be displayed as `{Resource}.{Action}`, for example `Resume.List`, but storage remains `(role_id, resource, action, attribute_conditions)`.

Unknown resource/action values are rejected by write APIs and seed validation. Business handlers must not construct permission strings by ad hoc concatenation.

## 5. Data Model

The foundation migration already contains the IAM tables:

- `roles`
- `permissions`
- `role_relations`
- `user_department_roles`
- `departments`
- `users`

E1 already seeds:

- `__system__` department;
- `__role_guest__` role;
- guest permissions for `Department.List` and `User.Get`.

IAM implementation must add idempotent seed data for all preset business roles, default permissions, and default role relations. Seed SQL must work in SQLite and PostgreSQL, use goose migrations, and be safe to run once per environment. Production schema evolution must not rely on GORM AutoMigrate.

## 6. Preset Roles

Preset role IDs are stable constants:

| Role ID | Label | System | Default scope |
| --- | --- | --- | --- |
| `__role_guest__` | 游客 | true | System department only |
| `__role_hrbp__` | HRBP | true | Bound department |
| `__role_hrd__` | HRD | true | Bound department or all departments through `__system__` |
| `__role_manager__` | 主管 | true | Bound department |
| `__role_trainee__` | 锻炼干部 | true | Bound department |
| `__role_social_owner__` | 社招负责人 | true | Channel `social`; department scope depends on binding |
| `__role_campus_owner__` | 校招负责人 | true | Channel `campus`; department scope depends on binding |
| `__role_super_admin__` | 超级管理员 | true | All resources, all departments |

System role labels and `is_system` cannot be changed. System role descriptions, `enabled`, direct permissions, and role relations may be changed later by E8 according to its SPEC. A disabled role is hidden from assignment dropdowns, but existing `UserDepartmentRole` rows continue to work unless the role is reached through `RoleRelation`; runtime expansion skips disabled child roles reached through a relation.

Only these role IDs may use `department_id = "__system__"` as all-department scope:

- `__role_hrd__`
- `__role_social_owner__`
- `__role_campus_owner__`
- `__role_super_admin__`

`__role_guest__` is bound to `__system__` for baseline identity only and does not gain all-department business scope. `__role_hrbp__`, `__role_manager__`, and `__role_trainee__` must use concrete departments. Future UserDepartmentRole write APIs must reject unsupported `__system__` bindings with `IAM_SCOPE_UNSUPPORTED`; runtime resolution must also fail closed if such a row exists due to manual data repair or legacy import.

## 7. Default Role Relations

Default inheritance is:

```text
HRD -> HRBP
HRD -> 主管
HRD -> 锻炼干部
超级管理员 -> HRD
超级管理员 -> 社招负责人
超级管理员 -> 校招负责人
```

Inheritance is directed and recursive. The parent role includes the child role's effective permissions. Runtime expansion must:

- reject direct self-relations;
- detect indirect cycles;
- enforce a maximum depth of 16;
- return a stable error code when a cycle or depth violation is encountered;
- skip disabled child roles reached through a relation.

The implementation must never recurse without cycle tracking.

## 8. Permission Whitelist

IAM owns a centralized permission whitelist. The whitelist is shared by:

- seed generation;
- role-management validation in future E8;
- backend tests;
- OpenAPI schema enum values when exposed.

Initial whitelist:

| Permission | Attribute conditions allowed |
| --- | --- |
| `User.List` | none |
| `User.Get` | `self` |
| `Department.List` | none |
| `Department.Get` | none |
| `Position.List` | none |
| `Position.Get` | none |
| `Position.Create` | none |
| `Position.Update` | none |
| `Position.Delete` | none |
| `Resume.List` | `chan`, `expired` |
| `Resume.Get` | `chan`, `expired` |
| `Resume.Create` | `chan` |
| `Resume.Update` | `chan`, `expired` |
| `Resume.Delete` | `chan`, `expired` |
| `DepartmentPosition.List` | none |
| `DepartmentPosition.Create` | none |
| `DepartmentPosition.Delete` | none |
| `DepartmentResume.List` | none |
| `DepartmentResume.Create` | none |
| `DepartmentResume.Delete` | none |
| `PositionResume.List` | none |
| `PositionResume.Get` | none |
| `PositionResume.Create` | none |
| `PositionResume.Update` | none |
| `Notification.List` | none |
| `Notification.Get` | none |
| `Notification.Create` | none |
| `Notification.Update` | none |
| `Role.List` | none |
| `Role.Get` | none |
| `Role.Create` | none |
| `Role.Update` | none |
| `Role.Delete` | none |
| `Permission.List` | none |
| `Permission.Create` | none |
| `Permission.Delete` | none |
| `RoleRelation.List` | none |
| `RoleRelation.Create` | none |
| `RoleRelation.Delete` | none |
| `UserDepartmentRole.List` | none |
| `UserDepartmentRole.Create` | none |
| `UserDepartmentRole.Delete` | none |
| `AuditLog.List` | none |
| `Job.List` | none |
| `Job.Get` | none |

Any permission outside this whitelist is invalid. Future EPICs may extend the whitelist through their SPECs before implementation.

## 9. Preset Role Permission Matrix

Seeded default permissions are direct permissions before RoleRelation inheritance.

| Role | Direct permissions | Attribute conditions |
| --- | --- | --- |
| 游客 | `Department.List`, `User.Get` | `User.Get` uses `{"self": true}` |
| HRBP | `User.List`, `User.Get`, `Department.List`, `Department.Get`, `UserDepartmentRole.List`, `DepartmentPosition.List`, `Resume.List`, `Resume.Get`, `Resume.Create`, `Resume.Update`, `Resume.Delete`, `Position.List`, `Position.Get`, `DepartmentResume.Create`, `PositionResume.Create`, `Notification.List`, `Notification.Get`, `Notification.Create`, `Notification.Update` | none |
| 主管 | `User.List`, `User.Get`, `Department.List`, `Department.Get`, `UserDepartmentRole.List`, `DepartmentPosition.List`, `Resume.List`, `Resume.Get`, `Resume.Create`, `Position.List`, `Position.Get`, `DepartmentResume.Create`, `PositionResume.Create`, `Notification.Create` | none |
| 锻炼干部 | `User.List`, `User.Get`, `Department.List`, `Department.Get`, `UserDepartmentRole.List`, `DepartmentPosition.List`, `Resume.List`, `Resume.Get`, `Position.List`, `Position.Get`, `PositionResume.Create` | none |
| HRD | `UserDepartmentRole.Create`, `UserDepartmentRole.Delete`, `DepartmentResume.Create` | none; HRD also inherits HRBP, 主管, and 锻炼干部 |
| 社招负责人 | `User.List`, `User.Get`, `Department.List`, `Department.Get`, `UserDepartmentRole.List`, `DepartmentPosition.List`, `Resume.List`, `Resume.Get`, `Resume.Create`, `Resume.Update`, `Resume.Delete`, `Position.List`, `Position.Get`, `DepartmentResume.Create`, `PositionResume.Create`, `Notification.List`, `Notification.Get`, `Notification.Create`, `Notification.Update` | all direct `Resume.*` permissions use `{"chan": ["social"]}` |
| 校招负责人 | same as 社招负责人 | all direct `Resume.*` permissions use `{"chan": ["campus"]}` |
| 超级管理员 | every permission in the whitelist | no attribute conditions; 超级管理员 also inherits HRD, 社招负责人, and 校招负责人 |

Super administrator is the only preset role with department and position write permissions by default. It receives whitelist-wide direct permissions so future removal of an inherited role does not accidentally remove baseline administration. Future E8 role editing may narrow this, but the initial seed must match the PRD default of all resources and all CRUD-like operations that are present in the whitelist.

## 10. Attribute Conditions

`permissions.attribute_conditions` stores a JSON object. The initial allowed keys are:

```json
{
  "chan": ["social", "campus"],
  "expired": [false, true],
  "self": true
}
```

Rules:

- `chan` applies only to `Resume` permissions in this SPEC.
- `expired` applies only to `Resume` permissions in this SPEC.
- `self` applies only to `User.Get`.
- Unknown keys are rejected by write/seed validation.
- Empty `{}` means no attribute narrowing.
- Multiple values inside one key are OR.
- Multiple keys are AND.
- Multiple matching permissions across bindings are OR.

Example: a user with one `Resume.List` permission for `chan=social` and another for `chan=campus` may list both channels. A user with `chan=social` and `expired=false` may only list non-expired social resumes.

## 11. Department Scope

Each `UserDepartmentRole` row is an independent authorization source:

```text
user_id + department_id + role_id
```

`department_id = "__system__"` means all departments only for the global-scope-capable role IDs listed in section 6. For regular concrete departments, access is limited to resources associated with that department.

Department scoping rules:

- `Department`, `DepartmentPosition`, `DepartmentResume`, and `UserDepartmentRole` are scoped directly by `department_id`.
- `Position` is scoped through `department_positions`.
- `Resume` is scoped through `department_resumes`.
- `PositionResume` is allowed only when the user can access both the referenced position and resume for the requested action family.
- `Notification.List/Get/Update` is scoped to `to_user_id = current_user_id`.
- `Notification.Create` is allowed only for services that have already checked the triggering business action and recipient data scope.
- `User.Get` with `self=true` is allowed only for the current user.
- `User.List` and public `User.Get` are allowed by roles that need collaborator display.
- `AuditLog.List`, role administration, and permission administration are system-scope permissions.

The final allowed scope for a user is the OR union of all expanded bindings.

## 12. Runtime IAM API

Backend IAM exposes a narrow service boundary:

```text
ResolvePrincipal(ctx, userID) -> Principal
Can(ctx, principal, resource, action, target?) -> Decision
Scope(ctx, principal, resource, action) -> ScopePredicate
RoleSummary(ctx, userID) -> RoleSummary
InvalidateUser(ctx, userID)
InvalidateAll()
```

`Principal` includes:

- user identity;
- direct role bindings;
- expanded role IDs;
- effective permissions;
- department scope summary;
- channel scope summary;
- permission version metadata for cache invalidation.

`Decision` includes:

- `allowed`;
- stable denial reason code;
- request-safe details;
- matched binding IDs for audit/debug when useful.

`ScopePredicate` is a structured predicate object, not raw SQL. Repositories translate it into GORM query clauses. Handlers and services must not concatenate SQL snippets from IAM output.

## 13. Permission Guard

All authenticated business routes use the existing session guard first, then IAM authorization.

Guard behavior:

1. Load the authenticated session.
2. Resolve the principal through IAM.
3. Check the route's required resource/action.
4. For List routes, attach a `ScopePredicate` to request context for repository use.
5. For Get/Update/Delete routes, either pre-check target identity or require the service to load the target through an authorized query.
6. Return a stable 403 error on denial.

Business handlers must declare required permissions at route registration. They must not perform role-label checks such as `if role == "HRBP"`.

## 14. `/me` Contract Additions

E1 `/me` returns identity, role bindings, page access, and default route. IAM extends it with:

```json
{
  "permissions": ["Resume.List", "Position.List"],
  "dataScope": {
    "departments": [
      {"id": "dept_1", "name": "算力训练平台部"}
    ],
    "allDepartments": false,
    "channels": ["social", "campus"]
  },
  "pageAccess": ["resume-parse", "resume-recommend"]
}
```

The frontend uses this for navigation and disabled states only. API authorization remains backend-controlled.

## 15. Page Access Mapping

IAM owns the mapping from effective permissions to page access:

| Page key | Required permissions |
| --- | --- |
| `resume-parse` | `Resume.List`, `Resume.Get`, `Position.List`, `PositionResume.Create` |
| `resume-recommend` | `Resume.List`, `Resume.Get`, `Resume.Create`, `Notification.Create`, `DepartmentResume.Create`, `PositionResume.Create` |
| `resume-library` | `Resume.List` |
| `departments-positions` | `Department.List`, `Position.List` |
| `users` | `User.List`, `UserDepartmentRole.List` |
| `roles` | `Role.List`, `Permission.List`, `RoleRelation.List` |
| `notifications` | `Notification.List` |
| `audit-logs` | `AuditLog.List` |

Guest users keep page access to `resume-parse` and `resume-recommend` for UX continuity, but resource operations still fail unless the backend grants the required resource permissions. This preserves the E1 decision that guests do not see a separate empty state.

## 16. Error Codes

Add stable IAM error codes:

- `IAM_PERMISSION_DENIED`
- `IAM_PERMISSION_NOT_FOUND`
- `IAM_INVALID_RESOURCE`
- `IAM_INVALID_ACTION`
- `IAM_INVALID_ATTRIBUTE_CONDITION`
- `IAM_ROLE_RELATION_CYCLE`
- `IAM_ROLE_RELATION_DEPTH_EXCEEDED`
- `IAM_PRINCIPAL_NOT_FOUND`
- `IAM_SCOPE_UNSUPPORTED`

Chinese fallback messages live in backend error definitions. Frontend display text maps through i18n.

## 17. Audit Requirements

This SPEC must preserve audit boundaries even before E6/E8 UI exists.

Audit events are required for:

- `UserDepartmentRole.Create`;
- `UserDepartmentRole.Delete`;
- `Permission.Create`;
- `Permission.Delete`;
- `RoleRelation.Create`;
- `RoleRelation.Delete`;
- role `enabled` changes when E8 implements them;
- authorization denial for sensitive writes when the route reaches IAM guard.

Audit details must not include secrets. For permission changes, details may include role ID, resource, action, and attribute condition summary.

## 18. Caching and Invalidation

IAM may start with no cache or an in-memory cache. If caching is used, correctness requirements are:

- cache key includes `userID`;
- cached value includes role/permission version metadata;
- `UserDepartmentRole`, `Permission`, and `RoleRelation` writes invalidate affected users immediately;
- affected users for a `Permission` write include users directly bound to the changed role and users bound to any ancestor role that reaches the changed role through RoleRelation;
- affected users for a `RoleRelation` write include users bound to either endpoint role and users bound to any ancestor role of either endpoint;
- ancestor closure must be computed with the same cycle-safe traversal used by runtime expansion;
- unknown or unsafe affected-user sets fall back to `InvalidateAll`;
- cache misses must be safe and recompute from the database;
- stale cache must never grant more permission after a write returns success.

The first implementation may choose no cache if tests still meet project performance expectations.

## 19. Backend Components

Add or extend backend boundaries:

- `internal/iam`: resource/action enums, whitelist, role expansion, attribute validation, principal resolution, decision service, scope predicates.
- `internal/http/middleware` or `internal/app`: route guard integration using IAM.
- `internal/auth`: `/me` calls IAM for enriched role summary and page access.
- `internal/audit`: write audit records for IAM changes and denials.
- domain repositories: consume `ScopePredicate` once E4/E5/E2 APIs exist.

IAM must not depend on resume, position, or notification services. It may know resource relationships as predicate builders but must not implement business workflows.

## 20. Frontend Behavior

Frontend IAM behavior in this SPEC is limited to shell-level UX:

- consume generated `/me` client types;
- render navigation from `pageAccess`;
- show data-scope summary text from `dataScope`;
- translate IAM error codes through i18n;
- never treat hidden navigation or disabled buttons as security.

Business pages must continue using shadcn/ui or project-wrapped components for interactive controls.

## 21. Testing Requirements

Implementation must follow red-green-refactor TDD.

Backend unit tests:

- preset role expansion includes recursive child permissions;
- RoleRelation cycle detection rejects direct and indirect cycles;
- max-depth protection rejects chains longer than 16;
- disabled child roles are skipped when reached through RoleRelation;
- multiple UserDepartmentRole bindings OR their scopes;
- `__system__` department binding grants all-department scope only for global-scope-capable roles;
- concrete department binding limits scope to that department;
- attribute conditions merge correctly for `Resume.List`;
- unknown resources/actions are rejected;
- invalid attribute condition keys and values are rejected;
- guest has `Department.List` and `User.Get(self)`;
- unsupported `__system__` binding for HRBP, 主管, or 锻炼干部 fails closed with `IAM_SCOPE_UNSUPPORTED`;
- preset role seed matrix matches this SPEC;
- `RoleSummary` returns stable labels and department names.

Backend integration/API tests:

- `/me` includes permissions, data scope, page access, and default route;
- guard returns 401 for unauthenticated access;
- guard returns 403 with `IAM_PERMISSION_DENIED` for authenticated but unauthorized access;
- List guards attach a structured scope predicate;
- UserDepartmentRole mutation invalidates affected principal cache;
- Permission mutation invalidates direct role users and ancestor-role users;
- RoleRelation mutation invalidates endpoint-role users and ancestor-role users;
- audit rows are written for IAM mutations without secret data;
- OpenAPI and generated client stay in sync.

Future domain tests must prove List APIs push IAM predicates into backend queries rather than filtering in the frontend.

## 22. Acceptance Criteria

IAM is complete when:

- preset roles, permissions, and role relations are seeded idempotently;
- permission whitelist and attribute-condition validation are centralized;
- `ResolvePrincipal`, `Can`, `Scope`, and `RoleSummary` are covered by tests;
- RoleRelation recursion is cycle-safe and depth-limited;
- department and channel scopes combine correctly across multiple bindings;
- `/me` exposes effective permissions and data-scope summary through generated OpenAPI/client types;
- authenticated business route guards can enforce resource/action permissions centrally;
- stable IAM error codes and i18n mappings exist;
- IAM mutation hooks invalidate permission cache and write audit logs;
- `docs/project-status.md` and `AGENTS.md` point to this SPEC as the active implementation source;
- all behavior is covered by failing-first tests before implementation.
