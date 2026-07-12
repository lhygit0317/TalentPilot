import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const apiMocks = vi.hoisted(() => ({
  generateInterviewQuestions: vi.fn(),
  getDepartment: vi.fn(),
  getCurrentUser: vi.fn(),
  getJob: vi.fn(),
  getPosition: vi.fn(),
  getResume: vi.fn(),
  importResume: vi.fn(),
  assignUserRoleBindings: vi.fn(),
  deleteUserRoleBinding: vi.fn(),
  getUser: vi.fn(),
  getNotificationSummary: vi.fn(),
  listAssignableRoles: vi.fn(),
  listDepartments: vi.fn(),
  listNotifications: vi.fn(),
  listPositions: vi.fn(),
  listResumes: vi.fn(),
  listUsers: vi.fn(),
  loginWithW3: vi.fn(),
  logout: vi.fn(),
  markAllNotificationsRead: vi.fn(),
  markNotificationRead: vi.fn(),
  parseResumeMatch: vi.fn(),
  routeRecommendation: vi.fn(),
  sendRecommendation: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const guestSession = {
  defaultRoute: "/resume-parse",
  dataScope: { allDepartments: false, channels: [], departments: [] },
  pageAccess: ["resume-parse", "resume-recommend"],
  permissions: ["Department.List", "User.Get"],
  roleBindings: [
    {
      departmentId: "__system__",
      departmentName: "system",
      roleLabel: "游客",
    },
  ],
  roleLabels: ["游客"],
  user: {
    employeeId: "A12345",
    id: "w3-user-id",
    name: "张三",
  },
};

const hrbpSession = {
  ...guestSession,
  dataScope: {
    allDepartments: false,
    channels: ["social", "campus"],
    departments: [{ id: "dept_a", name: "算力训练平台部" }],
  },
  pageAccess: ["resume-parse", "resume-recommend", "resume-library", "departments-positions"],
  permissions: ["Resume.List", "Resume.Get", "Position.List", "PositionResume.Create"],
  roleBindings: [
    {
      departmentId: "dept_a",
      departmentName: "算力训练平台部",
      roleLabel: "HRBP",
    },
  ],
  roleLabels: ["HRBP"],
};

const userAdminSession = {
  ...hrbpSession,
  defaultRoute: "/users",
  pageAccess: ["users"],
  permissions: ["User.List", "User.Get", "UserDepartmentRole.List", "UserDepartmentRole.Create", "UserDepartmentRole.Delete"],
};

const notificationSession = {
  ...hrbpSession,
  permissions: [...hrbpSession.permissions, "Notification.List", "Notification.Get", "Notification.Update"],
};

async function renderLoginApp() {
  render(<App />);
  await screen.findByLabelText("公司账号");
}

describe("App", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.getCurrentUser.mockReset();
    apiMocks.getCurrentUser.mockResolvedValue({ data: undefined, error: undefined });
    apiMocks.listDepartments.mockReset();
    apiMocks.listDepartments.mockResolvedValue({
      data: {
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
      },
      error: undefined,
    });
    apiMocks.listPositions.mockReset();
    apiMocks.listPositions.mockResolvedValue({
      data: {
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
      },
      error: undefined,
    });
    apiMocks.listResumes.mockReset();
    apiMocks.listResumes.mockResolvedValue({
      data: {
        availableChannels: ["social"],
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
            keywords: ["Go"],
            canGet: true,
            canDelete: false,
          },
        ],
        nextCursor: "",
      },
      error: undefined,
    });
    apiMocks.parseResumeMatch.mockReset();
    apiMocks.routeRecommendation.mockReset();
    apiMocks.sendRecommendation.mockReset();
    apiMocks.generateInterviewQuestions.mockReset();
    apiMocks.assignUserRoleBindings.mockReset();
    apiMocks.deleteUserRoleBinding.mockReset();
    apiMocks.getUser.mockReset();
    apiMocks.listAssignableRoles.mockReset();
    apiMocks.listAssignableRoles.mockResolvedValue({ data: { items: [] }, error: undefined });
    apiMocks.listUsers.mockReset();
    apiMocks.listUsers.mockResolvedValue({
      data: {
        canAssignRoles: true,
        dataScopeSummary: "负责部门:算力训练平台部",
        items: [
          {
            id: "user_a",
            employeeId: "A10001",
            name: "张敏",
            departments: [{ id: "dept_a", name: "算力训练平台部", system: false }],
            roleBindings: [],
            roleSummary: "HRBP(部门:算力训练平台部)",
            guestOnly: false,
            canAssign: true,
          },
        ],
        nextCursor: "",
      },
      error: undefined,
    });
    apiMocks.importResume.mockReset();
    apiMocks.getJob.mockReset();
    apiMocks.getNotificationSummary.mockReset();
    apiMocks.getNotificationSummary.mockResolvedValue({ data: { unreadCount: 2 }, error: undefined });
    apiMocks.listNotifications.mockReset();
    apiMocks.listNotifications.mockResolvedValue({
      data: {
        items: [
          {
            id: "notification_1",
            resumeId: "resume_1",
            candidateName: "张三",
            department: { id: "dept_a", name: "智算调度部" },
            recommender: { id: "user_1", name: "李四" },
            chan: "social",
            createdAt: "2026-07-12T08:00:00Z",
            read: false,
            canOpenResumeLibrary: true,
          },
        ],
        unreadCount: 2,
        nextCursor: "",
      },
      error: undefined,
    });
    apiMocks.markAllNotificationsRead.mockReset();
    apiMocks.markAllNotificationsRead.mockResolvedValue({ data: { updatedCount: 2, unreadCount: 0 }, error: undefined });
    apiMocks.markNotificationRead.mockReset();
    apiMocks.markNotificationRead.mockResolvedValue({
      data: {
        notification: {
          id: "notification_1",
          resumeId: "resume_1",
          candidateName: "张三",
          department: { id: "dept_a", name: "智算调度部" },
          recommender: { id: "user_1", name: "李四" },
          chan: "social",
          createdAt: "2026-07-12T08:00:00Z",
          read: true,
          canOpenResumeLibrary: true,
        },
        unreadCount: 1,
      },
      error: undefined,
    });
    apiMocks.loginWithW3.mockReset();
    apiMocks.logout.mockReset();
    window.location.hash = "";
  });

  it("renders the company account login form for unauthenticated users", async () => {
    await renderLoginApp();

    expect(screen.getByLabelText("公司账号")).toBeInTheDocument();
    expect(screen.getByLabelText("公司密码")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "登录" })).toBeInTheDocument();
  });

  it("loads the current user session before showing the workspace", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({ data: guestSession, error: undefined });

    render(<App />);

    expect(apiMocks.getCurrentUser).toHaveBeenCalled();
    expect(await screen.findByText("A12345")).toBeInTheDocument();
    expect(screen.queryByLabelText("公司账号")).not.toBeInTheDocument();
  });

  it("shows the login form when current session loading throws", async () => {
    apiMocks.getCurrentUser.mockRejectedValue(new Error("session load failed"));

    render(<App />);

    expect(await screen.findByLabelText("公司账号")).toBeInTheDocument();
    expect(screen.getByLabelText("公司密码")).toBeInTheDocument();
  });

  it("submits W3 credentials and shows the signed-in user identity", async () => {
    apiMocks.loginWithW3.mockResolvedValue({ data: guestSession, error: undefined });

    const user = userEvent.setup();
    await renderLoginApp();

    await user.type(screen.getByLabelText("公司账号"), "zhangsan");
    await user.type(screen.getByLabelText("公司密码"), "secret");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(apiMocks.loginWithW3).toHaveBeenCalledWith("zhangsan", "secret");
    expect(await screen.findByText("A12345")).toBeInTheDocument();
    expect(screen.getByText("A12345")).toBeInTheDocument();
    expect(screen.getByText("游客")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("已通过 W3 登录 · 游客");
  });

  it("shows a loading state while login is pending", async () => {
    let resolveLogin: (value: unknown) => void = () => {};
    apiMocks.loginWithW3.mockReturnValue(
      new Promise((resolve) => {
        resolveLogin = resolve;
      }),
    );

    const user = userEvent.setup();
    await renderLoginApp();

    await user.type(screen.getByLabelText("公司账号"), "zhangsan");
    await user.type(screen.getByLabelText("公司密码"), "secret");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(screen.getByRole("button", { name: "登录中" })).toBeDisabled();

    resolveLogin({ data: guestSession, error: undefined });
    expect(await screen.findByText("A12345")).toBeInTheDocument();
  });

  it("shows a safe error message when login fails", async () => {
    apiMocks.loginWithW3.mockResolvedValue({ data: undefined, error: { code: "AUTH_W3_INVALID_CREDENTIALS" } });

    const user = userEvent.setup();
    await renderLoginApp();

    await user.type(screen.getByLabelText("公司账号"), "zhangsan");
    await user.type(screen.getByLabelText("公司密码"), "secret");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("登录失败，请检查账号和密码后重试。");
    expect(screen.getByLabelText("公司账号")).toBeInTheDocument();
  });

  it("recovers the login form when the login request throws", async () => {
    apiMocks.loginWithW3.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    await renderLoginApp();

    await user.type(screen.getByLabelText("公司账号"), "zhangsan");
    await user.type(screen.getByLabelText("公司密码"), "secret");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("登录失败，请检查账号和密码后重试。");
    expect(screen.getByRole("button", { name: "登录" })).toBeEnabled();
  });

  it("shows only guest navigation after login succeeds", async () => {
    apiMocks.loginWithW3.mockResolvedValue({ data: guestSession, error: undefined });

    const user = userEvent.setup();
    await renderLoginApp();

    await user.type(screen.getByLabelText("公司账号"), "zhangsan");
    await user.type(screen.getByLabelText("公司密码"), "secret");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(await screen.findByRole("link", { name: "简历解析" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "简历推荐" })).toBeInTheDocument();
    expect(screen.queryByText("您当前为游客身份")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "简历库" })).not.toBeInTheDocument();
  });

  it("shows IAM data scope summary in the workspace header", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({ data: hrbpSession, error: undefined });

    render(<App />);

    expect(await screen.findByText("算力训练平台部")).toBeInTheDocument();
    expect(screen.getByText("社招、校招")).toBeInTheDocument();
  });

  it("renders navigation from IAM page access", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({ data: hrbpSession, error: undefined });

    render(<App />);

    expect(await screen.findByRole("link", { name: "简历库" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "部门与岗位" })).toBeInTheDocument();
  });

  it("shows notification badges only for users with Notification.List", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({ data: notificationSession, error: undefined });

    render(<App />);

    expect(await screen.findByRole("button", { name: "推荐提醒" })).toBeInTheDocument();
    await waitFor(() => expect(apiMocks.getNotificationSummary).toHaveBeenCalled());
    expect((await screen.findAllByText("2")).length).toBeGreaterThanOrEqual(2);

    cleanup();
    apiMocks.getCurrentUser.mockResolvedValue({ data: hrbpSession, error: undefined });
    render(<App />);

    expect(await screen.findByRole("link", { name: "简历库" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "推荐提醒" })).not.toBeInTheDocument();
  });

  it("marks a notification read and jumps into the resume library", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({ data: notificationSession, error: undefined });
    const user = userEvent.setup();

    render(<App />);

    await user.click(await screen.findByRole("button", { name: "推荐提醒" }));
    await user.click(await screen.findByRole("button", { name: /张三 被推荐到/ }));

    expect(apiMocks.markNotificationRead).toHaveBeenCalledWith("notification_1");
    expect(await screen.findByRole("heading", { name: "简历库" })).toBeInTheDocument();
    expect(apiMocks.listResumes).toHaveBeenCalledWith({ chan: "social", search: "张三" });
    expect(await screen.findByText("有 1 份简历被推荐到你可查看的部门")).toBeInTheDocument();
  });

  it("renders the resume parse workspace when it is the active IAM route", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({ data: hrbpSession, error: undefined });

    render(<App />);

    expect(await screen.findByRole("heading", { name: "简历解析" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "从简历库选择" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "导入新简历" })).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: /张三/ })).toBeInTheDocument();
    expect(apiMocks.listResumes).toHaveBeenCalledWith({ chan: "social" });
    expect(apiMocks.listPositions).toHaveBeenCalledWith({ chan: "social", status: "on" });
  });

  it("renders the resume library page when it is the active IAM route", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({
      data: { ...hrbpSession, defaultRoute: "/resume-library" },
      error: undefined,
    });
    apiMocks.listResumes.mockResolvedValue({
      data: {
        availableChannels: ["social"],
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
            keywords: ["Go"],
            canGet: true,
            canDelete: false,
          },
        ],
        nextCursor: "",
      },
      error: undefined,
    });

    render(<App />);

    expect(await screen.findByRole("heading", { name: "简历库" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "候选人" })).toBeInTheDocument();
  });

  it("renders the resume recommendation page when it is the active IAM route", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({
      data: { ...hrbpSession, defaultRoute: "/resume-recommend" },
      error: undefined,
    });

    render(<App />);

    expect(await screen.findByRole("heading", { name: "简历推荐" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "智能分流" })).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: /张三/ })).toBeInTheDocument();
    expect(apiMocks.listResumes).toHaveBeenCalledWith({ chan: "social" });
  });

  it("renders the department position page when it is the active IAM route", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({
      data: { ...hrbpSession, defaultRoute: "/departments-positions" },
      error: undefined,
    });

    render(<App />);

    expect(await screen.findByRole("heading", { name: "部门与岗位" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "部门名称" })).toBeInTheDocument();
    expect(apiMocks.listDepartments).toHaveBeenCalled();
    expect(apiMocks.listPositions).toHaveBeenCalled();
  });

  it("renders the users page when it is the active IAM route", async () => {
    apiMocks.getCurrentUser.mockResolvedValue({ data: userAdminSession, error: undefined });

    render(<App />);

    expect(await screen.findByRole("heading", { name: "用户管理" })).toBeInTheDocument();
    expect(apiMocks.listUsers).toHaveBeenCalledWith({});
    expect(await screen.findByRole("row", { name: /张敏/ })).toBeInTheDocument();
  });

  it("returns to the login form after logout", async () => {
    apiMocks.loginWithW3.mockResolvedValue({ data: guestSession, error: undefined });
    apiMocks.logout.mockResolvedValue({ data: undefined, error: undefined });

    const user = userEvent.setup();
    await renderLoginApp();

    await user.type(screen.getByLabelText("公司账号"), "zhangsan");
    await user.type(screen.getByLabelText("公司密码"), "secret");
    await user.click(screen.getByRole("button", { name: "登录" }));
    await user.click(await screen.findByRole("button", { name: "退出登录" }));

    expect(apiMocks.logout).toHaveBeenCalled();
    expect(screen.getByLabelText("公司账号")).toBeInTheDocument();
    expect(screen.getByLabelText("公司密码")).toBeInTheDocument();
  });
});
