import { create } from "zustand";
import type { User } from "./types";

/** "unknown" until the first answer from /auth/me; there is no token or session in localStorage. */
export type AuthStatus = "unknown" | "authenticated" | "anonymous";

interface AuthState {
  user: User | null;
  status: AuthStatus;
  setUser: (user: User) => void;
  /** The server said nobody is signed in (or the session ended). */
  clear: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  status: "unknown",
  setUser: (user) => set({ user, status: "authenticated" }),
  clear: () => set({ user: null, status: "anonymous" }),
}));

/** Back to the initial state; for tests. */
export function resetAuthStore(): void {
  useAuthStore.setState({ user: null, status: "unknown" });
}
