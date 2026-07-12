# Quality Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce the frontend component rule automatically and make `make test-e2e` run useful Playwright smoke coverage.

**Architecture:** Add a small TypeScript-AST repository check that is run by the web lint script, then migrate remaining raw table usage to project UI wrappers. Add Playwright under `apps/web` with mocked API routes for smoke-level frontend workflow coverage; backend business correctness remains covered by existing Go tests.

**Tech Stack:** Node ESM scripts, TypeScript compiler API, Vitest, React, Vite, Playwright Chromium, pnpm, Makefile, GitHub Actions.

---

## Scope Check

`docs/specs/010-quality-hardening.md` has two workstreams. They are independent but both update quality gates, so this plan keeps them together with separate task boundaries and commits:

1. Component-rule enforcement.
2. Playwright smoke setup and CI documentation.

No new business API behavior is part of this plan.

## File Structure

- `apps/web/scripts/check-business-ui-elements.mjs`: Node ESM checker using the installed `typescript` package to parse TSX and report raw interactive JSX tags in business source files.
- `apps/web/scripts/check-business-ui-elements.test.mjs`: Vitest coverage for checker allow/deny behavior.
- `apps/web/src/components/ui/table.tsx`: project table wrapper primitives.
- Business pages with existing raw `<table>` usage:
  - `apps/web/src/users/UsersPage.tsx`
  - `apps/web/src/roles/RoleManagementPage.tsx`
  - `apps/web/src/resume-library/ResumeLibraryPage.tsx`
  - `apps/web/src/department-position/DepartmentPositionPage.tsx`
- `apps/web/package.json`: wire checker into `lint`; add Playwright dependency.
- `apps/web/playwright.config.ts`: Playwright config with Vite web server.
- `apps/web/tsconfig.node.json`: include Playwright config in node-side typecheck.
- `apps/web/e2e/api-fixtures.ts`: deterministic route mocks for smoke tests.
- `apps/web/e2e/workspace-smoke.spec.ts`: shell, parse, recommendation, notification smoke tests.
- `apps/web/e2e/admin-smoke.spec.ts`: user/role management smoke tests.
- `.github/workflows/ci.yml`: install Playwright Chromium and run `make test-e2e`.
- `AGENTS.md` and `docs/project-status.md`: record changed command behavior and evidence.

---

### Task 1: Add Component Rule Checker Tests

**Files:**
- Create: `apps/web/scripts/check-business-ui-elements.test.mjs`
- Expected later implementation file: `apps/web/scripts/check-business-ui-elements.mjs`

- [ ] **Step 1: Write the failing checker test**

Create `apps/web/scripts/check-business-ui-elements.test.mjs`:

```js
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { findViolations } from "./check-business-ui-elements.mjs";

const tempRoots = [];

async function makeRoot() {
  const root = await mkdtemp(join(tmpdir(), "talentpilot-ui-check-"));
  tempRoots.push(root);
  return root;
}

async function write(root, file, source) {
  const path = join(root, file);
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, source, "utf8");
}

describe("check-business-ui-elements", () => {
  afterEach(async () => {
    await Promise.all(tempRoots.splice(0).map((root) => rm(root, { force: true, recursive: true })));
  });

  it("reports raw interactive elements in business TSX files", async () => {
    const root = await makeRoot();
    await write(root, "src/resume-library/BadPage.tsx", "export function BadPage() { return <button>Bad</button>; }");

    const violations = await findViolations(root);

    expect(violations).toEqual([
      expect.objectContaining({
        element: "button",
        file: expect.stringContaining("src/resume-library/BadPage.tsx"),
      }),
    ]);
  });

  it("allows UI wrappers and test files to render raw elements", async () => {
    const root = await makeRoot();
    await write(root, "src/components/ui/button.tsx", "export function Button() { return <button>OK</button>; }");
    await write(root, "src/users/UsersPage.test.tsx", "export function Fixture() { return <input />; }");
    await write(root, "src/users/UsersPage.tsx", "export function UsersPage() { return <Button />; }");

    await expect(findViolations(root)).resolves.toEqual([]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run scripts/check-business-ui-elements.test.mjs
```

