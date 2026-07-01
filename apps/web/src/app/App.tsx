import * as React from "react";
import { Button } from "../components/ui/button";
import { Field, Form } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { NavLink } from "../components/ui/nav-link";
import { zhCN } from "../i18n/zh-CN";
import { getCurrentUser, loginWithW3, logout } from "../api/client";
import "../styles/globals.css";

type SessionView = {
  defaultRoute: string;
  pageAccess: string[];
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
  "resume-parse": zhCN.session.nav.resumeParse,
  "resume-recommend": zhCN.session.nav.resumeRecommend,
};

export function App() {
  const text = zhCN.session;
  const [session, setSession] = React.useState<SessionView | null>(null);
  const [account, setAccount] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [isCheckingSession, setIsCheckingSession] = React.useState(true);
  const [isSubmitting, setIsSubmitting] = React.useState(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);

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
    setPassword("");
  }

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
    .map((page) => ({ href: `#${page}`, label: routeLabels[page] }))
    .filter((link): link is { href: string; label: string } => Boolean(link.label));
  const activePage = session.defaultRoute.replace(/^\//, "");

  return (
    <main aria-label={text.workspace.mainLabel} className="min-h-screen bg-bg text-fg">
      <header className="flex flex-wrap items-center justify-between gap-4 border-b border-white/10 px-6 py-4">
        <nav aria-label={text.workspace.navLabel} className="flex flex-wrap gap-2">
          {links.map((link) => (
            <NavLink active={link.href === `#${activePage}`} href={link.href} key={link.href}>
              {link.label}
            </NavLink>
          ))}
        </nav>
        <div className="flex flex-wrap items-center gap-4 text-sm">
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
          </div>
          <Button onClick={handleLogout} type="button">
            {text.workspace.logoutAction}
          </Button>
        </div>
      </header>
      <section className="px-6 py-8">
        <h1 className="text-2xl font-semibold tracking-normal">{routeLabels[activePage] ?? text.nav.resumeParse}</h1>
      </section>
    </main>
  );
}
