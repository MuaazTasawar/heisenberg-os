import { describe, expect, it } from "vitest";
import { clampEffort, normalizeName, parseAmount, validateConsult, validateFront, validateLaunder } from "../validation";

const RANGE = { min_effort: 1, max_effort: 10 };
const TYPES = ["restaurant", "car_wash"] as const;

describe("clampEffort", () => {
  it("keeps values inside the range", () => {
    expect(clampEffort(0, RANGE)).toBe(1);
    expect(clampEffort(-5, RANGE)).toBe(1);
    expect(clampEffort(11, RANGE)).toBe(10);
    expect(clampEffort(1, RANGE)).toBe(1);
    expect(clampEffort(10, RANGE)).toBe(10);
  });

  it("rounds to two decimals", () => {
    expect(clampEffort(8.333333, RANGE)).toBe(8.33);
    expect(clampEffort(8.336, RANGE)).toBe(8.34);
  });

  it("falls back to the minimum for values that are not numbers", () => {
    expect(clampEffort(Number.NaN, RANGE)).toBe(1);
    expect(clampEffort(Infinity, RANGE)).toBe(1);
  });
});

describe("normalizeName", () => {
  it("trims and collapses white space", () => {
    expect(normalizeName("  Los   Pollos \t Hermanos \n")).toBe("Los Pollos Hermanos");
    expect(normalizeName("   ")).toBe("");
  });
});

describe("validateFront", () => {
  it("accepts a good front", () => {
    expect(validateFront({ name: "Vamonos Pest", type: "restaurant" }, TYPES)).toEqual({});
  });

  it("asks for a name and a type", () => {
    expect(validateFront({ name: "   ", type: "" }, TYPES)).toEqual({ name: "Give the front a name.", type: "Pick a type of business." });
  });

  it("checks the length after white space is collapsed", () => {
    expect(validateFront({ name: "a", type: "car_wash" }, TYPES).name).toBe("The name must be 2-40 characters.");
    expect(validateFront({ name: "ab", type: "car_wash" }, TYPES).name).toBeUndefined();
    expect(validateFront({ name: "a".repeat(40), type: "car_wash" }, TYPES).name).toBeUndefined();
    expect(validateFront({ name: "a".repeat(41), type: "car_wash" }, TYPES).name).toBe("The name must be 2-40 characters.");
    expect(validateFront({ name: "a      b", type: "car_wash" }, TYPES).name).toBeUndefined();
  });

  it("counts characters, not UTF-16 units", () => {
    expect(validateFront({ name: "\u{1F600}".repeat(40), type: "car_wash" }, TYPES).name).toBeUndefined();
  });

  it("rejects angle brackets and control characters", () => {
    const message = "The name cannot contain < or > or control characters.";
    expect(validateFront({ name: "<b>Shop</b>", type: "car_wash" }, TYPES).name).toBe(message);
    expect(validateFront({ name: "Shop\u0007x", type: "car_wash" }, TYPES).name).toBe(message);
  });

  it("rejects a type that is not offered", () => {
    expect(validateFront({ name: "Shop", type: "lab" }, TYPES).type).toBe("Pick one of the listed types.");
  });
});

describe("parseAmount", () => {
  it.each([
    ["5000", 5000],
    ["$1,234.50", 1234.5],
    [" $ 1,000 ", 1000],
    ["0.5", 0.5],
    ["12.3", 12.3],
    ["1,000,000", 1000000],
  ])("reads %s", (text, expected) => {
    expect(parseAmount(text)).toBe(expected);
  });

  it.each(["", "abc", "-5", "1.234", "1e3", "1.", ".5", "12 34", "$$5", "NaN"])("rejects %j", (text) => {
    expect(parseAmount(text)).toBeNull();
  });
});

describe("validateLaunder", () => {
  const limits = { min: 100, dirty: 5000 };

  it("accepts an amount between the minimum and the dirty money", () => {
    expect(validateLaunder({ amount: "100", frontId: "3" }, limits, [3])).toEqual({});
    expect(validateLaunder({ amount: "5,000", frontId: "3" }, limits, [3])).toEqual({});
  });

  it("explains what is wrong with the amount", () => {
    expect(validateLaunder({ amount: "x", frontId: "3" }, limits, [3]).amount).toBe("Enter an amount in dollars, for example 5000.");
    expect(validateLaunder({ amount: "99.99", frontId: "3" }, limits, [3]).amount).toBe("The minimum transfer is $100.");
    expect(validateLaunder({ amount: "5000.01", frontId: "3" }, limits, [3]).amount).toBe("You only have $5,000 in dirty money.");
  });

  it("needs one of the open fronts", () => {
    expect(validateLaunder({ amount: "500", frontId: "" }, limits, [3]).front).toBe("Choose which front business to use.");
    expect(validateLaunder({ amount: "500", frontId: "9" }, limits, [3]).front).toBe("Choose one of your open front businesses.");
    expect(validateLaunder({ amount: "500", frontId: "3" }, limits, []).front).toBe("Choose one of your open front businesses.");
  });
});

describe("validateConsult", () => {
  it("needs a charge", () => {
    expect(validateConsult({ charge: "  ", evidence: "", context: "" })).toEqual({ charge: "Describe the charge." });
  });

  it("accepts the limits and rejects one more", () => {
    expect(validateConsult({ charge: "c".repeat(200), evidence: "e".repeat(2000), context: "x".repeat(2000) })).toEqual({});
    const errors = validateConsult({ charge: "c".repeat(201), evidence: "e".repeat(2001), context: "x".repeat(2001) });
    expect(errors.charge).toBe("The charge must be at most 200 characters.");
    expect(errors.evidence).toBe("The evidence must be at most 2000 characters.");
    expect(errors.context).toBe("Your situation must be at most 2000 characters.");
  });

  it("does not count the white space around the text", () => {
    expect(validateConsult({ charge: `  ${"c".repeat(200)}  `, evidence: "", context: "" })).toEqual({});
  });
});
