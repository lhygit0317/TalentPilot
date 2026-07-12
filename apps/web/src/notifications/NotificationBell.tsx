import * as React from "react";
import { Bell } from "lucide-react";
import {
  getNotificationSummary,
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
} from "../api/client";
import { Button } from "../components/ui/button";
import { zhCN } from "../i18n/zh-CN";
import type { NotificationItem, NotificationJumpContext } from "./types";

type NotificationBellProps = {
  canUpdate: boolean;
  onUnreadCountChange: (count: number) => void;
  onOpenResume: (jump: NotificationJumpContext) => void;
  unreadCount: number;
};

const text = zhCN.notifications;

export function NotificationBell({ canUpdate, onOpenResume, onUnreadCountChange, unreadCount }: NotificationBellProps) {
  const [isOpen, setIsOpen] = React.useState(false);
  const [items, setItems] = React.useState<NotificationItem[]>([]);
  const [status, setStatus] = React.useState("");

  React.useEffect(() => {
    let isCurrent = true;

    async function loadSummary() {
      try {
        const { data, error } = await getNotificationSummary();
        if (isCurrent && !error && data) {
          onUnreadCountChange(data.unreadCount);
        }
      } catch {
        // Notification badges stay at the caller-provided value when summary refresh fails.
      }
    }

    void loadSummary();

    return () => {
      isCurrent = false;
    };
  }, [onUnreadCountChange]);

  async function handleOpen() {
    const nextOpen = !isOpen;
    setIsOpen(nextOpen);
    setStatus("");
    if (!nextOpen) {
      return;
    }

    try {
      const { data, error } = await listNotifications({ limit: 20 });
      if (error || !data) {
        return;
      }
      setItems((data.items ?? []) as NotificationItem[]);
      onUnreadCountChange(data.unreadCount);
    } catch {
      // Keep the dropdown stable; the next open can retry.
    }
  }

  async function handleMarkAllRead() {
    const { data, error } = await markAllNotificationsRead();
    if (error || !data) {
      return;
    }
    setItems([]);
    onUnreadCountChange(data.unreadCount);
    setStatus(text.markAllDone);
  }

  async function handleNotificationClick(item: NotificationItem) {
    if (!canUpdate || !item.canOpenResumeLibrary) {
      return;
    }
    const { data, error } = await markNotificationRead(item.id);
    if (error || !data) {
      return;
    }
    onUnreadCountChange(data.unreadCount);
    setIsOpen(false);
    onOpenResume({
      items: [
        {
          chan: item.chan,
          resumeId: item.resumeId,
          candidateName: item.candidateName,
          department: item.department,
          recommender: item.recommender,
        },
      ],
    });
  }

  return (
    <div className="relative">
      <Button aria-label={text.buttonLabel} className="relative gap-2" onClick={() => void handleOpen()} type="button">
        <Bell aria-hidden="true" className="h-4 w-4" />
        <span>{text.buttonLabel}</span>
        {unreadCount > 0 ? (
          <span className="min-w-5 border border-red-300 bg-red-500 px-1 text-center text-xs font-semibold text-white">
            {unreadCount}
          </span>
        ) : null}
      </Button>

      {isOpen ? (
        <div
          aria-label={text.buttonLabel}
          className="absolute right-0 z-20 mt-2 grid w-[min(360px,calc(100vw-2rem))] gap-3 border border-white/15 bg-bg p-3 shadow-xl"
          role="dialog"
        >
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-sm font-semibold tracking-normal">{text.title(unreadCount)}</h2>
            {canUpdate && unreadCount > 0 ? (
              <Button onClick={() => void handleMarkAllRead()} type="button">
                {text.markAll}
              </Button>
            ) : null}
          </div>

          {items.length > 0 ? (
            <div className="grid gap-2">
              {items.map((item) =>
                canUpdate && item.canOpenResumeLibrary ? (
                  <Button
                    className="h-auto justify-start px-3 py-2 text-left"
                    key={item.id}
                    onClick={() => void handleNotificationClick(item)}
                    type="button"
                  >
                    <NotificationRow item={item} />
                  </Button>
                ) : (
                  <div className="border border-white/10 bg-white/[0.03] px-3 py-2" key={item.id}>
                    <NotificationRow item={item} />
                  </div>
                ),
              )}
            </div>
          ) : (
            <p className="py-4 text-center text-sm text-muted">{text.empty}</p>
          )}

          {status ? (
            <p className="text-sm text-accent" role="status">
              {status}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function NotificationRow({ item }: { item: NotificationItem }) {
  return (
    <span className="grid gap-1">
      <span className="text-sm font-medium">{text.rowTitle(item.candidateName, item.department.name)}</span>
      <span className="text-xs text-muted">
        {item.recommender.name} · {formatNotificationTime(item.createdAt)}
      </span>
    </span>
  );
}

function formatNotificationTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}
