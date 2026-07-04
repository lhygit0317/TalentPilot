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
