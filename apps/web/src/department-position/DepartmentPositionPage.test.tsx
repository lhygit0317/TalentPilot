import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DepartmentPositionPage } from "./DepartmentPositionPage";

const apiMocks = vi.hoisted(() => ({
  createDepartment: vi.fn(),
  createPosition: vi.fn(),
  deleteDepartment: vi.fn(),
  deletePosition: vi.fn(),
  getDepartment: vi.fn(),
  getPosition: vi.fn(),
  listDepartments: vi.fn(),
  listPositions: vi.fn(),
  updateDepartment: vi.fn(),
  updatePosition: vi.fn(),
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

const superAdminSession = {
  ...session,
  permissions: [
    "Department.List",
    "Department.Get",
    "Department.Create",
    "Department.Update",
    "Department.Delete",
    "Position.List",
    "Position.Get",
    "Position.Create",
    "Position.Update",
    "Position.Delete",
    "DepartmentPosition.List",
    "DepartmentPosition.Create",
    "DepartmentPosition.Delete",
  ],
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
      canUpdate: true,
      canDelete: true,
    },
    {
      id: "dept_b",
      name: "智算调度部",
      positionCount: 0,
      resumeCount: 0,
      updatedAt: "2026-07-04T08:00:00Z",
      canGet: true,
      canUpdate: true,
      canDelete: true,
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

const writablePositionList = {
  items: [{ ...positionList.items[0], canUpdate: true, canDelete: true }],
};

describe("DepartmentPositionPage", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.createDepartment.mockReset();
    apiMocks.createDepartment.mockResolvedValue({ data: { ...departmentList.items[1], positions: [] }, error: undefined });
    apiMocks.updateDepartment.mockReset();
    apiMocks.updateDepartment.mockResolvedValue({ data: { ...departmentList.items[0], name: "算力平台部", positions: [] }, error: undefined });
    apiMocks.deleteDepartment.mockReset();
    apiMocks.deleteDepartment.mockResolvedValue({ data: undefined, error: undefined });
    apiMocks.createPosition.mockReset();
    apiMocks.createPosition.mockResolvedValue({ data: { ...positionList.items[0], duties: [], must: [], keywords: [], implicitTags: [] }, error: undefined });
    apiMocks.updatePosition.mockReset();
    apiMocks.updatePosition.mockResolvedValue({ data: { ...positionList.items[0], duties: [], must: [], keywords: [], implicitTags: [] }, error: undefined });
    apiMocks.deletePosition.mockReset();
    apiMocks.deletePosition.mockResolvedValue({ data: undefined, error: undefined });
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

    const row = await screen.findByRole("row", { name: /算力训练平台部/ });
    await user.click(within(row).getByRole("button", { name: "查看" }));

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

  it("creates and updates a department as super admin", async () => {
    const user = userEvent.setup();
    render(<DepartmentPositionPage session={superAdminSession} />);

    await user.click(await screen.findByRole("button", { name: "新增部门" }));
    await user.type(screen.getByLabelText("部门名称"), "新能源计算部");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(apiMocks.createDepartment).toHaveBeenCalledWith({ name: "新能源计算部" });
    expect(await screen.findByRole("status")).toHaveTextContent("已新增部门");

    const row = screen.getByRole("row", { name: /算力训练平台部/ });
    await user.click(within(row).getByRole("button", { name: "编辑" }));
    await user.clear(screen.getByLabelText("部门名称"));
    await user.type(screen.getByLabelText("部门名称"), "算力平台部");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(apiMocks.updateDepartment).toHaveBeenCalledWith("dept_a", { name: "算力平台部" });
    expect(await screen.findByRole("status")).toHaveTextContent("已更新部门");
  });

  it("shows translated department delete protection errors", async () => {
    apiMocks.deleteDepartment.mockResolvedValue({ data: undefined, error: { code: "DEPARTMENT_DELETE_HAS_RELATIONS" } });
    const user = userEvent.setup();
    render(<DepartmentPositionPage session={superAdminSession} />);

    const row = await screen.findByRole("row", { name: /算力训练平台部/ });
    await user.click(within(row).getByRole("button", { name: "删除" }));

    expect(apiMocks.deleteDepartment).toHaveBeenCalledWith("dept_a");
    expect(await screen.findByRole("alert")).toHaveTextContent("部门仍有关联数据，不能删除");
  });

  it("creates and updates a position with department relation", async () => {
    apiMocks.listPositions.mockResolvedValue({ data: writablePositionList, error: undefined });
    const user = userEvent.setup();
    render(<DepartmentPositionPage session={superAdminSession} />);

    await user.click(await screen.findByRole("button", { name: "岗位管理" }));
    await user.click(screen.getByRole("button", { name: "新增岗位" }));
    await user.type(screen.getByLabelText("岗位名称"), "新平台工程师");
    await user.selectOptions(screen.getByLabelText("所属部门"), "dept_b");
    await user.selectOptions(screen.getByLabelText("渠道"), "social");
    await user.type(screen.getByLabelText("职级"), "P6");
    await user.selectOptions(screen.getByLabelText("状态"), "on");
    await user.type(screen.getByLabelText("岗位职责"), "负责训练平台");
    await user.type(screen.getByLabelText("硬性要求"), "熟悉 Go");
    await user.type(screen.getByLabelText("匹配关键词"), "Go,调度");
    await user.type(screen.getByLabelText("隐性标签"), "系统设计:40");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(apiMocks.createPosition).toHaveBeenCalledWith({
      name: "新平台工程师",
      departmentId: "dept_b",
      chan: "social",
      level: "P6",
      status: "on",
      duties: ["负责训练平台"],
      must: ["熟悉 Go"],
      keywords: ["Go", "调度"],
      implicitTags: [{ name: "系统设计", w: 40 }],
    });
    expect(await screen.findByRole("status")).toHaveTextContent("已新增岗位");

    const row = screen.getByRole("row", { name: /平台工程师/ });
    await user.click(within(row).getByRole("button", { name: "编辑" }));
    await user.clear(screen.getByLabelText("岗位名称"));
    await user.type(screen.getByLabelText("岗位名称"), "平台高级工程师");
    await user.selectOptions(screen.getByLabelText("所属部门"), "dept_b");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(apiMocks.updatePosition).toHaveBeenCalledWith(
      "position_a",
      expect.objectContaining({ departmentId: "dept_b", name: "平台高级工程师" }),
    );
    expect(await screen.findByRole("status")).toHaveTextContent("已更新岗位");
  });

  it("rejects duplicate keywords and implicit tags before submit", async () => {
    const user = userEvent.setup();
    render(<DepartmentPositionPage session={superAdminSession} />);

    await user.click(await screen.findByRole("button", { name: "岗位管理" }));
    await user.click(screen.getByRole("button", { name: "新增岗位" }));
    await user.type(screen.getByLabelText("岗位名称"), "重复校验岗位");
    await user.selectOptions(screen.getByLabelText("所属部门"), "dept_a");
    await user.selectOptions(screen.getByLabelText("渠道"), "social");
    await user.type(screen.getByLabelText("职级"), "P6");
    await user.selectOptions(screen.getByLabelText("状态"), "on");
    await user.type(screen.getByLabelText("匹配关键词"), "Go, go");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("岗位关键词不能重复");
    expect(apiMocks.createPosition).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText("匹配关键词"));
    await user.type(screen.getByLabelText("匹配关键词"), "Go,调度");
    await user.type(screen.getByLabelText("隐性标签"), "稳定性:40, 稳定性:50");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("隐性标签不能重复");
    expect(apiMocks.createPosition).not.toHaveBeenCalled();
  });

  it("toggles position on and off", async () => {
    apiMocks.listPositions.mockResolvedValue({ data: writablePositionList, error: undefined });
    const user = userEvent.setup();
    render(<DepartmentPositionPage session={superAdminSession} />);

    await user.click(await screen.findByRole("button", { name: "岗位管理" }));
    const row = await screen.findByRole("row", { name: /平台工程师/ });
    await user.click(within(row).getByRole("button", { name: "下架" }));

    expect(apiMocks.getPosition).toHaveBeenCalledWith("position_a");
    expect(apiMocks.updatePosition).toHaveBeenCalledWith("position_a", expect.objectContaining({ status: "off" }));
    expect(await screen.findByRole("status")).toHaveTextContent("岗位已下架");
  });

  it("deletes a safe position and refreshes the list", async () => {
    apiMocks.listPositions.mockResolvedValue({ data: writablePositionList, error: undefined });
    const user = userEvent.setup();
    render(<DepartmentPositionPage session={superAdminSession} />);

    await user.click(await screen.findByRole("button", { name: "岗位管理" }));
    const row = await screen.findByRole("row", { name: /平台工程师/ });
    await user.click(within(row).getByRole("button", { name: "删除" }));

    expect(apiMocks.deletePosition).toHaveBeenCalledWith("position_a");
    expect(await screen.findByRole("status")).toHaveTextContent("已删除岗位");
    expect(apiMocks.listPositions).toHaveBeenCalledTimes(2);
  });
});
