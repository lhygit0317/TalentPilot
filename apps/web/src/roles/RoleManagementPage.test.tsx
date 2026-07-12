import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RoleManagementPage } from "./RoleManagementPage";

const apiMocks = vi.hoisted(() => ({
  createRoleDefinition: vi.fn(),
  deleteRoleDefinition: vi.fn(),
  getRole: vi.fn(),
  getRolePermissionOptions: vi.fn(),
  listRoles: vi.fn(),
  toggleRoleEnabled: vi.fn(),
  updateRoleDefinition: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const session = {
  permissions: [
    "Role.List",
    "Role.Get",
    "Role.Create",
    "Role.Update",
    "Role.Delete",
    "Permission.List",
    "Permission.Create",
    "Permission.Delete",
    "RoleRelation.Create",
    "RoleRelation.Delete",
  ],
};

const rolesResponse = {
  canCreate: true,
  total: 2,
  items: [
    {
      id: "__role_hrbp__",
      label: "HRBP",
      description: "部门 HRBP",
      isSystem: true,
      enabled: true,
      permissionCount: 3,
      childRoleCount: 0,
      referenceCount: 2,
      conditionSummary: "全部渠道",
      canEdit: true,
      canDelete: false,
      canToggleEnabled: true,
    },
    {
      id: "role_custom_reviewer",
      label: "高级评审者",
      description: "查看社招简历",
      isSystem: false,
      enabled: true,
      permissionCount: 1,
      childRoleCount: 1,
      referenceCount: 1,
      conditionSummary: "社招",
      canEdit: true,
      canDelete: false,
      canToggleEnabled: true,
    },
  ],
};

const permissionOptionsResponse = {
  conditionOptions: { chan: ["social", "campus"], expired: [false, true] },
  resources: [
    {
      resource: "Resume",
      actions: [
        { action: "List", supportsConditions: { chan: true, expired: true, self: false } },
        { action: "Get", supportsConditions: { chan: true, expired: true, self: false } },
      ],
    },
    {
      resource: "User",
      actions: [{ action: "List", supportsConditions: { chan: false, expired: false, self: false } }],
    },
  ],
};

describe("RoleManagementPage", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.createRoleDefinition.mockReset();
    apiMocks.createRoleDefinition.mockResolvedValue({
      data: { id: "role_custom_reviewer", label: "高级评审者" },
      error: undefined,
    });
    apiMocks.deleteRoleDefinition.mockReset();
    apiMocks.getRole.mockReset();
    apiMocks.getRolePermissionOptions.mockReset();
    apiMocks.getRolePermissionOptions.mockResolvedValue({ data: permissionOptionsResponse, error: undefined });
    apiMocks.listRoles.mockReset();
    apiMocks.listRoles.mockResolvedValue({ data: rolesResponse, error: undefined });
    apiMocks.toggleRoleEnabled.mockReset();
    apiMocks.toggleRoleEnabled.mockResolvedValue({
      data: { id: "__role_hrbp__", label: "HRBP", enabled: false },
      error: undefined,
    });
    apiMocks.updateRoleDefinition.mockReset();
  });

  it("renders role rows with status, counts, and delete disabled reason", async () => {
    render(<RoleManagementPage session={session} />);

    expect(await screen.findByRole("heading", { name: "角色管理" })).toBeInTheDocument();

    const systemRow = screen.getByRole("row", { name: /HRBP/ });
    expect(within(systemRow).getByText("系统")).toBeInTheDocument();
    expect(within(systemRow).getByText("启用")).toBeInTheDocument();
    expect(within(systemRow).getByText("3")).toBeInTheDocument();
    expect(within(systemRow).getByText("2")).toBeInTheDocument();

    const customRow = screen.getByRole("row", { name: /高级评审者/ });
    expect(within(customRow).getByText("自定义")).toBeInTheDocument();
    expect(within(customRow).getByText("社招")).toBeInTheDocument();
    expect(within(customRow).getByText("该角色被 1 个绑定引用")).toBeInTheDocument();
    expect(within(customRow).getByRole("button", { name: "删除" })).toBeDisabled();
  });

  it("builds permission matrix payload when creating a role", async () => {
    const user = userEvent.setup();
    render(<RoleManagementPage session={session} />);

    await user.click(await screen.findByRole("button", { name: "新建角色" }));
    await user.type(await screen.findByLabelText("角色名称"), "高级评审者");
    await user.type(screen.getByLabelText("角色描述"), "查看社招简历");
    await user.click(screen.getByLabelText("Resume List"));
    await user.click(screen.getByLabelText("社招"));
    await user.click(screen.getByLabelText("未过期"));
    await user.click(screen.getByLabelText("包含 HRBP"));
    await user.click(screen.getByRole("button", { name: "保存角色" }));

    await waitFor(() => expect(apiMocks.createRoleDefinition).toHaveBeenCalledTimes(1));
    expect(apiMocks.createRoleDefinition).toHaveBeenCalledWith({
      label: "高级评审者",
      description: "查看社招简历",
      enabled: true,
      permissions: [
        {
          resource: "Resume",
          action: "List",
          attributeConditions: { chan: ["social"], expired: [false] },
        },
      ],
      childRoleIds: ["__role_hrbp__"],
    });
  });

  it("shows confirmation before disabling a system role", async () => {
    const user = userEvent.setup();
    render(<RoleManagementPage session={session} />);

    const systemRow = await screen.findByRole("row", { name: /HRBP/ });
    await user.click(within(systemRow).getByRole("button", { name: "禁用" }));

    expect(apiMocks.toggleRoleEnabled).not.toHaveBeenCalled();
    expect(screen.getByText("禁用系统预置角色前请确认影响范围")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "确认禁用" }));

    await waitFor(() => expect(apiMocks.toggleRoleEnabled).toHaveBeenCalledWith("__role_hrbp__", false));
  });

  it("renders localized backend errors", async () => {
    apiMocks.createRoleDefinition.mockResolvedValueOnce({
      data: undefined,
      error: { code: "ROLE_LABEL_DUPLICATE" },
    });
    const user = userEvent.setup();
    render(<RoleManagementPage session={session} />);

    await user.click(await screen.findByRole("button", { name: "新建角色" }));
    await user.type(await screen.findByLabelText("角色名称"), "高级评审者");
    await user.click(screen.getByRole("button", { name: "保存角色" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("角色名称已存在");
  });
});
