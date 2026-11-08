// Shapes of what the backend sends. They mirror the Go models (snake_case JSON).

export type RiskLevel = "low" | "medium" | "high" | "critical";

export interface Envelope<T> {
  success: boolean;
  message: string;
  data?: T;
  error?: string;
}

export interface Timestamps {
  id: number;
  created_at: string;
  updated_at: string;
}

// ----------------------------------------------------------------------------- auth

export interface User extends Timestamps {
  username: string;
  email: string;
  alias: string;
}

export interface RegisterInput {
  username: string;
  email: string;
  password: string;
  alias?: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface LoginResult {
  token: string;
  expires_at: string;
  user: User;
}

// ---------------------------------------------------------------------------- empire

export interface FrontBusiness extends Timestamps {
  empire_id: number;
  name: string;
  type: string;
  revenue: number;
  /** 0-100; the DEA raids the front at 100. */
  suspicion: number;
  is_active: boolean;
  shutdown_reason?: string;
}

export interface Empire extends Timestamps {
  user_id: number;
  name: string;
  purity_rating: number;
  batch_yield: number;
  dirty_money: number;
  clean_money: number;
  heat_level: number;
  heat_updated_at?: string;
  territory_count: number;
  front_businesses: FrontBusiness[];
}

export interface FrontType {
  type: string;
  label: string;
  suspicion_factor: number;
  description: string;
}

export interface CookResult {
  empire: Empire;
  effort: number;
  yield_gained: number;
  dirty_gained: number;
  heat_added: number;
  purity_delta: number;
}

export interface CookAdvice {
  source: "ml" | "local";
  effort: number;
  predicted_yield: number;
  optimal_effort: number;
  optimal_yield: number;
  purity_projection: number;
  efficiency_score: number;
}

// -------------------------------------------------------------------------- laundering

export type TransactionType = "dirty" | "clean" | "laundered";

export interface Transaction extends Timestamps {
  empire_id: number;
  amount: number;
  type: TransactionType;
  description: string;
  business_id: number;
  flagged: boolean;
  flag_reason?: string;
  risk_score: number;
}

export interface LaunderAnalysis {
  source: "ml" | "local";
  risk_score: number;
  anomaly: boolean;
  patterns: string[];
  recommendation: string;
}

export interface LaunderInput {
  amount: number;
  business_id: number;
}

export interface LaunderResult {
  transaction: Transaction;
  cleaned_amount: number;
  fee: number;
  suspicion_gain: number;
  front_closed: boolean;
  front: FrontBusiness;
  empire: Empire;
  analysis: LaunderAnalysis;
}

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface LedgerQuery {
  limit?: number;
  offset?: number;
  flagged?: boolean;
}

// ------------------------------------------------------------------------------ DEA

export interface DeaThreat extends Timestamps {
  empire_id: number;
  threat_score: number;
  evidence_count: number;
  lead_agent: string;
  is_active: boolean;
  risk_level: RiskLevel;
  prediction: string;
  recommendation: string;
  source: "ml" | "heuristic";
}

export interface EvidenceSource {
  source: string;
  kind: string;
  weight: number;
}

export interface EvidenceFile {
  source: "ml" | "local";
  count: number;
  total_weight: number;
  top_sources: EvidenceSource[];
}

// ----------------------------------------------------------------------------- legal

export type CaseStatus = "open" | "dismissed" | "convicted";

export interface LegalCase extends Timestamps {
  empire_id: number;
  charge: string;
  evidence: string;
  status: CaseStatus;
  /** 1 (low) to 4 (critical). */
  severity: number;
  risk: RiskLevel;
  advice: string;
  defense: string;
}

export interface ConsultInput {
  charge: string;
  evidence?: string;
  player_context?: string;
}

export interface CaseQuery {
  limit?: number;
  offset?: number;
}

// ------------------------------------------------------------------- game state, stats

/** The game constants the server owns; clients must not hard-code them. */
export interface Rules {
  min_effort: number;
  max_effort: number;
  optimal_effort: number;
  dirty_per_kg: number;
  heat_per_effort: number;
  heat_decay_per_hour: number;
  launder_rate: number;
  launder_min: number;
  large_transfer: number;
  territory_bonus: number;
  /** null once the player holds the maximum number of territories. */
  next_territory_cost: number | null;
  max_territories: number;
  max_active_fronts: number;
}

export interface GameState {
  empire: Empire;
  dea_threat: DeaThreat;
  legal_cases: LegalCase[];
  rules: Rules;
}

export interface Stats {
  empires: number;
  kilograms_cooked: number;
  dirty_laundered: number;
  flagged_transfers: number;
  raids: number;
}
