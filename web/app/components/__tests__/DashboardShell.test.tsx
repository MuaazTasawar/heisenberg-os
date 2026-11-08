import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../lib/api";
import { resetAuthStore, useAuthStore } from "../../lib/store";
import type { User } from "../../lib/types";
import DashboardShell from "../DashboardShell";

const mocks = vi.hoisted(() => ({ replace: vi.fn(), pathname: { value: "/dashboard" }, me: vi.fn(), logout: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
  usePathname: () => mocks.pathname.value,
}));

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, api: { auth: { me: mocks.me, logout: mocks.logout } } };
});

const user: User = { id: 1, created_at: "", updated_at: "", username: "walter", email: "w@example.com", alias: "Heisenberg" };

function renderShell(ui: ReactNode = <DashboardShell><p>the page</p></DashboardShell>, client = new QueryClient({ defaultOptions: { queries: { retry: false } } })) {
  return { client, ...render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>) };
}

beforeEach(() => {
  resetAuthStore();
  mocks.replace.mockReset();
  mocks.me.mockReset();
  mocks.logout.mockReset().mockResolvedValue(undefined);
  mocks.pathname.value = "/dashboard";
});

describe("DashboardShell", () => {
  it("shows nothing of the page while it checks who is signed in", () => {
    mocks.me.mockReturnValue(new Promise(() => {}));
    renderShell();
    expect(screen.getByRole("status")).toHaveTextContent("Checking credentials");
    expect(screen.queryByText("the page")).not.toBeInTheDocument();
  });

  it("frames the page for a signed-in player", async () => {
    mocks.me.mockResolvedValue(user);
    renderShell(<DashboardShell title="Command center"><p>the page</p></DashboardShell>);

    expect(await screen.findByText("the page")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Command center" })).toBeInTheDocument();
    expect(screen.getByText("Heisenberg")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /skip to content/i })).toHaveAttribute("href", "#main");

    const nav = screen.getByRole("navigation", { name: "Main" });
    const links = Array.from(nav.querySelectorAll("a")).map((a) => [a.textContent?.trim(), a.getAttribute("href")]);
    expect(links).toEqual([
      ["Dashboard", "/dashboard"],
      ["Empire", "/dashboard/empire"],
      ["Fronts", "/dashboard/fronts"],
      ["Launder", "/dashboard/launder"],
      ["DEA", "/dashboard/dea"],
      ["Legal", "/dashboard/legal"],
    ]);
    expect(mocks.replace).not.toHaveBeenCalled();
  });

  it("marks the current page, and /dashboard only when it is exactly that page", async () => {
    mocks.me.mockResolvedValue(user);
    mocks.pathname.value = "/dashboard/empire";
    const first = renderShell(<DashboardShell><p>x</p></DashboardShell>);
    await screen.findByText("x");
    const current = () => screen.getAllByRole("link").filter((a) => a.getAttribute("aria-current") === "page").map((a) => a.textContent?.trim());
    expect(current()).toEqual(["Empire"]);
    first.unmount();

    mocks.pathname.value = "/dashboard";
    renderShell(<DashboardShell><p>y</p></DashboardShell>);
    await screen.findByText("y");
    expect(current()).toEqual(["Dashboard"]);
  });

  it("sends anonymous visitors to the login page, which brings them back, and never shows the page", async () => {
    mocks.me.mockRejectedValue(new ApiError("authentication required", 401));
    mocks.pathname.value = "/dashboard/empire";
    renderShell(<DashboardShell><p>secret page</p></DashboardShell>);

    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fdashboard%2Fempire"));
    expect(screen.queryByText("secret page")).not.toBeInTheDocument();
    expect(useAuthStore.getState().status).toBe("anonymous");
  });

  it("tells the player when the server cannot be reached, and tries again on request", async () => {
    mocks.me.mockRejectedValueOnce(new ApiError("Cannot reach the server. Check your connection and try again.", 0));
    renderShell(<DashboardShell><p>the page</p></DashboardShell>);

    expect(await screen.findByRole("alert")).toHaveTextContent("Cannot reach the server");
    expect(mocks.replace).not.toHaveBeenCalled(); // an outage is not a sign-out

    mocks.me.mockResolvedValue(user);
    await userEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(await screen.findByText("the page")).toBeInTheDocument();
    expect(mocks.me).toHaveBeenCalledTimes(2);
  });

  it("signs out: asks the server, forgets everything and goes to the login page", async () => {
    mocks.me.mockResolvedValue(user);
    const { client } = renderShell(<DashboardShell><p>the page</p></DashboardShell>);
    await screen.findByText("the page");
    client.setQueryData(["game", "state"], { cached: true });
    // by now the server has cleared the cookie, so it no longer knows this player
    mocks.me.mockRejectedValue(new ApiError("authentication required", 401));

    await userEvent.click(screen.getByRole("button", { name: /sign out/i }));

    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login"));
    expect(mocks.logout).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState()).toMatchObject({ user: null, status: "anonymous" });
    expect(client.getQueryData(["game", "state"])).toBeUndefined();
  });

  it("signs out here even when the server cannot be told", async () => {
    mocks.me.mockResolvedValue(user);
    mocks.logout.mockRejectedValue(new ApiError("Cannot reach the server.", 0));
    renderShell(<DashboardShell><p>the page</p></DashboardShell>);
    await screen.findByText("the page");
    mocks.me.mockRejectedValue(new ApiError("Cannot reach the server.", 0));

    await userEvent.click(screen.getByRole("button", { name: /sign out/i }));
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login"));
    expect(useAuthStore.getState().status).toBe("anonymous");
  });
});
