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

  it("parses resumes and generates interview questions", async () => {
    const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ POST: post })) }));

    const { generateInterviewQuestions, parseResumeMatch } = await import("./client");
    await parseResumeMatch({ resumeId: "resume_1", positionId: "position_1" });
    await generateInterviewQuestions({ resumeId: "resume_1", positionId: "position_1", matchScore: 76 });

    expect(post).toHaveBeenCalledWith("/matching/parse", {
      body: { resumeId: "resume_1", positionId: "position_1" },
    });
    expect(post).toHaveBeenCalledWith("/matching/interview-questions", {
      body: { resumeId: "resume_1", positionId: "position_1", matchScore: 76 },
    });
  });

  it("lists creates updates and deletes departments", async () => {
    const del = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    const get = vi.fn().mockResolvedValue({ data: { items: [] }, error: undefined });
    const patch = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({
      createTalentPilotClient: vi.fn(() => ({ DELETE: del, GET: get, PATCH: patch, POST: post })),
    }));

    const { createDepartment, deleteDepartment, listDepartments, updateDepartment } = await import("./client");
    await listDepartments({ search: "算力", limit: 25 });
    await createDepartment({ name: "智算调度部" });
    await updateDepartment("dept_a", { name: "算力训练平台部" });
    await deleteDepartment("dept_a");

    expect(get).toHaveBeenCalledWith("/departments", { params: { query: { search: "算力", limit: 25 } } });
    expect(post).toHaveBeenCalledWith("/departments", { body: { name: "智算调度部" } });
    expect(patch).toHaveBeenCalledWith("/departments/{departmentId}", {
      params: { path: { departmentId: "dept_a" } },
      body: { name: "算力训练平台部" },
    });
    expect(del).toHaveBeenCalledWith("/departments/{departmentId}", { params: { path: { departmentId: "dept_a" } } });
  });

  it("lists creates updates and deletes positions", async () => {
    const del = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    const get = vi.fn().mockResolvedValue({ data: { items: [] }, error: undefined });
    const patch = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({
      createTalentPilotClient: vi.fn(() => ({ DELETE: del, GET: get, PATCH: patch, POST: post })),
    }));
    const body = {
      name: "平台工程师",
      departmentId: "dept_a",
      chan: "social" as const,
      level: "P6",
      status: "on" as const,
      duties: ["负责训练平台"],
      must: ["熟悉 Go"],
      keywords: ["Go"],
      implicitTags: [{ name: "系统设计", w: 40 }],
    };

    const { createPosition, deletePosition, listPositions, updatePosition } = await import("./client");
    await listPositions({ departmentId: "dept_a", chan: "social", status: "on", search: "Go", limit: 50 });
    await createPosition(body);
    await updatePosition("position_a", { ...body, status: "off" });
    await deletePosition("position_a");

    expect(get).toHaveBeenCalledWith("/positions", {
      params: { query: { departmentId: "dept_a", chan: "social", status: "on", search: "Go", limit: 50 } },
    });
    expect(post).toHaveBeenCalledWith("/positions", { body });
    expect(patch).toHaveBeenCalledWith("/positions/{positionId}", {
      params: { path: { positionId: "position_a" } },
      body: { ...body, status: "off" },
    });
    expect(del).toHaveBeenCalledWith("/positions/{positionId}", { params: { path: { positionId: "position_a" } } });
  });

  it("routes and sends recommendations", async () => {
    const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({ createTalentPilotClient: vi.fn(() => ({ POST: post })) }));

    const { routeRecommendation, sendRecommendation } = await import("./client");
    await routeRecommendation({ resumeId: "resume_1" });
    await sendRecommendation({ resumeId: "resume_1", departmentId: "dept_a", positionId: "position_a" });

    expect(post).toHaveBeenCalledWith("/recommendations/route", { body: { resumeId: "resume_1" } });
    expect(post).toHaveBeenCalledWith("/recommendations/send", {
      body: { resumeId: "resume_1", departmentId: "dept_a", positionId: "position_a" },
    });
  });

  it("manages users and role bindings", async () => {
    const del = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    const get = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({
      createTalentPilotClient: vi.fn(() => ({ DELETE: del, GET: get, POST: post })),
    }));

    const { assignUserRoleBindings, deleteUserRoleBinding, getUser, listAssignableRoles, listUsers } = await import(
      "./client"
    );
    await listUsers({ search: "张", limit: 25 });
    await getUser("user_a");
    await listAssignableRoles();
    await assignUserRoleBindings("user_a", { bindings: [{ departmentId: "dept_a", roleId: "__role_manager__" }] });
    await deleteUserRoleBinding("user_a", "udr_1");

    expect(get).toHaveBeenCalledWith("/users", { params: { query: { search: "张", limit: 25 } } });
    expect(get).toHaveBeenCalledWith("/users/{userId}", { params: { path: { userId: "user_a" } } });
    expect(get).toHaveBeenCalledWith("/roles/assignable");
    expect(post).toHaveBeenCalledWith("/users/{userId}/role-bindings", {
      params: { path: { userId: "user_a" } },
      body: { bindings: [{ departmentId: "dept_a", roleId: "__role_manager__" }] },
    });
    expect(del).toHaveBeenCalledWith("/users/{userId}/role-bindings/{bindingId}", {
      params: { path: { userId: "user_a", bindingId: "udr_1" } },
    });
  });

  it("reads and acknowledges notifications", async () => {
    const get = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    const post = vi.fn().mockResolvedValue({ data: undefined, error: undefined });
    vi.doMock("@talentpilot/api-client", () => ({
      createTalentPilotClient: vi.fn(() => ({ GET: get, POST: post })),
    }));

    const { getNotificationSummary, listNotifications, markAllNotificationsRead, markNotificationRead } = await import(
      "./client"
    );
    await getNotificationSummary();
    await listNotifications({ limit: 20 });
    await markAllNotificationsRead();
    await markNotificationRead("notification_1");

    expect(get).toHaveBeenCalledWith("/notifications/summary");
    expect(get).toHaveBeenCalledWith("/notifications", { params: { query: { limit: 20 } } });
    expect(post).toHaveBeenCalledWith("/notifications/read-all");
    expect(post).toHaveBeenCalledWith("/notifications/{notificationId}/read", {
      params: { path: { notificationId: "notification_1" } },
    });
  });
});
