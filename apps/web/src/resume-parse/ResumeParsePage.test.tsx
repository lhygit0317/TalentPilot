import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ResumeParsePage } from "./ResumeParsePage";

const apiMocks = vi.hoisted(() => ({
  generateInterviewQuestions: vi.fn(),
  getJob: vi.fn(),
  importResume: vi.fn(),
  listPositions: vi.fn(),
  listResumes: vi.fn(),
  parseResumeMatch: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const parseSession = {
  dataScope: {
    allDepartments: false,
    channels: ["social", "campus"],
    departments: [{ id: "dept_a", name: "算力训练平台部" }],
  },
  permissions: ["Resume.List", "Resume.Get", "Resume.Create", "DepartmentResume.Create", "Position.List", "Position.Get", "PositionResume.Create"],
};

const resumeList = {
  availableChannels: ["social", "campus"],
  channelCounts: { social: 1, campus: 0 },
  dataScopeSummary: "算力训练平台部",
  items: [
    {
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
    },
  ],
  nextCursor: "",
};

const positionList = {
  items: [
    {
      id: "position_1",
      name: "平台工程师",
      department: { id: "dept_a", name: "算力训练平台部" },
      chan: "social",
      level: "P6",
      status: "on",
      keywordCount: 2,
      implicitTagCount: 1,
      updatedAt: "2026-07-04T08:00:00Z",
      canGet: true,
      canUpdate: false,
      canDelete: false,
    },
  ],
};

const parseResult = {
  id: "position_resume_1",
  resume: {
    ...resumeList.items[0],
    traits: ["稳定"],
    expBase: 82,
  },
  position: {
    id: "position_1",
    name: "平台工程师",
    department: { id: "dept_a", name: "算力训练平台部" },
    chan: "social",
    level: "P6",
    status: "on",
    keywords: ["Go", "Kubernetes"],
    implicitTags: [{ name: "稳定", w: 40 }],
  },
  score: {
    total: 76,
    skill: 50,
    experience: 82,
    implicit: 100,
    judgement: "建议进入面试",
  },
  evidence: {
    keywords: [
      { name: "Go", matched: true },
      { name: "Kubernetes", matched: false },
    ],
    implicitTags: [{ name: "稳定", w: 40, matched: true }],
    analysis: "技能命中 1/2；隐性要求命中 1/1；建议进入面试。",
  },
  createdAt: "2026-07-07T08:00:00Z",
};

const questionResult = {
  groups: [
    {
      type: "professional",
      label: "专业面试",
      questions: [{ order: 1, question: "请介绍 Go 项目", why: "验证经验", difficulty: "核心" }],
    },
    {
      type: "manager",
      label: "主管面试",
      questions: [{ order: 1, question: "为什么选择算力训练平台部，以及你期待如何协作？", why: "确认动机", difficulty: "动机" }],
    },
    {
      type: "qualification",
      label: "资格面试",
      questions: [{ order: 1, question: "请确认到岗时间", why: "确认流程", difficulty: "流程" }],
    },
  ],
};

describe("ResumeParsePage", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.listResumes.mockReset();
    apiMocks.listResumes.mockResolvedValue({ data: resumeList, error: undefined });
    apiMocks.listPositions.mockReset();
    apiMocks.listPositions.mockResolvedValue({ data: positionList, error: undefined });
    apiMocks.parseResumeMatch.mockReset();
    apiMocks.parseResumeMatch.mockResolvedValue({ data: parseResult, error: undefined });
    apiMocks.generateInterviewQuestions.mockReset();
    apiMocks.generateInterviewQuestions.mockResolvedValue({ data: questionResult, error: undefined });
    apiMocks.importResume.mockReset();
    apiMocks.importResume.mockResolvedValue({ data: { id: "job_1", status: "pending" }, error: undefined });
    apiMocks.getJob.mockReset();
    apiMocks.getJob.mockResolvedValue({
      data: {
        id: "job_1",
        results: [{ fileName: "lisi.pdf", name: "李四", resumeId: "resume_2", status: "succeeded" }],
        status: "succeeded",
        summary: { failed: 0, succeeded: 1, total: 1 },
        type: "resume_import",
      },
      error: undefined,
    });
  });

  it("renders source selection, authorized channels, resumes, and active positions", async () => {
    render(<ResumeParsePage session={parseSession} />);

    expect(await screen.findByRole("button", { name: "从简历库选择" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "导入新简历" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "社招 SOCIAL" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "校招 CAMPUS" })).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: /张三/ })).toBeInTheDocument();
    expect(await screen.findByLabelText("目标岗位 JD")).toHaveDisplayValue("算力训练平台部 · 平台工程师(社招)");
    expect(apiMocks.listResumes).toHaveBeenCalledWith({ chan: "social" });
    expect(apiMocks.listPositions).toHaveBeenCalledWith({ chan: "social", status: "on" });
  });

  it("renders a parse result and generated interview questions", async () => {
    const user = userEvent.setup();
    render(<ResumeParsePage session={parseSession} />);

    await user.click(await screen.findByRole("button", { name: /张三/ }));
    await user.click(screen.getByRole("button", { name: "开始解析" }));

    expect(apiMocks.parseResumeMatch).toHaveBeenCalledWith({ resumeId: "resume_1", positionId: "position_1" });
    expect(await screen.findByText("建议进入面试")).toBeInTheDocument();
    expect(screen.getByText("技能匹配")).toBeInTheDocument();
    expect(screen.getByText("Kubernetes")).toBeInTheDocument();
    expect(screen.getByText("技能命中 1/2；隐性要求命中 1/1；建议进入面试。")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "生成面试题" }));

    expect(apiMocks.generateInterviewQuestions).toHaveBeenCalledWith({
      resumeId: "resume_1",
      positionId: "position_1",
      matchScore: 76,
    });
    expect(await screen.findByRole("tab", { name: "专业" })).toHaveAttribute("aria-selected", "true");
    await user.click(screen.getByRole("tab", { name: "主管" }));
    expect(await screen.findByText(/为什么选择算力训练平台部/)).toBeInTheDocument();
  });

  it("clears results when the selected channel changes", async () => {
    const user = userEvent.setup();
    render(<ResumeParsePage session={parseSession} />);

    await user.click(await screen.findByRole("button", { name: /张三/ }));
    await user.click(screen.getByRole("button", { name: "开始解析" }));
    expect(await screen.findByText("建议进入面试")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "校招 CAMPUS" }));

    expect(screen.queryByText("建议进入面试")).not.toBeInTheDocument();
    expect(apiMocks.listResumes).toHaveBeenLastCalledWith({ chan: "campus" });
    expect(apiMocks.listPositions).toHaveBeenLastCalledWith({ chan: "campus", status: "on" });
  });

  it("imports a new PDF in upload mode and selects the imported resume", async () => {
    const user = userEvent.setup();
    render(<ResumeParsePage session={parseSession} />);

    await user.click(await screen.findByRole("button", { name: "导入新简历" }));
    const upload = screen.getByLabelText("点击上传简历");
    const file = new File(["%PDF-1.7"], "lisi.pdf", { type: "application/pdf" });
    await user.upload(upload, file);

    expect(apiMocks.importResume).toHaveBeenCalled();
    const body = apiMocks.importResume.mock.calls[0][0] as FormData;
    expect(body.get("file")).toBe(file);
    expect(body.get("chan")).toBe("social");
    expect(body.get("targetDepartmentId")).toBe("dept_a");
    expect(await screen.findByRole("status")).toHaveTextContent("✓ 已导入「李四」并加入简历库");
    expect(screen.getByText("已选择: 李四")).toBeInTheDocument();
  });

  it("shows an empty position message when no active JD is available", async () => {
    apiMocks.listPositions.mockResolvedValue({ data: { items: [] }, error: undefined });

    render(<ResumeParsePage session={parseSession} />);

    expect(await screen.findByText("请先在「部门与岗位管理」中维护岗位")).toBeInTheDocument();
  });
});
