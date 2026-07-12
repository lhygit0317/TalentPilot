# 008 Custom Role Management SPEC

## 1. Scope and Goals

This SPEC defines E8: custom role management. It covers the "角色管理" page and backend role-definition APIs for listing roles, viewing role details, creating custom roles, editing system and custom role definitions, deleting unused custom roles, and enabling or disabling roles.

E8 manages role definitions: `roles`, direct `permissions`, and direct `role_relations`. E6 remains responsible for assigning existing roles to users through `user_department_roles`.

Backend authorization remains authoritative. Frontend navigation, disabled buttons, and form validation are workflow aids only.

## 2. Dependencies and Readiness

Required before E8 implementation:

- E1 auth/session behavior from [001 Auth Session W3 SPEC](001-auth-session-w3.md), especially `/me` and unsafe-method CSRF protection.
- IAM runtime behavior from [002 IAM Permission Model SPEC](002-iam-permission-model.md), especially permission whitelist validation, `RoleRelation` cycle/depth validation, direct permission replacement, ancestor closure cache invalidation, and audit boundaries.
- E6 user-role binding behavior from [007 User and Role Binding Management SPEC](007-user-role-management.md), especially assignable role dropdown behavior and `enabled=true` filtering.
- Existing tables: `roles`, `permissions`, `role_relations`, and `user_department_roles`.

No schema change is expected for E8. The existing foreign keys and unique constraints remain the source of persistence integrity.

## 3. Non-Goals

- User-role assignment. E6 continues to own `UserDepartmentRole` creation and deletion.
- Permission templates or preset wizard flows. E8 first release exposes the complete product-approved IAM whitelist matrix.
- Audit-log browsing UI.
- Bulk role import/export.
- Playwright end-to-end coverage. `make test-e2e` remains reserved.
- Frontend-composed writes to low-level Permission or RoleRelation CRUD APIs.
- Editing `is_system` or creating new system roles.

## 4. Design Approach

Three implementation scopes were considered:

1. Aggregated RoleAdmin API.
2. Fine-grained Role, Permission, and RoleRelation CRUD APIs composed by the frontend.
3. Split management APIs for role metadata, permission sync, and role-relation sync.

This SPEC chooses option 1. E8 adds a focused backend `roleadmin` domain that accepts one role-definition payload and applies metadata, direct permissions, and direct child-role relations in a single transaction. This keeps consistency, validation, audit, and IAM cache invalidation in the backend security boundary.

## 5. Domain Model

E8 introduces `apps/api/internal/roleadmin`, responsible for:

- listing role definitions with counts and UI capability flags;
- loading one role's editable definition;
- exposing permission options derived from `iam.PermissionWhitelist()`;
- creating custom roles;
- updating role metadata, direct permissions, and direct child roles atomically;
- toggling `roles.enabled`;
- deleting unused custom roles;
- enforcing system role protection, reference-count checks, permission whitelist validation, and role-relation graph validation;
- invalidating IAM cache for the changed role and its ancestor closure;
- recording audit events for successful mutations.

Role definition terms:

- Direct permissions are rows in `permissions` where `role_id = role.id`.
- Direct child roles are rows in `role_relations` where `parent_role_id = role.id`.
- Effective permissions are produced by IAM runtime expansion and are not persisted by E8.
- `referenceCount` is the count of `user_department_roles` rows that reference the role directly.

The frontend must display direct permissions and direct child-role selections. It may show counts and summaries, but it must not calculate effective authorization.

## 6. Permissions and Page Access

Routes must use session authentication first, then IAM guards:

| Capability | Required permissions |
| --- | --- |
| Show "角色管理" navigation | `/me.pageAccess` contains `roles`, derived from `Role.List` |
| List roles | `Role.List` |
| View one role | `Role.Get` |
| Load permission options | `Permission.List` |
| Create role | `Role.Create` + `Permission.Create` + `RoleRelation.Create` |
| Edit metadata only | `Role.Update` |
| Replace direct permissions | `Role.Update` + `Permission.Delete` + `Permission.Create` |
| Replace direct child roles | `Role.Update` + `RoleRelation.Delete` + `RoleRelation.Create` |
| Toggle enabled | `Role.Update` |
| Delete role | `Role.Delete` |

The super administrator has these permissions through the existing IAM seed. Future custom roles may also be granted these permissions, but the frontend entry copy may still describe the page as administrator-focused.

Every mutation must re-check the target role server-side. Frontend capability flags are not trusted.

## 7. API Contract

Backend route definitions remain the OpenAPI source of truth. DTOs must not expose GORM persistence models directly.

### `GET /roles`

Query:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `search` | string | no | Matches role label or description. |
| `system` | boolean | no | `true` returns system roles, `false` returns custom roles. |
| `enabled` | boolean | no | Filters by enabled state. |
| `limit` | integer | no | Default 100, maximum 200. |

