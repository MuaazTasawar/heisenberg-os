import { afterEach, describe, expect, it, vi } from "vitest";
import { resetAuthStore, useAuthStore } from "../store";
import type { User } from "../types";

const user: User = { id: 1, created_at: "", updated_at: "", username: "walter", email: "w@example.com", alias: "Heisenberg" };

afterEach(() => resetAuthStore());

describe("auth store", () => {
  it("starts unknown, because nobody has asked the server yet", () => {
    expect(useAuthStore.getState()).toMatchObject({ user: null, status: "unknown" });
  });

  it("is authenticated once the user is known and anonymous after clear", () => {
    useAuthStore.getState().setUser(user);
    expect(useAuthStore.getState()).toMatchObject({ user, status: "authenticated" });
    useAuthStore.getState().clear();
    expect(useAuthStore.getState()).toMatchObject({ user: null, status: "anonymous" });
  });

  it("never touches the browser storage", () => {
    const get = vi.spyOn(Storage.prototype, "getItem");
    const set = vi.spyOn(Storage.prototype, "setItem");
    useAuthStore.getState().setUser(user);
    useAuthStore.getState().clear();
    expect(get).not.toHaveBeenCalled();
    expect(set).not.toHaveBeenCalled();
    expect(Object.keys(localStorage)).toHaveLength(0);
  });
});
