# 009 Notification Center SPEC

## 1. Scope and Goals

This SPEC defines E7: notification and reminder consumption. It covers the top notification bell, unread count, unread recommendation dropdown, mark-all-read, single notification read acknowledgement, and click-through navigation into the resume library.

E7 consumes notification rows created by E3 recommendation sending. It does not create business notifications directly. Backend authorization remains authoritative; frontend badges and hidden controls only improve workflow clarity.

## 2. Dependencies and Readiness

Required before E7 implementation:

- E1 auth/session behavior from [001 Auth Session W3 SPEC](001-auth-session-w3.md), especially `/me` and authenticated unsafe-method CSRF protection.
- IAM runtime behavior from [002 IAM Permission Model SPEC](002-iam-permission-model.md), especially `Notification.List`, `Notification.Get`, `Notification.Update`, `Resume.List`, page access, and `to_user_id = current_user_id` notification scoping.
- E3 recommendation behavior from [006 Resume Recommendation and Notification SPEC](006-resume-recommendation-notification.md), especially creation of recommendation notification rows after successful send.
- E4 resume library behavior from [003 Resume Library and Import SPEC](003-resume-library-import.md), especially channel switching, search, and permission-filtered `GET /resumes`.
- Existing `notifications`, `users`, `departments`, `positions`, and `resumes` tables.

The existing `notifications` schema is sufficient. E7 needs a permission seed migration because E3 can create notifications for business roles that can see recommended resumes but do not currently all have `Notification.List/Get/Update`.

## 3. Non-Goals

- Creating new notification types.
- Editing or deleting notifications.
- Listing read notification history.
- A full standalone notification center page.
- WebSocket, server-sent events, background polling, browser notifications, or email delivery.
- Notification deduplication. E3 already records one row per recipient per send event.
- Changing E3 recommendation transaction behavior.
- Showing unread notification data to users without backend `Notification.List`.

## 4. Design Approach

Three implementation scopes were considered:

1. Lightweight notification center: backend read/update APIs plus a top bell dropdown and resume-library jump context.
2. Badge and dropdown only, without resume-library banner integration.
3. Full notification center page with read history and notification management.

This SPEC chooses option 1. It fully covers PRD Story 7.1-7.4 while keeping E7 focused on consuming E3-created rows. Read history and real-time delivery can be added later without changing the E7 API shape.

## 5. Domain Model

E7 introduces a focused backend boundary, recommended as `apps/api/internal/notification`, responsible for:

- counting unread notifications for the current user;
- listing unread recommendation notifications for the current user;
- enriching notification rows with department, position, and recommender display labels;
- marking all of the current user's unread notifications as read;
- marking one current-user notification as read and returning the jump payload;
- preserving notification snapshot fields when optional related records are missing.

No schema change is required for notification rows.

Notification list rows are read models over the existing table:

- `id`: notification ID;
- `to_user_id`: must equal the current session user for every read or update;
- `resume_id`: final target-department resume ID created or reused by E3;
- `department_id`: target department ID;
- `position_id`: optional target position ID;
- `name`: candidate display-name snapshot;
- `by_user_id`: recommender user ID;
- `chan`: `social` or `campus`;
- `time`: notification creation time;
- `read`: current read state.

Enrichment rules:

- department name comes from `departments.name`;
- position name comes from `positions.name` when the row still exists;
- recommender name comes from `users.name` when available, otherwise fall back to `by_user_id`;
- candidate display uses `notifications.name`, not live resume name, so historical notifications remain stable.

Read-state updates are personal acknowledgement updates. They do not require audit-log rows unless a future audit SPEC expands audit coverage to low-risk personal state.

## 6. Permissions and Data Scope

Routes must use session authentication first, then IAM guards:

| Capability | Required permissions |
| --- | --- |
| Show notification bell and unread count | `Notification.List` |
| List unread notifications | `Notification.List` |
| Mark all read | `Notification.Update` |
| Mark one notification read | `Notification.Get` + `Notification.Update` |
| Click through to resume library | frontend requires `resume-library` page access, derived from `Resume.List` |

Every backend query and mutation must enforce `notifications.to_user_id = session.user.id`.

Authorization rules:

- `GET /notifications/summary` returns only the current user's unread count.
- `GET /notifications` returns only the current user's rows and defaults to unread rows.
- `POST /notifications/read-all` updates only current-user unread rows.
- `POST /notifications/{notificationId}/read` updates only when the row belongs to the current user.
- A notification owned by another user must not be distinguishable from a missing notification.
- Frontend state must not be trusted for unread count, ownership, or read status.