Response:

```json
{
  "items": [
    {
      "id": "__role_hrbp__",
      "label": "HRBP",
      "description": "HRBP",
      "isSystem": true,
      "enabled": true,
      "permissionCount": 19,
      "childRoleCount": 0,
      "referenceCount": 6,
      "conditionSummary": "全部渠道",
      "canEdit": true,
      "canDelete": false,
      "canToggleEnabled": true
    }
  ],
  "total": 8,
  "canCreate": true
}
```

Behavior:

1. Validate auth and `Role.List`.
2. Apply query filters in SQL.
3. Count direct permissions, direct child roles, and direct user-role references.
4. Sort by `is_system DESC`, then `label ASC`.
5. Return capability flags based on role protection rules and caller permissions.

### `GET /roles/{roleId}`

Response:

```json
{
  "id": "role_custom_interviewer",
  "label": "高级评审者",
  "description": "可查看并推荐指定渠道简历",
  "isSystem": false,
  "enabled": true,
  "referenceCount": 0,
  "permissions": [
    {
      "resource": "Resume",
      "action": "List",
      "attributeConditions": { "chan": ["social"], "expired": [false] }
    }
  ],
  "childRoleIds": ["__role_trainee__"],
  "canEdit": true,
  "canDelete": true,
  "canToggleEnabled": true
}
```

Behavior:

1. Validate auth and `Role.Get`.
2. Return 404 when the role does not exist.
3. Return direct permissions only.
4. Return direct child role IDs only.

### `GET /roles/permission-options`

Response:

```json
{
  "resources": [
    {
      "resource": "Resume",
      "actions": [
        {
          "action": "List",
          "supportsConditions": {
            "chan": true,
            "expired": true,
            "self": false
          }
        }
      ]
    }
  ],
  "conditionOptions": {
    "chan": ["social", "campus"],
    "expired": [false, true]
  }
}
```

Behavior:

1. Validate auth and `Permission.List`.
2. Derive options from `iam.PermissionWhitelist()`.
3. Return resources and actions in stable enum order.

### `POST /roles`

Request:

```json
{
  "label": "高级评审者",
  "description": "可查看并推荐指定渠道简历",
  "enabled": true,
  "permissions": [
    {
      "resource": "Resume",
      "action": "List",
      "attributeConditions": { "chan": ["social"], "expired": [false] }
    }
  ],
  "childRoleIds": ["__role_trainee__"]
}
```

Response: same shape as `GET /roles/{roleId}`.

Behavior:

1. Validate auth and `Role.Create`, `Permission.Create`, `RoleRelation.Create`.
2. Validate `label`, uniqueness, permissions, child roles, and relation graph.
3. Create `roles` row with `is_system=false`.
4. Insert direct permissions and direct child relations.
5. Invalidate IAM role closure for the new role.
6. Record role-created audit details.

### `PATCH /roles/{roleId}`

Request shape matches `POST /roles`. System roles ignore request `label` and must keep their existing label.

Behavior:

1. Validate auth and `Role.Update`.
2. Require `Permission.Delete` + `Permission.Create` when direct permissions are changed.
3. Require `RoleRelation.Delete` + `RoleRelation.Create` when direct child roles are changed.
4. Validate system role protection, label uniqueness, permissions, child roles, and relation graph.
5. In one transaction, update role metadata and replace direct permissions and direct child relations.
6. Invalidate IAM role closure for the edited role.
7. Record role-updated audit details with before/after summary.

### `PATCH /roles/{roleId}/enabled`

Request:

```json
{ "enabled": false }
```

Behavior:

1. Validate auth and `Role.Update`.
2. Update `roles.enabled`.
3. Invalidate IAM role closure for the toggled role.
4. Existing direct `UserDepartmentRole` rows remain.
5. E6 assignable role list stops returning disabled roles.

### `DELETE /roles/{roleId}`

Behavior:

1. Validate auth and `Role.Delete`.
2. Reject system roles.
3. Reject roles with `referenceCount > 0`.
4. Delete the role. Database cascades direct permissions and role relations.
5. Invalidate IAM role closure for the deleted role and affected ancestors.
6. Record role-deleted audit details with role ID and label snapshot.

## 8. Validation Rules

Role metadata:

- `label` is required for custom roles, trimmed, length 2-20, and globally unique.
- System role labels cannot be changed.
- `description` is trimmed and limited to 200 characters.
- `isSystem` cannot be set through any API.
- `enabled` defaults to `true` on create.

Permission rules:

- Every submitted permission must pass `iam.ValidatePermissionGrant`.
- Duplicate submitted permissions are rejected after canonicalizing `resource`, `action`, and `attributeConditions`.
- E8 stores direct permissions only.
- `User.Get` with `self` is displayed as a fixed supported condition; the backend still validates it through IAM.

