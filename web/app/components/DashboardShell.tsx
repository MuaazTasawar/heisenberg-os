"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Banknote, FlaskConical, LayoutDashboard, LogOut, Scale, ShieldAlert, Store } from "lucide-react";
import { useLogout, useRequireSession } from "../lib/session";
import ErrorNotice from "./ErrorNotice";

const NAV = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard, exact: true },
  { href: "/dashboard/empire", label: "Empire", icon: FlaskConical, exact: false },
  { href: "/dashboard/fronts", label: "Fronts", icon: Store, exact: false },
  { href: "/dashboard/launder", label: "Launder", icon: Banknote, exact: false },
  { href: "/dashboard/dea", label: "DEA", icon: ShieldAlert, exact: false },
  { href: "/dashboard/legal", label: "Legal", icon: Scale, exact: false },
] as const;

function isActive(pathname: string, href: string, exact: boolean): boolean {
  return pathname === href || (!exact && pathname.startsWith(`${href}/`));
}

/**
 * The frame of every signed-in page. It checks the session first: visitors who are not signed
 * in are sent to the login page and never see the page content.
 */
export default function DashboardShell({ title, children }: { title?: string; children: ReactNode }) {
  const pathname = usePathname();
  const { status, user, error, retry } = useRequireSession();
  const logout = useLogout();

  if (error) {
    return (
      <div className="shell-message">
        <ErrorNotice error={error} title="Cannot check your session" onRetry={retry} />
      </div>
    );
  }
  if (status === "unknown") {
    return (
      <div className="shell-message" role="status">
        Checking credentials&hellip;
      </div>
    );
  }
  if (status === "anonymous") {
    return (
      <div className="shell-message" role="status">
        Redirecting to sign in&hellip;
      </div>
    );
  }

  return (
    <div className="shell">
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <header className="shell__header">
        <Link href="/dashboard" className="brand">
          HEISENBERG <span>OS</span>
        </Link>
        <div className="shell__user">
          <span className="mono">{user?.alias || user?.username}</span>
          <button type="button" className="btn btn--ghost" onClick={() => logout.mutate()} disabled={logout.isPending}>
            <LogOut aria-hidden="true" size={16} /> Sign out
          </button>
        </div>
      </header>
      <nav className="shell__nav" aria-label="Main">
        {NAV.map(({ href, label, icon: Icon, exact }) => {
          const active = isActive(pathname, href, exact);
          return (
            <Link key={href} href={href} className="nav-link" aria-current={active ? "page" : undefined}>
              <Icon aria-hidden="true" size={16} /> {label}
            </Link>
          );
        })}
      </nav>
      <main id="main" className="shell__main" tabIndex={-1}>
        {title && <h1 className="page-title">{title}</h1>}
        {children}
      </main>
    </div>
  );
}