Permission seed adjustment:

- Grant `Notification.List`, `Notification.Get`, and `Notification.Update` to preset business roles that can receive E3 recommendation notifications.
- At minimum this adds those permissions to `__role_manager__` and `__role_trainee__`; HRBP, social owner, campus owner, and super admin already have them, while HRD inherits them through role relations.
- Guest users do not need notification read permissions unless a future product rule allows guest-visible notifications.

## 7. API Contract

Backend route definitions remain the OpenAPI source of truth. DTOs must not expose GORM persistence models directly.

### `GET /notifications/summary`

Response:

```json
{
  "unreadCount": 3
}
```

Behavior:

1. Validate auth and `Notification.List`.
2. Count `notifications` where `to_user_id = session.user.id` and `read = false`.
3. Return zero when the user has no unread notifications.

### `GET /notifications`

Query:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `limit` | integer | no | Default 20, maximum 50. |
| `cursor` | string | no | Opaque cursor for future pagination. Initial implementation may return an empty cursor when rows fit in one page. |

Response:

```json
{
  "items": [
    {
      "id": "notification_1",
      "resumeId": "resume_copy_1",
      "candidateName": "张三",
      "department": { "id": "dept_1", "name": "算力训练平台部" },
      "position": { "id": "position_1", "name": "平台工程师" },
      "recommender": { "id": "user_1", "name": "李四" },
      "chan": "social",
      "createdAt": "2026-07-12T08:00:00Z",
      "read": false,
      "canOpenResumeLibrary": true
    }
  ],
  "unreadCount": 3,
  "nextCursor": ""
}
```

Behavior:

1. Validate auth and `Notification.List`.
2. Filter by current user.
3. Apply `read=false`. E7 does not expose read notification history.
4. Sort by `time DESC`, then `id DESC`.
5. Enrich display labels with safe fallbacks.
6. Set `canOpenResumeLibrary` from the current session's effective page access or permission summary, not from client input.

### `POST /notifications/read-all`

Response:

```json
{
  "updatedCount": 3,
  "unreadCount": 0
}
```

Behavior:

1. Validate auth, CSRF, and `Notification.Update`.
2. Update current-user unread rows to `read=true`.
3. Return the number of rows changed and the fresh unread count.
4. Treat an already-empty inbox as success with `updatedCount=0`.

### `POST /notifications/{notificationId}/read`

Response:

```json
{
  "notification": {
    "id": "notification_1",
    "resumeId": "resume_copy_1",
    "candidateName": "张三",
    "department": { "id": "dept_1", "name": "算力训练平台部" },
    "position": { "id": "position_1", "name": "平台工程师" },
    "recommender": { "id": "user_1", "name": "李四" },
    "chan": "social",
    "createdAt": "2026-07-12T08:00:00Z",
    "read": true,
    "canOpenResumeLibrary": true
  },
  "unreadCount": 2
}
```

Behavior:

1. Validate auth, CSRF, `Notification.Get`, and `Notification.Update`.
2. Load the notification only when `to_user_id = session.user.id`.
3. Return a not-found-shaped error when the row is missing or belongs to another user.
4. Set `read=true` idempotently.
5. Return the enriched notification row and fresh unread count for frontend state reconciliation.

## 8. Error Codes

E7 uses the central error response shape with request IDs and default Chinese messages.

Stable codes:

| Code | HTTP | Meaning |
| --- | --- | --- |
| `NOTIFICATION_ACCESS_DENIED` | 403 | The caller lacks a required notification permission. |
| `NOTIFICATION_NOT_FOUND` | 404 | The notification does not exist or is not owned by the current user. |
| `NOTIFICATION_LIST_FAILED` | 500 | The backend failed to list or count notifications. |
| `NOTIFICATION_UPDATE_FAILED` | 500 | The backend failed to update read state. |

Permission denials may also use the existing IAM denial code when routed through shared IAM helpers. Frontend copy maps both IAM denial and E7-specific codes to user-safe Chinese messages.

## 9. Frontend Workflow

E7 adds a notification dropdown to the authenticated shell:

- show a bell button when the session has `Notification.List`;
- fetch summary after login and after each read-state update;
- show a red unread badge when `unreadCount > 0`;
- hide the badge when `unreadCount = 0`;
- show the same unread badge beside the "简历库" navigation item when that nav item is visible;
- click the bell to load unread notifications and show a dropdown titled `推荐提醒(N 条未读)`;
- display each row as `<姓名> 被推荐到「{部门}」` and `<推荐人> · <时间>`;
- show empty state `暂无新的推荐提醒` when the unread list is empty;
- show `全部已读` only when `unreadCount > 0` and the session has `Notification.Update`.