RoleRelation rules:

- Child role IDs must exist.
- A role cannot include itself.
- Saving the requested direct child roles must not create a direct or indirect cycle.
- Maximum inheritance depth remains 16.
- Disabled child roles may be saved; IAM runtime skips disabled child roles when expanding inheritance.

Deletion and enabled-state rules:

- System roles cannot be deleted.
- Custom roles can be deleted only when direct `UserDepartmentRole` reference count is zero.
- Disabling a role does not delete bindings and does not revoke direct bindings by itself.
- Disabling a child role prevents parent roles from inheriting that child at runtime.

## 9. Error Codes

E8 adds or reuses stable API error codes:

| Code | HTTP | Default Chinese message |
| --- | --- | --- |
| `ROLE_NOT_FOUND` | 404 | 角色不存在 |
| `ROLE_LABEL_INVALID` | 422 | 角色名称需为 2-20 个字符 |
| `ROLE_LABEL_DUPLICATE` | 409 | 角色名称已存在 |
| `ROLE_SYSTEM_PROTECTED` | 422 | 系统预置角色不允许执行该操作 |
| `ROLE_IN_USE` | 422 | 该角色仍被用户角色绑定引用 |
| `ROLE_PERMISSION_INVALID` | 422 | 角色权限不在允许范围内 |
| `ROLE_PERMISSION_DUPLICATE` | 422 | 角色权限重复 |
| `ROLE_RELATION_INVALID` | 422 | 角色包含关系无效 |
| `IAM_ROLE_RELATION_CYCLE` | 422 | 角色包含关系不能形成循环 |
| `IAM_ROLE_RELATION_DEPTH_EXCEEDED` | 422 | 角色包含层级过深 |

`PERMISSION_DENIED`, `UNAUTHENTICATED`, and CSRF errors continue to use existing shared errors.

## 10. Frontend Behavior

Add `apps/web/src/roles/RoleManagementPage.tsx` and connect the existing `roles` shell navigation.

Page structure:

- Header with title, search input, system/custom filter, enabled/disabled filter, and `新建角色` action.
- Role table columns: label, system badge, enabled state, direct permission count, child role count, condition summary, reference count, and actions.
- System roles show edit and enable/disable actions. Delete is unavailable.
- Custom roles show edit, enable/disable, and delete. Delete is disabled when `referenceCount > 0`.

Editor dialog:

- Basic fields: label, description, enabled.
- Permission matrix grouped by resource. Only whitelist actions are visible.
- Resume permissions show `社招/校招` and `未过期/已过期` condition controls when supported.
- Child-role multi-select lists existing roles and excludes the current role when editing.
- Save submits one aggregated role-definition payload.

Interaction rules:

- Frontend performs form pre-validation only.
- Backend errors are translated through i18n.
- Successful save/toggle/delete refreshes role list.
- Direct hash navigation to `#roles` without page access returns to the session default page.

## 11. Audit and Cache Invalidation

Every successful mutation records an audit event:

- role created;
- role updated;
- role enabled/disabled;
- role deleted.

Audit details include actor user ID, target role ID, role label, and counts of direct permissions and child roles before and after the mutation.

After role mutation, IAM cache invalidation must include:

- users directly bound to the changed role;
- users bound to ancestor roles that inherit from the changed role;
- fallback full cache clear if ancestor closure cannot be calculated.

## 12. Testing Requirements

Backend service/store tests:

- list roles returns permission, child-role, and reference counts;
- create role writes role, direct permissions, and direct child relations atomically;
- duplicate labels are rejected;
- system role delete and system label edits are rejected;
- delete rejects referenced custom roles;
- duplicate permissions are rejected;
- invalid attribute conditions are rejected;
- role-relation cycles and excessive depth are rejected;
- edit replaces direct permissions and direct child roles atomically;
- delete custom role cascades direct permissions and relations;
- enabled toggle updates E6 assignable-role behavior;
- cache invalidation covers direct and ancestor-bound users.

Backend route tests:

- list/detail/options require their read permissions;
- create/edit/delete/toggle require their mutation permissions;
- stable errors map to the expected problem codes.

Frontend tests:

- role list renders counts, status, badges, filters, and search;
- create/edit dialog builds the permission matrix payload;
- referenced custom role delete is disabled with a reason;
- disabling a system role shows confirmation copy;
- backend error codes render localized messages.

Verification:

- `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make test-api`
- `CI=true make test-web`
- `make openapi-check`
- `make client-check`
- `make typecheck`
- `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make lint`
- `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH make build`
- final `PATH=/Users/lhy/.cache/codex-go/go1.26.4-darwin-arm64/go/bin:$PATH CI=true make ci`
