# 007 User and Role Binding Management SPEC

## 1. Scope and Goals

This SPEC defines E6: user management and user-role binding management. It covers the "用户管理" page, permission-filtered user listing, display of current `UserDepartmentRole` bindings, assignment of one or more role bindings to a user, removal of role bindings, duplicate prevention, guest fallback, cache invalidation, and audit boundaries.

E6 manages **who has which existing role in which department**. It does not manage role definitions. Role, Permission, and RoleRelation editing remains in E8.

Backend authorization remains authoritative. Frontend `pageAccess`, hidden buttons, and read-only UI states are workflow aids only.

## 2. Dependencies and Readiness

Required before E6 implementation:

- E1 auth/session behavior from [001 Auth Session W3 SPEC](001-auth-session-w3.md), especially W3-created users and guest binding creation.
- IAM runtime behavior from [002 IAM Permission Model SPEC](002-iam-permission-model.md), especially `User.List`, `User.Get`, `UserDepartmentRole.List`, `UserDepartmentRole.Create`, `UserDepartmentRole.Delete`, `Department.List`, role expansion, data-scope predicates, `RoleSupportsGlobalScope`, and cache invalidation.
- E5 department behavior from [004 Department and Position Management SPEC](004-position-department-management.md), especially permission-filtered `GET /departments`.
- Existing IAM tables: `users`, `roles`, `departments`, `permissions`, `role_relations`, and `user_department_roles`.

No schema change is expected for E6. The existing unique key `(user_id, department_id, role_id)` on `user_department_roles` is required.

## 3. Non-Goals

- Creating, editing, deleting, enabling, or disabling roles.
- Editing role permissions or role inheritance.
- Adding RoleRelation management UI.
- Manually creating users or syncing users from W3. Users enter the system only through W3 login.
- Editing user profile fields. W3 login remains the only writer for `id`, `name`, and `employeeId`.
- Audit-log browsing. E6 writes audit events but does not implement the audit UI.
- Notification center behavior.

## 4. Design Approach

Three implementation scopes were considered:

1. User-role binding management only, with role definitions read-only for assignment.
2. Combine user-role binding management with full custom role definition management.
3. Backend user-role APIs only and defer the "用户管理" page.

This SPEC chooses option 1. It delivers PRD Story 6.1-6.4 without mixing in E8. Existing IAM remains the single source of truth for role definitions, permission expansion, and cache invalidation.

## 5. Domain Model

E6 introduces a focused backend user administration boundary, recommended as `apps/api/internal/useradmin`, responsible for:

- listing users visible under the caller's `User.List` and `UserDepartmentRole.List` permissions;
- projecting visible `UserDepartmentRole` rows with role labels and department names;
- listing enabled roles that can be assigned through the user-role workflow;
- validating and creating one or more role bindings atomically;
- deleting non-guest role bindings;
- ensuring guest fallback for abnormal users with no bindings;
- invalidating the target user's IAM cache after successful mutation;
- recording audit events for each `UserDepartmentRole` create/delete.

The service must reuse IAM validators where possible:

- resource/action permission checks;
- `RoleSupportsGlobalScope`;
- `__system__` department rules;
- role expansion and scope predicates;
- cache invalidation semantics.

The service must not duplicate or bypass IAM permission expansion logic.

## 6. Permissions and Data Scope

Routes must use session authentication first, then IAM guards:

| Capability | Required permissions |
| --- | --- |
| Use user management page | `/me.pageAccess` includes `users`, currently derived from `User.List` + `UserDepartmentRole.List` |
| List users and visible bindings | `User.List` + `UserDepartmentRole.List` |
| View one user's binding detail | `User.Get` + `UserDepartmentRole.List` |
| Load assignable role options | `UserDepartmentRole.Create` |
| Load department options | existing `GET /departments` with `Department.List` |
| Assign one or more bindings | `UserDepartmentRole.Create` |
| Remove a binding | `UserDepartmentRole.Delete` |

List APIs must apply permission predicates in backend queries:

- `UserDepartmentRole.List` scope determines which bindings are visible.
- User rows are returned when they have at least one visible binding, or when the caller can see the user through `User.Get(self)` for their own row.
- System-scope callers can see system department guest bindings and global role bindings.
- Concrete department callers see bindings in their allowed departments.
- The frontend must not fetch all users and filter them locally.

Assignment and deletion must re-check target state server-side:

- target user exists;
- target role exists;
- new target role is `enabled=true`;
- target department exists;
- target department is allowed by caller's `UserDepartmentRole.Create` or `UserDepartmentRole.Delete` scope;
- `departmentId="__system__"` is allowed only for `__role_guest__` and role IDs supported by IAM `RoleSupportsGlobalScope`;
- duplicate `(userId, departmentId, roleId)` rows are rejected before insert and by the database unique key.

## 7. Binding Rules

Guest behavior:

- A W3-created user always receives a guest `UserDepartmentRole` in `__system__`.
- Guest bindings are retained permanently for audit traceability.
- E6 must not allow deleting a guest binding through the UI or API.
- If legacy or manually repaired data leaves a user with no bindings after a mutation, the backend must recreate a guest binding for that user before returning success.

Assignment behavior:

- A single assignment and a batch assignment use the same API with a `bindings` array.
- The entire batch is atomic: either all requested bindings are created or none are.
- Duplicate entries inside the request are rejected.
- Duplicate entries already present in storage are rejected.
- Assigning the same role to the same user in a different department is allowed.
- Assigning a different role to the same user in the same department is allowed.
- Assigning disabled roles is rejected.
- Assigning a non-global role to `__system__` is rejected.
- Assigning a global-capable role to a concrete department is allowed when IAM permits it; its scope is then limited to that concrete department.

Deletion behavior:

- Deleting non-guest bindings is allowed when the caller has `UserDepartmentRole.Delete` for the binding's department scope.
- Deleting a guest binding is rejected.
- Deleting the current actor's last non-guest binding is rejected to avoid self-lockout.
- When a target user is left with only the guest binding, the user falls back to guest permissions on the next IAM resolution.

Role options:

- E6 role assignment lists only `enabled=true` roles.
- Options are grouped as system roles and custom roles using `roles.is_system`.
- Each role option includes a concise Resume attribute condition summary derived from its direct permissions, for example `社招` or `校招`; it does not expose editable Permission rows.
- Existing disabled bindings can still appear in user binding lists, marked disabled, because disabling a role does not revoke historical bindings.

## 8. API Contract

Backend route definitions remain the OpenAPI source of truth. DTOs must not expose GORM persistence models directly.

### `GET /users`

Query:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `search` | string | no | Matches user name, employee ID, department name, or role label. |
| `limit` | integer | no | Default 50, maximum 100. |
| `cursor` | string | no | Opaque cursor for pagination. |

Response:

```json
{
  "items": [
    {
      "id": "w3_zhangmin",
      "employeeId": "A10001",
      "name": "张敏",
      "departments": [
        { "id": "dept_a", "name": "算力训练平台部" },
        { "id": "dept_b", "name": "智算调度部" }
      ],
      "roleBindings": [
        {
          "id": "udr_1",
          "role": { "id": "__role_hrbp__", "label": "HRBP", "isSystem": true, "enabled": true },
          "department": { "id": "dept_a", "name": "算力训练平台部", "system": false },
          "guest": false,
          "createdAt": "2026-07-12T08:00:00Z",
          "createdBy": "admin_1",
          "canDelete": true
        }
      ],
      "roleSummary": "HRBP(部门:算力训练平台部、智算调度部)",
      "guestOnly": false,
      "canAssign": true
    }
  ],
  "nextCursor": "",
  "dataScopeSummary": "负责部门:算力训练平台部、智算调度部",
  "canAssignRoles": true
}
```

Behavior:

