# 001 Auth Session W3 SPEC

## 1. Scope and Goals

This SPEC defines E1: login and identity authentication. It covers the TalentPilot login form, backend W3 credential authentication, user upsert, first-login guest binding, token Cookie sessions, CSRF protection, single-device login, current-user loading, logout, and frontend page access after login.

This SPEC supersedes the earlier PRD wording that showed a guest empty state. The current product decision is:

- Unauthenticated users see the login page.
- Authenticated users land on "简历解析".
- Guest users have page access only to "简历解析" and "简历推荐".
- Guest users do not see a guest empty state.
- Backend authorization remains the trusted boundary for every resource operation.

## 2. Non-Goals

- This SPEC does not implement the complete IAM permission model. That belongs to `002-iam-permission-model.md`.
- This SPEC does not define resume parsing or recommendation business behavior.
- This SPEC does not expose full role, permission, or role-relation administration.
- This SPEC does not store or synchronize W3 data beyond `id`, `name`, and `employeeId`.
- This SPEC does not choose the final production W3 wire protocol beyond defining the adapter boundary.

## 3. Login Flow

1. The user opens TalentPilot while unauthenticated and sees the login page.
2. The user enters company domain account and password.
3. The frontend calls `GET /auth/csrf`, then submits `POST /auth/w3/login` with the CSRF header.
4. The backend validates CSRF and `Origin` / `Referer`.
5. The backend passes the account and password to `W3Adapter.Authenticate`.
6. The W3 adapter calls the company W3 API and confirms the account exists and is active.
7. W3 returns exactly `id`, `name`, and `employeeId` to the application boundary.
8. The backend upserts the user in a transaction:
   - existing user: update `name` and `employeeId`;
   - new user: create `users` row, then create `UserDepartmentRole(userId, __system__, guestRoleId)`.
9. The backend creates a new auth session token and revokes prior active tokens for the same user.
10. The backend sets `tp_auth` and `tp_csrf` cookies and returns the current-user summary.
11. The frontend navigates to "简历解析" and shows the success toast `已通过 W3 登录 · {角色 label 列表}`.

W3 timeout behavior follows the PRD: retry once, then fail with a stable auth error code.

## 4. Credential Safety

TalentPilot receives the company account password only because the product requires an in-app login form. The implementation must keep this boundary narrow:

- Passwords are never stored in the database, cache, audit logs, request logs, metrics, traces, or error details.
- Password fields are redacted before structured logging and panic reporting.
- Passwords are kept only in request memory long enough to call W3.
- Production login requires HTTPS.
- The login API must not return whether the account or password was individually wrong.
- Login failures use stable error codes with safe Chinese fallback messages.
- W3 user info is limited to `id`, `name`, and `employeeId`.

## 5. Session Model

The backend issues an opaque random token and stores only its hash.

Cookie requirements:

- `tp_auth`: auth token, `HttpOnly`, `Secure` in production, `SameSite=Lax`, path `/`.
- `tp_csrf`: CSRF token, not `HttpOnly`, `Secure` in production, `SameSite=Lax`, path `/`.
- Frontend code must not store `tp_auth` in `localStorage` or `sessionStorage`.

Session storage:

- Add an `auth_sessions` table.
- Store `token_hash`, `csrf_token_hash`, `user_id`, `expires_at`, `revoked_at`, `created_at`, `last_seen_at`.
- Never store raw auth or CSRF tokens.
- A request is authenticated only when the token hash matches a non-revoked, non-expired session.
- Login revokes all other active sessions for the same user, implementing "后登录踢前登录".
- Logout revokes only the current session and expires both cookies.

## 6. CSRF Protection

CSRF protection is mandatory because authentication uses cookies.

- `GET /auth/csrf` issues or refreshes `tp_csrf`.
- `POST /auth/w3/login` requires `X-CSRF-Token` matching the `tp_csrf` cookie.
- All non-`GET` / `HEAD` / `OPTIONS` authenticated APIs require `X-CSRF-Token`.
- The backend also validates `Origin` or `Referer` against the configured frontend origin.
- A CSRF failure returns a stable auth error code and does not call W3 or mutate state.
- Successful login rotates the CSRF token.

## 7. API Contract

### `GET /auth/csrf`

Issues a CSRF cookie for the login page and returns no sensitive data.

### `POST /auth/w3/login`

Input:

```json
{
  "account": "zhangsan",
  "password": "company-password"
}
```

Output:

