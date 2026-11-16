"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";

/** One place for the cache keys, so a page that changes something can refresh what depends on it. */
export const KEYS = {
  gameState: ["game", "state"] as const,
  frontTypes: ["empire", "front-types"] as const,
  fronts: ["empire", "fronts"] as const,
  advice: (effort: number) => ["empire", "advice", effort] as const,
  ledger: (query: { limit?: number; offset?: number; flagged?: boolean }) => ["launder", "transactions", query] as const,
  threat: ["dea", "threat"] as const,
  history: ["dea", "history"] as const,
  evidence: ["dea", "evidence"] as const,
  cases: (query: { limit?: number; offset?: number }) => ["legal", "cases", query] as const,
};

/** The empire, the DEA's current assessment, the latest cases and the rules, in one request. */
export function useGameState() {
  // Heat cools with time and raids happen on their own, so the picture is refreshed regularly.
  return useQuery({
    queryKey: KEYS.gameState,
    queryFn: () => api.game.state(),
    staleTime: 10_000,
    refetchInterval: 30_000,
    refetchOnWindowFocus: true,
  });
}

/** After the player does something: everything that could have changed is loaded again. */
export function useRefreshGame() {
  const queryClient = useQueryClient();
  return () =>
    Promise.all(
      ["game", "empire", "launder", "dea", "legal"].map((key) => queryClient.invalidateQueries({ queryKey: [key] })),
    );
}