Expected: FAIL with a module import error for `./check-business-ui-elements.mjs`.

---

### Task 2: Implement Checker and Wire It Into Web Lint

**Files:**
- Create: `apps/web/scripts/check-business-ui-elements.mjs`
- Modify: `apps/web/package.json`

- [ ] **Step 1: Add the checker implementation**

Create `apps/web/scripts/check-business-ui-elements.mjs`:

```js
import { readdir, readFile } from "node:fs/promises";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import ts from "typescript";

const disallowedElements = new Set(["button", "input", "select", "textarea", "dialog", "form", "table"]);

function normalize(path) {
  return path.split(sep).join("/");
}

function isIgnored(file) {
  const normalized = normalize(file);
  return (
    normalized.includes("/components/ui/") ||
    normalized.includes("/test/") ||
    normalized.endsWith(".test.tsx") ||
    normalized.endsWith(".test.ts") ||
    normalized.endsWith(".spec.tsx") ||
    normalized.endsWith(".spec.ts")
  );
}

async function collectTsxFiles(dir, files = []) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== "dist" && entry.name !== "node_modules") {
        await collectTsxFiles(path, files);
      }
      continue;
    }
    if (entry.isFile() && path.endsWith(".tsx") && !isIgnored(path)) {
      files.push(path);
    }
  }
  return files;
}

function jsxTagName(node) {
  const tag = node.tagName;
  return ts.isIdentifier(tag) ? tag.text : "";
}

export async function findViolations(root = process.cwd()) {
  const srcRoot = join(root, "src");
  const files = await collectTsxFiles(srcRoot);
  const violations = [];

  for (const file of files) {
    const source = await readFile(file, "utf8");
    const sourceFile = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

    function visit(node) {
      if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
        const element = jsxTagName(node);
        if (disallowedElements.has(element)) {
          const { line, character } = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
          violations.push({
            column: character + 1,
            element,
            file: relative(root, file),
            line: line + 1,
          });
        }
      }
      ts.forEachChild(node, visit);
    }

    visit(sourceFile);
  }

  return violations;
}

export function formatViolations(violations) {
  return violations
    .map((violation) => `${violation.file}:${violation.line}:${violation.column} raw <${violation.element}> is not allowed in business pages`)
    .join("\n");
}

async function main() {
  const root = join(dirname(fileURLToPath(import.meta.url)), "..");
  const violations = await findViolations(root);
  if (violations.length > 0) {
    console.error(formatViolations(violations));
    process.exitCode = 1;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
```

- [ ] **Step 2: Run checker test to verify it passes**

Run:

```bash
pnpm --filter @talentpilot/web test -- --run scripts/check-business-ui-elements.test.mjs
```

Expected: PASS.

- [ ] **Step 3: Wire checker into web lint**

Modify `apps/web/package.json`:

```json
"lint": "eslint . && node ./scripts/check-business-ui-elements.mjs"
```

- [ ] **Step 4: Run lint to verify expected business-page failures**

Run:

```bash
pnpm --filter @talentpilot/web lint
```

Expected: FAIL, reporting the existing raw `<table>` usage in:

- `src/users/UsersPage.tsx`
- `src/roles/RoleManagementPage.tsx`
- `src/resume-library/ResumeLibraryPage.tsx`
- `src/department-position/DepartmentPositionPage.tsx`

Do not commit while lint is red.

---

### Task 3: Add Table Wrapper and Migrate Business Tables

**Files:**
- Create: `apps/web/src/components/ui/table.tsx`
- Modify: `apps/web/src/users/UsersPage.tsx`
- Modify: `apps/web/src/roles/RoleManagementPage.tsx`
- Modify: `apps/web/src/resume-library/ResumeLibraryPage.tsx`
- Modify: `apps/web/src/department-position/DepartmentPositionPage.tsx`

- [ ] **Step 1: Add the table UI wrapper**

Create `apps/web/src/components/ui/table.tsx`:

