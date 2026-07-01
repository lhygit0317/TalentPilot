import { afterEach, describe, expect, it, vi } from "vitest";

describe("api client", () => {
  afterEach(() => {
    vi.doUnmock("@talentpilot/api-client");
    vi.resetModules();
    vi.unstubAllEnvs();
  });

  it("uses the documented WEB_API_BASE_URL", async () => {
    const createTalentPilotClient = vi.fn(() => ({
      GET: vi.fn(),
      POST: vi.fn(),
    }));
    vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient }));
    vi.stubEnv("WEB_API_BASE_URL", "http://localhost:8080");
    vi.stubEnv("VITE_API_BASE_URL", "");

    await import("./client");

    expect(createTalentPilotClient).toHaveBeenCalledWith("http://localhost:8080");
  });
});
