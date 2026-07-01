import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const apiMocks = vi.hoisted(() => ({
  getCurrentUser: vi.fn(),
  loginWithW3: vi.fn(),
  logout: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const guestSession = {
  defaultRoute: "/resume-parse",
  pageAccess: ["resume-parse", "resume-recommend"],
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
    apiMocks.loginWithW3.mockReset();
    apiMocks.logout.mockReset();
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
    expect(await screen.findByText("张三")).toBeInTheDocument();
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
    expect(await screen.findByText("张三")).toBeInTheDocument();
    expect(screen.getByText("A12345")).toBeInTheDocument();
    expect(screen.getByText("游客")).toBeInTheDocument();
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
    expect(await screen.findByText("张三")).toBeInTheDocument();
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
