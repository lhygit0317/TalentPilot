import { afterEach, describe, expect, it, vi } from "vitest";

describe("api client", () => {
  afterEach(() => {
    vi.doUnmock("@talentpilot/api-client");
    vi.resetModules();
    vi.unstubAllEnvs();
  });

  it("uses the documented WEB_API_BASE_URL", async () => {
    const createTalentPilotClient = vi.fn(() => ({
      DELETE: vi.fn(),
      GET: vi.fn(),
      POST: vi.fn(),
    }));
    vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient }));
    vi.stubEnv("WEB_API_BASE_URL", "http://localhost:8080");
    vi.stubEnv("VITE_API_BASE_URL", "");

    await import("./client");

    expect(createTalentPilotClient).toHaveBeenCalledWith("http://localhost:8080");
  });

  it("lists resumes with channel and search parameters", async () => {
    const get = vi.fn().mockResolvedValue({ data: { items: [] }, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ GET: get })) }));

    const { listResumes } = await import("./client");
    await listResumes({ chan: "social", search: "Go" });

    expect(get).toHaveBeenCalledWith("/resumes", { params: { query: { chan: "social", search: "Go" } } });
  });

  it("gets and deletes a resume by id", async () => {
    const get = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    const del = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ DELETE: del, GET: get })) }));

    const { deleteResume, getResume } = await import("./client");
    await getResume("resume_1");
    await deleteResume("resume_1");

    expect(get).toHaveBeenCalledWith("/resumes/{resumeId}", { params: { path: { resumeId: "resume_1" } } });
    expect(del).toHaveBeenCalledWith("/resumes/{resumeId}", { params: { path: { resumeId: "resume_1" } } });
  });

  it("imports single and batch resumes with form data", async () => {
    const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ POST: post })) }));
    const single = new FormData();
    const batch = new FormData();

    const { importResume, importResumesBatch } = await import("./client");
    await importResume(single);
    await importResumesBatch(batch);

    expect(post).toHaveBeenCalledWith("/resumes/imports", { body: single });
    expect(post).toHaveBeenCalledWith("/resumes/batch-imports", { body: batch });
  });

  it("gets an import job by id", async () => {
    const get = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ GET: get })) }));

    const { getJob } = await import("./client");
    await getJob("job_1");

    expect(get).toHaveBeenCalledWith("/jobs/{jobId}", { params: { path: { jobId: "job_1" } } });
  });
});