```tsx
import * as React from "react";
import { cn } from "./cn";

export type TableProps = React.TableHTMLAttributes<HTMLTableElement>;

export const Table = React.forwardRef<HTMLTableElement, TableProps>(({ className, ...props }, ref) => (
  <table ref={ref} className={cn("w-full border-collapse text-left text-sm", className)} {...props} />
));
Table.displayName = "Table";

export const TableHeader = React.forwardRef<HTMLTableSectionElement, React.HTMLAttributes<HTMLTableSectionElement>>(
  ({ className, ...props }, ref) => <thead ref={ref} className={cn("bg-white/[0.04] text-muted", className)} {...props} />,
);
TableHeader.displayName = "TableHeader";

export const TableBody = React.forwardRef<HTMLTableSectionElement, React.HTMLAttributes<HTMLTableSectionElement>>(
  ({ className, ...props }, ref) => <tbody ref={ref} className={className} {...props} />,
);
TableBody.displayName = "TableBody";

export const TableRow = React.forwardRef<HTMLTableRowElement, React.HTMLAttributes<HTMLTableRowElement>>(
  ({ className, ...props }, ref) => <tr ref={ref} className={cn("border-b border-white/10 last:border-0", className)} {...props} />,
);
TableRow.displayName = "TableRow";

export const TableHead = React.forwardRef<HTMLTableCellElement, React.ThHTMLAttributes<HTMLTableCellElement>>(
  ({ className, ...props }, ref) => <th ref={ref} className={cn("border-b border-white/10 px-3 py-3 font-medium", className)} scope="col" {...props} />,
);
TableHead.displayName = "TableHead";

export const TableCell = React.forwardRef<HTMLTableCellElement, React.TdHTMLAttributes<HTMLTableCellElement>>(
  ({ className, ...props }, ref) => <td ref={ref} className={cn("px-3 py-3", className)} {...props} />,
);
TableCell.displayName = "TableCell";
```

- [ ] **Step 2: Migrate each business page to wrappers**

In each modified page, add:

```tsx
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
```

Apply these exact tag mappings:

```text
<table className="w-full min-w-[900px] border-collapse text-left text-sm"> -> <Table className="min-w-[900px]">
<table className="w-full min-w-[960px] border-collapse text-left text-sm"> -> <Table className="min-w-[960px]">
<table className="w-full min-w-[920px] border-collapse text-left text-sm"> -> <Table className="min-w-[920px]">
<table className="w-full min-w-[760px] border-collapse text-left text-sm"> -> <Table className="min-w-[760px]">
<table className="w-full min-w-[980px] border-collapse text-left text-sm"> -> <Table className="min-w-[980px]">
</table> -> </Table>
<thead className="bg-white/[0.04] text-muted"> -> <TableHeader>
</thead> -> </TableHeader>
<tbody> -> <TableBody>
</tbody> -> </TableBody>
<tr> -> <TableRow>
<tr className="border-b border-white/10 last:border-0" ...> -> <TableRow ...>
</tr> -> </TableRow>
<th className="border-b border-white/10 px-3 py-3 font-medium" key={label} scope="col"> -> <TableHead key={label}>
</th> -> </TableHead>
<td className="px-3 py-3 ..."> -> <TableCell className="...">
<td className="px-3 py-3" ...> -> <TableCell ...>
</td> -> </TableCell>
```

Keep `colSpan` and any existing row/cell-specific classes.

- [ ] **Step 3: Run targeted checker and frontend tests**

Run:

```bash
pnpm --filter @talentpilot/web lint
CI=true pnpm --filter @talentpilot/web test -- --run
```

Expected: PASS. If tests fail because accessible table roles changed, fix the wrapper migration so semantic table markup is preserved.

- [ ] **Step 4: Commit component enforcement**

Run:

```bash
git add apps/web/package.json apps/web/scripts/check-business-ui-elements.mjs apps/web/scripts/check-business-ui-elements.test.mjs apps/web/src/components/ui/table.tsx apps/web/src/users/UsersPage.tsx apps/web/src/roles/RoleManagementPage.tsx apps/web/src/resume-library/ResumeLibraryPage.tsx apps/web/src/department-position/DepartmentPositionPage.tsx
git commit -m "test: enforce business ui element wrappers"
```

