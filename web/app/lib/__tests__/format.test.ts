import { describe, expect, it } from "vitest";
import {
  clamp,
  formatCompactMoney,
  formatDate,
  formatDateTime,
  formatKg,
  formatMoney,
  formatNumber,
  formatPercent,
  formatRelative,
  formatScore,
  NOT_AVAILABLE,
  pluralize,
} from "../format";

describe("formatMoney", () => {
  it("shows whole dollars with separators", () => {
    expect(formatMoney(0)).toBe("$0");
    expect(formatMoney(1234)).toBe("$1,234");
    expect(formatMoney(15600.4)).toBe("$15,600");
    expect(formatMoney(15600.5)).toBe("$15,601");
    expect(formatMoney(1_234_567_890)).toBe("$1,234,567,890");
  });

  it("shows cents when asked", () => {
    expect(formatMoney(1234.5, { cents: true })).toBe("$1,234.50");
    expect(formatMoney(0, { cents: true })).toBe("$0.00");
  });

  it("puts the sign in front and never shows a negative zero", () => {
    expect(formatMoney(-1234)).toBe("-$1,234");
    expect(formatMoney(-0.4)).toBe("$0");
    expect(formatMoney(-0.001, { cents: true })).toBe("$0.00");
  });

  it("does not print NaN for missing values", () => {
    for (const bad of [undefined, null, Number.NaN, Infinity, -Infinity]) {
      expect(formatMoney(bad)).toBe(NOT_AVAILABLE);
    }
  });
});

describe("formatCompactMoney", () => {
  it("uses K, M and B with one decimal and no trailing zero", () => {
    expect(formatCompactMoney(999)).toBe("$999");
    expect(formatCompactMoney(1000)).toBe("$1K");
    expect(formatCompactMoney(15600)).toBe("$15.6K");
    expect(formatCompactMoney(1_500_000)).toBe("$1.5M");
    expect(formatCompactMoney(2_000_000_000)).toBe("$2B");
    expect(formatCompactMoney(-15600)).toBe("-$15.6K");
  });

  it("rolls 999.96K over to 1M instead of printing 1000K", () => {
    expect(formatCompactMoney(999_960)).toBe("$1M");
    expect(formatCompactMoney(999_940)).toBe("$999.9K");
  });

  it("handles missing values", () => {
    expect(formatCompactMoney(undefined)).toBe(NOT_AVAILABLE);
    expect(formatCompactMoney(Number.NaN)).toBe(NOT_AVAILABLE);
  });
});

describe("numbers", () => {
  it("formats kilograms without trailing zeros", () => {
    expect(formatKg(20)).toBe("20 kg");
    expect(formatKg(20.4)).toBe("20.4 kg");
    expect(formatKg(20.456)).toBe("20.46 kg");
    expect(formatKg(1234.5)).toBe("1,234.5 kg");
    expect(formatKg(null)).toBe(NOT_AVAILABLE);
  });

  it("formats percentages that are already 0-100", () => {
    expect(formatPercent(75)).toBe("75%");
    expect(formatPercent(61.456, 1)).toBe("61.5%");
    expect(formatPercent(5, 1)).toBe("5.0%");
    expect(formatPercent(Number.NaN)).toBe(NOT_AVAILABLE);
  });

  it("formats scores with one decimal", () => {
    expect(formatScore(61.51)).toBe("61.5");
    expect(formatScore(3)).toBe("3.0");
    expect(formatScore(undefined)).toBe(NOT_AVAILABLE);
  });

  it("formats plain numbers and counts", () => {
    expect(formatNumber(1234567)).toBe("1,234,567");
    expect(formatNumber(1.23456, 2)).toBe("1.23");
    expect(pluralize(1, "front")).toBe("1 front");
    expect(pluralize(2, "front")).toBe("2 fronts");
    expect(pluralize(0, "front")).toBe("0 fronts");
    expect(pluralize(1500, "case", "cases")).toBe("1,500 cases");
  });

  it("clamps", () => {
    expect(clamp(5, 1, 10)).toBe(5);
    expect(clamp(-5, 1, 10)).toBe(1);
    expect(clamp(50, 1, 10)).toBe(10);
  });
});

describe("dates", () => {
  const utc = { timeZone: "UTC" };

  it("formats dates and times in a given time zone", () => {
    expect(formatDateTime("2026-11-04T15:05:00Z", utc)).toBe("Nov 4, 2026, 3:05 PM");
    expect(formatDate("2026-11-04T15:05:00Z", utc)).toBe("Nov 4, 2026");
  });

  it("does not print Invalid Date", () => {
    for (const bad of [undefined, null, "", "not a date"]) {
      expect(formatDateTime(bad)).toBe(NOT_AVAILABLE);
      expect(formatDate(bad)).toBe(NOT_AVAILABLE);
      expect(formatRelative(bad)).toBe(NOT_AVAILABLE);
    }
  });

  describe("formatRelative", () => {
    const now = Date.parse("2026-11-04T12:00:00Z");
    const ago = (seconds: number) => new Date(now - seconds * 1000).toISOString();

    it("steps from seconds to days", () => {
      expect(formatRelative(ago(0), now)).toBe("just now");
      expect(formatRelative(ago(44), now)).toBe("just now");
      expect(formatRelative(ago(60), now)).toBe("1 min ago");
      expect(formatRelative(ago(5 * 60), now)).toBe("5 min ago");
      expect(formatRelative(ago(59 * 60), now)).toBe("59 min ago");
      expect(formatRelative(ago(60 * 60), now)).toBe("1 h ago");
      expect(formatRelative(ago(5 * 3600), now)).toBe("5 h ago");
      expect(formatRelative(ago(24 * 3600), now)).toBe("1 d ago");
      expect(formatRelative(ago(3 * 86400), now)).toBe("3 d ago");
    });

    it("falls back to the date after a week", () => {
      expect(formatRelative(ago(7 * 86400), now, utc)).toBe("Oct 28, 2026");
      expect(formatRelative(ago(90 * 86400), now, utc)).toBe("Aug 6, 2026");
    });

    it("treats a few seconds in the future as now and a far future as a date", () => {
      expect(formatRelative(new Date(now + 20_000).toISOString(), now)).toBe("just now");
      expect(formatRelative(new Date(now + 3_600_000).toISOString(), now, utc)).toBe("Nov 4, 2026, 1:00 PM");
    });
  });
});
