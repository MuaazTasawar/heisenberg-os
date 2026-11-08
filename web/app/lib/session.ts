"use client";

import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { usePathname, useRouter } from "next/navigation";
import { api, isUnauthorized } from "./api";
import { loginUrl } from "./redirect";
import { useAuthStore } from "./store";

export const SESSION_KEY = ["me"] as const;

/** Asks the server who is signed in and keeps the auth store in step with the answer. */
export function useSession() {
  return useQuery({
    queryKey: SESSION_KEY,
    staleTime: 60_000,
    queryFn: async () => {
      try {
        const user = await api.auth.me();
        useAuthStore.getState().setUser(user);
        return user;
      } catch (error) {
        if (isUnauthorized(error)) useAuthStore.getState().clear();
        throw error;
      }
    },
  });
}

/**
 * For pages that need a login: checks the session and sends anonymous visitors to the login
 * page, which sends them back here afterwards. `error` is a failure that is not "signed out"
 * (the server is down, say), with `retry` to ask again.
 */
export function useRequireSession() {
  const router = useRouter();
  const pathname = usePathname();
  const query = useSession();
  const status = useAuthStore((state) => state.status);
  const user = useAuthStore((state) => state.user);

  useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(pathname));
  }, [status, pathname, router]);

  const error = query.error && !isUnauthorized(query.error) ? query.error : null;
  return { status, user, error, retry: () => void query.refetch() };
}

/** Signs out on the server, forgets everything that was loaded and goes to the login page. */
export function useLogout() {
  const router = useRouter();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.auth.logout(),
    onSettled: () => {
      // The cookie is cleared by the server; even if that call failed, this browser forgets the session.
      queryClient.clear();
      useAuthStore.getState().clear();
      router.replace("/login");
    },
  });
}
