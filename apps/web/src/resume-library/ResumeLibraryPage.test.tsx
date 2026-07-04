import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ResumeLibraryPage } from "./ResumeLibraryPage";

const apiMocks = vi.hoisted(() => ({
  deleteResume: vi.fn(),
  getResume: vi.fn(),
  getJob: vi.fn(),
  importResume: vi.fn(),
  importResumesBatch: vi.fn(),
  listResumes: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const session = {
  dataScope: {
    allDepartments: false,
    channels: ["social", "campus"],
    departments: [{ id: "dept_a", name: "算力训练平台部" }],
  },
  permissions: ["Resume.List", "Resume.Get"],
};

const importSession = {
  ...session,
  permissions: ["Resume.List", "Resume.Get", "Resume.Create", "DepartmentResume.Create", "Resume.Delete"],
};

const listResponse = {
  availableChannels: ["social", "campus"],
  channelCounts: { social: 2, campus: 1 },
  dataScopeSummary: "算力训练平台部",
  items: [
    {
      id: "resume_1",
      name: "张三 C++",
      age: 29,
      school: "浙江大学",
      yearsExp: 6,
      currentDepartment: { id: "dept_a", name: "算力训练平台部" },
      pos: "平台工程师",
      source: "导入",
      sourceBy: "李四",
      chan: "social",
      keywords: ["Go", "C++"],
      canGet: true,
      canDelete: false,
    },
  ],
  nextCursor: "",
};

describe("ResumeLibraryPage", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.deleteResume.mockReset();
    apiMocks.deleteResume.mockResolvedValue({ data: undefined, error: undefined });
    apiMocks.getResume.mockReset();
    apiMocks.getResume.mockResolvedValue({
      data: {
        ...listResponse.items[0],
        createdAt: "2026-07-04T08:00:00Z",
        expired: false,
        profile: {
          basic: {},
          certificates: [],
          education: [],
          projects: [],
          rawTextRef: "",
          skills: [],
          workExperience: [],
        },
      },
      error: undefined,
    });
    apiMocks.getJob.mockReset();
    apiMocks.getJob.mockResolvedValue({
      data: {
        id: "job_1",
        results: [{ fileName: "zhangsan.pdf", name: "张三", resumeId: "resume_1", status: "succeeded" }],
        status: "succeeded",
        summary: { failed: 0, succeeded: 1, total: 1 },
        type: "resume_import",
      },
      error: undefined,
    });
    apiMocks.importResume.mockReset();
    apiMocks.importResume.mockResolvedValue({ data: { jobId: "job_1", status: "pending" }, error: undefined });
    apiMocks.importResumesBatch.mockReset();
    apiMocks.importResumesBatch.mockResolvedValue({ data: { jobId: "job_1", status: "pending" }, error: undefined });
    apiMocks.listResumes.mockReset();
    apiMocks.listResumes.mockResolvedValue({ data: listResponse, error: undefined });
  });

  it("shows channel counts and the data scope banner", async () => {
    render(<ResumeLibraryPage session={session} />);

    expect(await screen.findByRole("button", { name: "社招 2" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "校招 1" })).toBeInTheDocument();
    expect(screen.getByText("算力训练平台部")).toBeInTheDocument();
  });

  it("renders table columns without an avatar", async () => {
    render(<ResumeLibraryPage session={session} />);

    expect(await screen.findByRole("columnheader", { name: "候选人" })).toBeInTheDocument();
    for (const header of ["年龄", "学校", "工作年限", "当前部门", "意向岗位", "来源", "关键词", "操作"]) {
      expect(screen.getByRole("columnheader", { name: header })).toBeInTheDocument();
    }
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("preserves search focus and highlights escaped literal matches", async () => {
    const user = userEvent.setup();
    render(<ResumeLibraryPage session={session} />);

    const search = await screen.findByPlaceholderText("搜索姓名、岗位或关键词");
    await user.type(search, "C++");

    expect(search).toHaveFocus();
    expect(screen.getByText("C++", { selector: "mark" })).toBeInTheDocument();
  });

  it("opens detail and shows 未解析到 for empty sections", async () => {
    const user = userEvent.setup();
    render(<ResumeLibraryPage session={session} />);

    await user.click(await screen.findByRole("button", { name: "查看详情" }));

    expect(apiMocks.getResume).toHaveBeenCalledWith("resume_1");
    expect(await screen.findByRole("heading", { name: "张三 C++" })).toBeInTheDocument();
    expect(screen.getAllByText("未解析到").length).toBeGreaterThan(0);
  });

  it("hides delete action when canDelete is false", async () => {
    render(<ResumeLibraryPage session={session} />);

    const row = await screen.findByRole("row", { name: /张三 C\+\+/ });
    expect(within(row).queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
  });

  it("rejects non-PDF and files over 10 MB before upload", async () => {
    const user = userEvent.setup();
    render(<ResumeLibraryPage session={importSession} />);

    const singleImport = (await screen.findByLabelText("单份导入")) as HTMLInputElement;
    await user.upload(singleImport, new File(["plain text"], "resume.txt", { type: "text/plain" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("仅支持 PDF 文件");
    expect(apiMocks.importResume).not.toHaveBeenCalled();

    const largePDF = new File([new Uint8Array(10 * 1024 * 1024 + 1)], "large.pdf", {
      type: "application/pdf",
    });
    await user.upload(singleImport, largePDF);

    expect(await screen.findByRole("alert")).toHaveTextContent("文件不能超过 10MB");
    expect(apiMocks.importResume).not.toHaveBeenCalled();
  });

  it("submits single import form data and shows imported toast after job success", async () => {
    const user = userEvent.setup();
    render(<ResumeLibraryPage session={importSession} />);

    const file = new File(["%PDF-1.7"], "zhangsan.pdf", { type: "application/pdf" });
    await user.upload(await screen.findByLabelText("单份导入"), file);

    expect(apiMocks.importResume).toHaveBeenCalled();
    const body = apiMocks.importResume.mock.calls[0][0] as FormData;
    expect(body.get("file")).toBe(file);
    expect(body.get("chan")).toBe("social");
    expect(body.get("targetDepartmentId")).toBe("dept_a");
    expect(apiMocks.getJob).toHaveBeenCalledWith("job_1");
    expect(await screen.findByRole("status")).toHaveTextContent("✓ 已导入「张三」并加入简历库");
  });

  it("submits batch import form data and shows success count toast", async () => {
    apiMocks.getJob.mockResolvedValue({
      data: {
        id: "job_1",
        results: [
          { fileName: "a.pdf", name: "张三", resumeId: "resume_1", status: "succeeded" },
          { fileName: "b.pdf", name: "李四", resumeId: "resume_2", status: "succeeded" },
        ],
        status: "succeeded",
        summary: { failed: 0, succeeded: 2, total: 2 },
        type: "resume_import",
      },
      error: undefined,
    });
    const user = userEvent.setup();
    render(<ResumeLibraryPage session={importSession} />);

    const first = new File(["%PDF-1.7"], "a.pdf", { type: "application/pdf" });
    const second = new File(["%PDF-1.7"], "b.pdf", { type: "application/pdf" });
    await user.upload(await screen.findByLabelText("批量导入简历"), [first, second]);

    expect(apiMocks.importResumesBatch).toHaveBeenCalled();
    const body = apiMocks.importResumesBatch.mock.calls[0][0] as FormData;
    expect(body.getAll("files")).toEqual([first, second]);
    expect(body.get("chan")).toBe("social");
    expect(body.get("targetDepartmentId")).toBe("dept_a");
    expect(await screen.findByRole("status")).toHaveTextContent("已批量导入 2 份社招简历");
  });

  it("deletes a deletable resume and refreshes the list", async () => {
    apiMocks.listResumes.mockResolvedValue({
      data: {
        ...listResponse,
        items: [{ ...listResponse.items[0], canDelete: true }],
      },
      error: undefined,
    });
    const user = userEvent.setup();
    render(<ResumeLibraryPage session={importSession} />);

    await user.click(await screen.findByRole("button", { name: "删除" }));

    expect(apiMocks.deleteResume).toHaveBeenCalledWith("resume_1");
    expect(await screen.findByRole("status")).toHaveTextContent("已删除该简历");
    expect(apiMocks.listResumes).toHaveBeenCalledTimes(2);
  });

  it("shows stable translated errors for import failures", async () => {
    apiMocks.getJob.mockResolvedValue({
      data: {
        id: "job_1",
        results: [{ errorCode: "RESUME_IMPORT_PARSE_FAILED", fileName: "broken.pdf", status: "failed" }],
        status: "failed",
        summary: { failed: 1, succeeded: 0, total: 1 },
        type: "resume_import",
      },
      error: undefined,
    });
    const user = userEvent.setup();
    render(<ResumeLibraryPage session={importSession} />);

    await user.upload(await screen.findByLabelText("单份导入"), new File(["%PDF-1.7"], "broken.pdf", {
      type: "application/pdf",
    }));

    expect(await screen.findByRole("alert")).toHaveTextContent("简历解析失败，请重新上传 PDF。");
  });
});
