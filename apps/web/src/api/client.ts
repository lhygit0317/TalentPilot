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
