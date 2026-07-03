import { createTalentPilotClient } from "@talentpilot/api-client";

export const apiClient = createTalentPilotClient(
  import.meta.env.WEB_API_BASE_URL || import.meta.env.VITE_API_BASE_URL || "",
);

export async function getCurrentUser() {
  return apiClient.GET("/me");
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
