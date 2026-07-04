import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DepartmentPositionPage } from "./DepartmentPositionPage";

const apiMocks = vi.hoisted(() => ({
  getDepartment: vi.fn(),
  getPosition: vi.fn(),
  listDepartments: vi.fn(),
  listPositions: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const session = {
  dataScope: {
    allDepartments: false,
    channels: ["social", "campus"],
    departments: [{ id: "dept_a", name: "算力训练平台部" }],
  },
  permissions: ["Department.List", "Department.Get", "Position.List", "Position.Get", "DepartmentPosition.List"],
};

const departmentList = {
  items: [
    {
      id: "dept_a",
      name: "算力训练平台部",
      positionCount: 1,
      resumeCount: 12,
      updatedAt: "2026-07-04T08:00:00Z",
      canGet: true,
      canUpdate: false,
      canDelete: false,
    },
  ],
};

const positionList = {
  items: [
    {
      id: "position_a",
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

describe("DepartmentPositionPage", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.listDepartments.mockReset();
    apiMocks.listDepartments.mockResolvedValue({ data: departmentList, error: undefined });
    apiMocks.getDepartment.mockReset();
    apiMocks.getDepartment.mockResolvedValue({
      data: {
        ...departmentList.items[0],
        positions: [{ id: "position_a", name: "平台工程师", chan: "social", level: "P6", status: "on" }],
      },
      error: undefined,
    });
    apiMocks.listPositions.mockReset();
    apiMocks.listPositions.mockResolvedValue({ data: positionList, error: undefined });
    apiMocks.getPosition.mockReset();
    apiMocks.getPosition.mockResolvedValue({
      data: {
        ...positionList.items[0],
        duties: ["负责训练平台服务端研发"],
        must: ["熟悉 Go"],
        keywords: ["Go", "调度"],
        implicitTags: [{ name: "系统设计", w: 40 }],
      },
      error: undefined,
    });
  });

  it("shows department columns and read-only view action", async () => {
    render(<DepartmentPositionPage session={session} />);

    expect(await screen.findByRole("heading", { name: "部门与岗位" })).toBeInTheDocument();
    for (const header of ["部门名称", "关联岗位", "关联简历", "更新时间", "操作"]) {
      expect(screen.getByRole("columnheader", { name: header })).toBeInTheDocument();
    }
    const row = screen.getByRole("row", { name: /算力训练平台部/ });
    expect(within(row).getByRole("button", { name: "查看" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "新增部门" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
  });

  it("opens department detail without staff lists", async () => {
    const user = userEvent.setup();
    render(<DepartmentPositionPage session={session} />);

    await user.click(await screen.findByRole("button", { name: "查看" }));

    expect(apiMocks.getDepartment).toHaveBeenCalledWith("dept_a");
    expect(await screen.findByRole("heading", { name: "算力训练平台部" })).toBeInTheDocument();
    expect(screen.getByText("平台工程师")).toBeInTheDocument();
    expect(screen.queryByText("HRBP")).not.toBeInTheDocument();
    expect(screen.queryByText("主管")).not.toBeInTheDocument();
    expect(screen.queryByText("锻炼干部")).not.toBeInTheDocument();
  });

  it("shows position filters list and JD detail", async () => {
    const user = userEvent.setup();
    render(<DepartmentPositionPage session={session} />);

    await user.click(await screen.findByRole("button", { name: "岗位管理" }));

    expect(screen.getByLabelText("部门")).toBeInTheDocument();
    expect(screen.getByLabelText("渠道")).toBeInTheDocument();
    expect(screen.getByLabelText("状态")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("搜索岗位或关键词")).toBeInTheDocument();
    expect(await screen.findByRole("row", { name: /平台工程师/ })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "查看" }));

    expect(apiMocks.getPosition).toHaveBeenCalledWith("position_a");
    expect(await screen.findByRole("heading", { name: "平台工程师" })).toBeInTheDocument();
    expect(screen.getByText("负责训练平台服务端研发")).toBeInTheDocument();
    expect(screen.getByText("熟悉 Go")).toBeInTheDocument();
    expect(screen.getByText("Go")).toBeInTheDocument();
    expect(screen.getByText("系统设计 · 40")).toBeInTheDocument();
  });

  it("hides write actions for non-super-admin users", async () => {
    render(<DepartmentPositionPage session={session} />);

    expect(await screen.findByRole("heading", { name: "部门与岗位" })).toBeInTheDocument();
    for (const action of ["新增部门", "新增岗位", "编辑", "删除", "上架", "下架"]) {
      expect(screen.queryByRole("button", { name: action })).not.toBeInTheDocument();
    }
  });
});