---

### Task 4: Install Playwright and Add Authenticated Shell Smoke

**Files:**
- Modify: `apps/web/package.json`
- Modify: `pnpm-lock.yaml`
- Modify: `apps/web/tsconfig.node.json`
- Create: `apps/web/playwright.config.ts`
- Create: `apps/web/e2e/api-fixtures.ts`
- Create: `apps/web/e2e/workspace-smoke.spec.ts`

- [ ] **Step 1: Verify current reserved command fails**

Run:

```bash
make test-e2e
```

Expected: FAIL because `playwright` is not installed. This is the setup exception documented in `docs/specs/010-quality-hardening.md`.

- [ ] **Step 2: Add Playwright dependency**

Run:

```bash
pnpm --filter @talentpilot/web add -D @playwright/test
```

- [ ] **Step 3: Add Playwright config**

Create `apps/web/playwright.config.ts`:

```ts
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://127.0.0.1:5173",
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "pnpm dev -- --host 127.0.0.1",
    reuseExistingServer: !process.env.CI,
    url: "http://127.0.0.1:5173",
  },
});
```

Modify `apps/web/tsconfig.node.json`:

```json
"include": ["vite.config.ts", "vitest.config.ts", "tailwind.config.ts", "playwright.config.ts"]
```

- [ ] **Step 4: Add minimal API fixtures**

Create `apps/web/e2e/api-fixtures.ts` with a first-pass authenticated session mock:

```ts
import type { Page, Route } from "@playwright/test";

export const adminSession = {
  dataScope: { allDepartments: true, channels: ["social", "campus"], departments: [] },
  defaultRoute: "/resume-parse",
  pageAccess: ["resume-parse", "resume-recommend", "resume-library", "departments-positions", "users", "roles"],
  permissions: [
    "Resume.List", "Resume.Get", "Resume.Create", "Resume.Update", "Resume.Delete",
    "Position.List", "Position.Get", "PositionResume.Create", "DepartmentResume.Create",
    "Notification.List", "Notification.Get", "Notification.Update", "Notification.Create",
    "User.List", "User.Get", "UserDepartmentRole.List", "UserDepartmentRole.Create", "UserDepartmentRole.Delete",
    "Role.List", "Role.Get", "Role.Create", "Role.Update", "Role.Delete", "Permission.List",
  ],
  roleBindings: [{ departmentId: "__system__", departmentName: "system", roleLabel: "超级管理员" }],
  roleLabels: ["超级管理员"],
  user: { employeeId: "A00001", id: "admin", name: "管理员" },
};

export async function installApiMocks(page: Page) {
  await page.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/me") {
      return json(route, adminSession);
    }
    if (url.pathname === "/notifications/summary") {
      return json(route, { unreadCount: 2 });
    }
    return route.fallback();
  });
}

export async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    body: JSON.stringify(body),
    contentType: "application/json",
    status,
  });
}
```

- [ ] **Step 5: Add first smoke test**

Create `apps/web/e2e/workspace-smoke.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { installApiMocks } from "./api-fixtures";

test.beforeEach(async ({ page }) => {
  await installApiMocks(page);
});

test("authenticated shell renders permission-scoped navigation", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByText("A00001")).toBeVisible();
  await expect(page.getByRole("link", { name: "简历解析" })).toBeVisible();
  await expect(page.getByRole("link", { name: "简历推荐" })).toBeVisible();
  await expect(page.getByRole("link", { name: "用户管理" })).toBeVisible();
  await expect(page.getByRole("link", { name: "角色管理" })).toBeVisible();
  await expect(page.getByRole("button", { name: "推荐提醒" })).toBeVisible();
});
```

- [ ] **Step 6: Install browser and run E2E**

Run:

```bash
pnpm --filter @talentpilot/web exec playwright install chromium
make test-e2e
```

