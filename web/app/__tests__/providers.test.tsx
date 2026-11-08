import { AxiosError, type AxiosAdapter } from "axios";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError, http } from "../lib/api";
import { resetAuthStore, useAuthStore } from "../lib/store";
import type { User } from "../lib/types";
import Providers, { makeQueryClient, shouldRetry } from "../providers";

const mocks = vi.hoisted(() => ({ replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: mocks.replace, push: vi.fn() }), usePathname: () => "/" }));

const user: User = { id: 1, created_at: "", updated_at: "", username: "walter", email: "w@example.com", alias: "Heisenberg" };
const originalAdapter = http.defaults.adapter;

function serverAnswers(status: number) {
  const adapter: AxiosAdapter = async (config) => {
    const response = { data: { success: false, message: "", error: "authentication required" }, status, statusText: "", headers: {}, config };
    throw new AxiosError("Request failed", "ERR_BAD_REQUEST", config, null, response);
  };
  http.defaults.adapter = adapter;
}

beforeEach(() => {
  mocks.replace.mockReset();
  resetAuthStore();
});
afterEach(() => {
  http.defaults.adapter = originalAdapter;
  window.history.pushState({}, "", "/");
});

describe("shouldRetry", () => {
  it("does not retry what the player has to fix", () => {
    for (const status of [400, 401, 403, 404, 409, 429]) expect(shouldRetry(0, new ApiError("x", status))).toBe(false);
  });

  it("retries network and server trouble twice", () => {
    for (const error of [new ApiError("x", 0), new ApiError("x", 500), new ApiError("x", 503), new Error("boom")]) {
      expect(shouldRetry(0, error)).toBe(true);
      expect(shouldRetry(1, error)).toBe(true);
      expect(shouldRetry(2, error)).toBe(false);
    }
  });

  it("is what the query client uses, and mutations are never retried", () => {
    const defaults = makeQueryClient().getDefaultOptions();
    expect(defaults.queries?.retry).toBe(shouldRetry);
    expect(defaults.mutations?.retry).toBe(false);
  });
});

describe("Providers", () => {
  it("renders its children", () => {
    render(<Providers><p>hello</p></Providers>);
    expect(screen.getByText("hello")).toBeInTheDocument();
  });

  it("ends the session when the server says it is over: forgets the player and goes to login, coming back afterwards", async () => {
    render(<Providers><p>app</p></Providers>);
    useAuthStore.getState().setUser(user);
    window.history.pushState({}, "", "/dashboard/empire?tab=fronts");
    serverAnswers(401);

    await expect(api.empire.get()).rejects.toMatchObject({ status: 401 });

    expect(useAuthStore.getState()).toMatchObject({ user: null, status: "anonymous" });
    expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fdashboard%2Fempire%3Ftab%3Dfronts");
  });

  it("does not redirect from a public page", async () => {
    render(<Providers><p>app</p></Providers>);
    window.history.pushState({}, "", "/login");
    serverAnswers(401);

    await expect(api.empire.get()).rejects.toMatchObject({ status: 401 });
    expect(mocks.replace).not.toHaveBeenCalled();
    expect(useAuthStore.getState().status).toBe("anonymous");
  });

  it("stops listening when it is removed", async () => {
    const { unmount } = render(<Providers><p>app</p></Providers>);
    unmount();
    window.history.pushState({}, "", "/dashboard");
    serverAnswers(401);

    await expect(api.empire.get()).rejects.toMatchObject({ status: 401 });
    expect(mocks.replace).not.toHaveBeenCalled();
  });
});
