import { describe, expect, it } from "vitest";
import { isRiskLevel, parseRisk, RISK_LEVELS, riskColor, riskFromScore, riskFromSeverity, riskLabel, riskRank } from "../risk";

describe("riskFromScore", () => {
  it("uses the backend bands: below 25 low, below 50 medium, below 75 high, otherwise critical", () => {
    const cases: [number, string][] = [
      [0, "low"], [24.99, "low"], [25, "medium"], [49.99, "medium"],
      [50, "high"], [74.99, "high"], [75, "critical"], [100, "critical"], [250, "critical"],
    ];
    for (const [score, level] of cases) expect(riskFromScore(score), `score ${score}`).toBe(level);
  });

  it("treats scores that are not numbers as low rather than crashing", () => {
    expect(riskFromScore(Number.NaN)).toBe("low");
    expect(riskFromScore(-5)).toBe("low");
  });
});

describe("riskFromSeverity", () => {
  it("maps 1-4 to low-critical and clamps the rest", () => {
    expect([1, 2, 3, 4].map(riskFromSeverity)).toEqual(["low", "medium", "high", "critical"]);
    expect(riskFromSeverity(0)).toBe("low");
    expect(riskFromSeverity(9)).toBe("critical");
    expect(riskFromSeverity(2.6)).toBe("high");
    expect(riskFromSeverity(Number.NaN)).toBe("medium");
  });
});

describe("levels", () => {
  it("has exactly the four levels the backend sends, lowest first", () => {
    expect(RISK_LEVELS).toEqual(["low", "medium", "high", "critical"]);
    expect(RISK_LEVELS.map(riskRank)).toEqual([0, 1, 2, 3]);
  });

  it("recognises and parses untrusted values", () => {
    expect(isRiskLevel("high")).toBe(true);
    expect(isRiskLevel("elevated")).toBe(false); // the old web label
    expect(isRiskLevel("HIGH")).toBe(false);
    expect(isRiskLevel(3)).toBe(false);
    expect(parseRisk("critical")).toBe("critical");
    expect(parseRisk("elevated")).toBe("medium");
    expect(parseRisk(undefined)).toBe("medium");
    expect(parseRisk(null, "low")).toBe("low");
  });

  it("gives every level a label and a colour token", () => {
    for (const level of RISK_LEVELS) {
      expect(riskLabel(level)).toMatch(/^[A-Z][a-z]+$/);
      expect(riskColor(level)).toMatch(/^var\(--[a-z-]+\)$/);
    }
    expect(new Set(RISK_LEVELS.map(riskColor)).size).toBe(4);
  });
});