Expected: PASS for the authenticated shell test.

- [ ] **Step 7: Commit Playwright baseline**

Run:

```bash
git add apps/web/package.json pnpm-lock.yaml apps/web/tsconfig.node.json apps/web/playwright.config.ts apps/web/e2e/api-fixtures.ts apps/web/e2e/workspace-smoke.spec.ts
git commit -m "test: add playwright shell smoke"
```

---

### Task 5: Add Parse, Recommendation, and Notification Smoke Flows

**Files:**
- Modify: `apps/web/e2e/api-fixtures.ts`
- Modify: `apps/web/e2e/workspace-smoke.spec.ts`

- [ ] **Step 1: Add failing workflow tests**

Append tests to `apps/web/e2e/workspace-smoke.spec.ts`:

```ts
test("parses a resume and generates interview questions", async ({ page }) => {
  await page.goto("/#resume-parse");

  await page.getByRole("button", { name: /张三/ }).click();
  await page.getByRole("button", { name: "开始解析" }).click();
  await expect(page.getByText("建议进入面试")).toBeVisible();

  await page.getByRole("button", { name: "生成面试题" }).click();
  await page.getByRole("tab", { name: "主管" }).click();
  await expect(page.getByText(/为什么选择算力训练平台部/)).toBeVisible();
});

test("sends a recommendation and consumes notification read state", async ({ page }) => {
  await page.goto("/#resume-recommend");

  await page.getByRole("button", { name: /张三/ }).click();
  await page.getByRole("button", { name: "智能分流" }).click();
  await expect(page.getByText("最佳去向")).toBeVisible();
  await page.getByRole("button", { name: "推荐到" }).click();
  await expect(page.getByRole("status")).toContainText("已推荐到「智算调度部」");

  await page.getByRole("button", { name: "推荐提醒" }).click();
  await page.getByRole("button", { name: /张三 被推荐到/ }).click();
  await expect(page.getByRole("heading", { name: "简历库" })).toBeVisible();
  await expect(page.getByText("有 1 份简历被推荐到你可查看的部门")).toBeVisible();

  await page.getByRole("button", { name: "推荐提醒" }).click();
  await page.getByRole("button", { name: "全部已读" }).click();
  await expect(page.getByRole("status")).toContainText("已全部标记为已读");
});
```

Run:

```bash
make test-e2e
```

Expected: FAIL because the API fixture does not yet handle resume, position, matching, recommendation, and notification read endpoints.

- [ ] **Step 2: Extend route fixtures**

Extend `installApiMocks` in `apps/web/e2e/api-fixtures.ts` with deterministic responses for:

```ts
if (url.pathname === "/resumes" && route.request().method() === "GET") {
  return json(route, {
    availableChannels: ["social", "campus"],
    channelCounts: { social: 1, campus: 0 },
    dataScopeSummary: "全部部门",
    items: [{
      id: "resume_1",
      name: "张三",
      age: 29,
      school: "浙江大学",
      yearsExp: 6,
      currentDepartment: { id: "dept_a", name: "算力训练平台部" },
      pos: "平台工程师",
      source: "导入",
      sourceBy: "李四",
      chan: "social",
      keywords: ["Go", "调度"],
      canGet: true,
      canDelete: false,
    }],
    nextCursor: "",
  });
}

if (url.pathname === "/positions" && route.request().method() === "GET") {
  return json(route, { items: [{
    id: "position_1",
    name: "平台工程师",
    department: { id: "dept_a", name: "算力训练平台部" },
    chan: "social",
    level: "P6",
    status: "on",
    keywordCount: 2,
    implicitTagCount: 1,
    updatedAt: "2026-07-12T08:00:00Z",
    canGet: true,
    canUpdate: true,
    canDelete: true,
  }] });
}

if (url.pathname === "/matching/parse") {
  return json(route, {
    id: "position_resume_1",
    resume: { id: "resume_1", name: "张三", traits: ["稳定"], expBase: 82 },
    position: { id: "position_1", name: "平台工程师", department: { id: "dept_a", name: "算力训练平台部" }, chan: "social", level: "P6", status: "on", keywords: ["Go", "Kubernetes"], implicitTags: [{ name: "稳定", w: 40 }] },
    score: { total: 76, skill: 50, experience: 82, implicit: 100, judgement: "建议进入面试" },
    evidence: { keywords: [{ name: "Go", matched: true }, { name: "Kubernetes", matched: false }], implicitTags: [{ name: "稳定", w: 40, matched: true }], analysis: "技能命中 1/2；隐性要求命中 1/1；建议进入面试。" },
    createdAt: "2026-07-12T08:00:00Z",
  });
}

if (url.pathname === "/matching/interview-questions") {
  return json(route, { groups: [
    { type: "professional", label: "专业面试", questions: [{ order: 1, question: "请介绍 Go 项目", why: "验证经验", difficulty: "核心" }] },
    { type: "manager", label: "主管面试", questions: [{ order: 1, question: "为什么选择算力训练平台部，以及你期待如何协作？", why: "确认动机", difficulty: "动机" }] },
    { type: "qualification", label: "资格面试", questions: [{ order: 1, question: "请确认到岗时间", why: "确认流程", difficulty: "流程" }] },
  ] });
}

if (url.pathname === "/recommendations/route") {
  return json(route, {
    resume: { id: "resume_1", name: "张三", chan: "social", pos: "平台工程师", currentDepartment: { id: "dept_a", name: "算力训练平台部" }, keywords: ["Go"] },
    routes: [{ department: { id: "dept_b", name: "智算调度部" }, position: { id: "position_b", name: "调度工程师", chan: "social", level: "P6" }, score: { total: 86, skill: 100, experience: 82, implicit: 80, judgement: "强烈推荐" }, contacts: { hrbps: ["李四"], managers: ["王五"], trainees: ["赵六"] }, best: true }],
    createdAt: "2026-07-12T08:00:00Z",
  });
}

if (url.pathname === "/recommendations/send") {
  return json(route, { resumeId: "resume_copy", sourceResumeId: "resume_1", department: { id: "dept_b", name: "智算调度部" }, position: { id: "position_b", name: "调度工程师" }, candidateName: "张三", reusedExistingCopy: false, notifiedCount: 4, selfNotificationRead: true, message: "已推荐到「智算调度部」· 已通知 4 位相关人员" });
}

if (url.pathname === "/notifications") {
  return json(route, { items: [{ id: "notification_1", resumeId: "resume_copy", candidateName: "张三", department: { id: "dept_b", name: "智算调度部" }, recommender: { id: "admin", name: "管理员" }, chan: "social", createdAt: "2026-07-12T08:00:00Z", read: false, canOpenResumeLibrary: true }], unreadCount: 2, nextCursor: "" });
}

if (url.pathname === "/notifications/notification_1/read") {
  return json(route, { notification: { id: "notification_1", resumeId: "resume_copy", candidateName: "张三", department: { id: "dept_b", name: "智算调度部" }, recommender: { id: "admin", name: "管理员" }, chan: "social", createdAt: "2026-07-12T08:00:00Z", read: true, canOpenResumeLibrary: true }, unreadCount: 1 });
}

if (url.pathname === "/notifications/read-all") {
  return json(route, { updatedCount: 1, unreadCount: 0 });
}
```

- [ ] **Step 3: Run E2E and web tests**

Run:

```bash
make test-e2e
CI=true pnpm --filter @talentpilot/web test -- --run
```

Expected: PASS.

- [ ] **Step 4: Commit workflow smoke coverage**

Run:

```bash
git add apps/web/e2e/api-fixtures.ts apps/web/e2e/workspace-smoke.spec.ts
git commit -m "test: cover recruiting workflow smoke"
```

---

### Task 6: Add User and Role Management Smoke Flows

**Files:**
- Modify: `apps/web/e2e/api-fixtures.ts`
- Create: `apps/web/e2e/admin-smoke.spec.ts`

- [ ] **Step 1: Add failing admin smoke tests**

