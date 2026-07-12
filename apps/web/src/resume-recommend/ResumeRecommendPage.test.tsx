import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ResumeRecommendPage } from "./ResumeRecommendPage";

const apiMocks = vi.hoisted(() => ({
  getJob: vi.fn(),
  importResume: vi.fn(),
  listResumes: vi.fn(),
  routeRecommendation: vi.fn(),
  sendRecommendation: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const recommendSession = {
  dataScope: {
    allDepartments: false,
    channels: ["social", "campus"],
    departments: [{ id: "dept_source", name: "来源部门" }],
  },
  permissions: ["Resume.List", "Resume.Get", "Resume.Create", "DepartmentResume.Create", "PositionResume.Create", "Notification.Create"],
};

const resumeList = {
  availableChannels: ["social"],
  channelCounts: { social: 1, campus: 0 },
  dataScopeSummary: "来源部门",
  items: [
    {
      id: "resume_1",
      name: "张三",
      chan: "social",
      pos: "平台工程师",
      currentDepartment: { id: "dept_source", name: "来源部门" },
      keywords: ["Go"],
      source: "导入",
      sourceBy: "李四",
      school: "浙江大学",
      canGet: true,
      canDelete: false,
    },
  ],
  nextCursor: "",
};

const routeResult = {
  resume: {
    id: "resume_1",
    name: "张三",
    chan: "social",
    pos: "平台工程师",
    currentDepartment: { id: "dept_source", name: "来源部门" },
    keywords: ["Go"],
  },
  routes: [
    {
      department: { id: "dept_a", name: "智算调度部" },
      position: { id: "position_a", name: "调度工程师", chan: "social", level: "P6" },
      score: { total: 86, skill: 100, experience: 82, implicit: 80, judgement: "强烈推荐" },
      contacts: { hrbps: ["李四"], managers: ["王五"], trainees: ["赵六"] },
      best: true,
    },
  ],
  createdAt: "2026-07-12T08:00:00Z",
};

describe("ResumeRecommendPage", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.listResumes.mockReset();
    apiMocks.listResumes.mockResolvedValue({ data: resumeList, error: undefined });
    apiMocks.routeRecommendation.mockReset();
    apiMocks.routeRecommendation.mockResolvedValue({ data: routeResult, error: undefined });
    apiMocks.sendRecommendation.mockReset();
    apiMocks.sendRecommendation.mockResolvedValue({
      data: {
        resumeId: "resume_copy",
        sourceResumeId: "resume_1",
        department: { id: "dept_a", name: "智算调度部" },
        position: { id: "position_a", name: "调度工程师" },
        candidateName: "张三",
        reusedExistingCopy: false,
        notifiedCount: 4,
        selfNotificationRead: true,
        message: "已推荐到「智算调度部」· 已通知 4 位相关人员",
      },
      error: undefined,
    });
    apiMocks.importResume.mockReset();
    apiMocks.getJob.mockReset();
  });

  it("routes a selected resume and sends the best recommendation", async () => {
    const user = userEvent.setup();
    render(<ResumeRecommendPage session={recommendSession} />);

    expect(await screen.findByText("分流结果将显示在这里")).toBeInTheDocument();
    await user.click(await screen.findByRole("button", { name: /张三/ }));
    await user.click(screen.getByRole("button", { name: "智能分流" }));

    expect(apiMocks.routeRecommendation).toHaveBeenCalledWith({ resumeId: "resume_1" });
    expect(await screen.findByText("最佳去向")).toBeInTheDocument();
    expect(screen.getByText("智算调度部")).toBeInTheDocument();
    expect(screen.getByText("李四")).toBeInTheDocument();
    expect(screen.getByText("王五")).toBeInTheDocument();
    expect(screen.getByText("赵六")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "推荐到" }));

    expect(apiMocks.sendRecommendation).toHaveBeenCalledWith({
      resumeId: "resume_1",
      departmentId: "dept_a",
      positionId: "position_a",
    });
    expect(await screen.findByRole("status")).toHaveTextContent("已推荐到「智算调度部」· 已通知 4 位相关人员");
  });

  it("clears routing results when the channel changes", async () => {
    const user = userEvent.setup();
    render(<ResumeRecommendPage session={recommendSession} />);

    await user.click(await screen.findByRole("button", { name: /张三/ }));
    await user.click(screen.getByRole("button", { name: "智能分流" }));
    expect(await screen.findByText("最佳去向")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "校招 CAMPUS" }));

    expect(screen.queryByText("最佳去向")).not.toBeInTheDocument();
    expect(screen.getByText("分流结果将显示在这里")).toBeInTheDocument();
    expect(apiMocks.listResumes).toHaveBeenLastCalledWith({ chan: "campus" });
  });

  it("shows the no-route empty state", async () => {
    apiMocks.routeRecommendation.mockResolvedValue({ data: { ...routeResult, routes: [] }, error: undefined });
    const user = userEvent.setup();
    render(<ResumeRecommendPage session={recommendSession} />);

    await user.click(await screen.findByRole("button", { name: /张三/ }));
    await user.click(screen.getByRole("button", { name: "智能分流" }));

    expect(await screen.findByText("该渠道下暂无在架岗位")).toBeInTheDocument();
  });
});