Clicking `全部已读`:

1. Calls `POST /notifications/read-all`.
2. Sets local unread count from the response.
3. Clears the dropdown list.
4. Shows toast/status text `已全部标记为已读`.

Clicking one notification:

1. Requires `Notification.Get`, `Notification.Update`, and visible `resume-library` page access.
2. Calls `POST /notifications/{notificationId}/read`.
3. Sets local unread count from the response.
4. Closes the dropdown.
5. Navigates the shell to `#resume-library`.
6. Passes a resume-library jump context containing `chan`, `resumeId`, `candidateName`, department, and recommender.

The app shell should introduce explicit hash-route state instead of deriving the active page only from `session.defaultRoute`. This enables notification click-through and existing nav links to render the selected page reliably.

Resume library integration:

- accept an optional notification jump context;
- switch the channel tab to the notification `chan`;
- set the search text to the notification candidate name;
- display a green banner above the list: `有 N 份简历被推荐到你可查看的部门`;
- list candidate name, department name, and recommender name in the banner;
- for E7, `N` is usually `1` because the jump starts from one notification row, but the state shape should allow an array for future grouped jumps;
- keep backend resume list filtering authoritative through the existing `GET /resumes` API.

If `canOpenResumeLibrary=false` or the session lacks `resume-library` page access, the row is displayed without click-through. The backend still allows read acknowledgement through the read endpoint only when the caller has `Notification.Get` and `Notification.Update`.

## 10. Testing Requirements

Backend tests:

- service summary counts only current-user unread rows;
- list returns only current-user unread rows, sorted newest first;
- list enrichment falls back safely when optional recommender or position data is missing;
- mark-all updates only current-user unread rows and returns a fresh count;
- single mark-read is idempotent for an owned row;
- single mark-read does not reveal another user's notification;
- routes enforce `Notification.List`, `Notification.Get`, and `Notification.Update`;
- preset role seed migration grants notification read/update permissions to manager and trainee roles.

Frontend tests:

- API client wrappers call the generated notification endpoints;
- bell hides the badge at zero and shows unread count above zero;
- opening the bell loads unread notifications and renders PRD row copy;
- mark-all-read clears the badge and dropdown list;
- clicking one notification marks it read, navigates to resume library, switches channel, and shows the recommendation banner;
- users without `Notification.List` do not see the bell;
- users without `Notification.Update` cannot mark read.

Generated artifact checks:

- OpenAPI contract includes the E7 endpoints and DTOs.
- Generated API client schema includes E7 operations.
- `make openapi-check` and `make client-check` must pass after generation.

## 11. Acceptance Checklist

- Top bell shows unread count for the current user after login.
- Red badge is hidden at zero unread.
- Resume library nav item shows the same unread badge when visible.
- Bell dropdown lists unread recommendation notifications with candidate, department, recommender, and time.
- Empty dropdown shows `暂无新的推荐提醒`.
- `全部已读` updates current-user notifications only and clears unread badges.
- Clicking a notification marks it read, closes the dropdown, and navigates to the resume library.
- Resume library switches to the notification channel and shows a green recommendation banner.
- Backend APIs cannot read or update another user's notification rows.
- Manager and trainee recipients created by E3 can read and acknowledge their own notifications.

## 12. Implementation Notes

Suggested file boundaries for the later implementation plan:

- `apps/api/internal/notification/types.go`: DTOs and service inputs.
- `apps/api/internal/notification/service.go`: permission-independent orchestration and validation.
- `apps/api/internal/notification/sql_store.go`: SQL read/update behavior.
- `apps/api/internal/app/notification_routes.go`: Huma route registration and IAM guards.
- `apps/api/migrations/*_grant_notification_read_permissions.sql`: idempotent preset permission grants.
- `apps/web/src/notifications/NotificationBell.tsx`: bell, dropdown, mark-all, and item click behavior.
- `apps/web/src/notifications/types.ts`: frontend notification types.
- `apps/web/src/api/client.ts`: generated-client wrappers for notification APIs.
- `apps/web/src/app/App.tsx`: hash route state, badge state, and resume-library jump context.
- `apps/web/src/resume-library/ResumeLibraryPage.tsx`: channel/search jump handling and green banner.

Implementation must follow red-green-refactor TDD. Generated OpenAPI and TypeScript client files may be regenerated after backend route tests are green.