Create `apps/web/e2e/admin-smoke.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { installApiMocks } from "./api-fixtures";

test.beforeEach(async ({ page }) => {
  await installApiMocks(page);
});

test("assigns a user role binding", async ({ page }) => {
  await page.goto("/#users");

  const row = page.getByRole("row", { name: /张敏/ });
  await expect(row).toBeVisible();
  await row.getByRole("button", { name: "分配角色" }).click();
  await page.getByLabel("角色").selectOption("__role_manager__");
  await page.getByLabel("部门").selectOption("dept_b");
  await page.getByRole("button", { name: "加入待分配" }).click();
  await page.getByRole("button", { name: "保存分配" }).click();

  await expect(page.getByRole("status")).toContainText("已为 张敏 分配 1 个角色绑定");
});

test("creates a custom role and shows invalid relation errors", async ({ page }) => {
  await page.goto("/#roles");

  await page.getByRole("button", { name: "新建角色" }).click();
  await page.getByLabel("角色名称").fill("高级评审者");
  await page.getByLabel("角色描述").fill("查看社招简历");
  await page.getByLabel("Resume List").check();
  await page.getByLabel("社招").check();
  await page.getByRole("button", { name: "保存角色" }).click();
  await expect(page.getByRole("status")).toContainText("已创建角色");

  await page.getByRole("button", { name: "新建角色" }).click();
  await page.getByLabel("角色名称").fill("循环角色");
  await page.getByLabel("包含 HRBP").check();
  await page.getByRole("button", { name: "保存角色" }).click();
  await expect(page.getByRole("alert")).toContainText("角色包含关系不能形成循环");
});
```

Run:

```bash
make test-e2e
```

Expected: FAIL because `/users`, `/departments`, `/roles`, `/roles/assignable`, `/roles/permission-options`, and `POST /roles` fixture responses are missing.

- [ ] **Step 2: Extend admin route fixtures**

Add route handling in `apps/web/e2e/api-fixtures.ts`:

```ts
if (url.pathname === "/users" && route.request().method() === "GET") {
  return json(route, { canAssignRoles: true, dataScopeSummary: "全部部门", items: [{ id: "user_a", employeeId: "A10001", name: "张敏", departments: [{ id: "dept_a", name: "算力训练平台部", system: false }], roleBindings: [{ id: "udr_a", role: { id: "__role_hrbp__", label: "HRBP", isSystem: true, enabled: true }, department: { id: "dept_a", name: "算力训练平台部", system: false }, guest: false, createdAt: "2026-07-12T08:00:00Z", createdBy: "admin", canDelete: true }], roleSummary: "HRBP(部门:算力训练平台部)", guestOnly: false, canAssign: true }], nextCursor: "" });
}

if (url.pathname === "/departments" && route.request().method() === "GET") {
  return json(route, { items: [{ id: "dept_a", name: "算力训练平台部", positionCount: 1, resumeCount: 2, updatedAt: "2026-07-12T08:00:00Z", canGet: true, canUpdate: true, canDelete: false }, { id: "dept_b", name: "智算调度部", positionCount: 1, resumeCount: 0, updatedAt: "2026-07-12T08:00:00Z", canGet: true, canUpdate: true, canDelete: false }] });
}

if (url.pathname === "/roles/assignable") {
  return json(route, { items: [{ id: "__role_hrbp__", label: "HRBP", description: "部门 HRBP", isSystem: true, supportsSystemDepartment: false, attributeConditionSummary: "" }, { id: "__role_manager__", label: "主管", description: "业务主管", isSystem: true, supportsSystemDepartment: false, attributeConditionSummary: "" }] });
}

if (url.pathname === "/users/user_a/role-bindings") {
  return json(route, { message: "已为 张敏 分配 1 个角色绑定", created: [], user: { id: "user_a", employeeId: "A10001", name: "张敏" } });
}

if (url.pathname === "/roles" && route.request().method() === "GET") {
  return json(route, { canCreate: true, total: 1, items: [{ id: "__role_hrbp__", label: "HRBP", description: "部门 HRBP", isSystem: true, enabled: true, permissionCount: 3, childRoleCount: 0, referenceCount: 2, conditionSummary: "全部渠道", canEdit: true, canDelete: false, canToggleEnabled: true }] });
}

if (url.pathname === "/roles/permission-options") {
  return json(route, { conditionOptions: { chan: ["social", "campus"], expired: [false, true] }, resources: [{ resource: "Resume", actions: [{ action: "List", supportsConditions: { chan: true, expired: true, self: false } }] }] });
}

if (url.pathname === "/roles" && route.request().method() === "POST") {
  const body = route.request().postDataJSON() as { label?: string };
  if (body.label === "循环角色") {
    return json(route, { code: "IAM_ROLE_RELATION_CYCLE", message: "角色包含关系不能形成循环。" }, 400);
  }
  return json(route, { id: "role_custom_reviewer", label: body.label ?? "高级评审者" });
}
```

