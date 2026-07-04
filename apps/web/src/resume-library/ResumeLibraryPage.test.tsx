import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ResumeLibraryPage } from "./ResumeLibraryPage";

const apiMocks = vi.hoisted(() => ({
  getResume: vi.fn(),
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
});
