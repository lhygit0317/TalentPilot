import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { UsersPage } from "./UsersPage";

const apiMocks = vi.hoisted(() => ({
  assignUserRoleBindings: vi.fn(),
  deleteUserRoleBinding: vi.fn(),
  getUser: vi.fn(),
  listAssignableRoles: vi.fn(),
  listDepartments: vi.fn(),
  listUsers: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const writableSession = {
  dataScope: {
    allDepartments: false,
    channels: ["social", "campus"],
    departments: [{ id: "dept_a", name: "算力训练平台部" }],
  },
  permissions: ["User.List", "User.Get", "UserDepartmentRole.List", "UserDepartmentRole.Create", "UserDepartmentRole.Delete"],
};

const readOnlySession = {
  ...writableSession,
  permissions: ["User.List", "User.Get", "UserDepartmentRole.List"],
};

const usersResponse = {
  canAssignRoles: true,
  dataScopeSummary: "负责部门:算力训练平台部",
  items: [
    {
      id: "user_a",
      employeeId: "A10001",
      name: "张敏",
      departments: [{ id: "dept_a", name: "算力训练平台部", system: false }],
      roleBindings: [
        {
          id: "udr_a",
          role: { id: "__role_hrbp__", label: "HRBP", isSystem: true, enabled: true },
          department: { id: "dept_a", name: "算力训练平台部", system: false },
          guest: false,
          createdAt: "2026-07-12T08:00:00Z",
          createdBy: "admin",
          canDelete: true,
        },
      ],
      roleSummary: "HRBP(部门:算力训练平台部)",
      guestOnly: false,
      canAssign: true,
    },
    {
      id: "user_guest",
      employeeId: "A10002",
      name: "游客用户",
      departments: [],
      roleBindings: [
        {
          id: "udr_guest",
          role: { id: "__role_guest__", label: "游客", isSystem: true, enabled: true },
          department: { id: "__system__", name: "system", system: true },
          guest: true,
          createdAt: "2026-07-12T08:00:00Z",
          createdBy: "system",
          canDelete: false,
        },
      ],
      roleSummary: "游客(部门:system)",
      guestOnly: true,
      canAssign: true,
    },
  ],
  nextCursor: "",
};

const departmentsResponse = {
  items: [
    { id: "dept_a", name: "算力训练平台部", positionCount: 1, resumeCount: 2, updatedAt: "2026-07-12T08:00:00Z", canGet: true, canUpdate: false, canDelete: false },
    { id: "dept_b", name: "智算调度部", positionCount: 1, resumeCount: 0, updatedAt: "2026-07-12T08:00:00Z", canGet: true, canUpdate: false, canDelete: false },
  ],
};

const assignableRolesResponse = {
  items: [
    {
      id: "__role_hrbp__",
      label: "HRBP",
      description: "部门 HRBP",
      isSystem: true,
      supportsSystemDepartment: false,
      attributeConditionSummary: "",
    },
    {
      id: "__role_manager__",
      label: "主管",
      description: "业务主管",
      isSystem: true,
      supportsSystemDepartment: false,
      attributeConditionSummary: "",
    },
  ],
};

describe("UsersPage", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.assignUserRoleBindings.mockReset();
    apiMocks.assignUserRoleBindings.mockResolvedValue({
      data: { message: "已为 张敏 分配 2 个角色绑定", created: [], user: { id: "user_a", employeeId: "A10001", name: "张敏" } },
      error: undefined,
    });
    apiMocks.deleteUserRoleBinding.mockReset();
    apiMocks.deleteUserRoleBinding.mockResolvedValue({
      data: { deletedBindingId: "udr_a", userId: "user_a", message: "已解除 HRBP(部门:算力训练平台部)" },
      error: undefined,
    });
    apiMocks.getUser.mockReset();
    apiMocks.listAssignableRoles.mockReset();
    apiMocks.listAssignableRoles.mockResolvedValue({ data: assignableRolesResponse, error: undefined });
    apiMocks.listDepartments.mockReset();
    apiMocks.listDepartments.mockResolvedValue({ data: departmentsResponse, error: undefined });
    apiMocks.listUsers.mockReset();
    apiMocks.listUsers.mockResolvedValue({ data: usersResponse, error: undefined });
  });

  it("renders user columns and binding chips", async () => {
    render(<UsersPage session={writableSession} />);

    expect(await screen.findByRole("heading", { name: "用户管理" })).toBeInTheDocument();
    expect(screen.getByText("负责部门:算力训练平台部")).toBeInTheDocument();
    for (const header of ["姓名", "工号", "当前角色集合", "所属部门", "操作"]) {
      expect(screen.getByRole("columnheader", { name: header })).toBeInTheDocument();
    }
    expect(screen.getByRole("row", { name: /张敏/ })).toBeInTheDocument();
    expect(screen.getByText("HRBP")).toBeInTheDocument();
    expect(screen.getByText("游客")).toBeInTheDocument();
  });

  it("searches users and safely highlights visible matches", async () => {
    const user = userEvent.setup();
    render(<UsersPage session={writableSession} />);

    await screen.findByRole("row", { name: /张敏/ });
    await user.type(screen.getByPlaceholderText("搜索姓名、工号、部门或角色"), "张");

    await waitFor(() => expect(apiMocks.listUsers).toHaveBeenLastCalledWith({ search: "张" }));
    expect(screen.getByText("张").tagName).toBe("MARK");
  });

  it("shows read-only state without assignment action", async () => {
    apiMocks.listUsers.mockResolvedValue({ data: { ...usersResponse, canAssignRoles: false }, error: undefined });

    render(<UsersPage session={readOnlySession} />);

    expect(await screen.findByText("当前账号只能查看用户角色绑定")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "分配角色" })).not.toBeInTheDocument();
  });

  it("adds multiple pending bindings and saves once", async () => {
    const user = userEvent.setup();
    render(<UsersPage session={writableSession} />);

    const row = await screen.findByRole("row", { name: /张敏/ });
    await user.click(within(row).getByRole("button", { name: "分配角色" }));
    await user.selectOptions(await screen.findByLabelText("角色"), "__role_manager__");
    await user.selectOptions(screen.getByLabelText("部门"), "dept_b");
    await user.click(screen.getByRole("button", { name: "加入待分配" }));
    await user.selectOptions(screen.getByLabelText("角色"), "__role_hrbp__");
    await user.selectOptions(screen.getByLabelText("部门"), "dept_a");
    await user.click(screen.getByRole("button", { name: "加入待分配" }));
    await user.click(screen.getByRole("button", { name: "保存分配" }));

    expect(apiMocks.assignUserRoleBindings).toHaveBeenCalledTimes(1);
    expect(apiMocks.assignUserRoleBindings).toHaveBeenCalledWith("user_a", {
      bindings: [
        { departmentId: "dept_b", roleId: "__role_manager__" },
        { departmentId: "dept_a", roleId: "__role_hrbp__" },
      ],
    });
    expect(await screen.findByRole("status")).toHaveTextContent("已为 张敏 分配 2 个角色绑定");
  });

  it("blocks duplicate pending bindings before submit", async () => {
    const user = userEvent.setup();
    render(<UsersPage session={writableSession} />);

    const row = await screen.findByRole("row", { name: /张敏/ });
    await user.click(within(row).getByRole("button", { name: "分配角色" }));
    await user.click(await screen.findByRole("button", { name: "加入待分配" }));
    await user.click(screen.getByRole("button", { name: "加入待分配" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("待分配列表已有相同绑定");
    expect(apiMocks.assignUserRoleBindings).not.toHaveBeenCalled();
  });

  it("does not render remove action for guest bindings", async () => {
    render(<UsersPage session={writableSession} />);

    const row = await screen.findByRole("row", { name: /游客用户/ });

    expect(within(row).queryByRole("button", { name: "解除" })).not.toBeInTheDocument();
  });

  it("deletes a binding then refreshes the list", async () => {
    const user = userEvent.setup();
    render(<UsersPage session={writableSession} />);

    const row = await screen.findByRole("row", { name: /张敏/ });
    await user.click(within(row).getByRole("button", { name: "解除" }));

    expect(apiMocks.deleteUserRoleBinding).toHaveBeenCalledWith("user_a", "udr_a");
    await waitFor(() => expect(apiMocks.listUsers).toHaveBeenCalledTimes(2));
    expect(await screen.findByRole("status")).toHaveTextContent("已解除 HRBP");
  });
});
