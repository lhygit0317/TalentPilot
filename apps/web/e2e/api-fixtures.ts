import type { Page, Route } from "@playwright/test";

export const adminSession = {
  dataScope: { allDepartments: true, channels: ["social", "campus"], departments: [] },
  defaultRoute: "/resume-parse",
  pageAccess: ["resume-parse", "resume-recommend", "resume-library", "departments-positions", "users", "roles"],
  permissions: [
    "Resume.List",
    "Resume.Get",
    "Resume.Create",
    "Resume.Update",
    "Resume.Delete",
    "Position.List",
    "Position.Get",
    "PositionResume.Create",
    "DepartmentResume.Create",
    "Notification.List",
    "Notification.Get",
    "Notification.Update",
    "Notification.Create",
    "User.List",
    "User.Get",
    "UserDepartmentRole.List",
    "UserDepartmentRole.Create",
    "UserDepartmentRole.Delete",
    "Role.List",
    "Role.Get",
    "Role.Create",
    "Role.Update",
    "Role.Delete",
    "Permission.List",
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
    if (url.pathname === "/resumes") {
      return json(route, { items: [], nextCursor: "" });
    }
    if (url.pathname === "/positions") {
      return json(route, { items: [] });
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
