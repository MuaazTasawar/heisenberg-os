import { clamp } from "./format";
import { riskFromScore, type RiskLevel } from "./risk";
import type { DeaThreat } from "./types";

export interface ThreatPoint {
  /** Milliseconds since the epoch. */
  time: number;
  /** 0-100, to one decimal. */
  score: number;
  evidence: number;
  level: RiskLevel;
}

/**
 * The DEA's history (newest first, as the server sends it) as points for a chart, oldest first.
 * Entries without a usable date or score are left out rather than drawn at the wrong place.
 */
export function buildThreatSeries(history: readonly DeaThreat[]): ThreatPoint[] {
  const points: ThreatPoint[] = [];
  for (const entry of history) {
    const time = Date.parse(entry.created_at);
    if (!Number.isFinite(time) || !Number.isFinite(entry.threat_score)) continue;
    const score = Math.round(clamp(entry.threat_score, 0, 100) * 10) / 10;
    points.push({ time, score, evidence: entry.evidence_count, level: riskFromScore(score) });
  }
  return points.sort((a, b) => a.time - b.time);
}

export interface ThreatTrend {
  direction: "none" | "up" | "down" | "flat";
  /** Points gained or lost since the assessment before the latest, to one decimal. */
  change: number;
}

/** How the latest assessment compares with the one before it. */
export function threatTrend(series: readonly ThreatPoint[]): ThreatTrend {
  if (series.length < 2) return { direction: "none", change: 0 };
  const change = Math.round((series[series.length - 1].score - series[series.length - 2].score) * 10) / 10;
  if (change === 0) return { direction: "flat", change: 0 };
  return { direction: change > 0 ? "up" : "down", change };
}
