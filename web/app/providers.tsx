"use client";

import { useEffect, useState, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { ApiError, setUnauthorizedHandler } from "./lib/api";
import { isPublicPath, loginUrl } from "./lib/redirect";
import { useAuthStore } from "./lib/store";

/** Do not retry what the player must fix (4xx); retry network and server trouble twice. */
export function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false;
  return failureCount < 2;
}

export function makeQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { staleTime: 15_000, refetchOnWindowFocus: false, retry: shouldRetry },
      mutations: { retry: false },
    },
  });
}

export default function Providers({ children }: { children: ReactNode }) {
  const [client] = useState(makeQueryClient);
  const router = useRouter();

  // When the server says the session is over, forget everything and go to the login page.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      client.clear();
      useAuthStore.getState().clear();
      const { pathname, search } = window.location;
      if (!isPublicPath(pathname)) router.replace(loginUrl(pathname + search));
    });
    return () => setUnauthorizedHandler(null);
  }, [client, router]);

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
