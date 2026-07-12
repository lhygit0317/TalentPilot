import { createTalentPilotClient } from "@talentpilot/api-client";

export const apiClient = createTalentPilotClient(
  import.meta.env.WEB_API_BASE_URL || import.meta.env.VITE_API_BASE_URL || "",
);

export async function getCurrentUser() {
  return apiClient.GET("/me");
}

export function listResumes(query: { chan?: "social" | "campus"; search?: string; limit?: number; cursor?: string }) {
  return apiClient.GET("/resumes", { params: { query } });
}

export function getResume(resumeId: string) {
  return apiClient.GET("/resumes/{resumeId}", { params: { path: { resumeId } } });
}

export function deleteResume(resumeId: string) {
  return apiClient.DELETE("/resumes/{resumeId}", { params: { path: { resumeId } } });
}

export function importResume(body: FormData) {
  return apiClient.POST("/resumes/imports", { body: body as never });
}

export function importResumesBatch(body: FormData) {
  return apiClient.POST("/resumes/batch-imports", { body: body as never });
}

export function getJob(jobId: string) {
  return apiClient.GET("/jobs/{jobId}", { params: { path: { jobId } } });
}

export function parseResumeMatch(body: { resumeId: string; positionId: string }) {
  return apiClient.POST("/matching/parse", { body });
}

export function generateInterviewQuestions(body: { resumeId: string; positionId: string; matchScore?: number }) {
  return apiClient.POST("/matching/interview-questions", { body });
}

export function listDepartments(query: { search?: string; limit?: number } = {}) {
  return apiClient.GET("/departments", { params: { query } });
}

export function getDepartment(departmentId: string) {
  return apiClient.GET("/departments/{departmentId}", { params: { path: { departmentId } } });
}

export function createDepartment(body: { name: string }) {
  return apiClient.POST("/departments", { body });
}

export function updateDepartment(departmentId: string, body: { name: string }) {
  return apiClient.PATCH("/departments/{departmentId}", { params: { path: { departmentId } }, body });
}

export function deleteDepartment(departmentId: string) {
  return apiClient.DELETE("/departments/{departmentId}", { params: { path: { departmentId } } });
}

export type PositionMutation = {
  name: string;
  departmentId: string;
  chan: "social" | "campus";
  level: string;
  status: "on" | "off";
  duties: string[];
  must: string[];
  keywords: string[];
  implicitTags: Array<{ name: string; w?: number }>;
};

export function listPositions(
  query: {
    departmentId?: string;
    chan?: "social" | "campus";
    status?: "on" | "off";
    search?: string;
    limit?: number;
  } = {},
) {
  return apiClient.GET("/positions", { params: { query } });
}

export function getPosition(positionId: string) {
  return apiClient.GET("/positions/{positionId}", { params: { path: { positionId } } });
}

export function createPosition(body: PositionMutation) {
  return apiClient.POST("/positions", { body });
}

export function updatePosition(positionId: string, body: PositionMutation) {
  return apiClient.PATCH("/positions/{positionId}", { params: { path: { positionId } }, body });
}

export function deletePosition(positionId: string) {
  return apiClient.DELETE("/positions/{positionId}", { params: { path: { positionId } } });
}

export function routeRecommendation(body: { resumeId: string }) {
  return apiClient.POST("/recommendations/route", { body });
}

export function sendRecommendation(body: { resumeId: string; departmentId: string; positionId: string }) {
  return apiClient.POST("/recommendations/send", { body });
}

export function listUsers(query: { search?: string; limit?: number; cursor?: string } = {}) {
  return apiClient.GET("/users", { params: { query } });
}

export function getUser(userId: string) {
  return apiClient.GET("/users/{userId}", { params: { path: { userId } } });
}

export function listAssignableRoles() {
  return apiClient.GET("/roles/assignable");
}

export function assignUserRoleBindings(
  userId: string,
  body: { bindings: Array<{ departmentId: string; roleId: string }> },
) {
  return apiClient.POST("/users/{userId}/role-bindings", { params: { path: { userId } }, body });
}

export function deleteUserRoleBinding(userId: string, bindingId: string) {
  return apiClient.DELETE("/users/{userId}/role-bindings/{bindingId}", {
    params: { path: { userId, bindingId } },
  });
}

export type RolePermissionMutation = {
  resource: string;
  action: string;
  attributeConditions?: {
    chan?: Array<"social" | "campus">;
    expired?: boolean[];
    self?: boolean;
  };
};

export type RoleDefinitionMutation = {
  label: string;
  description?: string;
  enabled: boolean;
  permissions: RolePermissionMutation[];
  childRoleIds: string[];
};

export function listRoles(
  query: { search?: string; system?: boolean; enabled?: boolean; limit?: number } = {},
) {
  return apiClient.GET("/roles", {
    params: {
      query: {
        ...query,
        system: booleanQuery(query.system),
        enabled: booleanQuery(query.enabled),
      },
    },
  });
}

export function getRole(roleId: string) {
  return apiClient.GET("/roles/{roleId}", { params: { path: { roleId } } });
}

export function getRolePermissionOptions() {
  return apiClient.GET("/roles/permission-options");
}

export function createRoleDefinition(body: RoleDefinitionMutation) {
  return apiClient.POST("/roles", { body });
}

export function updateRoleDefinition(roleId: string, body: RoleDefinitionMutation) {
  return apiClient.PATCH("/roles/{roleId}", { params: { path: { roleId } }, body });
}

export function toggleRoleEnabled(roleId: string, enabled: boolean) {
  return apiClient.PATCH("/roles/{roleId}/enabled", { params: { path: { roleId } }, body: { enabled } });
}

export function deleteRoleDefinition(roleId: string) {
  return apiClient.DELETE("/roles/{roleId}", { params: { path: { roleId } } });
}

export function getNotificationSummary() {
  return apiClient.GET("/notifications/summary");
}

export function listNotifications(query: { limit?: number; cursor?: string } = {}) {
  return apiClient.GET("/notifications", { params: { query } });
}

export function markAllNotificationsRead() {
  return apiClient.POST("/notifications/read-all");
}

export function markNotificationRead(notificationId: string) {
  return apiClient.POST("/notifications/{notificationId}/read", { params: { path: { notificationId } } });
}

export async function loginWithW3(account: string, password: string) {
  await apiClient.GET("/auth/csrf");

  return apiClient.POST("/auth/w3/login", {
    body: { account, password },
    headers: { "X-CSRF-Token": readCookie("tp_csrf") },
  });
}

export async function logout() {
  return apiClient.POST("/auth/logout", {
    headers: { "X-CSRF-Token": readCookie("tp_csrf") },
  });
}

function readCookie(name: string) {
  const prefix = `${name}=`;
  const match = document.cookie.split("; ").find((item) => item.startsWith(prefix));

  return match ? decodeURIComponent(match.slice(prefix.length)) : "";
}

function booleanQuery(value: boolean | undefined): "true" | "false" | undefined {
  if (value === undefined) {
    return undefined;
  }
  return value ? "true" : "false";
}