- [ ] **Step 3: Run E2E and typecheck**

Run:

```bash
make test-e2e
make typecheck
```

Expected: PASS.

- [ ] **Step 4: Commit admin smoke coverage**

Run:

```bash
git add apps/web/e2e/api-fixtures.ts apps/web/e2e/admin-smoke.spec.ts
git commit -m "test: cover admin smoke workflows"
```

---

### Task 7: Wire CI and Update Project Documentation

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `AGENTS.md`
- Modify: `docs/project-status.md`

- [ ] **Step 1: Add Playwright to GitHub Actions**

Modify `.github/workflows/ci.yml` after the Build step:

```yaml
      - name: Install Playwright browsers
        run: pnpm --filter @talentpilot/web exec playwright install --with-deps chromium

      - name: Test E2E
        run: make test-e2e
```

- [ ] **Step 2: Update command documentation**

Update `AGENTS.md` command index:

```markdown
- `make test-e2e`: run Playwright smoke coverage for the web app. Requires pnpm dependencies and installed Playwright Chromium browser; GitHub Actions installs it before running the command.
```

Update the CI bullet:

```markdown
- `make ci`: run lint, typecheck, tests, OpenAPI drift check, client drift check, and builds. Requires pnpm dependencies and Go. Playwright E2E is run by the GitHub Actions CI workflow as a separate browser-backed step.
```

- [ ] **Step 3: Update status checklist**

Update `docs/project-status.md`:

- Foundation `Frontend | Component rules documented`: set `Done`, with evidence `apps/web/scripts/check-business-ui-elements.mjs` and `make lint`.
- Quality `Frontend | Component rule enforcement`: set `Done`, with command evidence from Task 3.
- Quality `Testing | Playwright E2E smoke coverage`: set `Done`, with evidence `apps/web/playwright.config.ts`, `apps/web/e2e/*.spec.ts`, `make test-e2e`, and GitHub Actions browser install step.
- Current recommended order: move to “choose next product phase or hardening target.”

- [ ] **Step 4: Run final verification**

Run:

```bash
make lint
make typecheck
make test
make test-e2e
git diff --check
```

Expected: PASS.

- [ ] **Step 5: Commit documentation and CI wiring**

Run:

```bash
git add .github/workflows/ci.yml AGENTS.md docs/project-status.md
git commit -m "ci: run playwright smoke coverage"
```

---

## Final Verification

After all tasks are complete, run:

```bash
make lint
make typecheck
make test
make openapi-check
make client-check
make build
make test-e2e
git status --short
```

Expected:

- all commands pass;
- no OpenAPI or generated client drift unless a prior task intentionally changed API code, which this plan should not do;
- working tree is clean except for unrelated pre-existing files such as `.codex-work/`.

## Self-Review Notes

- SPEC Workstream A is covered by Tasks 1-3 and status updates in Task 7.
- SPEC Workstream B is covered by Tasks 4-6 and CI/status updates in Task 7.
- No backend business behavior is changed.
- Playwright uses mocked API routes by design for the first smoke suite; backend correctness stays in Go tests and OpenAPI/client checks.
