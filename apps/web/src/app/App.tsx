import * as React from "react";
import { Button } from "../components/ui/button";
import { Field, Form } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { NavLink } from "../components/ui/nav-link";
import { zhCN } from "../i18n/zh-CN";
import { getCurrentUser, loginWithW3, logout } from "../api/client";
import { DepartmentPositionPage } from "../department-position/DepartmentPositionPage";
import { NotificationBell } from "../notifications/NotificationBell";
import type { NotificationJumpContext } from "../notifications/types";
import { ResumeLibraryPage } from "../resume-library/ResumeLibraryPage";
import { ResumeParsePage } from "../resume-parse/ResumeParsePage";
import { ResumeRecommendPage } from "../resume-recommend/ResumeRecommendPage";
import { RoleManagementPage } from "../roles/RoleManagementPage";
import { UsersPage } from "../users/UsersPage";
import "../styles/globals.css";

type SessionView = {
  dataScope: {
    allDepartments: boolean;
    channels: string[];
    departments: Array<{ id: string; name: string }>;
  };
  defaultRoute: string;
  pageAccess: string[];
  permissions: string[];
  roleBindings: Array<{
    departmentId: string;
    departmentName: string;
    roleLabel: string;
  }>;
  roleLabels: string[];
  user: {
    employeeId: string;
    id: string;
    name: string;
  };
};

const routeLabels: Record<string, string> = {
  "audit-logs": zhCN.session.nav.auditLogs,
  "departments-positions": zhCN.session.nav.departmentsPositions,
  notifications: zhCN.session.nav.notifications,
  "resume-library": zhCN.session.nav.resumeLibrary,
  "resume-parse": zhCN.session.nav.resumeParse,
  "resume-recommend": zhCN.session.nav.resumeRecommend,
  roles: zhCN.session.nav.roles,
  users: zhCN.session.nav.users,
};

const channelLabels: Record<string, string> = {
  campus: zhCN.session.workspace.channels.campus,
  social: zhCN.session.workspace.channels.social,
};

