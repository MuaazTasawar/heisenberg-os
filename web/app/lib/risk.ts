import type { RiskLevel } from "./types";

export type { RiskLevel } from "./types";

/** Lowest to highest. The same four levels (and the same bands) as the backend and the ML service. */
export const RISK_LEVELS: readonly RiskLevel[] = ["low", "medium", "high", "critical"];

const LABELS: Record<RiskLevel, string> = {
  low: "Low",
  medium: "Medium",
  high: "High",
  critical: "Critical",
};

const COLORS: Record<RiskLevel, string> = {
  low: "var(--green-bright)",
  medium: "var(--gold)",
  high: "var(--orange)",
  critical: "var(--red)",
};

export function isRiskLevel(value: unknown): value is RiskLevel {
  return typeof value === "string" && (RISK_LEVELS as readonly string[]).includes(value);
}

/** Reads a risk level from untrusted data; anything unknown (such as the old "elevated") is medium. */
export function parseRisk(value: unknown, fallback: RiskLevel = "medium"): RiskLevel {
  return isRiskLevel(value) ? value : fallback;
}

/** Maps a 0-100 score to a level: below 25 low, below 50 medium, below 75 high, otherwise critical. */
export function riskFromScore(score: number): RiskLevel {
  if (!Number.isFinite(score) || score < 25) return "low";
  if (score < 50) return "medium";
  if (score < 75) return "high";
  return "critical";
}

/** Maps a legal case severity (1-4) to a level; out-of-range values are clamped. */
export function riskFromSeverity(severity: number): RiskLevel {
  if (!Number.isFinite(severity)) return "medium";
  const index = Math.min(Math.max(Math.round(severity), 1), 4) - 1;
  return RISK_LEVELS[index];
}

export function riskLabel(level: RiskLevel): string {
  return LABELS[level];
}

/** A CSS colour (a design token) for a level. */
export function riskColor(level: RiskLevel): string {
  return COLORS[level];
}

/** Position of a level from 0 (low) to 3 (critical), for sorting and meters. */
export function riskRank(level: RiskLevel): number {
  return RISK_LEVELS.indexOf(level);
}
