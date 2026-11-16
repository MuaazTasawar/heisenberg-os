"use client";

import type { ReactNode } from "react";
import { useGameState } from "../lib/queries";
import type { GameState } from "../lib/types";
import ErrorNotice from "./ErrorNotice";

export function LoadingTiles() {
  return (
    <div className="stat-grid" aria-busy="true" role="status" aria-label="Loading your empire">
      {Array.from({ length: 8 }, (_, index) => (
        <div key={index} className="stat">
          <span className="skeleton skeleton--label" aria-hidden="true" />
          <span className="skeleton skeleton--value" aria-hidden="true" />
        </div>
      ))}
    </div>
  );
}

/**
 * Loads the game state and shows placeholders, an error with a retry, or the page. When a
 * refresh fails the last known numbers stay on screen with a notice.
 */
export default function GameStateGate({ children }: { children: (data: GameState) => ReactNode }) {
  const state = useGameState();
  return (
    <>
      {state.isPending && <LoadingTiles />}
      {state.error && (
        <ErrorNotice
          error={state.error}
          title={state.data ? "Could not refresh - showing the last known numbers" : "Cannot load your empire"}
          onRetry={() => void state.refetch()}
        />
      )}
      {state.data && children(state.data)}
    </>
  );
}