```json
{
  "user": {
    "id": "w3-user-id",
    "employeeId": "A12345",
    "name": "张三"
  },
  "roleBindings": [
    {
      "roleLabel": "游客",
      "departmentId": "__system__",
      "departmentName": "system"
    }
  ],
  "roleLabels": ["游客"],
  "pageAccess": ["resume-parse", "resume-recommend"],
  "defaultRoute": "/resume-parse"
}
```

The response sets `tp_auth` and `tp_csrf` cookies. It never returns the auth token in JSON.

### `GET /me`

Returns the current authenticated user, role summary, page access, and default route. If the auth token is missing, revoked, expired, or replaced by a newer login, it returns `AUTH_UNAUTHENTICATED`.

### `POST /auth/logout`

Revokes the current session and expires `tp_auth` and `tp_csrf`. It is also used by "退出登录 / 切换账号"; the frontend then routes to the login page.

## 8. Frontend Behavior

- The login page uses project UI components, not raw interactive HTML elements directly in business page code.
- The login form contains company account, password, submit, loading, and error states.
- User-visible login text and errors live in i18n dictionaries.
- After login, the app calls or uses the login response equivalent to `GET /me`.
- The default route is "简历解析".
- Guest navigation shows only "简历解析" and "简历推荐"; no guest empty state is shown.
- Other business pages are hidden or inaccessible in the frontend for guests.
- Direct URL access to unauthorized pages must still be rejected by backend authorization once those APIs exist.
- The top-right user menu shows name, employee ID, and role bindings with department labels.

## 9. Backend Components

E1 introduces these backend boundaries:

- `internal/auth`: login service, session service, W3 adapter interface.
- `internal/auth/w3`: production and mock adapter implementations.
- `internal/http/middleware`: session and CSRF middleware.
- `internal/iam`: minimal role summary query for `GET /me`; full permission expansion remains in SPEC 002.
- `internal/audit`: login success/failure and logout audit hooks without credential details.

The W3 adapter interface should accept credentials and return only:

```text
W3Identity { id, name, employeeId }
```

Development and test may use a mock W3 adapter, but production must not include sample users or sample passwords in migrations.

## 10. Data and Seed Requirements

E1 depends on the foundation tables:

- `users`
- `departments`
- `roles`
- `user_department_roles`

E1 must also ensure seed data exists for:

- system department `__system__`;
- system role `游客`;
- any minimum Permission rows needed for later IAM compatibility.

The guest `UserDepartmentRole` is permanently retained for audit continuity, even after the user receives business roles.

## 11. Error Codes

Add stable auth error codes as needed:

- `AUTH_CSRF_INVALID`
- `AUTH_W3_INVALID_CREDENTIALS`
- `AUTH_W3_UNAVAILABLE`
- `AUTH_W3_TIMEOUT`
- `AUTH_SESSION_EXPIRED`
- `AUTH_SESSION_REVOKED`
- `AUTH_LOGIN_FAILED`

Messages are Chinese fallbacks. Frontend display text maps through i18n and must not depend on raw backend strings.

## 12. Testing Requirements

Implementation must follow red-green-refactor TDD.

Backend tests:

- CSRF is required before W3 login.
- Invalid CSRF does not call W3.
- W3 success creates a new user and guest binding.
- W3 success updates an existing user's `name` and `employeeId`.
- W3 invalid credentials do not create a user.
- W3 timeout retries once and then returns a stable error.
- Login sets auth and CSRF cookies.
- `GET /me` accepts a valid token and rejects missing, expired, revoked, or replaced tokens.
- Second login revokes the first active token for that user.
- Logout revokes only the current session.
- Password values are redacted in structured logs and audit details.

Frontend tests:

- Unauthenticated users see the login form.
- Login submits account and password with CSRF header.
- Login success navigates to "简历解析".
- Guest navigation includes only "简历解析" and "简历推荐".
- The guest experience has no empty-state copy.
- Logout returns to the login page.

Contract and integration checks:

- OpenAPI is generated from backend route definitions.
- The TypeScript client is regenerated from OpenAPI.
- `make openapi-check` and `make client-check` are required after API changes.

## 13. Acceptance Criteria

E1 is complete when:

- The login page supports company account/password W3 authentication through the backend.
- Successful W3 authentication creates or refreshes `User`.
- First login creates the permanent guest binding.
- Auth token lives only in a secure Cookie and is verified by the backend.
- CSRF is enforced for login and state-changing APIs.
- A second login invalidates the old session.
- Logout clears the current session.
- `GET /me` returns user identity, role bindings, page access, and default route.
- Guests see only "简历解析" and "简历推荐" pages, with no guest empty state.
- All E1 behavior is covered by failing-first tests before implementation.