1. Validate auth and `User.List` + `UserDepartmentRole.List`.
2. Apply `UserDepartmentRole.List` scope in SQL.
3. Join roles and departments for display labels.
4. Return paginated user rows sorted by name, then employee ID.
5. Apply search in SQL using escaped LIKE predicates.

### `GET /users/{userId}`

Response shape matches one `items[]` entry from `GET /users` and must return the freshest visible bindings for the modal.

Behavior:

1. Validate auth and `User.Get` + `UserDepartmentRole.List`.
2. Return 404 when the user does not exist.
3. Return 403 when the user exists but none of the caller's scopes can see the user or any visible binding.

### `GET /roles/assignable`

Response:

```json
{
  "items": [
    {
      "id": "__role_hrd__",
      "label": "HRD",
      "description": "HRD，可继承 HRBP、主管与锻炼干部权限",
      "isSystem": true,
      "supportsSystemDepartment": true,
      "attributeConditionSummary": ""
    },
    {
      "id": "__role_social_owner__",
      "label": "社招负责人",
      "description": "社招负责人，默认仅访问社招简历",
      "isSystem": true,
      "supportsSystemDepartment": true,
      "attributeConditionSummary": "社招"
    }
  ]
}
```

Behavior:

1. Validate auth and `UserDepartmentRole.Create`.
2. Return only roles with `enabled=true`.
3. Sort by system roles first, then label.
4. Include minimal display metadata only. This endpoint is not a role-definition editor and must not return editable Permission or RoleRelation payloads.

### `POST /users/{userId}/role-bindings`

Request:

```json
{
  "bindings": [
    { "departmentId": "dept_a", "roleId": "__role_hrbp__" },
    { "departmentId": "dept_b", "roleId": "__role_manager__" }
  ]
}
```

Response:

```json
{
  "user": {
    "id": "w3_zhangmin",
    "employeeId": "A10001",
    "name": "张敏"
  },
  "created": [
    {
      "id": "udr_2",
      "role": { "id": "__role_manager__", "label": "主管", "isSystem": true, "enabled": true },
      "department": { "id": "dept_b", "name": "智算调度部", "system": false },
      "guest": false,
      "createdAt": "2026-07-12T08:10:00Z",
      "createdBy": "admin_1"
    }
  ],
  "message": "已为 张敏 分配 2 个角色绑定"
}
```

Behavior:

1. Validate auth and `UserDepartmentRole.Create`.
2. Validate request contains 1-20 bindings.
3. Validate target user, roles, and departments exist.
4. Validate all roles are enabled.
5. Validate no duplicate binding appears inside the request or storage.
6. Validate each department is inside the caller's create scope.
7. Validate `__system__` department compatibility with the role.
8. Insert all bindings in one transaction.
9. Invalidate target user's IAM cache after commit.
10. Record one audit event per created binding.
11. Return the created binding projections.

### `DELETE /users/{userId}/role-bindings/{bindingId}`

Response:

```json
{
  "deletedBindingId": "udr_2",
  "userId": "w3_zhangmin",
  "message": "已解除 主管(部门:智算调度部)"
}
```

Behavior:

1. Validate auth and `UserDepartmentRole.Delete`.
2. Load the binding and verify it belongs to `userId`.
3. Reject guest binding deletion.
4. Reject deletion if the target binding is outside the caller's delete scope.
5. Reject deletion of the current actor's last non-guest binding.
6. Delete the binding in a transaction.
7. Recreate a guest binding if abnormal data leaves the user with no bindings.
8. Invalidate target user's IAM cache after commit.
9. Record an audit event.

## 9. Frontend Behavior

The `apps/web/src/users` page must use generated-client wrappers only.

Required page behavior:

- Render under active page `users`.
- Show title `用户管理`.
- Show data-scope banner using the API `dataScopeSummary`.
- Load `GET /users` on entry.
- Support search by name, employee ID, department, or role; visible matches should use safe literal highlighting.
- Render columns: 姓名, 工号, 当前角色集合, 所属部门, 操作.
- Show guest-only users with a `游客` chip.
- If the session lacks `UserDepartmentRole.Create`, show a read-only banner and no assignment button.
- If the session has `UserDepartmentRole.Create`, show `分配角色`.
- The role binding modal shows user identity, assignable roles, department options, existing bindings, and a pending-add list.
- Existing guest binding rows are shown as protected and cannot be removed.
- Existing disabled role bindings are shown with a disabled marker.
- Batch save calls `POST /users/{userId}/role-bindings`.
- Remove calls `DELETE /users/{userId}/role-bindings/{bindingId}` after confirmation.
- On success, reload the user row/detail and show the returned message.

Interactive controls must use shadcn/ui or project-wrapped components, not raw business-page HTML controls.

## 10. Error Codes and Messages

Backend errors must use stable codes and default Chinese messages.

| Code | Default message |
| --- | --- |
| `USER_NOT_FOUND` | 用户不存在 |
| `USER_ROLE_BINDING_NOT_FOUND` | 角色绑定不存在 |
| `USER_ROLE_BINDING_DUPLICATE` | 该用户已存在相同角色绑定 |
| `USER_ROLE_BINDING_BATCH_EMPTY` | 请至少添加一条角色绑定 |
| `USER_ROLE_BINDING_BATCH_TOO_LARGE` | 一次最多添加 20 条角色绑定 |
| `USER_ROLE_BINDING_GUEST_PROTECTED` | 游客身份不可解除 |
| `USER_ROLE_BINDING_SELF_LOCKOUT` | 不能解除自己的最后一个业务角色 |
| `USER_ROLE_BINDING_ROLE_DISABLED` | 该角色已禁用，不能分配 |
| `IAM_SCOPE_UNSUPPORTED` | 该角色不能绑定到系统部门 |
| `IAM_PERMISSION_DENIED` | 没有权限执行该操作 |

Frontend must translate these through i18n and keep backend messages as fallback only.

## 11. Audit and Cache Invalidation

E6 mutations must create audit records for:

- `UserDepartmentRole.Create`;
- `UserDepartmentRole.Delete`;
- abnormal guest fallback creation after deletion, if it occurs.

Audit details must include:

- target `userId`;
- target `departmentId`;
- target `roleId`;
- actor user ID;
- binding ID;
- result.

After successful mutation, IAM cache for the target user must be invalidated. If the actor mutates their own bindings, the actor's active session may continue until the next request, but the next authorization decision must use the updated permissions. The frontend should reload `/me` or force a workspace refresh after self-mutation.

## 12. Testing Requirements

Backend TDD must cover:

- `GET /users` requires `User.List` and `UserDepartmentRole.List`;
- list query applies `UserDepartmentRole.List` department scope in SQL;
- `GET /roles/assignable` requires `UserDepartmentRole.Create` and excludes disabled roles;
- batch create is atomic;
- duplicate binding in request and storage is rejected;
- assigning non-global role to `__system__` is rejected;
- disabled role assignment is rejected;
- delete rejects guest binding;
- delete rejects actor self-lockout;
- delete leaves target user as guest-only when all business bindings are removed;
- mutation invalidates IAM cache and writes audit.

Frontend TDD must cover:

- users page renders list columns and binding chips;
- search filters/highlights name, employee ID, department, and role;
- read-only users see no assignment button;
- assignment modal adds multiple pending bindings and saves once;
- duplicate pending binding is blocked before submit;
- guest binding remove action is not available;
- successful delete refreshes the row and shows toast.

Contract verification must include:

- `make openapi-check`;
- `make client-check`;
- generated client wrappers for all E6 endpoints.

## 13. Status and Follow-Up

E6 is complete only when:

- backend user-role APIs are implemented and authorization is enforced server-side;
- frontend `用户管理` page is wired through `App.tsx`;
- generated OpenAPI and TypeScript client artifacts are updated;
- project status is updated with passing verification evidence.

E8 should later implement role definition management using the same IAM tables and whitelist. E6 must not add temporary role-editing shortcuts that E8 would need to undo.
