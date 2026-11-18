import { describe, expect, it } from "vitest";
import { buildThreatSeries, threatTrend, type ThreatPoint } from "../dea";
import { aThreat } from "../../test-utils";

const at = (day: number) => `2026-11-${String(day).padStart(2, "0")}T10:00:00Z`;
const point = (score: number, time = 0): ThreatPoint => ({ time, score, evidence: 0, level: "low" });

describe("buildThreatSeries", () => {
  it("turns the newest-first history into points, oldest first", () => {
    const series = buildThreatSeries([
      aThreat({ id: 3, created_at: at(12), threat_score: 60 }),
      aThreat({ id: 2, created_at: at(8), threat_score: 30 }),
      aThreat({ id: 1, created_at: at(2), threat_score: 5 }),
    ]);
    expect(series.map((item) => item.score)).toEqual([5, 30, 60]);
    expect(series.map((item) => item.time)).toEqual([Date.parse(at(2)), Date.parse(at(8)), Date.parse(at(12))]);
  });

  it("carries the evidence count and the risk level of the score", () => {
    const [low, critical] = buildThreatSeries([
      aThreat({ created_at: at(2), threat_score: 24.9, evidence_count: 1 }),
      aThreat({ created_at: at(3), threat_score: 75, evidence_count: 7 }),
    ]);
    expect(low).toMatchObject({ evidence: 1, level: "low" });
    expect(critical).toMatchObject({ evidence: 7, level: "critical" });
  });

  it("keeps scores inside 0-100 and to one decimal", () => {
    const scores = buildThreatSeries([
      aThreat({ created_at: at(2), threat_score: 123.456 }),
      aThreat({ created_at: at(3), threat_score: -5 }),
      aThreat({ created_at: at(4), threat_score: 12.34 }),
    ]).map((item) => item.score);
    expect(scores).toEqual([100, 0, 12.3]); // the three days in order: 123.456, -5, 12.34
  });

  it("leaves out entries without a usable date or score", () => {
    const series = buildThreatSeries([
      aThreat({ created_at: "not a date", threat_score: 50 }),
      aThreat({ created_at: at(3), threat_score: Number.NaN }),
      aThreat({ created_at: at(4), threat_score: 20 }),
    ]);
    expect(series).toHaveLength(1);
    expect(series[0].score).toBe(20);
  });

  it("handles an empty history and does not change its input", () => {
    expect(buildThreatSeries([])).toEqual([]);
    const input = [aThreat({ id: 2, created_at: at(9) }), aThreat({ id: 1, created_at: at(1) })];
    buildThreatSeries(input);
    expect(input.map((item) => item.id)).toEqual([2, 1]);
  });
});

describe("threatTrend", () => {
  it("has nothing to compare with fewer than two points", () => {
    expect(threatTrend([])).toEqual({ direction: "none", change: 0 });
    expect(threatTrend([point(10)])).toEqual({ direction: "none", change: 0 });
  });

  it("reports a rise, a fall and no change", () => {
    expect(threatTrend([point(10), point(25)])).toEqual({ direction: "up", change: 15 });
    expect(threatTrend([point(40), point(12.5)])).toEqual({ direction: "down", change: -27.5 });
    expect(threatTrend([point(40), point(40)])).toEqual({ direction: "flat", change: 0 });
  });

  it("compares only the last two points", () => {
    expect(threatTrend([point(90), point(10), point(30)])).toEqual({ direction: "up", change: 20 });
  });

  it("does not show floating-point noise", () => {
    expect(threatTrend([point(10.1), point(10.4)]).change).toBe(0.3);
  });
});
