import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NotificationBell } from "./NotificationBell";

const apiMocks = vi.hoisted(() => ({
  getNotificationSummary: vi.fn(),
  listNotifications: vi.fn(),
  markAllNotificationsRead: vi.fn(),
  markNotificationRead: vi.fn(),
}));

vi.mock("../api/client", () => apiMocks);

const notification = {
  id: "notification_1",
  resumeId: "resume_1",
  candidateName: "张三",
  department: { id: "dept_a", name: "智算调度部" },
  position: { id: "position_1", name: "平台工程师" },
  recommender: { id: "user_1", name: "李四" },
  chan: "social",
  createdAt: "2026-07-12T08:00:00Z",
  read: false,
  canOpenResumeLibrary: true,
};

describe("NotificationBell", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMocks.getNotificationSummary.mockReset();
    apiMocks.getNotificationSummary.mockResolvedValue({ data: { unreadCount: 0 }, error: undefined });
    apiMocks.listNotifications.mockReset();
    apiMocks.listNotifications.mockResolvedValue({
      data: { items: [notification], unreadCount: 2, nextCursor: "" },
      error: undefined,
    });
    apiMocks.markAllNotificationsRead.mockReset();
    apiMocks.markAllNotificationsRead.mockResolvedValue({ data: { updatedCount: 2, unreadCount: 0 }, error: undefined });
    apiMocks.markNotificationRead.mockReset();
    apiMocks.markNotificationRead.mockResolvedValue({
      data: { notification: { ...notification, read: true }, unreadCount: 1 },
      error: undefined,
    });
  });

  it("hides the badge when unread count is zero", () => {
    render(<NotificationBell canUpdate onOpenResume={vi.fn()} onUnreadCountChange={vi.fn()} unreadCount={0} />);

    expect(screen.getByRole("button", { name: "推荐提醒" })).toBeInTheDocument();
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it("shows unread count and loads unread notifications on open", async () => {
    const user = userEvent.setup();
    render(<NotificationBell canUpdate onOpenResume={vi.fn()} onUnreadCountChange={vi.fn()} unreadCount={2} />);

    expect(screen.getByText("2")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "推荐提醒" }));

    expect(apiMocks.listNotifications).toHaveBeenCalledWith({ limit: 20 });
    expect(await screen.findByText("推荐提醒(2 条未读)")).toBeInTheDocument();
    expect(screen.getByText("张三 被推荐到「智算调度部」")).toBeInTheDocument();
    expect(screen.getByText(/李四/)).toBeInTheDocument();
  });

  it("marks all notifications read", async () => {
    const onUnreadCountChange = vi.fn();
    const user = userEvent.setup();
    render(<NotificationBell canUpdate onOpenResume={vi.fn()} onUnreadCountChange={onUnreadCountChange} unreadCount={2} />);

    await user.click(screen.getByRole("button", { name: "推荐提醒" }));
    await user.click(await screen.findByRole("button", { name: "全部已读" }));

    expect(apiMocks.markAllNotificationsRead).toHaveBeenCalled();
    expect(onUnreadCountChange).toHaveBeenCalledWith(0);
    expect(await screen.findByRole("status")).toHaveTextContent("已全部标记为已读");
    expect(screen.getByText("暂无新的推荐提醒")).toBeInTheDocument();
  });

  it("clicks one notification and returns jump context", async () => {
    const onOpenResume = vi.fn();
    const onUnreadCountChange = vi.fn();
    const user = userEvent.setup();
    render(<NotificationBell canUpdate onOpenResume={onOpenResume} onUnreadCountChange={onUnreadCountChange} unreadCount={2} />);

    await user.click(screen.getByRole("button", { name: "推荐提醒" }));
    const item = await screen.findByRole("button", { name: /张三 被推荐到/ });
    await user.click(item);

    expect(apiMocks.markNotificationRead).toHaveBeenCalledWith("notification_1");
    expect(onUnreadCountChange).toHaveBeenCalledWith(1);
    expect(onOpenResume).toHaveBeenCalledWith({
      items: [
        {
          chan: "social",
          resumeId: "resume_1",
          candidateName: "张三",
          department: { id: "dept_a", name: "智算调度部" },
          recommender: { id: "user_1", name: "李四" },
        },
      ],
    });
    expect(screen.queryByText("推荐提醒(2 条未读)")).not.toBeInTheDocument();
  });

  it("hides read actions without Notification.Update", async () => {
    const user = userEvent.setup();
    render(<NotificationBell canUpdate={false} onOpenResume={vi.fn()} onUnreadCountChange={vi.fn()} unreadCount={2} />);

    await user.click(screen.getByRole("button", { name: "推荐提醒" }));
    const menu = await screen.findByRole("dialog", { name: "推荐提醒" });

    expect(within(menu).queryByRole("button", { name: "全部已读" })).not.toBeInTheDocument();
    expect(within(menu).getByText("张三 被推荐到「智算调度部」")).toBeInTheDocument();
  });
});