export function App() {
  const text = zhCN.session;
  const [session, setSession] = React.useState<SessionView | null>(null);
  const [account, setAccount] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [isCheckingSession, setIsCheckingSession] = React.useState(true);
  const [isSubmitting, setIsSubmitting] = React.useState(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);
  const [successMessage, setSuccessMessage] = React.useState<string | null>(null);
  const [activePage, setActivePage] = React.useState("");
  const [notificationJump, setNotificationJump] = React.useState<NotificationJumpContext | null>(null);
  const [unreadCount, setUnreadCount] = React.useState(0);

  React.useEffect(() => {
    let isMounted = true;

    async function loadCurrentUser() {
      try {
        const { data, error } = await getCurrentUser();
        if (isMounted && !error && data) {
          setSession(data);
        }
      } catch {
        // Missing or unreachable session state falls back to the login form.
      } finally {
        if (isMounted) {
          setIsCheckingSession(false);
        }
      }
    }

    void loadCurrentUser();

    return () => {
      isMounted = false;
    };
  }, []);

  async function handleLogin(event: React.FormEvent) {
    event.preventDefault();
    setIsSubmitting(true);
    setErrorMessage(null);

    try {
      const { data, error } = await loginWithW3(account, password);
      if (error || !data) {
        setErrorMessage(text.login.error);
        return;
      }
      setSession(data);
      setActivePage(resolveInitialPage(data));
      setSuccessMessage(`${text.login.successPrefix} · ${data.roleLabels.join("、")}`);
      setAccount("");
      setPassword("");
    } catch {
      setErrorMessage(text.login.error);
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleLogout() {
    await logout();
    setSession(null);
    setActivePage("");
    setNotificationJump(null);
    setUnreadCount(0);
    setSuccessMessage(null);
    setPassword("");
  }

  React.useEffect(() => {
    if (!session) {
      return;
    }
    const currentSession = session;
    setActivePage(resolveInitialPage(currentSession));

    function handleHashChange() {
      const nextPage = resolveHashPage(currentSession);
      if (nextPage) {
        setActivePage(nextPage);
        if (nextPage !== "resume-library") {
          setNotificationJump(null);
        }
      }
    }

    window.addEventListener("hashchange", handleHashChange);
    return () => window.removeEventListener("hashchange", handleHashChange);
  }, [session]);

  if (isCheckingSession) {
    return (
      <main aria-label={text.login.mainLabel} className="min-h-screen bg-bg px-6 py-10 text-fg">
        <section className="mx-auto grid max-w-sm gap-3 border border-white/15 bg-white/[0.04] p-6">
          <p className="text-xs font-semibold uppercase tracking-normal text-muted">{zhCN.appName}</p>
          <p className="text-sm text-muted">{text.login.checkingSession}</p>
        </section>
      </main>
    );
  }

  if (!session) {
    return (
      <main aria-label={text.login.mainLabel} className="min-h-screen bg-bg px-6 py-10 text-fg">
        <section className="mx-auto grid max-w-sm gap-6 border border-white/15 bg-white/[0.04] p-6">
          <div className="grid gap-2">
            <p className="text-xs font-semibold uppercase tracking-normal text-muted">{zhCN.appName}</p>
            <h1 className="text-2xl font-semibold tracking-normal">{text.login.title}</h1>
          </div>

          <Form className="gap-4" onSubmit={handleLogin}>
            <Field label={text.login.accountLabel}>
              <Input
                autoComplete="username"
                name="account"
                onChange={(event) => setAccount(event.target.value)}
                required
                value={account}
              />
            </Field>
            <Field label={text.login.passwordLabel}>
              <Input
                autoComplete="current-password"
                name="password"
                onChange={(event) => setPassword(event.target.value)}
                required
                type="password"
                value={password}
              />
            </Field>
            {errorMessage ? (
              <p className="text-sm text-red-300" role="alert">
                {errorMessage}
              </p>
            ) : null}
            <Button disabled={isSubmitting} type="submit" variant="primary">
              {isSubmitting ? text.login.loadingAction : text.login.submitAction}
            </Button>
          </Form>
        </section>
      </main>
    );
  }

  const links = session.pageAccess
    .map((page) => ({ href: `#${page}`, label: routeLabels[page], page }))
    .filter((link): link is { href: string; label: string; page: string } => Boolean(link.label));
  const selectedPage = activePage || session.defaultRoute.replace(/^\//, "");
  const departmentScope = formatDepartmentScope(session.dataScope);
  const channelScope = formatChannelScope(session.dataScope.channels);
  const canListNotifications = hasPermission(session.permissions, "Notification.List");
  const canUpdateNotifications = hasPermission(session.permissions, "Notification.Update");

  function handleOpenResumeFromNotification(jump: NotificationJumpContext) {
    setNotificationJump(jump);
    setActivePage("resume-library");
    window.location.hash = "resume-library";
  }

  function handleNavClick(page: string) {
    setActivePage(page);
    if (page !== "resume-library") {
      setNotificationJump(null);
    }
  }

  return (
    <main aria-label={text.workspace.mainLabel} className="min-h-screen bg-bg text-fg">
      <header className="flex flex-wrap items-center justify-between gap-4 border-b border-white/10 px-6 py-4">
        <nav aria-label={text.workspace.navLabel} className="flex flex-wrap gap-2">
          {links.map((link) => (
            <NavLink active={link.page === selectedPage} href={link.href} key={link.href} onClick={() => handleNavClick(link.page)}>
              {link.label}
              {link.page === "resume-library" && unreadCount > 0 ? (
                <span aria-hidden="true" className="ml-2 min-w-5 border border-red-300 bg-red-500 px-1 text-center text-xs font-semibold text-white">
                  {unreadCount}
                </span>
              ) : null}
            </NavLink>
          ))}
        </nav>
        <div className="flex flex-wrap items-center gap-4 text-sm">
          {canListNotifications ? (
            <NotificationBell
              canUpdate={canUpdateNotifications}
              onOpenResume={handleOpenResumeFromNotification}
              onUnreadCountChange={setUnreadCount}
              unreadCount={unreadCount}
            />
          ) : null}
          <div className="grid gap-1 text-right">
            <span className="font-medium">{session.user.name}</span>
            <span className="text-muted">{session.user.employeeId}</span>
            <span className="text-muted">
              {session.roleBindings.map((binding) => (
                <span key={`${binding.departmentId}-${binding.roleLabel}`}>
                  <span>{binding.roleLabel}</span>
                  <span> · {binding.departmentName}</span>
                </span>
              ))}
            </span>
            <span className="text-muted">{departmentScope}</span>
            <span className="text-muted">{channelScope}</span>
          </div>
          <Button onClick={handleLogout} type="button">
            {text.workspace.logoutAction}
          </Button>
        </div>
      </header>
      <section className="px-6 py-8">
        {successMessage ? (
          <p className="mb-4 text-sm text-accent" role="status">
            {successMessage}
          </p>
        ) : null}
        {selectedPage === "resume-parse" ? (
          <ResumeParsePage session={session} />
        ) : selectedPage === "resume-recommend" ? (
          <ResumeRecommendPage session={session} />
        ) : selectedPage === "resume-library" ? (
          <ResumeLibraryPage notificationJump={notificationJump} session={session} />
        ) : selectedPage === "departments-positions" ? (
          <DepartmentPositionPage session={session} />
        ) : selectedPage === "users" ? (
          <UsersPage session={session} />
        ) : selectedPage === "roles" ? (
          <RoleManagementPage session={session} />
        ) : (
          <h1 className="text-2xl font-semibold tracking-normal">{routeLabels[selectedPage] ?? text.nav.resumeParse}</h1>
        )}
      </section>
    </main>
  );
}

function formatDepartmentScope(dataScope: SessionView["dataScope"]) {
  if (dataScope.allDepartments) {
    return zhCN.session.workspace.allDepartments;
  }
  if (dataScope.departments.length === 0) {
    return zhCN.session.workspace.noDepartmentScope;
  }
  return dataScope.departments.map((department) => department.name).join("、");
}

function formatChannelScope(channels: string[]) {
  if (channels.length === 0) {
    return zhCN.session.workspace.noChannelScope;
  }
  return channels.map((channel) => channelLabels[channel] ?? channel).join("、");
}

function hasPermission(permissions: string[], permission: string) {
  return permissions.includes(permission);
}

function resolveInitialPage(session: SessionView) {
  return resolveHashPage(session) || session.defaultRoute.replace(/^\//, "");
}

function resolveHashPage(session: SessionView) {
  const hashPage = window.location.hash.replace(/^#/, "");
  if (!hashPage) {
    return "";
  }
  return session.pageAccess.includes(hashPage) ? hashPage : "";
}
