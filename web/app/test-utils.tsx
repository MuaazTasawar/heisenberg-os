import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import type { DeaThreat, Empire, FrontBusiness, GameState, LegalCase, Rules, User } from "./lib/types";

// Builders and helpers for the tests. Not part of the application.

const STAMP = "2026-11-01T10:00:00Z";

export const aUser = (overrides: Partial<User> = {}): User => ({
  id: 1, created_at: STAMP, updated_at: STAMP, username: "walter", email: "walter@example.com", alias: "Heisenberg", ...overrides,
});

export const aRules = (overrides: Partial<Rules> = {}): Rules => ({
  min_effort: 1, max_effort: 10, optimal_effort: 8.33, dirty_per_kg: 1500, heat_per_effort: 2.5, heat_decay_per_hour: 6,
  launder_rate: 0.75, launder_min: 100, large_transfer: 50000, territory_bonus: 0.05, next_territory_cost: 25000,
  max_territories: 10, max_active_fronts: 5, ...overrides,
});

export const aFront = (overrides: Partial<FrontBusiness> = {}): FrontBusiness => ({
  id: 1, created_at: STAMP, updated_at: STAMP, empire_id: 1, name: "Los Pollos Hermanos", type: "restaurant",
  revenue: 0, suspicion: 10, is_active: true, ...overrides,
});

export const anEmpire = (overrides: Partial<Empire> = {}): Empire => ({
  id: 1, created_at: STAMP, updated_at: STAMP, user_id: 1, name: "Heisenberg's Empire", purity_rating: 50, batch_yield: 10,
  dirty_money: 0, clean_money: 0, heat_level: 0, territory_count: 0, front_businesses: [], ...overrides,
});

export const aThreat = (overrides: Partial<DeaThreat> = {}): DeaThreat => ({
  id: 1, created_at: STAMP, updated_at: STAMP, empire_id: 1, threat_score: 1.8, evidence_count: 0, lead_agent: "Hank Schrader",
  is_active: true, risk_level: "low", prediction: "Hank suspects nothing. Stay clean.",
  recommendation: "Continue operations. Keep front businesses active.", source: "heuristic", ...overrides,
});

export const aCase = (overrides: Partial<LegalCase> = {}): LegalCase => ({
  id: 1, created_at: new Date(Date.now() - 3_600_000).toISOString(), updated_at: STAMP, empire_id: 1, charge: "Possession with intent",
  evidence: "", status: "open", severity: 3, risk: "high", advice: "Say nothing.", defense: "Chain of custody.", ...overrides,
});

export const aGameState = (overrides: Partial<GameState> = {}): GameState => ({
  empire: anEmpire(), dea_threat: aThreat(), legal_cases: [], rules: aRules(), ...overrides,
});

/** A client that does not retry or wait, so failures show up at once in tests. */
export const makeTestClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, retryDelay: 0 } } });

export function renderWithClient(ui: ReactElement, client: QueryClient = makeTestClient()) {
  return { client, ...render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>) };
}
